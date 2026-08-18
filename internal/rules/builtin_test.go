package rules

import "testing"

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
	} {
		if _, err := NewBuiltinArtifact(testManifest(), sources); err == nil {
			t.Fatalf("invalid built-in sources were accepted: %#v", sources)
		}
	}
}
