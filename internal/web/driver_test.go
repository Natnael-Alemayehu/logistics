package web

import (
	"encoding/json"
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// Every file the driver app's service worker caches for offline use, and
// every icon its manifest names, is there: one missing file makes the
// whole install fail, and the app no longer opens without signal.
func TestDriverShellFilesExist(t *testing.T) {
	sw, err := fs.ReadFile(embedded, "static/driver/sw.js")
	if err != nil {
		t.Fatal(err)
	}
	shell := regexp.MustCompile(`(?s)const SHELL = \[(.*?)\];`).FindSubmatch(sw)
	if shell == nil {
		t.Fatal("no SHELL list in sw.js")
	}
	var paths []string
	for _, m := range regexp.MustCompile(`"(/driver/[^"]*)"`).FindAllSubmatch(shell[1], -1) {
		paths = append(paths, string(m[1]))
	}

	raw, err := fs.ReadFile(embedded, "static/driver/manifest.webmanifest")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Icons []struct {
			Src string `json:"src"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	for _, icon := range manifest.Icons {
		paths = append(paths, icon.Src)
	}

	for _, p := range paths {
		name := strings.TrimPrefix(p, "/driver/")
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(embedded, "static/driver/"+name); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
}
