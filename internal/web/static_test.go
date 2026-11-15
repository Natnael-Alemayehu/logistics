package web

import (
	"io/fs"
	"path"
	"regexp"
	"strings"
	"testing"
)

// Every static file a page, a script or a stylesheet names is embedded in
// the binary. The map library once wasn't: an ignore rule kept it out of
// git, so every build from a clone served a 404 for it and no test asked.
func TestStaticFilesExist(t *testing.T) {
	var (
		// {{asset "app.css"}} in a template.
		assetRe = regexp.MustCompile(`\{\{\s*asset\s+"([^"]+)"\s*\}\}`)
		// "/static/vendor/leaflet/leaflet.js" in a script or a page.
		staticRe = regexp.MustCompile(`/static/([A-Za-z0-9_/-]+(?:\.[A-Za-z0-9_/-]+)+)`)
		// url(images/layers.png) in a stylesheet; not data: or #fragments.
		urlRe = regexp.MustCompile(`url\(["']?([^)"'#:]+)["']?\)`)
	)

	named := map[string]string{} // embedded file -> the file that names it
	note := func(file, by string) {
		if _, seen := named[file]; !seen {
			named[file] = by
		}
	}
	err := fs.WalkDir(embedded, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		switch path.Ext(p) {
		case ".html", ".js", ".css", ".webmanifest":
		default:
			return nil
		}
		b, err := fs.ReadFile(embedded, p)
		if err != nil {
			return err
		}
		for _, m := range assetRe.FindAllSubmatch(b, -1) {
			note("static/"+string(m[1]), p)
		}
		for _, m := range staticRe.FindAllSubmatch(b, -1) {
			note("static/"+string(m[1]), p)
		}
		if path.Ext(p) != ".css" {
			return nil
		}
		for _, m := range urlRe.FindAllSubmatch(b, -1) {
			switch u := string(m[1]); {
			case strings.HasPrefix(u, "/static/"), strings.HasPrefix(u, "/driver/"):
				// The driver app is served from static/driver.
				note("static/"+strings.TrimPrefix(strings.TrimPrefix(u, "/static/"), "/"), p)
			case !strings.HasPrefix(u, "/"):
				note(path.Join(path.Dir(p), u), p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Enough were found that the patterns still match how files are named.
	for _, file := range []string{
		"static/app.css",
		"static/vendor/leaflet/leaflet.js",
		"static/vendor/leaflet/images/marker-icon.png",
	} {
		if _, ok := named[file]; !ok {
			t.Errorf("nothing names %s: the search for names has stopped working", file)
		}
	}
	for file, by := range named {
		if _, err := fs.Stat(embedded, file); err != nil {
			t.Errorf("%s names %s, which is not embedded", by, file)
		}
	}
}
