//go:build integration

package integration

// The S3 storage adapter against a real S3-compatible server, SeaweedFS,
// which is also staging's object store: MinIO no longer publishes images.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/natnael-alemayehu/logistics/internal/geo"
	"github.com/natnael-alemayehu/logistics/internal/service"
	"github.com/natnael-alemayehu/logistics/pkg/storage"
)

// seaweedfsImage is the version deployments/staging runs.
const seaweedfsImage = "chrislusf/seaweedfs:3.97"

// objectStore is a running S3 server with one bucket.
type objectStore struct {
	endpoint, bucket, access, secret string
}

// sharedObjectStore starts one S3 server for the tests that need one;
// TestMain stops it.
var sharedObjectStore = sync.OnceValues(startObjectStore)

// stopObjectStore stops the shared S3 server, if one was started.
var stopObjectStore = func() {}

func objectStoreFor(t *testing.T) objectStore {
	t.Helper()
	s, err := sharedObjectStore()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func startObjectStore() (objectStore, error) {
	ctx := context.Background()
	s := objectStore{bucket: "menged", access: "integration-access", secret: "integration-secret-0123456789"}
	identities := fmt.Sprintf(`{"identities": [{"name": "menged", "actions": ["Admin", "Read", "List", "Tagging", "Write"],
		"credentials": [{"accessKey": %q, "secretKey": %q}]}]}`, s.access, s.secret)
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        seaweedfsImage,
			Cmd:          []string{"server", "-dir=/data", "-s3", "-s3.port=8333", "-s3.config=/etc/seaweedfs/s3.json"},
			ExposedPorts: []string{"8333/tcp"},
			Files: []testcontainers.ContainerFile{{
				Reader: strings.NewReader(identities), ContainerFilePath: "/etc/seaweedfs/s3.json", FileMode: 0o644,
			}},
			// Any answer: an unsigned request is refused, once it's up.
			WaitingFor: wait.ForHTTP("/").WithPort("8333/tcp").WithStatusCodeMatcher(func(int) bool { return true }).
				WithStartupTimeout(2 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		return s, err
	}
	stopObjectStore = func() { _ = c.Terminate(ctx, testcontainers.StopTimeout(time.Second)) }
	// The bucket, as staging's set-up job makes it. The filer may still be
	// joining the master.
	var out string
	for range 30 {
		code, r, err := c.Exec(ctx, []string{"sh", "-c", "echo 's3.bucket.create -name " + s.bucket + "' | weed shell -master=localhost:9333"})
		if err == nil {
			b, _ := io.ReadAll(r)
			if out = string(b); code == 0 && strings.Contains(out, "created bucket") {
				break
			}
		}
		time.Sleep(time.Second)
	}
	if !strings.Contains(out, "created bucket") {
		return s, fmt.Errorf("creating the bucket: %s", out)
	}
	s.endpoint, err = c.PortEndpoint(ctx, "8333/tcp", "http")
	return s, err
}

func TestS3Storage(t *testing.T) {
	ctx := context.Background()
	s := objectStoreFor(t)
	store, err := storage.NewS3Storage(s.bucket, "us-east-1", s.endpoint, s.access, s.secret, false)
	if err != nil {
		t.Fatal(err)
	}
	body := bytes.Repeat([]byte("signature "), 10_000)
	key, err := store.Upload(ctx, "/tenant/pod/signature.png", bytes.NewReader(body), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if key != "tenant/pod/signature.png" {
		t.Errorf("stored as %q", key)
	}
	r, err := store.Download(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	r.Close()
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("downloaded %d bytes (%v), want the %d uploaded", len(got), err, len(body))
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Download(ctx, key); !errors.Is(err, storage.ErrFileNotFound) {
		t.Errorf("after deleting: %v, want not found", err)
	}

	wrong, err := storage.NewS3Storage(s.bucket, "us-east-1", s.endpoint, s.access, "not-the-secret", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wrong.Upload(ctx, "tenant/x.png", strings.NewReader("x"), "image/png"); err == nil {
		t.Error("an upload signed with the wrong secret was accepted")
	}
}

// The app keeps a delivery's signature and photos in the bucket and serves
// them to the office through itself, so the bucket needn't be public.
func TestDeliveriesStoredInS3(t *testing.T) {
	s := objectStoreFor(t)
	e := newEnv(t, func(c *setup) {
		c.cfg.StorageType, c.cfg.S3Endpoint, c.cfg.S3Bucket, c.cfg.S3Region = "s3", s.endpoint, s.bucket, "us-east-1"
		c.cfg.S3AccessKey, c.cfg.S3SecretKey = s.access, s.secret
	})
	office := e.company("Object Store Freight")
	driverID, phone, pin := office.addDriver("Kebede Alemu")
	shipmentID, _ := office.book("Addis Ababa", "Adama", driverID)
	d := e.signIn(phone, pin)
	for _, st := range []string{"in_transit", "arrived"} {
		if r := d.status(clientID(), shipmentID, st); !r.OK {
			t.Fatalf("%s rejected: %+v", st, r)
		}
	}
	dest, _ := geo.Lookup("Adama")
	photo := photoJPEG(t)
	if r := d.pod(service.SyncPOD{ShipmentID: shipmentID, RecipientName: "Tigist Haile", Lat: &dest.Lat, Lng: &dest.Lng,
		Photos: []string{photo}}); !r.OK {
		t.Fatalf("proof of delivery: %+v", r)
	}
	if files := storedFiles(t, e.files); len(files) != 0 {
		t.Errorf("files on the app's disk: %v", files)
	}
	if p := office.get("/app/shipments/" + shipmentID + "/media/0"); p.Status != http.StatusOK || !strings.HasPrefix(p.Body, "\x89PNG") {
		t.Errorf("signature from the bucket: status %d", p.Status)
	}
	if p := office.get("/app/shipments/" + shipmentID + "/media/1"); p.Status != http.StatusOK || !strings.HasPrefix(p.Body, "\xff\xd8") {
		t.Errorf("photo from the bucket: status %d", p.Status)
	}
}
