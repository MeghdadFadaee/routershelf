package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticPagesAndAssets(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "site"), 0700)
	files := map[string]string{"site/index.html": "<h1>Home</h1>", "page.HTML": "<h1>Direct</h1>", "site/app.js": "console.log('ok')", "site/style.css": "h1{color:red}", "archive.zip": "archive", "script.php": "<?php echo 1;"}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	h := handler(root)
	for _, tc := range []struct {
		url, ctype, body string
		code             int
		attachment       bool
	}{
		{"/site/", "text/html; charset=utf-8", "<h1>Home</h1>", 200, false},
		{"/site/index.html", "text/html; charset=utf-8", "<h1>Home</h1>", 200, false},
		{"/page.HTML", "text/html; charset=utf-8", "<h1>Direct</h1>", 200, false},
		{"/site/app.js", "text/javascript; charset=utf-8", "console.log('ok')", 200, false},
		{"/site/style.css", "text/css; charset=utf-8", "h1{color:red}", 200, false},
		{"/archive.zip", "application/octet-stream", "archive", 200, true},
		{"/script.php", "application/octet-stream", "<?php echo 1;", 200, true},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", tc.url, nil))
		if w.Code != tc.code || w.Header().Get("Content-Type") != tc.ctype || w.Body.String() != tc.body {
			t.Fatalf("%s: %d %s %q", tc.url, w.Code, w.Header().Get("Content-Type"), w.Body.String())
		}
		if strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment") != tc.attachment {
			t.Fatalf("wrong disposition: %s", tc.url)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/site?x=1", nil))
	if w.Code != 301 || w.Header().Get("Location") != "/site/" {
		t.Fatalf("bad directory redirect: %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("HEAD", "/site/", nil))
	if w.Code != 200 || w.Body.Len() != 0 {
		t.Fatal("index HEAD failed")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	body := w.Body.String()
	if strings.Contains(body, `href="/page.HTML" data-file="true" download`) || !strings.Contains(body, `href="/archive.zip" data-file="true" download`) {
		t.Fatal("listing link download policy")
	}
}
func TestEscapingIndexSymlinkAndRootIndex(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.html")
	os.WriteFile(outside, []byte("private page"), 0600)
	os.Symlink(outside, filepath.Join(root, "index.html"))
	h := handler(root)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), "private page") {
		t.Fatal("outside index escaped root")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/index.html", nil))
	if w.Code != 404 {
		t.Fatal("direct escaping symlink allowed")
	}
	os.Remove(filepath.Join(root, "index.html"))
	os.WriteFile(filepath.Join(root, "index.html"), []byte("<h1>Root index</h1>"), 0600)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Body.String() != "<h1>Root index</h1>" {
		t.Fatal("root index not served")
	}
}
