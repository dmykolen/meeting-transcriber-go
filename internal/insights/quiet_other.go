//go:build !windows

package insights

import "os/exec"

// quiet matters only on Windows, where a console program gets a window.
func quiet(cmd *exec.Cmd) *exec.Cmd { return cmd }
