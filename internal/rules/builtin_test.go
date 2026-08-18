package rules

import (
	"bytes"
	"strings"
	"testing"
)

func TestBuiltinArtifactIdentityTracksCanonicalSource(t *testing.T) {
	manifest := testManifest()
	first, err := NewBuiltinArtifact(manifest, map[string]string{
		"package/main.go":  "package example\r\n",
		"shared/helper.go": "package shared\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	reordered, err := NewBuiltinArtifact(manifest, map[string]string{
		"shared/helper.go": "package shared\n",
		"package/main.go":  "package example\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest() != reordered.Digest() {
		t.Fatal("source order or platform line endings changed the built-in digest")
	}
	changed, err := NewBuiltinArtifact(manifest, map[string]string{
		"package/main.go":  "package example\n// behavior changed\n",
		"shared/helper.go": "package shared\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest() == changed.Digest() {
		t.Fatal("production source change retained the same built-in digest")
	}
}

func TestBuiltinArtifactRejectsInvalidInputs(t *testing.T) {
	external := testManifest()
	external.Runtime = Runtime{Kind: RuntimeStarlark, Entrypoint: "main.star"}
	if _, err := NewBuiltinArtifact(external, map[string]string{"main.go": "package x"}); err == nil {
		t.Fatal("external runtime was accepted as a built-in artifact")
	}
	for _, sources := range []map[string]string{
		nil,
		{"../main.go": "package x"},
		{"main.go": ""},
		{kernelSourceName: "forged"},
	} {
		if _, err := NewBuiltinArtifact(testManifest(), sources); err == nil {
			t.Fatalf("invalid built-in sources were accepted: %#v", sources)
		}
	}
}

func TestBuiltinArtifactAutomaticallyIncludesKernelIdentity(t *testing.T) {
	manifest := testManifest()
	artifact, err := NewBuiltinArtifact(manifest, map[string]string{"package/test.go": "package test"})
	if err != nil {
		t.Fatal(err)
	}

	var packageOnly bytes.Buffer
	writeBuiltinFrame(&packageOnly, builtinSourceFormat)
	writeBuiltinFrame(&packageOnly, "package/test.go")
	writeBuiltinFrame(&packageOnly, "package test")
	withoutKernel, err := NewArtifact(manifest, bytes.NewReader(packageOnly.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Digest() == withoutKernel.Digest() {
		t.Fatal("built-in digest omitted the rules kernel identity")
	}
	if identity := SourceIdentity(); !strings.HasPrefix(identity, kernelSourceFormat+"\nsha256:") {
		t.Fatalf("invalid kernel source identity %q", identity)
	}
	if !bytes.Contains(kernelSourceMaterial(), []byte("step.go")) {
		t.Fatal("kernel identity does not cover production step validation")
	}
}
