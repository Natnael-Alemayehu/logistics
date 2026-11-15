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

// TestBackupAndRestore is the restore drill: scripts/backup.sh,
// fetch-backup.sh and restore.sh, run on staging as on a production machine
// with staging's backup.env. The backup is encrypted with a key the test
// makes and copied to a bucket (a second one in SeaweedFS). Then the copies
// on the machine are lost, as with the machine, and the backup comes back
// from the bucket. A company that signed up before the backup is back after
// the restore, still signed in, and one that signed up after it is gone.
//
// It replaces staging's data, so it is in the last file: Go runs a
// package's tests in the order of their files.
func TestBackupAndRestore(t *testing.T) {
	for _, tool := range []string{"age", "age-keygen"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("%s: backups are encrypted with age; install it (apt install age, dnf install age or brew install age)", err)
		}
	}
	key, recipient := ageKey(t)
	env := []string{"BACKUP_ENV_FILE=" + setting("E2E_BACKUP_ENV_FILE", "deployments/staging/backup.env")}

	if out, err := run(t, "backup.sh", append(env, "BACKUP_DIR="+t.TempDir())); err == nil || !strings.Contains(out, "only encrypted") {
		t.Fatalf("backup.sh with a bucket and no key: %v, want it to refuse", err)
	}

	before := signUp(t, "Before The Backup Freight")
	env = append(env, "BACKUP_AGE_RECIPIENTS="+recipient)
	out := script(t, "backup.sh", append(env, "BACKUP_DIR="+t.TempDir()))
	made := backupFiles(t, out)
	if !strings.Contains(out, "\nOff-site: kiflet-backups\n") {
		t.Error("backup.sh copied nothing off the machine")
	}
	for _, f := range made {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if info, _ := os.Stat(f); info.Mode().Perm() != 0o600 {
			t.Errorf("backup %s: mode %v, want a file only its owner can read", f, info.Mode().Perm())
		}
		if !strings.HasSuffix(f, ".age") || !bytes.HasPrefix(raw, []byte("age-encryption.org/v1\n")) {
			t.Errorf("backup %s isn't encrypted with age", f)
		}
	}
	if names := archived(t, decrypt(t, key, made["Files"])); !names["./keys/app_secret"] {
		t.Errorf("the files backup holds %v, without the app secret", names)
	}
	envFile := setting("E2E_ENV_FILE", "deployments/staging/staging.env")
	if names := archived(t, decrypt(t, key, made["Settings"])); !names[envFile] || !names["deployments/staging/backup.env"] {
		t.Errorf("the settings backup holds %v, without %s and backup.env", names, envFile)
	}

	// The machine is lost: the backup comes from the bucket, the same bytes.
	after := signUp(t, "After The Backup Freight")
	fetched := backupFiles(t, script(t, "fetch-backup.sh", append(env, "BACKUP_DIR="+t.TempDir())))
	for what, f := range fetched {
		got, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if want, _ := os.ReadFile(made[what]); filepath.Base(f) != filepath.Base(made[what]) || !bytes.Equal(got, want) {
			t.Fatalf("fetch-backup.sh brought %s back as %s, not as it was made", made[what], f)
		}
	}

	// With another key nothing changes, the app included.
	other, _ := ageKey(t)
	if out, err := run(t, "restore.sh", []string{"CONFIRM=yes", "AGE_IDENTITY=" + other}, fetched["Database"], fetched["Files"]); err == nil || !strings.Contains(out, "Nothing changed") {
		t.Fatalf("restore.sh with the wrong key: %v, want it to refuse", err)
	}
	if p := after.get("/app"); p.Status != http.StatusOK {
		t.Fatalf("after restore.sh refused, the app answers %d", p.Status)
	}

	script(t, "restore.sh", []string{"CONFIRM=yes", "AGE_IDENTITY=" + key}, fetched["Database"], fetched["Files"])
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

var backupFileRe = regexp.MustCompile(`(?m)^(Database|Files|Settings): +(\S+)$`)

// backupFiles reads the files a script named: Database, Files and Settings.
func backupFiles(t *testing.T, out string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, m := range backupFileRe.FindAllStringSubmatch(out, -1) {
		files[m[1]] = m[2]
	}
	if len(files) != 3 {
		t.Fatalf("the script named %v, want the database, files and settings", files)
	}
	return files
}

// ageKey makes an age key pair: the private key's file, and the public key.
func ageKey(t *testing.T) (file, recipient string) {
	t.Helper()
	file = filepath.Join(t.TempDir(), "backup.key")
	if b, err := exec.Command("age-keygen", "-o", file).CombinedOutput(); err != nil {
		t.Fatalf("age-keygen: %v\n%s", err, b)
	}
	b, err := exec.Command("age-keygen", "-y", file).Output()
	if err != nil {
		t.Fatalf("age-keygen -y: %v", err)
	}
	return file, strings.TrimSpace(string(b))
}

// decrypt decrypts a backup with the key in a file, into a file of its own.
func decrypt(t *testing.T, key, file string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), strings.TrimSuffix(filepath.Base(file), ".age"))
	if b, err := exec.Command("age", "-d", "-i", key, "-o", out, file).CombinedOutput(); err != nil {
		t.Fatalf("decrypting %s: %v\n%s", file, err, b)
	}
	return out
}

// script runs one of scripts/ on staging's stack, with extra variables in
// its environment, and returns what it printed.
func script(t *testing.T, name string, env []string, args ...string) string {
	t.Helper()
	out, err := run(t, name, env, args...)
	if err != nil {
		t.Fatalf("%s: %v\n%s", name, err, out)
	}
	return out
}

// run is script for a script that may fail.
func run(t *testing.T, name string, env []string, args ...string) (string, error) {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "scripts", name))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(path, args...)
	cmd.Env = append(os.Environ(),
		"COMPOSE_FILE="+setting("E2E_COMPOSE_FILE", "deployments/docker-compose.prod.yml:deployments/staging/compose.yml"),
		"ENV_FILE="+setting("E2E_ENV_FILE", "deployments/staging/staging.env"),
	)
	cmd.Env = append(cmd.Env, env...)
	b, err := cmd.CombinedOutput()
	t.Logf("%s:\n%s", name, b)
	return string(b), err
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
