package webui

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDevelopmentAssetsReadCurrentFilesWithoutEscapingRoot(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "index.html")
	if err := os.WriteFile(filename, []byte("first build"), 0600); err != nil {
		t.Fatal(err)
	}
	handler, closeAssets, err := DirectoryHandler(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer closeAssets()
	read := func(url string) (int, string) {
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, httptest.NewRequest("GET", url, nil))
		return out.Code, out.Body.String()
	}
	if status, body := read("/"); status != 200 || body != "first build" {
		t.Fatalf("initial assets: %d %s", status, body)
	}
	if err := os.WriteFile(filename, []byte("rebuilt assets"), 0600); err != nil {
		t.Fatal(err)
	}
	if status, body := read("/"); status != 200 || body != "rebuilt assets" {
		t.Fatal("disk assets did not refresh")
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("PRIVATE CONTENT"), 0600); err != nil {
		t.Fatal(err)
	}
	if status, _ := read("/.env"); status != 404 {
		t.Fatal("development asset handler exposed a dotfile")
	}
	if err := os.Mkdir(filepath.Join(dir, "unlisted"), 0700); err != nil {
		t.Fatal(err)
	}
	if status, _ := read("/unlisted/"); status != 404 {
		t.Fatal("development asset handler exposed a directory listing")
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "private.txt"), []byte("PRIVATE CONTENT"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "escape")); err == nil {
		if status, body := read("/escape/private.txt"); status == 200 || body == "PRIVATE CONTENT" {
			t.Fatal("asset root allowed a symlink escape")
		}
	}
	if _, _, err := DirectoryHandler(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("accepted missing directory")
	}
}
