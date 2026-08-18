package rules

import (
	"bytes"
	"encoding/binary"
	"path"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	builtinSourceFormat = "thaimaturgy-builtin-source-v1"
	maxBuiltinSources   = MaxCollectionItems
	maxBuiltinBytes     = 8 << 20
)

// NewBuiltinArtifact derives a host-attested artifact from the production
// source embedded in a built-in package. Unlike a hand-maintained ABI string,
// this makes every source change produce a different digest automatically.
// Callers should include behavior-affecting shared helpers as separate entries.
func NewBuiltinArtifact(manifest Manifest, sources map[string]string) (Artifact, error) {
	if manifest.Runtime.Kind != RuntimeBuiltin {
		return Artifact{}, invalid("builtin.manifest.runtime", "must be %q", RuntimeBuiltin)
	}
	if len(sources) == 0 || len(sources) > maxBuiltinSources {
		return Artifact{}, invalid("builtin.sources", "must contain 1..%d files", maxBuiltinSources)
	}

	names := make([]string, 0, len(sources))
	canonical := make(map[string]string, len(sources))
	total := len(builtinSourceFormat)
	for name, source := range sources {
		if err := validateBuiltinSourceName(name); err != nil {
			return Artifact{}, err
		}
		source = strings.ReplaceAll(source, "\r\n", "\n")
		source = strings.ReplaceAll(source, "\r", "")
		if source == "" || !utf8.ValidString(source) {
			return Artifact{}, invalid("builtin.sources", "%q must contain non-empty UTF-8 source", name)
		}
		total += len(name) + len(source) + 16
		if total > maxBuiltinBytes {
			return Artifact{}, invalid("builtin.sources", "exceed %d bytes", maxBuiltinBytes)
		}
		names = append(names, name)
		canonical[name] = source
	}
	slices.Sort(names)

	var material bytes.Buffer
	writeBuiltinFrame(&material, builtinSourceFormat)
	for _, name := range names {
		writeBuiltinFrame(&material, name)
		writeBuiltinFrame(&material, canonical[name])
	}
	return NewArtifact(manifest, bytes.NewReader(material.Bytes()))
}

func validateBuiltinSourceName(name string) error {
	if name == "" || len(name) > MaxTextBytes || strings.Contains(name, "\\") ||
		path.IsAbs(name) || path.Clean(name) != name {
		return invalid("builtin.source.name", "%q is not a clean relative slash path", name)
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return invalid("builtin.source.name", "%q contains an invalid segment", name)
		}
	}
	return nil
}

func writeBuiltinFrame(destination *bytes.Buffer, value string) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = destination.Write(size[:])
	_, _ = destination.WriteString(value)
}
