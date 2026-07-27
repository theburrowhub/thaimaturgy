// Package platform holds small OS-specific helpers.
package platform

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenPath opens a file (e.g. a map or art image) with the operating system's
// default application. It returns after launching the viewer without waiting
// for it to close.
func OpenPath(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	case "linux":
		cmd = exec.Command("xdg-open", path)
	default:
		return fmt.Errorf("opening files is not supported on %s", runtime.GOOS)
	}
	return cmd.Start()
}
