//go:build windows

package bundlestore

import "os"

// Windows FileMode permission bits are synthesized from file attributes and do
// not represent ACL grants. Rejecting their group/other bits would reject every
// directory, including one created by this process with mode 0700.
func validatePrivateDirectoryPermissions(os.FileInfo, string) error { return nil }
