//go:build !windows

package bundlestore

import (
	"fmt"
	"os"
)

func validatePrivateDirectoryPermissions(info os.FileInfo, label string) error {
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("rules bundle store: %s must not grant group or other permissions", label)
	}
	return nil
}
