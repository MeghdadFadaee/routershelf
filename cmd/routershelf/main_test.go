package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileScopeAndMethods(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "test.txt"), []byte("test"), 0600)
	os.WriteFile(filepath.Join(root, ".secret"), []byte("secret"), 0600)
	outside := filepath.Join(t.TempDir(), "outside")
	os.WriteFile(outside, []byte("private"), 0600)
	os.Symlink(outside, filepath.Join(root, "escape.txt"))
	os.Mkdir(filepath.Join(root, "sub"), 0700)
	os.WriteFile(filepath.Join(root, "<&.txt"), []byte("safe"), 0600)
	h := handler(root)
	for _, tc := range []struct {
		method, url string
		code        int
		body        string
	}{{"GET", "/", 200, "test.txt"}, {"GET", "/test.txt", 200, "test"}, {"HEAD", "/test.txt", 200, ""}, {"GET", "/.secret", 404, ""}, {"GET", "/../outside", 404, ""}, {"GET", "/%2e%2e/outside", 404, ""}, {"GET", "/escape.txt", 404, ""}, {"GET", "/sub/", 200, "This folder is empty"}, {"PUT", "/new.txt", 405, ""}, {"POST", "/", 405, ""}, {"DELETE", "/test.txt", 405, ""}} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(tc.method, tc.url, nil)
		h.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("%s %s: %d", tc.method, tc.url, w.Code)
		}
		if tc.body != "" && !strings.Contains(w.Body.String(), tc.body) {
			t.Fatalf("unexpected response %s", tc.url)
		}
		if tc.method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("HEAD body")
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if strings.Contains(w.Body.String(), ".secret") || strings.Contains(w.Body.String(), "escape.txt") {
		t.Fatal("private entry listed")
	}
	if !strings.Contains(w.Body.String(), "&lt;&amp;.txt") {
		t.Fatal("filename not escaped")
	}
	r := httptest.NewRequest("GET", "/test.txt", nil)
	r.Header.Set("Range", "bytes=1-2")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 206 || w.Body.String() != "es" {
		t.Fatal("range download failed")
	}
}

func TestHTTPSRedirect(t *testing.T) {
	h := redirectHandler("files.example.com")
	for _, tc := range []struct {
		target, method string
		status         int
		location       string
	}{
		{"http://files.example.com/folder/a%20b.txt?q=1", "GET", 308, "https://files.example.com/folder/a%20b.txt?q=1"},
		{"http://other.example/", "GET", 421, ""},
		{"http://files.example.com/", "POST", 405, ""},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.target, nil))
		if w.Code != tc.status || w.Header().Get("Location") != tc.location {
			t.Errorf("%s: status=%d location=%q", tc.target, w.Code, w.Header().Get("Location"))
		}
	}
}
