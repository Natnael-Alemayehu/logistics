//go:build e2e

package e2e

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestBackupAndRestore: scripts/backup.sh and scripts/restore.sh, run on
// staging as on a production machine. A company that signed up before the
// backup is back after the restore, still signed in, and one that signed up
// after it is gone.
//
// It replaces staging's data, so it is in the last file: Go runs a
// package's tests in the order of their files.
func TestBackupAndRestore(t *testing.T) {
	before := signUp(t, "Before The Backup Freight")

	out := script(t, "backup.sh", "BACKUP_DIR="+t.TempDir())
	dump, files := backupRe.FindStringSubmatch(out), filesRe.FindStringSubmatch(out)
	if dump == nil || files == nil {
		t.Fatalf("backup.sh named no backup:\n%s", out)
	}
	for _, f := range []string{dump[1], files[1]} {
		info, err := os.Stat(f)
		if err != nil || info.Size() == 0 || info.Mode().Perm() != 0o600 {
			t.Fatalf("backup %s: %v, want a file only its owner can read", f, err)
		}
	}
	if names := archived(t, files[1]); !names["./keys/app_secret"] {
		t.Errorf("the files backup holds %v, without the app secret", names)
	}

	after := signUp(t, "After The Backup Freight")
	script(t, "restore.sh", "CONFIRM=yes", dump[1], files[1])
	within(t, 2*time.Minute, "the app to be ready again", func() bool {
		res, err := newClient(nil).Get(appURL + "/readyz")
		if err != nil {
			return false
		}
		res.Body.Close()
		return res.StatusCode == http.StatusOK
	})

	// The session from before the backup still holds: the same database, and
	// the same secret.
	if p := before.get("/app"); p.Status != http.StatusOK || !strings.Contains(p.Body, before.company) {
		t.Errorf("the company from before the backup: status %d", p.Status)
	}
	if p := newBrowser(t).post("/login", url.Values{"identifier": {after.email}, "password": {password}}); p.Status != http.StatusUnauthorized {
		t.Errorf("the company from after the backup signing in: status %d, want it unknown", p.Status)
	}
}

var (
	backupRe = regexp.MustCompile(`(?m)^Database: +(\S+)$`)
	filesRe  = regexp.MustCompile(`(?m)^Files: +(\S+)$`)
)

// script runs one of scripts/ on staging's stack, with an extra variable
// in its environment, and returns what it printed.
func script(t *testing.T, name, env string, args ...string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "scripts", name))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(path, args...)
	cmd.Env = append(os.Environ(),
		"COMPOSE_FILE="+setting("E2E_COMPOSE_FILE", "deployments/docker-compose.prod.yml:deployments/staging/compose.yml"),
		"ENV_FILE="+setting("E2E_ENV_FILE", "deployments/staging/staging.env"),
		env,
	)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", name, err, b)
	}
	t.Logf("%s:\n%s", name, b)
	return string(b)
}

// archived lists the names in a .tar.gz.
func archived(t *testing.T, file string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	r := tar.NewReader(z)
	for {
		h, err := r.Next()
		if err == io.EOF {
			return names
		}
		if err != nil {
			t.Fatal(err)
		}
		names[h.Name] = true
	}
}
