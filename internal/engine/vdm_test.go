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

// soloParty puts a single named character in the party and returns it.
func soloParty(session *domain.Session, c *domain.Character) *domain.Character {
	session.State.Characters = []*domain.Character{c}
	return c
}

// TestUpdateHPTool exercises the update_hp tool's damage/heal/set behaviour.
func TestUpdateHPTool(t *testing.T) {
	session := createTestSession()
	session.State.SetMode(domain.ModeVirtualDM)
	pc := domain.NewCharacter("Kael", "Elf", "Wizard")
	pc.MaxHP, pc.CurrentHP = 20, 20
	soloParty(session, pc)
	tr := NewToolRouter(session)

	call := func(args map[string]any) types.ToolResult {
		b, _ := json.Marshal(args)
		return tr.Execute(types.ToolCall{Name: "update_hp", Arguments: b})
	}

	if r := call(map[string]any{"delta": -8}); r.Error != "" {
		t.Fatalf("damage failed: %s", r.Error)
	}
	if pc.CurrentHP != 12 {
		t.Errorf("HP after 8 damage = %d, want 12", pc.CurrentHP)
	}
	call(map[string]any{"delta": 3})
	if pc.CurrentHP != 15 {
		t.Errorf("HP after 3 heal = %d, want 15", pc.CurrentHP)
	}
	call(map[string]any{"set": 100}) // capped at MaxHP
	if pc.CurrentHP != 20 {
		t.Errorf("HP after set 100 = %d, want 20 (capped)", pc.CurrentHP)
	}
	call(map[string]any{"set": -10}) // clamped at 0, never negative
	if pc.CurrentHP != 0 {
		t.Errorf("HP after set -10 = %d, want 0 (clamped)", pc.CurrentHP)
	}
}

// TestUpdateGoldTool verifies the update_gold tool clamps negatives on both the
// delta and set paths.
func TestUpdateGoldTool(t *testing.T) {
	session := createTestSession()
	session.State.SetMode(domain.ModeVirtualDM)
	pc := soloParty(session, domain.NewCharacter("Kael", "Elf", "Wizard"))
	tr := NewToolRouter(session)

	call := func(args map[string]any) types.ToolResult {
		b, _ := json.Marshal(args)
		return tr.Execute(types.ToolCall{Name: "update_gold", Arguments: b})
	}

	call(map[string]any{"set": 50})
	if pc.Gold != 50 {
		t.Errorf("gold after set 50 = %d, want 50", pc.Gold)
	}
	call(map[string]any{"delta": -80}) // would go negative → clamped at 0
	if pc.Gold != 0 {
		t.Errorf("gold after -80 = %d, want 0 (clamped)", pc.Gold)
	}
	call(map[string]any{"set": -25}) // negative set → clamped at 0
	if pc.Gold != 0 {
		t.Errorf("gold after set -25 = %d, want 0 (clamped)", pc.Gold)
	}
}

// TestAwardXPTool verifies award_xp accumulates and ignores non-positive amounts.
func TestAwardXPTool(t *testing.T) {
	session := createTestSession()
	session.State.SetMode(domain.ModeVirtualDM)
	pc := soloParty(session, domain.NewCharacter("Kael", "Elf", "Wizard"))
	tr := NewToolRouter(session)

	call := func(amount int) types.ToolResult {
		b, _ := json.Marshal(map[string]any{"amount": amount})
		return tr.Execute(types.ToolCall{Name: "award_xp", Arguments: b})
	}

	call(100)
	if pc.XP != 100 {
		t.Errorf("XP after +100 = %d, want 100", pc.XP)
	}
	call(-500) // ignored, never reduces the total
	if pc.XP != 100 {
		t.Errorf("XP after -500 = %d, want 100 (non-positive ignored)", pc.XP)
	}
}

// TestPartyTargeting verifies a tool targets the named party member and errors
// helpfully when the name is missing/ambiguous or unknown.
func TestPartyTargeting(t *testing.T) {
	session := createTestSession()
	session.State.SetMode(domain.ModeVirtualDM)
	a := domain.NewCharacter("Alden", "Human", "Fighter")
	b := domain.NewCharacter("Naivara", "Elf", "Wizard")
	a.MaxHP, a.CurrentHP, b.MaxHP, b.CurrentHP = 20, 20, 12, 12
	session.State.Characters = []*domain.Character{a, b}
	tr := NewToolRouter(session)

	call := func(args map[string]any) types.ToolResult {
		bs, _ := json.Marshal(args)
		return tr.Execute(types.ToolCall{Name: "update_hp", Arguments: bs})
	}

	if r := call(map[string]any{"character": "Naivara", "delta": -5}); r.Error != "" {
		t.Fatalf("targeting Naivara failed: %s", r.Error)
	}
	if b.CurrentHP != 7 || a.CurrentHP != 20 {
		t.Errorf("only Naivara should take damage: Alden=%d Naivara=%d", a.CurrentHP, b.CurrentHP)
	}
	if r := call(map[string]any{"delta": -1}); r.Error == "" {
		t.Error("omitting 'character' with a multi-member party should error")
	}
	if r := call(map[string]any{"character": "Nobody", "delta": -1}); r.Error == "" {
		t.Error("unknown character should error")
	}
}

// TestEnsureParty verifies the default-party entry point and legacy PC migration.
func TestEnsureParty(t *testing.T) {
	st := domain.NewSessionState("t", nil)
	if len(st.Characters) != 0 {
		t.Fatal("new session should have no party")
	}
	if !st.EnsureParty() {
		t.Error("EnsureParty should report creation the first time")
	}
	if len(st.Characters) < 2 {
		t.Fatalf("default party should be heterogeneous (got %d)", len(st.Characters))
	}
	if st.EnsureParty() {
		t.Error("EnsureParty should report false when a party already exists")
	}

	// Legacy PC migrates into the party rather than being dropped.
	st2 := domain.NewSessionState("t2", nil)
	st2.PC = domain.NewCharacter("Legacy", "Human", "Fighter")
	if st2.EnsureParty() {
		t.Error("EnsureParty should not report creation when migrating a legacy PC")
	}
	if len(st2.Characters) != 1 || st2.Characters[0].Name != "Legacy" || st2.PC != nil {
		t.Errorf("legacy PC should migrate into the party; got %+v pc=%v", st2.Characters, st2.PC)
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
