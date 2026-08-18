package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func exampleRulesSource(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate command test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "../../examples/rules/simple-d6"))
}

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

func TestPackProducesAnInstallableBundle(t *testing.T) {
	bundle := filepath.Join(t.TempDir(), "simple-d6.rules.zip")
	var output, diagnostics bytes.Buffer
	if code := run([]string{"pack", exampleRulesSource(t), bundle}, &output, &diagnostics); code != 0 {
		t.Fatalf("pack exit=%d stderr=%s", code, diagnostics.String())
	}
	if !strings.Contains(output.String(), "packed simple-d6@0.1.0") ||
		!strings.Contains(output.String(), "digest: sha256:") ||
		!strings.Contains(output.String(), bundle) || diagnostics.Len() != 0 {
		t.Fatalf("pack stdout=%s stderr=%s", output.String(), diagnostics.String())
	}
	if info, err := os.Stat(bundle); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("packed bundle: %v", err)
	}

	// Packing the same source to the same path is an idempotent author workflow.
	output.Reset()
	if code := run([]string{"pack", exampleRulesSource(t), bundle}, &output, &diagnostics); code != 0 {
		t.Fatalf("repeated pack exit=%d stderr=%s", code, diagnostics.String())
	}

	data := t.TempDir()
	output.Reset()
	if code := run([]string{"--data-dir", data, "install", bundle}, &output, &diagnostics); code != 0 {
		t.Fatalf("install packed bundle exit=%d stderr=%s", code, diagnostics.String())
	}
	if !strings.Contains(output.String(), "installed simple-d6@0.1.0") {
		t.Fatalf("install stdout=%s", output.String())
	}
}

func TestPackUsageRequiresExactlyTwoPaths(t *testing.T) {
	var output, diagnostics bytes.Buffer
	if code := run([]string{"pack", "source-only"}, &output, &diagnostics); code != 2 ||
		!strings.Contains(diagnostics.String(), "source directory") {
		t.Fatalf("pack exit=%d stderr=%s", code, diagnostics.String())
	}
}
