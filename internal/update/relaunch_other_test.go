//go:build !windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
