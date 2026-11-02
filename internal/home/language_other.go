//go:build !windows

package home

import (
	"os/exec"
	"strings"
)

// system is "uk" when Ukrainian is among the Mac's preferred languages, and
// "en" otherwise.
func system() string {
	listed, _ := exec.Command("defaults", "read", "-g", "AppleLanguages").Output()
	if strings.Contains(string(listed), `"uk`) {
		return "uk"
	}
	return "en"
}
