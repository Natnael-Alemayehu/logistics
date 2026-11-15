package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// driverApp serves the driver PWA, and build.json, the version its service
// worker installs. Everything is revalidated on each load: the service
// worker keeps the app usable without signal, and installs a new version
// whole (static/driver/sw.js).
func (s *Server) driverApp(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/driver")
	name = strings.TrimPrefix(name, "/")
	if name == "" {
		name = "index.html"
	}
	w.Header().Set("Cache-Control", "no-cache")
	if name == "build.json" {
		s.driverVersion(w, r)
		return
	}
	f := "driver/" + name
	if _, err := fs.Stat(s.assets, f); err != nil {
		http.NotFound(w, r)
		return
	}
	if name == "sw.js" {
		w.Header().Set("Service-Worker-Allowed", "/driver/")
	}
	http.ServeFileFS(w, r, s.assets, f)
}

// driverVersion serves the version of the driver app (driverBuild).
func (s *Server) driverVersion(w http.ResponseWriter, r *http.Request) {
	build := s.driverBuild
	if build == nil {
		// DEV_RELOAD: the files may have changed since the last request.
		var err error
		if build, err = driverBuild(s.assets); err != nil {
			s.log.Error().Err(err).Msg("driver app version")
			http.Error(w, "the driver app's files are unreadable", http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	http.ServeContent(w, r, "build.json", time.Time{}, bytes.NewReader(build))
}

// driverBuild describes the version of the driver app in fsys: every file
// under driver/ but the service worker itself, by the path the app requests
// it at ("/driver/" for index.html), with its SHA-256, and a version that
// changes when any of them does. The service worker keeps a version only
// once every file it downloaded matches. Its own code isn't part of a
// version: the browser updates it.
func driverBuild(fsys fs.FS) ([]byte, error) {
	files := map[string]string{}
	all := sha256.New()
	err := fs.WalkDir(fsys, "driver", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || p == "driver/sw.js" {
			return err
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		at := "/" + p
		if p == "driver/index.html" {
			at = "/driver/"
		}
		sum := sha256.Sum256(b)
		files[at] = hex.EncodeToString(sum[:])
		// WalkDir goes in lexical order, so the version is the same for the
		// same files.
		all.Write([]byte(at + "\x00"))
		all.Write(sum[:])
		return nil
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Version string            `json:"version"`
		Files   map[string]string `json:"files"`
	}{hex.EncodeToString(all.Sum(nil))[:12], files})
}
