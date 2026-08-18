package rules

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"strings"
)

const kernelSourceFormat = "thaimaturgy-rules-kernel-source-v1"

// kernelSources includes every production Go source in the kernel package.
// SourceIdentity filters tests at runtime so newly added production files are
// automatically covered without making their inclusion a manual checklist.
//
//go:embed *.go
var kernelSources embed.FS

// SourceIdentity returns a canonical, framed snapshot of the rules kernel
// source that built-in implementations execute through. It is incorporated
// automatically by NewBuiltinArtifact.
func SourceIdentity() string {
	material := kernelSourceMaterial()
	digest := sha256.Sum256(material)
	return kernelSourceFormat + "\nsha256:" + hex.EncodeToString(digest[:])
}

func kernelSourceMaterial() []byte {
	entries, err := kernelSources.ReadDir(".")
	if err != nil {
		panic(fmt.Sprintf("rules: read embedded kernel sources: %v", err))
	}
	var identity bytes.Buffer
	writeBuiltinFrame(&identity, kernelSourceFormat)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		content, err := kernelSources.ReadFile(name)
		if err != nil {
			panic(fmt.Sprintf("rules: read embedded kernel source %s: %v", name, err))
		}
		writeBuiltinFrame(&identity, name)
		writeBuiltinFrame(&identity, canonicalBuiltinSource(string(content)))
	}
	return identity.Bytes()
}
