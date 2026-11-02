package home

import (
	"slices"
	"strings"

	"golang.org/x/sys/windows"
)

// system is "uk" when Ukrainian is among the display languages Windows prefers
// for this user, and "en" otherwise.
func system() string {
	listed, _ := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if slices.ContainsFunc(listed, func(l string) bool { return strings.HasPrefix(l, "uk") }) {
		return "uk"
	}
	return "en"
}
