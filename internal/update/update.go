// Package update finds a newer release on GitHub and swaps it in for the
// running app. The release is a disk image holding the signed .app, so an update
// is: download, check the digest, copy the app out of the image beside the
// installed one, check its signature, and exchange the two once the old one has
// quit. Every release is signed by the same identity, which is what keeps the
// microphone and system-audio permissions across the swap.
package update

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

// LatestURL is where the newest published release is described.
const LatestURL = "https://api.github.com/repos/dmykolen/meeting-transcriber-go/releases/latest"

// Release is a published version of the app.
type Release struct {
	Version string `json:"version"` // 1.4.0
	Notes   string `json:"notes"`
	Page    string `json:"page"`
	dmg     string
	digest  string // hex sha256, empty when GitHub gave none
	size    int64
}

// Latest reads the newest release from url.
func Latest(ctx context.Context, url string) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
		Text    string `json:"body"`
		Page    string `json:"html_url"`
		Assets  []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Size   int64  `json:"size"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Release{}, fmt.Errorf("the release could not be read: %w", err)
	}
	r := Release{Version: strings.TrimPrefix(body.TagName, "v"), Notes: strings.TrimSpace(body.Text), Page: body.Page}
	for _, a := range body.Assets {
		if strings.HasSuffix(a.Name, ".dmg") {
			r.dmg, r.size, r.digest = a.URL, a.Size, strings.TrimPrefix(a.Digest, "sha256:")
		}
	}
	return r, nil
}

// Newer reports whether version latest is later than current; both read
// major.minor.patch, and anything else is never newer.
func Newer(current, latest string) bool {
	a, b := parts(current), parts(latest)
	return a != nil && b != nil && slices.Compare(b, a) > 0
}

func parts(v string) []int {
	fields := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(fields) != 3 {
		return nil
	}
	out := make([]int, 3)
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil {
			return nil
		}
		out[i] = n
	}
	return out
}

// Bundle is the .app the running executable lives in, or an error when it was
// started some other way (a bare binary, go run), which nothing can update.
func Bundle() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	app := filepath.Dir(filepath.Dir(filepath.Dir(self))) // X.app/Contents/MacOS/exe
	if !strings.HasSuffix(app, ".app") {
		return "", errors.New("the app is not running from an .app bundle")
	}
	return app, nil
}

// Stage downloads the release and puts its app beside the installed one,
// verified, ready to be swapped in by Relaunch. progress gets 0..1 while the
// image downloads. It returns the staged app's path.
func Stage(ctx context.Context, r Release, app string, progress func(float64)) (string, error) {
	if r.dmg == "" {
		return "", errors.New("the release has no disk image")
	}
	// Fail before the download if the installed copy cannot be replaced.
	probe, err := os.CreateTemp(filepath.Dir(app), ".mt-update-*")
	if err != nil {
		return "", fmt.Errorf("the folder of the app cannot be written to: %w", err)
	}
	probe.Close()
	os.Remove(probe.Name())

	work, err := os.MkdirTemp("", "mt-update-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work)
	image := filepath.Join(work, "release.dmg")
	if err := download(ctx, r, image, progress); err != nil {
		return "", err
	}

	mount := filepath.Join(work, "image")
	if out, err := exec.CommandContext(ctx, "hdiutil", "attach", "-nobrowse", "-readonly", "-noverify",
		"-mountpoint", mount, image).CombinedOutput(); err != nil {
		return "", fmt.Errorf("the disk image would not open: %w: %s", err, bytesTrim(out))
	}
	defer exec.Command("hdiutil", "detach", "-force", mount).Run()
	found, _ := filepath.Glob(filepath.Join(mount, "*.app"))
	if len(found) != 1 {
		return "", fmt.Errorf("the disk image holds %d apps, not one", len(found))
	}

	staged := filepath.Join(filepath.Dir(app), "."+filepath.Base(app)+".update")
	os.RemoveAll(staged)
	if out, err := exec.CommandContext(ctx, "ditto", found[0], staged).CombinedOutput(); err != nil {
		os.RemoveAll(staged)
		return "", fmt.Errorf("the new app could not be copied: %w: %s", err, bytesTrim(out))
	}
	fail := func(err error) (string, error) { os.RemoveAll(staged); return "", err }
	if out, err := exec.CommandContext(ctx, "codesign", "--verify", "--deep", "--strict", staged).CombinedOutput(); err != nil {
		return fail(fmt.Errorf("the new app's signature is not valid: %w: %s", err, bytesTrim(out)))
	}
	version, err := exec.CommandContext(ctx, "plutil", "-extract", "CFBundleShortVersionString", "raw", "-o", "-",
		filepath.Join(staged, "Contents", "Info.plist")).Output()
	if err != nil || strings.TrimSpace(string(version)) != r.Version {
		return fail(fmt.Errorf("the disk image holds version %q, not %s", strings.TrimSpace(string(version)), r.Version))
	}
	return staged, nil
}

func download(ctx context.Context, r Release, to string, progress func(float64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.dmg, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the download answered %s", resp.Status)
	}
	file, err := os.Create(to)
	if err != nil {
		return err
	}
	defer file.Close()
	sum := sha256.New()
	total := cmp.Or(r.size, resp.ContentLength)
	var done int64
	body := io.TeeReader(resp.Body, writeFunc(func(p []byte) {
		sum.Write(p)
		done += int64(len(p))
		if total > 0 {
			progress(float64(done) / float64(total))
		}
	}))
	if _, err := io.Copy(file, body); err != nil {
		return fmt.Errorf("the download broke off: %w", err)
	}
	if got := hex.EncodeToString(sum.Sum(nil)); r.digest != "" && got != r.digest {
		return fmt.Errorf("the download is damaged: sha256 %s, expected %s", got, r.digest)
	}
	return nil
}

type writeFunc func([]byte)

func (f writeFunc) Write(p []byte) (int, error) { f(p); return len(p), nil }

func bytesTrim(b []byte) string { return strings.TrimSpace(string(b)) }

// swap waits for the process to quit, exchanges the installed app for the staged
// one, and opens it; an old app is put back if the exchange fails.
const swap = `while kill -0 "$1" 2>/dev/null; do sleep 0.2; done
old="$2.old"; rm -rf "$old"
mv "$2" "$old" && mv "$3" "$2" || { [ -e "$2" ] || mv "$old" "$2"; open "$2"; exit 1; }
rm -rf "$old"; open "$2"`

// Relaunch starts the helper that swaps the staged app in once this process has
// quit. The caller quits right after.
func Relaunch(app, staged string, log *os.File) error {
	cmd := exec.Command("/bin/sh", "-c", swap, "sh", strconv.Itoa(os.Getpid()), app, staged)
	if log != nil {
		cmd.Stdout, cmd.Stderr = log, log
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}
