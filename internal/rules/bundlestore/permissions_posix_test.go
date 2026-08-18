//go:build !windows

package bundlestore

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreRejectsNonPrivateDirectories(t *testing.T) {
	base := t.TempDir()
	insecureRoot := filepath.Join(base, "insecure")
	if err := os.Mkdir(insecureRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(insecureRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := New(insecureRoot, nil); err == nil || !strings.Contains(err.Error(), "group or other") {
		t.Fatalf("insecure root error = %v", err)
	}

	store, err := New(filepath.Join(base, "private"), nil)
	if err != nil {
		t.Fatal(err)
	}
	packageDirectory := filepath.Join(store.Root(), "test.insecure")
	if err := os.Mkdir(packageDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(packageDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Install(context.Background(), bytes.NewReader(testBundle(t, "test.insecure", "1.0.0"))); err == nil || !strings.Contains(err.Error(), "group or other") {
		t.Fatalf("insecure package directory error = %v", err)
	}
}
