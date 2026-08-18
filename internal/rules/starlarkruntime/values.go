package starlarkruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/theburrowhub/thaimaturgy/internal/jsonstrict"
	star "go.starlark.net/starlark"
)

type valueBudget struct {
	limits Limits
	nodes  int
}

func (b *valueBudget) use(depth int) error {
	if depth > b.limits.MaxValueDepth {
		return fmt.Errorf("%w: JSON value exceeds depth %d", ErrContract, b.limits.MaxValueDepth)
	}
	b.nodes++
	if b.nodes > b.limits.MaxValueNodes {
		return fmt.Errorf("%w: JSON value exceeds %d nodes", ErrContract, b.limits.MaxValueNodes)
	}
	return nil
}

func requestValue(value any, limits Limits) (star.Value, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%w: encode request: %v", ErrContract, err)
	}
	if len(raw) > limits.MaxCallBytes {
		return nil, fmt.Errorf("%w: request exceeds %d bytes", ErrContract, limits.MaxCallBytes)
	}
	var neutral any
	if err := jsonstrict.Decode(raw, &neutral); err != nil {
		return nil, fmt.Errorf("%w: request is not strict JSON: %v", ErrContract, err)
	}
	return toStarlark(neutral, &valueBudget{limits: limits}, 0)
}

func toStarlark(value any, budget *valueBudget, depth int) (star.Value, error) {
	if err := budget.use(depth); err != nil {
		return nil, err
	}
	switch value := value.(type) {
	case nil:
		return star.None, nil
	case bool:
		return star.Bool(value), nil
	case string:
		if !utf8.ValidString(value) {
			return nil, fmt.Errorf("%w: JSON string is not valid UTF-8", ErrContract)
		}
		return star.String(value), nil
	case json.Number:
		return numberToStarlark(value)
	case []any:
		if len(value) > budget.limits.MaxCollectionItems {
			return nil, fmt.Errorf("%w: JSON array exceeds %d items", ErrContract, budget.limits.MaxCollectionItems)
		}
		items := make([]star.Value, len(value))
		for index, item := range value {
			converted, err := toStarlark(item, budget, depth+1)
			if err != nil {
				return nil, err
			}
			items[index] = converted
		}
		return star.NewList(items), nil
	case map[string]any:
		if len(value) > budget.limits.MaxCollectionItems {
			return nil, fmt.Errorf("%w: JSON object exceeds %d members", ErrContract, budget.limits.MaxCollectionItems)
		}
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		dictionary := star.NewDict(len(keys))
		for _, key := range keys {
			converted, err := toStarlark(value[key], budget, depth+1)
			if err != nil {
				return nil, err
			}
			if err := dictionary.SetKey(star.String(key), converted); err != nil {
				return nil, fmt.Errorf("%w: construct request object: %v", ErrContract, err)
			}
		}
		return dictionary, nil
	default:
		return nil, fmt.Errorf("%w: unsupported Go JSON value %T", ErrContract, value)
	}
}

func numberToStarlark(number json.Number) (star.Value, error) {
	text := number.String()
	if strings.ContainsAny(text, ".eE") {
		value, err := number.Float64()
		if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
			return nil, fmt.Errorf("%w: invalid finite JSON number %q", ErrContract, text)
		}
		return star.Float(value), nil
	}
	integer, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return nil, fmt.Errorf("%w: invalid JSON integer %q", ErrContract, text)
	}
	return star.MakeBigInt(integer), nil
}

func resultJSON(value star.Value, limits Limits) ([]byte, error) {
	neutral, err := fromStarlark(value, &valueBudget{limits: limits}, 0)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(neutral)
	if err != nil {
		return nil, fmt.Errorf("%w: encode result: %v", ErrContract, err)
	}
	if len(raw) > limits.MaxCallBytes {
		return nil, fmt.Errorf("%w: result exceeds %d bytes", ErrContract, limits.MaxCallBytes)
	}
	return raw, nil
}

func fromStarlark(value star.Value, budget *valueBudget, depth int) (any, error) {
	if value == nil {
		return nil, fmt.Errorf("%w: Starlark returned Go nil", ErrContract)
	}
	if err := budget.use(depth); err != nil {
		return nil, err
	}
	switch value := value.(type) {
	case star.NoneType:
		return nil, nil
	case star.Bool:
		return bool(value), nil
	case star.String:
		text := string(value)
		if !utf8.ValidString(text) {
			return nil, fmt.Errorf("%w: result string is not valid UTF-8", ErrContract)
		}
		return text, nil
	case star.Int:
		text := value.BigInt().String()
		if len(text) > budget.limits.MaxCallBytes {
			return nil, fmt.Errorf("%w: result integer exceeds byte limit", ErrContract)
		}
		return json.Number(text), nil
	case star.Float:
		floating := float64(value)
		if math.IsInf(floating, 0) || math.IsNaN(floating) {
			return nil, fmt.Errorf("%w: result float must be finite", ErrContract)
		}
		return floating, nil
	case *star.List:
		if value.Len() > budget.limits.MaxCollectionItems {
			return nil, fmt.Errorf("%w: result list exceeds %d items", ErrContract, budget.limits.MaxCollectionItems)
		}
		items := make([]any, value.Len())
		for index := 0; index < value.Len(); index++ {
			converted, err := fromStarlark(value.Index(index), budget, depth+1)
			if err != nil {
				return nil, err
			}
			items[index] = converted
		}
		return items, nil
	case *star.Dict:
		if value.Len() > budget.limits.MaxCollectionItems {
			return nil, fmt.Errorf("%w: result dict exceeds %d members", ErrContract, budget.limits.MaxCollectionItems)
		}
		object := make(map[string]any, value.Len())
		for _, item := range value.Items() {
			key, ok := item[0].(star.String)
			if !ok {
				return nil, fmt.Errorf("%w: result dict key has type %s, want string", ErrContract, item[0].Type())
			}
			text := string(key)
			if !utf8.ValidString(text) {
				return nil, fmt.Errorf("%w: result object key is not valid UTF-8", ErrContract)
			}
			converted, err := fromStarlark(item[1], budget, depth+1)
			if err != nil {
				return nil, err
			}
			object[text] = converted
		}
		return object, nil
	default:
		return nil, fmt.Errorf("%w: Starlark value type %s is not JSON-neutral", ErrContract, value.Type())
	}
}

func decodeResult(raw []byte, destination any) error {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("%w: result must not be null", ErrContract)
	}
	if err := jsonstrict.Decode(raw, destination); err != nil {
		return fmt.Errorf("%w: invalid result: %v", ErrContract, err)
	}
	return nil
}
