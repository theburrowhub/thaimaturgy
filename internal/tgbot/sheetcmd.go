package tgbot

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

const (
	// maxSheetDelta bounds an HP/gold/XP amount so the subsequent arithmetic on the
	// character can never overflow (a player can't wrap HP negative with a huge
	// heal). maxItemQty bounds a single inventory quantity for the same reason.
	maxSheetDelta = 1_000_000
	maxItemQty    = 10_000
)

// This file implements editing a player character's sheet from Telegram (#31).
// Every command edits ONLY the sender's claimed character (a player can never
// touch another player's sheet — the host adjusts any sheet from the app or via
// the DM tools), applies the change under the session lock (normalizing the sheet
// afterwards), records it in the timeline as a LogParty entry so the DM sees it,
// and persists.

// knownConditions maps a lower-cased condition name to its canonical form, so a
// player can type "/condition poisoned" and get the standard "Poisoned".
var knownConditions = map[string]domain.Condition{
	"blinded": domain.ConditionBlinded, "charmed": domain.ConditionCharmed,
	"deafened": domain.ConditionDeafened, "exhausted": domain.ConditionExhausted,
	"frightened": domain.ConditionFrightened, "grappled": domain.ConditionGrappled,
	"incapacitated": domain.ConditionIncapacitated, "invisible": domain.ConditionInvisible,
	"paralyzed": domain.ConditionParalyzed, "petrified": domain.ConditionPetrified,
	"poisoned": domain.ConditionPoisoned, "prone": domain.ConditionProne,
	"restrained": domain.ConditionRestrained, "stunned": domain.ConditionStunned,
	"unconscious": domain.ConditionUnconscious,
}

// canonicalCondition resolves free-form input to a known 5e condition, reporting
// whether it matched.
func canonicalCondition(s string) (domain.Condition, bool) {
	c, ok := knownConditions[strings.ToLower(strings.TrimSpace(s))]
	return c, ok
}

// withOwnCharacter runs fn against the sender's claimed character under the
// session lock and returns the character name. It enforces the guards — a
// character must be claimed, and no edit may race an in-flight /dm resolution —
// replying and returning ok=false when one fails. The Character mutators used by
// these commands each clamp to valid ranges (HP within [0, MaxHP], gold ≥ 0, …),
// so no separate normalization pass is needed.
func (b *Bot) withOwnCharacter(m *tgbotapi.Message, fn func(*domain.Character)) (string, bool) {
	playerID := strconv.FormatInt(m.From.ID, 10)
	char := b.session.State.PlayerCharacterName(playerID)
	if char == "" {
		b.reply(m, "Pick a character first with /pick <name>, then you can edit your sheet.")
		return "", false
	}
	// A mutation applied while the DM's turn snapshot is open could be lost when
	// that snapshot is merged back, so serialize against it (mirrors /rest).
	if b.isResolving() {
		b.reply(m, "The DM is resolving the round — try again in a moment.")
		return "", false
	}
	name, ok := b.session.State.MutateCharacter(char, fn)
	if !ok {
		b.reply(m, "Character not found: "+char)
		return "", false
	}
	return name, true
}

// recordSheetChange logs a sheet edit to the timeline (visible to the DM and in
// /log), persists it, and replies. If persistence fails the reply says so
// explicitly (the mutation is already in memory but was NOT written to disk), so
// a full/unwritable disk never produces a false success.
func (b *Bot) recordSheetChange(m *tgbotapi.Message, name, desc, reply string) {
	b.session.State.AppendLog(domain.LogEntry{Type: domain.LogParty, Message: name + " " + desc})
	b.saveMu.Lock()
	err := b.store.SaveSession(b.session.State)
	b.saveMu.Unlock()
	if err != nil {
		log.Printf("save: %v", err)
		b.reply(m, "⚠ "+name+" "+desc+" — applied in memory but NOT saved to disk ("+err.Error()+"). It may be lost on restart.")
		return
	}
	b.event(name + " " + desc)
	b.reply(m, reply)
}

