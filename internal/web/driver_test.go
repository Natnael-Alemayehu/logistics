package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"testing/fstest"
)

// The driver app's version is served as build.json: every file of the app
// with its SHA-256. A file missing from it wouldn't be there without signal;
// a wrong sum would make every update fail.
func TestDriverBuild(t *testing.T) {
	assets, err := fs.Sub(embedded, "static")
	if err != nil {
		t.Fatal(err)
	}
	build, err := driverBuild(assets)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{assets: assets, driverBuild: build}
	get := func(path string) *httptest.ResponseRecorder {
		res := httptest.NewRecorder()
		s.driverApp(res, httptest.NewRequest(http.MethodGet, path, nil))
		if res.Code != http.StatusOK || res.Header().Get("Cache-Control") != "no-cache" {
			t.Fatalf("%s: status %d, headers %v", path, res.Code, res.Header())
		}
		return res
	}
	if res := get("/driver/sw.js"); res.Header().Get("Service-Worker-Allowed") != "/driver/" {
		t.Errorf("sw.js: headers %v", res.Header())
	}
	res := get("/driver/build.json")
	if res.Header().Get("Content-Type") != "application/json" {
		t.Errorf("build.json: headers %v", res.Header())
	}
	var got struct {
		Version string            `json:"version"`
		Files   map[string]string `json:"files"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatalf("build.json: %v", err)
	}
	if len(got.Version) != 12 {
		t.Errorf("version %q", got.Version)
	}

	// Every file under static/driver but sw.js, at the path the app asks for.
	want := map[string]string{}
	err = fs.WalkDir(assets, "driver", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || p == "driver/sw.js" {
			return err
		}
		b, err := fs.ReadFile(assets, p)
		sum := sha256.Sum256(b)
		at := "/" + p
		if p == "driver/index.html" {
			at = "/driver/"
		}
		want[at] = hex.EncodeToString(sum[:])
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for at, sum := range want {
		if got.Files[at] != sum {
			t.Errorf("BUILD has %s as %q, want %q", at, got.Files[at], sum)
		}
	}
	if len(got.Files) != len(want) {
		t.Errorf("BUILD lists %d files, want %d: %v", len(got.Files), len(want), got.Files)
	}

	// And so every file the page and the manifest name is installed.
	page, _ := fs.ReadFile(assets, "driver/index.html")
	var named []string
	for _, m := range regexp.MustCompile(`(?:href|src)="(/driver/[^"]+)"`).FindAllSubmatch(page, -1) {
		named = append(named, string(m[1]))
	}
	raw, _ := fs.ReadFile(assets, "driver/manifest.webmanifest")
	var manifest struct {
		Icons []struct {
			Src string `json:"src"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	for _, icon := range manifest.Icons {
		named = append(named, icon.Src)
	}
	if len(named) < 5 {
		t.Fatalf("found only %v named: the search has stopped working", named)
	}
	for _, p := range named {
		if _, ok := got.Files[p]; !ok {
			t.Errorf("%s is named by the app and not installed", p)
		}
	}
}

// The version is the same for the same files, and changes with any of them.
func TestDriverBuildVersion(t *testing.T) {
	version := func(files fstest.MapFS) string {
		t.Helper()
		b, err := driverBuild(files)
		if err != nil {
			t.Fatal(err)
		}
		var v struct{ Version string }
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		return v.Version
	}
	app := func(js, sw string) fstest.MapFS {
		return fstest.MapFS{
			"driver/index.html": {Data: []byte("<html>")},
			"driver/app.js":     {Data: []byte(js)},
			"driver/sw.js":      {Data: []byte(sw)},
		}
	}
	v := version(app("one", "worker"))
	if version(app("one", "worker")) != v {
		t.Error("the same files gave two versions")
	}
	if version(app("two", "worker")) == v {
		t.Error("a changed app.js kept the version")
	}
	// The worker's own code isn't part of a version: the browser updates it.
	if version(app("one", "worker, changed")) != v {
		t.Error("a changed sw.js changed the version of the files")
	}
}
