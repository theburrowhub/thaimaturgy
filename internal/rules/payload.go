package rules

import (
	"bytes"
	"encoding/json"

	"github.com/theburrowhub/thaimaturgy/internal/jsonstrict"
)

// Payload is an immutable, bounded JSON value. Its zero value means that a
// protocol field was omitted. At the protocol boundary JSON null also decodes
// to the zero value, allowing optional payloads to survive a JSON round trip.
//
// Bytes always returns a copy, so callers cannot mutate a Payload shared with a
// Ruleset.
type Payload struct {
	raw string
}

// NewPayload validates and copies raw JSON.
func NewPayload(raw []byte) (Payload, error) {
	if len(raw) == 0 {
		return Payload{}, invalid("payload", "must not be empty")
	}
	if len(raw) > MaxPayloadBytes {
		return Payload{}, invalid("payload", "exceeds %d bytes", MaxPayloadBytes)
	}
	if err := jsonstrict.Validate(raw); err != nil {
		return Payload{}, invalid("payload", "must be unambiguous UTF-8 JSON: %v", err)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return Payload{}, invalid("payload", "JSON null is reserved for the zero value")
	}
	return Payload{raw: string(bytes.Clone(raw))}, nil
}

// PayloadFrom marshals v into a Payload.
func PayloadFrom(v any) (Payload, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return Payload{}, err
	}
	return NewPayload(raw)
}

// IsZero reports whether the payload was omitted.
func (p Payload) IsZero() bool { return p.raw == "" }

// Bytes returns a copy of the encoded JSON, or nil for the zero value.
func (p Payload) Bytes() []byte {
	if p.IsZero() {
		return nil
	}
	return []byte(p.raw)
}

// String returns the encoded JSON, or an empty string for the zero value.
func (p Payload) String() string { return p.raw }

// Validate checks that p is present, bounded, and valid JSON.
func (p Payload) Validate() error {
	if p.IsZero() {
		return invalid("payload", "must be present")
	}
	if len(p.raw) > MaxPayloadBytes {
		return invalid("payload", "exceeds %d bytes", MaxPayloadBytes)
	}
	if err := jsonstrict.Validate([]byte(p.raw)); err != nil {
		return invalid("payload", "must be unambiguous UTF-8 JSON: %v", err)
	}
	return nil
}

// MarshalJSON implements json.Marshaler. An omitted Payload is encoded as null.
func (p Payload) MarshalJSON() ([]byte, error) {
	if p.IsZero() {
		return []byte("null"), nil
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return p.Bytes(), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (p *Payload) UnmarshalJSON(raw []byte) error {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		*p = Payload{}
		return nil
	}
	value, err := NewPayload(raw)
	if err != nil {
		return err
	}
	*p = value
	return nil
}

func validateOptionalPayload(field string, p Payload) error {
	if p.IsZero() {
		return nil
	}
	if err := p.Validate(); err != nil {
		return invalid(field, "%v", err)
	}
	return nil
}

func validateRequiredPayload(field string, p Payload) error {
	if p.IsZero() {
		return invalid(field, "must be present")
	}
	if err := p.Validate(); err != nil {
		return invalid(field, "%v", err)
	}
	return nil
}

func validateJSONObject(field string, p Payload, required bool) error {
	if p.IsZero() {
		if required {
			return invalid(field, "must be present")
		}
		return nil
	}
	if err := p.Validate(); err != nil {
		return invalid(field, "%v", err)
	}
	trimmed := bytes.TrimSpace(p.Bytes())
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return invalid(field, "must be a JSON object")
	}
	return nil
}
