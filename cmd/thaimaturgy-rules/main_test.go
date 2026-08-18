package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathAndUsageCommands(t *testing.T) {
	data := t.TempDir()
	var output, diagnostics bytes.Buffer
	if code := run([]string{"--data-dir", data, "path"}, &output, &diagnostics); code != 0 {
		t.Fatalf("path exit=%d stderr=%s", code, diagnostics.String())
	}
	want := filepath.Join(data, "rulesets")
	if strings.TrimSpace(output.String()) != want {
		t.Fatalf("path = %q, want %q", strings.TrimSpace(output.String()), want)
	}
	if info, err := os.Stat(want); err != nil || !info.IsDir() {
		t.Fatalf("rules directory was not created: %v", err)
	}

	output.Reset()
	diagnostics.Reset()
	if code := run([]string{"--data-dir", data, "unknown"}, &output, &diagnostics); code != 2 ||
		!strings.Contains(diagnostics.String(), "Usage:") {
		t.Fatalf("unknown exit=%d stderr=%s", code, diagnostics.String())
	}
}

func TestInstallRejectsInvalidBundleAndListReportsCleanStore(t *testing.T) {
	data := t.TempDir()
	invalid := filepath.Join(t.TempDir(), "invalid.rules.zip")
	if err := os.WriteFile(invalid, []byte("not zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output, diagnostics bytes.Buffer
	if code := run([]string{"--data-dir", data, "install", invalid}, &output, &diagnostics); code != 1 ||
		!strings.Contains(diagnostics.String(), "invalid bundle") {
		t.Fatalf("install exit=%d stderr=%s", code, diagnostics.String())
	}
	output.Reset()
	diagnostics.Reset()
	if code := run([]string{"--data-dir", data, "list"}, &output, &diagnostics); code != 0 || output.Len() != 0 || diagnostics.Len() != 0 {
		t.Fatalf("list exit=%d stdout=%s stderr=%s", code, output.String(), diagnostics.String())
	}
}
