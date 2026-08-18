package jsonstrict

import (
	"strings"
	"testing"
)

func TestDecodeRejectsAmbiguousProtocolJSON(t *testing.T) {
	type nested struct {
		Value int `json:"value"`
	}
	type envelope struct {
		Name   string `json:"name"`
		Nested nested `json:"nested"`
	}
	tests := []struct {
		name, raw, contains string
	}{
		{"duplicate top-level", `{"name":"a","name":"b","nested":{"value":1}}`, "duplicate JSON field"},
		{"duplicate nested", `{"name":"a","nested":{"value":1,"value":2}}`, "duplicate JSON field"},
		{"unknown field", `{"name":"a","nested":{"value":1},"extra":true}`, "unknown field"},
		{"case alias", `{"Name":"a","nested":{"value":1}}`, "unknown field"},
		{"nested case alias", `{"name":"a","nested":{"Value":1}}`, "unknown field"},
		{"case-confusable pair", `{"name":"a","NAME":"b","nested":{"value":1}}`, "unknown field"},
		{"multiple values", `{"name":"a","nested":{"value":1}} {}`, "multiple JSON values"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var decoded envelope
			err := Decode([]byte(test.raw), &decoded)
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("error = %v, want substring %q", err, test.contains)
			}
		})
	}
}

func TestDecodeAcceptsOneExactObject(t *testing.T) {
	var decoded struct {
		Name string `json:"name"`
	}
	if err := Decode([]byte(`{"name":"value"}`), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Name != "value" {
		t.Fatalf("name = %q", decoded.Name)
	}
}

func TestDecodeBoundsNestingBeforeRecursiveProtocolValidation(t *testing.T) {
	raw := strings.Repeat(`{"value":`, maxNestingDepth+1) + `0` + strings.Repeat(`}`, maxNestingDepth+1)
	var decoded any
	err := Decode([]byte(raw), &decoded)
	if err == nil || !strings.Contains(err.Error(), "nesting exceeds") {
		t.Fatalf("error = %v", err)
	}
}
