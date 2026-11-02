package media

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// quiet starts a console program without the console window Windows would
// open for it: the app has no console of its own to lend.
func quiet(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
	return cmd
}
