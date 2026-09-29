package models

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAHuggingFaceLinkBecomesADownload(t *testing.T) {
	want := "https://huggingface.co/owner/repo/resolve/main/model.gguf"
	for _, link := range []string{
		"owner/repo/model.gguf",
		" https://huggingface.co/owner/repo/blob/main/model.gguf ",
		want,
	} {
		m, err := Custom(link)
		if err != nil {
			t.Fatalf("%q: %v", link, err)
		}
		if m.URL != want || m.Key != "model.gguf" {
			t.Fatalf("%q became %+v", link, m)
		}
	}
	for _, bad := range []string{"", "owner/model.gguf", "https://example.com/model.bin"} {
		if _, err := Custom(bad); err == nil {
			t.Fatalf("%q was accepted as a model", bad)
		}
	}
}

// The Copilot CLI ships as a release archive; what the app runs is the one
// executable inside, and the checksum still covers the whole download.
func TestAnExecutableComesOutOfItsReleaseArchive(t *testing.T) {
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	for name, body := range map[string]string{"README.md": "read me", "copilot": "#!/bin/sh\necho copilot\n"} {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	sum := sha256.Sum256(archive.Bytes())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(archive.Bytes())
	}))
	defer server.Close()

	dir := filepath.Join(t.TempDir(), "models")
	m := Model{Name: "GitHub Copilot", Key: "copilot", URL: server.URL + "/copilot-darwin-arm64.tar.gz",
		Bytes: int64(archive.Len()), Check: hex.EncodeToString(sum[:]), Run: true}
	report := make(chan Progress, 64)
	go Fetch(context.Background(), dir, Set{m}, report)
	for p := range report {
		if p.Err != nil {
			t.Fatalf("download failed: %v", p.Err)
		}
	}
	if !Have(dir, m) {
		t.Fatal("the finished download is not marked as present")
	}
	info, err := os.Stat(Path(dir, m))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o100 == 0 {
		t.Fatal("the unpacked tool cannot be run")
	}
	if got, _ := os.ReadFile(Path(dir, m)); string(got) != "#!/bin/sh\necho copilot\n" {
		t.Fatalf("unpacked the wrong entry: %q", got)
	}
}