// deltaInRange reports whether an HP/gold/XP amount is within safe bounds.
func deltaInRange(n int) bool { return n >= -maxSheetDelta && n <= maxSheetDelta }

// parseDelta parses an amount that may be a set ("=10") or a signed delta
// ("+3", "-5", "7"), returning the value and whether it was a set.
func parseDelta(arg string) (n int, set bool, err error) {
	arg = strings.TrimSpace(arg)
	if strings.HasPrefix(arg, "=") {
		set = true
		arg = strings.TrimSpace(arg[1:])
	}
	n, err = strconv.Atoi(arg)
	return n, set, err
}

func (b *Bot) editHP(m *tgbotapi.Message, arg string) {
	n, set, err := parseDelta(arg)
	if strings.TrimSpace(arg) == "" || err != nil {
		b.reply(m, "Usage: /hp -5 (damage), /hp +3 (heal), /hp =10 (set current HP)")
		return
	}
	if !deltaInRange(n) {
		b.reply(m, fmt.Sprintf("HP amount out of range (use at most ±%d).", maxSheetDelta))
		return
	}
	var desc string
	name, ok := b.withOwnCharacter(m, func(c *domain.Character) {
		switch {
		case set:
			c.SetHP(n)
			desc = fmt.Sprintf("HP set to %d/%d", c.CurrentHP, c.MaxHP)
		case n < 0:
			c.TakeDamage(-n)
			desc = fmt.Sprintf("took %d damage → %d/%d HP", -n, c.CurrentHP, c.MaxHP)
		default:
			c.Heal(n)
			desc = fmt.Sprintf("healed %d → %d/%d HP", n, c.CurrentHP, c.MaxHP)
		}
	})
	if !ok {
		return
	}
	b.recordSheetChange(m, name, desc, "❤️ "+name+": "+desc)
}

func (b *Bot) editCondition(m *tgbotapi.Message, arg string, add bool) {
	cond, ok := canonicalCondition(arg)
	if !ok {
		verb := "condition"
		if !add {
			verb = "uncondition"
		}
		b.reply(m, fmt.Sprintf("Usage: /%s <name> — one of: blinded, charmed, deafened, exhausted, frightened, grappled, incapacitated, invisible, paralyzed, petrified, poisoned, prone, restrained, stunned, unconscious", verb))
		return
	}
	var desc string
	name, done := b.withOwnCharacter(m, func(c *domain.Character) {
		if add {
			c.AddCondition(cond)
			desc = "is now " + string(cond)
		} else {
			c.RemoveCondition(cond)
			desc = "is no longer " + string(cond)
		}
	})
	if !done {
		return
	}
	b.recordSheetChange(m, name, desc, "🩸 "+name+" "+desc)
}

func (b *Bot) editGold(m *tgbotapi.Message, arg string) {
	n, set, err := parseDelta(arg)
	if strings.TrimSpace(arg) == "" || err != nil {
		b.reply(m, "Usage: /gold +50, /gold -10, or /gold =100 (set)")
		return
	}
	if !deltaInRange(n) {
		b.reply(m, fmt.Sprintf("Gold amount out of range (use at most ±%d).", maxSheetDelta))
		return
	}
	var desc string
	name, ok := b.withOwnCharacter(m, func(c *domain.Character) {
		if set {
			c.SetGold(n)
		} else {
			// Clamp the pre-sum to avoid int overflow; SetGold clamps negatives.
			sum := c.Gold + n
			if (n > 0 && sum < c.Gold) || (n < 0 && sum > c.Gold) {
				sum = c.Gold // overflow guard (unreachable given deltaInRange, defensive)
			}
			c.SetGold(sum)
		}
		desc = fmt.Sprintf("gold is now %d", c.Gold)
	})
	if !ok {
		return
	}
	b.recordSheetChange(m, name, desc, "💰 "+name+": "+desc)
}

