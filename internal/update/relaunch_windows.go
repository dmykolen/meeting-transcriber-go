package update

import (
	"errors"
	"os"
)

// Relaunch is macOS-only: Windows runs from wherever its installer put it,
// Bundle finds no .app there, and the interface offers the release page.
func Relaunch(app, staged string, log *os.File) error {
	return errors.New("on Windows a new version is installed by running its installer")
}
