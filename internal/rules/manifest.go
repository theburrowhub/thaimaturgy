package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"path"
	"strings"
)

// ProtocolVersion is the host/package protocol implemented by this package.
const ProtocolVersion = "1.0.0"

// RuntimeKind identifies a host-provided package runtime. The constants are
// conveniences, not a closed enum: hosts may register additional runtimes.
type RuntimeKind string

const (
	RuntimeBuiltin  RuntimeKind = "builtin"
	RuntimeStarlark RuntimeKind = "starlark"
	RuntimeWASM     RuntimeKind = "wasm"
)

// Runtime declares how the host loads a rules package. Entrypoint is empty for
// built-in Go implementations and a portable package-relative path otherwise.
type Runtime struct {
	Kind       RuntimeKind `json:"kind"`
	Entrypoint string      `json:"entrypoint,omitempty"`
}

// Validate performs structural validation only. Runtime availability is a host
// concern and is checked when a package is loaded.
func (r Runtime) Validate() error {
	if err := validateIdentifier("runtime.kind", string(r.Kind)); err != nil {
		return err
	}
	if r.Kind == RuntimeBuiltin {
		if r.Entrypoint != "" {
			return invalid("runtime.entrypoint", "must be empty for the builtin runtime")
		}
		return nil
	}
	if r.Entrypoint == "" {
		return invalid("runtime.entrypoint", "must not be empty for an external runtime")
	}
	if len(r.Entrypoint) > MaxTextBytes {
		return invalid("runtime.entrypoint", "exceeds %d bytes", MaxTextBytes)
	}
	if strings.Contains(r.Entrypoint, "\\") || path.IsAbs(r.Entrypoint) || path.Clean(r.Entrypoint) != r.Entrypoint {
		return invalid("runtime.entrypoint", "must be a clean, package-relative slash path")
	}
	for _, segment := range strings.Split(r.Entrypoint, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return invalid("runtime.entrypoint", "must not contain empty, dot, or parent segments")
		}
		for i := 0; i < len(segment); i++ {
			c := segment[i]
			if !isASCIIAlphaNumeric(c) && c != '.' && c != '-' && c != '_' {
				return invalid("runtime.entrypoint", "contains unsupported or non-portable character %q", c)
			}
		}
	}
	return nil
}

// Manifest is the self-declared loading metadata of one rules package. Artifact
// digests deliberately live outside the manifest so hashing a bundle is not
// self-referential and the package cannot attest its own bytes.
type Manifest struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description,omitempty"`
	Version         string   `json:"version"`
	ProtocolVersion string   `json:"protocol_version"`
	Runtime         Runtime  `json:"runtime"`
	Capabilities    []string `json:"capabilities,omitempty"`
}

// Validate checks the manifest's bounded, canonical structure.
func (m Manifest) Validate() error {
	if err := validateIdentifier("manifest.id", m.ID); err != nil {
		return err
	}
	if err := validateText("manifest.name", m.Name, true); err != nil {
		return err
	}
	if err := validateText("manifest.description", m.Description, false); err != nil {
		return err
	}
	if err := validateSemver("manifest.version", m.Version); err != nil {
		return err
	}
	if err := validateSemver("manifest.protocol_version", m.ProtocolVersion); err != nil {
		return err
	}
	if err := m.Runtime.Validate(); err != nil {
		return err
	}
	return validateUniqueIdentifiers("manifest.capabilities", m.Capabilities)
}

// VersionConstraint is an unresolved, resolver-defined version expression. The
// kernel validates its transport form but deliberately does not interpret range
// syntax. A resolver turns a Requirement into an exact Lock.
type VersionConstraint string

// Validate checks that a version constraint is bounded, printable ASCII. Range
// grammar and matching semantics belong to the package resolver.
func (v VersionConstraint) Validate() error {
	value := string(v)
	if value == "" {
		return invalid("version_constraint", "must not be empty")
	}
	if len(value) > MaxIdentifierBytes {
		return invalid("version_constraint", "exceeds %d bytes", MaxIdentifierBytes)
	}
	if strings.TrimSpace(value) != value {
		return invalid("version_constraint", "must not have surrounding whitespace")
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c < 0x20 || c > 0x7e || c == '/' || c == '\\' {
			return invalid("version_constraint", "contains unsupported character %q", c)
		}
	}
	return nil
}

// Requirement is an unresolved dependency requested by content or another
// rules package. It never acts as a session identity.
type Requirement struct {
	ID      string            `json:"id"`
	Version VersionConstraint `json:"version"`
}

// Validate checks the unresolved requirement's transport form.
func (r Requirement) Validate() error {
	if err := validateIdentifier("requirement.id", r.ID); err != nil {
		return err
	}
	return r.Version.Validate()
}

// Artifact combines a manifest with a digest calculated by the host over the
// immutable bundle bytes. Its fields are intentionally private: callers obtain
// an Artifact only by hashing bytes through NewArtifact.
type Artifact struct {
	manifest Manifest
	digest   string
}

// NewArtifact validates manifest and consumes bundle to calculate its SHA-256
// digest. The reader must cover the exact immutable bytes the host will load;
// callers that accept untrusted bundles must enforce their own size limit.
func NewArtifact(manifest Manifest, bundle io.Reader) (Artifact, error) {
	if bundle == nil {
		return Artifact{}, invalid("artifact.bundle", "must not be nil")
	}
	manifest = cloneManifest(manifest)
	if err := manifest.Validate(); err != nil {
		return Artifact{}, err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, bundle); err != nil {
		return Artifact{}, err
	}
	return Artifact{
		manifest: manifest,
		digest:   "sha256:" + hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

// Manifest returns a defensive copy of the verified artifact manifest.
func (a Artifact) Manifest() Manifest { return cloneManifest(a.manifest) }

// Digest returns the host-calculated canonical SHA-256 digest.
func (a Artifact) Digest() string { return a.digest }

// Validate checks the internally attested artifact identity.
func (a Artifact) Validate() error {
	if err := a.manifest.Validate(); err != nil {
		return err
	}
	return validateDigest("artifact.digest", a.digest)
}

// Lock is the exact rules artifact fixed for a running session.
type Lock struct {
	ID              string `json:"id"`
	Version         string `json:"version"`
	Digest          string `json:"digest"`
	ProtocolVersion string `json:"protocol_version"`
}

// Validate checks that the lock identifies exactly one artifact.
func (l Lock) Validate() error {
	if err := validateIdentifier("lock.id", l.ID); err != nil {
		return err
	}
	if err := validateSemver("lock.version", l.Version); err != nil {
		return err
	}
	if err := validateDigest("lock.digest", l.Digest); err != nil {
		return err
	}
	return validateSemver("lock.protocol_version", l.ProtocolVersion)
}

// Lock returns the artifact's exact session identity.
func (a Artifact) Lock() Lock {
	return Lock{
		ID:              a.manifest.ID,
		Version:         a.manifest.Version,
		Digest:          a.digest,
		ProtocolVersion: a.manifest.ProtocolVersion,
	}
}

// Requirement returns an exact-version requirement for this manifest. It still
// requires host resolution because requirements never carry artifact digests.
func (m Manifest) Requirement() Requirement {
	return Requirement{ID: m.ID, Version: VersionConstraint(m.Version)}
}

func cloneManifest(m Manifest) Manifest {
	m.Capabilities = append([]string(nil), m.Capabilities...)
	return m
}

func cloneArtifact(a Artifact) Artifact {
	a.manifest = cloneManifest(a.manifest)
	return a
}
