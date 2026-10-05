package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNewerComparesNumbersNotText(t *testing.T) {
	for _, c := range []struct {
		current, latest string
		want            bool
	}{
		{"1.3.0", "1.3.1", true}, {"1.3.0", "1.10.0", true}, {"1.3.0", "v2.0.0", true},
		{"1.3.0", "1.3.0", false}, {"1.3.0", "1.2.9", false}, {"1.3.0", "nightly", false}, {"dev", "1.0.0", false},
	} {
		if got := Newer(c.current, c.latest); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.current, c.latest, got)
		}
	}
}

// app makes a minimal signed bundle that says it is this version.
func app(t *testing.T, where, version string) string {
	t.Helper()
	path := filepath.Join(where, "Meeting Transcriber.app")
	if err := os.MkdirAll(filepath.Join(path, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>run</string><key>CFBundleIdentifier</key><string>test.update</string>
<key>CFBundleShortVersionString</key><string>%s</string></dict></plist>`, version)
	os.WriteFile(filepath.Join(path, "Contents", "Info.plist"), []byte(plist), 0o644)
	os.WriteFile(filepath.Join(path, "Contents", "MacOS", "run"), []byte("#!/bin/sh\n"), 0o755)
	if out, err := exec.Command("codesign", "--force", "--sign", "-", path).CombinedOutput(); err != nil {
		t.Fatalf("codesign: %v %s", err, out)
	}
	return path
}

// release serves a release whose disk image holds an app of the given version.
func release(t *testing.T, version string) (Release, *httptest.Server) {
	t.Helper()
	build := t.TempDir()
	app(t, filepath.Join(build, "dmg"), version)
	image := filepath.Join(build, "release.dmg")
	if out, err := exec.Command("hdiutil", "create", "-quiet", "-volname", "Meeting Transcriber",
		"-srcfolder", filepath.Join(build, "dmg"), "-format", "UDZO", image).CombinedOutput(); err != nil {
		t.Fatalf("hdiutil: %v %s", err, out)
	}
	data, _ := os.ReadFile(image)
	sum := sha256.Sum256(data)
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name":"v%s","body":"## New\n- a thing","html_url":"https://example.test/r",
			"assets":[{"name":"MeetingTranscriber-%s.dmg","browser_download_url":%q,"size":%d,"digest":"sha256:%s"}]}`,
			version, version, server.URL+"/image.dmg", len(data), hex.EncodeToString(sum[:]))
	})
	mux.HandleFunc("/image.dmg", func(w http.ResponseWriter, r *http.Request) { w.Write(data) })
	r, err := Latest(context.Background(), server.URL+"/latest")
	if err != nil {
		t.Fatal(err)
	}
	return r, server
}

func TestStageUnpacksAVerifiedApp(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("disk images are a macOS thing")
	}
	r, _ := release(t, "9.9.9")
	if r.Version != "9.9.9" || r.Notes != "## New\n- a thing" || r.Page != "https://example.test/r" {
		t.Fatalf("release = %+v", r)
	}
	installed := app(t, t.TempDir(), "1.0.0")
	var last float64
	staged, err := Stage(context.Background(), r, installed, func(f float64) { last = f })
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(staged) != filepath.Dir(installed) || last != 1 {
		t.Fatalf("staged at %s, progress ended at %v", staged, last)
	}
	if _, err := os.Stat(filepath.Join(staged, "Contents", "MacOS", "run")); err != nil {
		t.Fatal("the staged app is not whole:", err)
	}
}

func TestStageRejectsADamagedDownload(t *testing.T) {
	r, _ := release(t, "9.9.9")
	r.digest = "00"
	installed := app(t, t.TempDir(), "1.0.0")
	if _, err := Stage(context.Background(), r, installed, func(float64) {}); err == nil {
		t.Fatal("a download with the wrong digest was accepted")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(installed), ".Meeting Transcriber.app.update")); err == nil {
		t.Fatal("a rejected download left an app behind")
	}
}

func TestSwapReplacesTheAppAndOpensIt(t *testing.T) {
	dir := t.TempDir()
	installed, staged := app(t, filepath.Join(dir, "old"), "1.0.0"), app(t, filepath.Join(dir, "new"), "2.0.0")
	// A stand-in for open(1) that says what it was asked to open.
	bin := filepath.Join(dir, "bin")
	os.MkdirAll(bin, 0o755)
	opened := filepath.Join(dir, "opened")
	os.WriteFile(filepath.Join(bin, "open"), []byte("#!/bin/sh\necho \"$1\" > "+opened+"\n"), 0o755)
	// A process that has already quit, as the app will have.
	gone := exec.Command("true")
	gone.Run()
	cmd := exec.Command("/bin/sh", "-c", swap, "sh", fmt.Sprint(gone.Process.Pid), installed, staged)
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	version, _ := exec.Command("plutil", "-extract", "CFBundleShortVersionString", "raw", "-o", "-",
		filepath.Join(installed, "Contents", "Info.plist")).Output()
	if strings.TrimSpace(string(version)) != "2.0.0" {
		t.Fatalf("the installed app is %q", version)
	}
	if got, _ := os.ReadFile(opened); string(got) != installed+"\n" {
		t.Fatalf("opened %q", got)
	}
	if _, err := os.Stat(installed + ".old"); err == nil {
		t.Fatal("the old app was left beside the new one")
	}
}
