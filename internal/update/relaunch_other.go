//go:build !windows

package update

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

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
