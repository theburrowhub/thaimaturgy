package engine

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/types"
)

// TestModeCommandToggles verifies /mode flips the session mode and signals the UI.
func TestModeCommandToggles(t *testing.T) {
	session := createTestSession()
	handler := NewCommandHandler(session)

	if session.State.EffectiveMode() != domain.ModeAssistant {
		t.Fatalf("default mode = %q, want assistant", session.State.EffectiveMode())
	}

	res := handler.Execute(ParseCommand("/mode"))
	if !res.NeedsUI || res.UIAction != "mode" {
		t.Errorf("expected NeedsUI mode action, got %+v", res)
	}
	if session.State.EffectiveMode() != domain.ModeVirtualDM {
		t.Errorf("after toggle mode = %q, want dm", session.State.EffectiveMode())
	}

	handler.Execute(ParseCommand("/mode oracle"))
	if session.State.EffectiveMode() != domain.ModeAssistant {
		t.Errorf("after /mode oracle = %q, want assistant", session.State.EffectiveMode())
	}

	handler.Execute(ParseCommand("/mode dm"))
	if session.State.EffectiveMode() != domain.ModeVirtualDM {
		t.Errorf("after /mode dm = %q, want dm", session.State.EffectiveMode())
	}
}

// TestPlayerToolsOnlyInDMMode verifies the PC mutation tools are exposed only in
// virtual-DM mode.
func TestPlayerToolsOnlyInDMMode(t *testing.T) {
	session := createTestSession()
	tr := NewToolRouter(session)

	has := func(name string) bool {
		for _, d := range tr.GetToolDefinitions() {
			if d.Name == name {
				return true
			}
		}
		return false
	}

	if has("update_hp") {
		t.Error("update_hp should not be exposed in assistant mode")
	}
	session.State.SetMode(domain.ModeVirtualDM)
	if !has("update_hp") || !has("add_item") || !has("award_xp") {
		t.Error("player tools should be exposed in virtual-DM mode")
	}
	// Retrieval tools remain available in both modes.
	if !has("get_room") {
		t.Error("get_room should always be available")
	}
}

// TestUpdateHPTool exercises the update_hp tool's damage/heal/set behaviour.
func TestUpdateHPTool(t *testing.T) {
	session := createTestSession()
	session.State.SetMode(domain.ModeVirtualDM)
	session.State.PC = domain.NewCharacter("Kael", "Elf", "Wizard")
	session.State.PC.MaxHP = 20
	session.State.PC.CurrentHP = 20
	tr := NewToolRouter(session)

	call := func(args map[string]any) types.ToolResult {
		b, _ := json.Marshal(args)
		return tr.Execute(types.ToolCall{Name: "update_hp", Arguments: b})
	}

	if r := call(map[string]any{"delta": -8}); r.Error != "" {
		t.Fatalf("damage failed: %s", r.Error)
	}
	if session.State.PC.CurrentHP != 12 {
		t.Errorf("HP after 8 damage = %d, want 12", session.State.PC.CurrentHP)
	}
	call(map[string]any{"delta": 3})
	if session.State.PC.CurrentHP != 15 {
		t.Errorf("HP after 3 heal = %d, want 15", session.State.PC.CurrentHP)
	}
	call(map[string]any{"set": 100}) // capped at MaxHP
	if session.State.PC.CurrentHP != 20 {
		t.Errorf("HP after set 100 = %d, want 20 (capped)", session.State.PC.CurrentHP)
	}
	call(map[string]any{"set": -10}) // clamped at 0, never negative
	if session.State.PC.CurrentHP != 0 {
		t.Errorf("HP after set -10 = %d, want 0 (clamped)", session.State.PC.CurrentHP)
	}
}

// TestUpdateGoldTool verifies the update_gold tool clamps negatives on both the
// delta and set paths.
func TestUpdateGoldTool(t *testing.T) {
	session := createTestSession()
	session.State.SetMode(domain.ModeVirtualDM)
	session.State.PC = domain.NewCharacter("Kael", "Elf", "Wizard")
	tr := NewToolRouter(session)

	call := func(args map[string]any) types.ToolResult {
		b, _ := json.Marshal(args)
		return tr.Execute(types.ToolCall{Name: "update_gold", Arguments: b})
	}

	call(map[string]any{"set": 50})
	if session.State.PC.Gold != 50 {
		t.Errorf("gold after set 50 = %d, want 50", session.State.PC.Gold)
	}
	call(map[string]any{"delta": -80}) // would go negative → clamped at 0
	if session.State.PC.Gold != 0 {
		t.Errorf("gold after -80 = %d, want 0 (clamped)", session.State.PC.Gold)
	}
	call(map[string]any{"set": -25}) // negative set → clamped at 0
	if session.State.PC.Gold != 0 {
		t.Errorf("gold after set -25 = %d, want 0 (clamped)", session.State.PC.Gold)
	}
}

// TestEnsurePC verifies the single default-character entry point.
func TestEnsurePC(t *testing.T) {
	st := domain.NewSessionState("t", nil)
	if st.PC != nil {
		t.Fatal("new session should have no PC")
	}
	if !st.EnsurePC() {
		t.Error("EnsurePC should report creation the first time")
	}
	if st.PC == nil {
		t.Fatal("EnsurePC should create a PC")
	}
	first := st.PC
	if st.EnsurePC() {
		t.Error("EnsurePC should report false when a PC already exists")
	}
	if st.PC != first {
		t.Error("EnsurePC must not replace an existing PC")
	}
}

// TestGMSystemPromptSelected verifies the oracle's base prompt switches with mode.
func TestGMSystemPromptSelected(t *testing.T) {
	session := createTestSession()
	o := NewOracle(session, nil)

	if got := o.systemPromptBase(); got != session.Config.GetSystemPrompt() {
		t.Error("assistant mode should use the configured oracle prompt")
	}

	session.State.SetMode(domain.ModeVirtualDM)
	got := o.systemPromptBase()
	if got != domain.GMSystemPrompt(session.Config.Language) {
		t.Error("virtual-DM mode should use the GM prompt")
	}
	if !strings.Contains(got, "Dungeon Master") {
		t.Error("GM prompt should cast the AI as the Dungeon Master")
	}
}
