package mcpserve

import (
	"strings"
	"testing"
)

// TestOpenStorageHonorsDataDir: the MCP subprocess must read the SAME data dir as
// the parent (server/GUI/bot). When THAIM_DATA_DIR is set, storage resolves under
// it rather than the default ~/.thaimaturgy — otherwise the Claude-CLI backend's
// tool subprocess would never find the parent's adventure/session in Docker.
func TestOpenStorageHonorsDataDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("THAIM_DATA_DIR", dir)
	store, err := openStorage()
	if err != nil {
		t.Fatalf("openStorage: %v", err)
	}
	if got := store.AdventureDir("x"); !strings.HasPrefix(got, dir) {
		t.Fatalf("AdventureDir = %q; want it under THAIM_DATA_DIR %q", got, dir)
	}
}
