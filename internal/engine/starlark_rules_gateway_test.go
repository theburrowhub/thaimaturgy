package engine

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/catalog"
	"github.com/theburrowhub/thaimaturgy/internal/rules/dnd5e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/starlarkruntime"
	"github.com/theburrowhub/thaimaturgy/internal/types"
)

func starlarkDiceBundle(t *testing.T) []byte {
	t.Helper()
	manifest := rules.Manifest{
		ID: "test.starlark-d6", Name: "Starlark d6", Version: "1.0.0", ProtocolVersion: rules.ProtocolVersion,
		Runtime: rules.Runtime{Kind: rules.RuntimeStarlark, Entrypoint: "main.star"}, Capabilities: []string{"check.roll"},
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	source := fmt.Sprintf(`def manifest():
    return %s

def initial_state():
    return {}

def list_actions(request):
    return [{
        "id": "check.roll",
        "label": "Roll d6",
        "input_schema": {"type": "object", "additionalProperties": False},
    }]

def start(request):
    return {
        "id": request["intent"]["id"],
        "kind": "need_random",
        "continuation": {"schema_version": 1},
        "need_random": {
            "method": "dice.roll",
            "specification": {"count": 1, "sides": 6},
        },
    }

def resume(request):
    return {
        "id": request["pending"]["step_id"],
        "kind": "complete",
        "complete": {
            "outcome": "test.starlark-d6.rolled",
            "result": {"roll": request["response"]["data"]["rolls"][0]},
        },
    }

def project(request):
    return {"view": request["snapshot"]["state"]}

def explain(request):
    return {"text": "Roll one six-sided die."}

def validate_state(request):
    return None

def reduce(request):
    return {"state": request["snapshot"]["state"]}

def migrate(request):
    return {"state": request["state"]}
`, manifestJSON)
	var bundle bytes.Buffer
	writer := zip.NewWriter(&bundle)
	for name, contents := range map[string][]byte{
		starlarkruntime.ManifestPath: manifestJSON,
		"main.star":                  []byte(source),
	} {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0o600)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(contents); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return bundle.Bytes()
}

func TestStableGameGatewayExecutesStarlarkDicePackage(t *testing.T) {
	loader, err := starlarkruntime.NewLoader(starlarkruntime.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := loader.Load(context.Background(), bytes.NewReader(starlarkDiceBundle(t)))
	if err != nil {
		t.Fatal(err)
	}
	available := catalog.New()
	if err := available.Register(context.Background(), loaded.Artifact, loaded.Ruleset, loaded.InitialState); err != nil {
		t.Fatal(err)
	}
	state := domain.NewSessionState("starlark-d6", nil)
	if _, err := state.BindRules(loaded.Artifact.Lock(), loaded.InitialState); err != nil {
		t.Fatal(err)
	}
	session := domain.NewSession(state, &domain.Adventure{System: "external"}, domain.DefaultConfig())
	session.RulesResolver = available
	router := NewToolRouter(session)
	if router.rules == nil || router.rulesErr != nil {
		t.Fatalf("gateway=%v error=%v", router.rules, router.rulesErr)
	}
	router.rules.resolveDice = deterministicDice(t, dnd5e.DiceRandomRequest{Count: 1, Sides: 6}, 6)
	result := router.Execute(types.ToolCall{
		ID: "starlark:d6", Name: "game_submit_intent",
		Arguments: json.RawMessage(`{"action_id":"check.roll","arguments":{}}`),
	})
	if result.Error != "" {
		t.Fatal(result.Error)
	}
	var envelope struct {
		Outcome string `json:"outcome"`
		Data    struct {
			Roll int `json:"roll"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(result.Content), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Outcome != "test.starlark-d6.rolled" || envelope.Data.Roll != 6 {
		t.Fatalf("result = %+v", envelope)
	}
	runtime, ok := session.State.RulesRuntimeSnapshot()
	if !ok || len(runtime.RandomDraws) != 1 || runtime.RandomDraws[0].Method != "dice.roll" {
		t.Fatalf("runtime: ok=%v value=%+v", ok, runtime)
	}
	if hasTool(router.GetToolDefinitions(), "roll_dice") {
		t.Fatal("external Starlark package advertised a D&D-only alias")
	}
}
