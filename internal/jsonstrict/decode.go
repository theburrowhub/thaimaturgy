// Package jsonstrict decodes bounded protocol objects without accepting unknown
// or duplicate fields. Callers remain responsible for their input size limit.
package jsonstrict

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
)

const maxNestingDepth = 128

// Decode decodes exactly one JSON value into dst. Object fields must be unique,
// and fields unknown to a struct destination are rejected at every depth.
func Decode(raw []byte, dst any) error {
	if err := rejectDuplicateFields(raw); err != nil {
		return err
	}
	if err := validateExactFieldNames(raw, reflect.TypeOf(dst)); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

var jsonUnmarshalerType = reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()
var rawMessageType = reflect.TypeOf(json.RawMessage{})

func validateExactFieldNames(raw []byte, destination reflect.Type) error {
	if destination == nil {
		return nil
	}
	for destination.Kind() == reflect.Pointer {
		destination = destination.Elem()
	}
	if destination == rawMessageType || destination.Implements(jsonUnmarshalerType) ||
		reflect.PointerTo(destination).Implements(jsonUnmarshalerType) {
		return nil
	}

	switch destination.Kind() {
	case reflect.Struct:
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil {
			return nil // The typed decoder below reports non-object/type errors.
		}
		fields := exactJSONFields(destination)
		for name, value := range object {
			fieldType, ok := fields[name]
			if !ok {
				return fmt.Errorf("json: unknown field %q", name)
			}
			if err := validateExactFieldNames(value, fieldType); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		var values []json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil
		}
		for _, value := range values {
			if err := validateExactFieldNames(value, destination.Elem()); err != nil {
				return err
			}
		}
	}
	return nil
}

func exactJSONFields(destination reflect.Type) map[string]reflect.Type {
	fields := make(map[string]reflect.Type)
	for _, field := range reflect.VisibleFields(destination) {
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}
		tag := field.Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		if name == "-" {
			continue
		}
		if field.Anonymous && name == "" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		fields[name] = field.Type
	}
	return fields
}

func rejectDuplicateFields(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	return consumeValue(decoder, 0)
}

func consumeValue(decoder *json.Decoder, depth int) error {
	if depth > maxNestingDepth {
		return fmt.Errorf("JSON nesting exceeds %d levels", maxNestingDepth)
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("JSON object member is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate JSON field %q", key)
			}
			seen[key] = struct{}{}
			if err := consumeValue(decoder, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := consumeValue(decoder, depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
	closing, err := decoder.Token()
	if err != nil {
		return err
	}
	if closingDelimiter, ok := closing.(json.Delim); !ok ||
		(delimiter == '{' && closingDelimiter != '}') ||
		(delimiter == '[' && closingDelimiter != ']') {
		return fmt.Errorf("mismatched JSON delimiter")
	}
	return nil
}
