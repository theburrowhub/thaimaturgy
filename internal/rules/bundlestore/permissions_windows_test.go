//go:build windows

package bundlestore

import (
	"path/filepath"
	"testing"
)

func TestStoreAcceptsWindowsSyntheticDirectoryPermissions(t *testing.T) {
	if _, err := New(filepath.Join(t.TempDir(), DirectoryName), nil); err != nil {
		t.Fatalf("Windows directory permissions were treated as POSIX ACLs: %v", err)
	}
}