func (b *Bot) editXP(m *tgbotapi.Message, arg string) {
	n, err := strconv.Atoi(strings.TrimSpace(arg))
	if err != nil || n <= 0 || n > maxSheetDelta {
		b.reply(m, fmt.Sprintf("Usage: /xp 150 — award experience (1..%d)", maxSheetDelta))
		return
	}
	var desc string
	name, ok := b.withOwnCharacter(m, func(c *domain.Character) {
		before := c.XP
		c.AwardXP(n)
		if c.XP < before {
			c.XP = before // overflow guard (unreachable given the bound, defensive)
		}
		desc = fmt.Sprintf("gained %d XP → %d total", n, c.XP)
	})
	if !ok {
		return
	}
	b.recordSheetChange(m, name, desc, "✨ "+name+": "+desc)
}

// parseItemArg parses "add <name> [xN]" / "remove <name> [xN]" into a normalized
// action ("add" or "remove"), the item name, and quantity (default 1). ok is
// false when the syntax is unusable (missing action/name or unknown action).
func parseItemArg(arg string) (action, itemName string, qty int, ok bool) {
	fields := strings.Fields(arg)
	if len(fields) < 2 {
		return "", "", 0, false
	}
	switch strings.ToLower(fields[0]) {
	case "add":
		action = "add"
	case "remove", "rm", "drop":
		action = "remove"
	default:
		return "", "", 0, false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(arg), fields[0]))
	qty = 1
	if i := strings.LastIndex(rest, " x"); i >= 0 {
		suffix := strings.TrimSpace(rest[i+2:])
		if v, err := strconv.Atoi(suffix); err == nil {
			// A numeric "xN" suffix is a quantity: it must be positive and bounded,
			// else the whole command is rejected (e.g. "x0" must not fall through and
			// be kept as part of the item name). A non-numeric suffix (e.g.
			// "x of holding") is left as part of the name.
			if v < 1 || v > maxItemQty {
				return "", "", 0, false
			}
			qty = v
			rest = strings.TrimSpace(rest[:i])
		}
	}
	itemName = strings.TrimSpace(rest)
	if itemName == "" {
		return "", "", 0, false
	}
	return action, itemName, qty, true
}

// editItem handles "/item add <name> [xN]" and "/item remove <name> [xN]".
func (b *Bot) editItem(m *tgbotapi.Message, arg string) {
	action, itemName, qty, ok := parseItemArg(arg)
	if !ok {
		b.reply(m, "Usage: /item add <name> [xN]  |  /item remove <name> [xN]")
		return
	}
	var desc string
	var applied bool
	name, done := b.withOwnCharacter(m, func(c *domain.Character) {
		if action == "add" {
			c.AddItem(domain.InventoryItem{Name: itemName, Quantity: qty})
			desc = fmt.Sprintf("picked up %s x%d", itemName, qty)
			applied = true
			return
		}
		// Removal: report the quantity ACTUALLY removed (bounded by what's carried),
		// so the DM-facing log never claims more was dropped than existed. RemoveItem
		// matches the name exactly, so use the same match to read the current stack.
		have := 0
		for _, it := range c.Inventory {
			if it.Name == itemName {
				have = it.Quantity
				break
			}
		}
		if have == 0 {
			return
		}
		removed := qty
		if removed > have {
			removed = have
		}
		c.RemoveItem(itemName, qty)
		desc = fmt.Sprintf("dropped %s x%d", itemName, removed)
		applied = true
	})
	if !done {
		return
	}
	if !applied {
		b.reply(m, fmt.Sprintf("%s isn't carrying %q.", name, itemName))
		return
	}
	b.recordSheetChange(m, name, desc, "🎒 "+name+": "+desc)
}

// editNote sets the character's personal sheet notes (distinct from /note, which
// appends a DM-only timeline note).
func (b *Bot) editNote(m *tgbotapi.Message, arg string) {
	text := strings.TrimSpace(arg)
	if text == "" {
		b.reply(m, "Usage: /setnote <text> — set your character's notes")
		return
	}
	name, ok := b.withOwnCharacter(m, func(c *domain.Character) {
		c.Notes = text
	})
	if !ok {
		return
	}
	b.recordSheetChange(m, name, "updated their character notes", "📝 "+name+": notes updated.")
}
