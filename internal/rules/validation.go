package rules

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// MaxIdentifierBytes bounds machine-readable identifiers accepted by this
	// package. Identifiers are intentionally ASCII and portable.
	MaxIdentifierBytes = 128
	// MaxTextBytes bounds individual human-readable fields.
	MaxTextBytes = 16 << 10
	// MaxPayloadBytes bounds each opaque JSON value crossing the rules boundary.
	MaxPayloadBytes = 1 << 20
	// MaxCollectionItems bounds lists in the rules protocol.
	MaxCollectionItems = 256
)

var semverPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-((?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

// ValidationError identifies one structurally invalid field.
type ValidationError struct {
	Field   string
	Problem string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return "rules: invalid value: " + e.Problem
	}
	return "rules: invalid " + e.Field + ": " + e.Problem
}

func invalid(field, format string, args ...any) error {
	return &ValidationError{Field: field, Problem: fmt.Sprintf(format, args...)}
}

func validateIdentifier(field, value string) error {
	if value == "" {
		return invalid(field, "must not be empty")
	}
	if len(value) > MaxIdentifierBytes {
		return invalid(field, "exceeds %d bytes", MaxIdentifierBytes)
	}
	if !isLowerAlphaNumeric(value[0]) || !isLowerAlphaNumeric(value[len(value)-1]) {
		return invalid(field, "must start and end with a lowercase letter or digit")
	}
	previousSeparator := false
	for i := 0; i < len(value); i++ {
		c := value[i]
		separator := c == '.' || c == '-' || c == '_'
		if !isLowerAlphaNumeric(c) && !separator {
			return invalid(field, "contains unsupported character %q", c)
		}
		if separator && previousSeparator {
			return invalid(field, "contains adjacent separators")
		}
		previousSeparator = separator
	}
	return nil
}

func validateOpaqueID(field, value string) error {
	if value == "" {
		return invalid(field, "must not be empty")
	}
	if len(value) > MaxIdentifierBytes {
		return invalid(field, "exceeds %d bytes", MaxIdentifierBytes)
	}
	if !utf8.ValidString(value) {
		return invalid(field, "is not valid UTF-8")
	}
	if strings.TrimSpace(value) != value {
		return invalid(field, "must not have surrounding whitespace")
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return invalid(field, "must not contain whitespace or control characters")
		}
	}
	return nil
}

func validateText(field, value string, required bool) error {
	if value == "" {
		if required {
			return invalid(field, "must not be empty")
		}
		return nil
	}
	if len(value) > MaxTextBytes {
		return invalid(field, "exceeds %d bytes", MaxTextBytes)
	}
	if !utf8.ValidString(value) {
		return invalid(field, "is not valid UTF-8")
	}
	if strings.TrimSpace(value) != value {
		return invalid(field, "must not have surrounding whitespace")
	}
	for _, r := range value {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return invalid(field, "contains a control character")
		}
	}
	return nil
}

func validateSemver(field, value string) error {
	if len(value) > MaxIdentifierBytes || !semverPattern.MatchString(value) {
		return invalid(field, "must be a canonical semantic version")
	}
	return nil
}

func validateDigest(field, value string) error {
	const prefix = "sha256:"
	if !strings.HasPrefix(value, prefix) {
		return invalid(field, "must use the sha256:<hex> form")
	}
	hexDigest := strings.TrimPrefix(value, prefix)
	if len(hexDigest) != 64 || strings.ToLower(hexDigest) != hexDigest {
		return invalid(field, "must contain 64 lowercase hexadecimal characters")
	}
	if _, err := hex.DecodeString(hexDigest); err != nil {
		return invalid(field, "must contain 64 lowercase hexadecimal characters")
	}
	return nil
}

func isLowerAlphaNumeric(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
}

func isASCIIAlphaNumeric(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func validateUniqueIdentifiers(field string, values []string) error {
	if len(values) > MaxCollectionItems {
		return invalid(field, "contains more than %d items", MaxCollectionItems)
	}
	seen := make(map[string]struct{}, len(values))
	for i, value := range values {
		itemField := fmt.Sprintf("%s[%d]", field, i)
		if err := validateIdentifier(itemField, value); err != nil {
			return err
		}
		if _, exists := seen[value]; exists {
			return invalid(itemField, "duplicates %q", value)
		}
		seen[value] = struct{}{}
	}
	return nil
}
