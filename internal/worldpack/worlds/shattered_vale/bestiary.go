package shattered_vale

import (
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/srd"
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func buildBestiary(p *worldpack.Pack) {
	srdCreatures := []struct {
		id, name, notes, lore string
		habitats, tags        []string
	}{
		{"creature_goblin", "goblin", "Ambush in packs of 2d4; use Nimble Escape to disengage into underbrush.", "Whisperwood goblins trade with fences in Millhaven.", []string{"forest", "underground", "urban"}, []string{"cr:1/4", "humanoid"}},
		{"creature_kobold", "kobold", "Trap-makers; 1d6 with sling focus fire on spellcasters.", "Kobolds nest in Ironspine mine scrap tunnels.", []string{"mountain", "underground"}, []string{"cr:1/8", "dragonkin"}},
		{"creature_wolf", "wolf", "Pairs patrol Whisperwood trails; pack of 4 near Moonlit Grove.", "Wolves avoid Ironhold ballistae range.", []string{"forest", "grassland"}, []string{"cr:1/4", "beast"}},
		{"creature_giant_rat", "giant rat", "Swarms in shipwreck cove and Undercrypt side tunnels.", "Carries filth fever on crit in home games if desired.", []string{"urban", "coast", "underground"}, []string{"cr:1/8", "beast"}},
		{"creature_bandit", "bandit", "Red Hand standard patrol; demand toll or fight.", "Often led by a thug (use bandit stats with +5 HP).", []string{"road", "grassland", "urban"}, []string{"cr:1/8", "humanoid"}},
		{"creature_guard", "guard", "Town guard stats for Millhaven and Ironhold soldiers.", "Will call reinforcements if outnumbered.", []string{"urban", "fortress"}, []string{"cr:1/8", "humanoid"}},
		{"creature_commoner", "commoner", "Citizens, farmers, dockhands — social encounters.", "Not combatants unless desperate.", []string{"urban", "grassland", "coast"}, []string{"cr:0", "humanoid"}},
		{"creature_skeleton", "skeleton", "Caer Mor ruins at night; 1d6 rise from rubble.", "Vulnerable to bludgeoning — hint via Religion DC 12.", []string{"ruins", "underground", "dungeon"}, []string{"cr:1/4", "undead"}},
		{"creature_zombie", "zombie", "Shambling near Undercrypt entrance; Undead Fortitude surprise.", "Often precedes ghoul packs.", []string{"underground", "ruins", "dungeon"}, []string{"cr:1/4", "undead"}},
		{"creature_ghoul", "ghoul", "Pack of 2d4 in Chamber of Bones; paralyze opens PCs to focus fire.", "Avoid elves — immunity to paralyze.", []string{"underground", "dungeon", "ruins"}, []string{"cr:1", "undead"}},
		{"creature_orc", "orc", "Ironspine raiding parties; Aggressive closes distance fast.", "Sometimes hire out as mercenaries in Thornwall.", []string{"mountain", "grassland"}, []string{"cr:1/2", "humanoid"}},
		{"creature_hobgoblin", "hobgoblin", "Disciplined squads of 4 with longbow volley then melee.", "Martial Advantage punishes clustered PCs.", []string{"mountain", "fortress"}, []string{"cr:1/2", "humanoid"}},
		{"creature_bugbear", "bugbear", "Whisperwood ambush elite; Surprise Attack alpha strike.", "Often paired with goblin scouts.", []string{"forest", "underground"}, []string{"cr:1", "humanoid"}},
		{"creature_ogre", "ogre", "Blocks Ironspine pass alone; dumb but brutal.", "May be bribed with food (Persuasion DC 15).", []string{"mountain", "road"}, []string{"cr:2", "giant"}},
		{"creature_giant_spider", "giant spider", "Webs across Undercrypt antechambers; reuse Stealth +7.", "Web Sense makes bypassing webs tricky.", []string{"forest", "underground", "dungeon"}, []string{"cr:1", "beast"}},
	}
	for _, c := range srdCreatures {
		worldpack.AddCreatureFromSRD(p, c.id, c.name, c.habitats, c.notes, c.lore, c.tags...)
	}
	// Verify all 17 SRD names present — add any missing from srd.Names()
	seen := map[string]bool{}
	for _, cr := range p.Creatures {
		seen[cr.SRDName] = true
	}
	for _, name := range srd.Names() {
		if !seen[name] {
			id := "creature_" + strings.ReplaceAll(name, " ", "_")
			worldpack.AddCreatureFromSRD(p, id, name,
				[]string{"wilderness"}, "Generic wilderness encounter.", "Standard SRD creature.", "srd")
		}
	}
}

func mustSRD(name string) domain.StatBlock {
	sb, ok := srd.Lookup(name)
	if !ok {
		return domain.StatBlock{}
	}
	return sb
}

func mustSRDPtr(name string) *domain.StatBlock {
	sb := mustSRD(name)
	return &sb
}

func priestessStatBlock() *domain.StatBlock {
	return &domain.StatBlock{
		AC: 15, MaxHP: 27, Speed: "30 ft.",
		CR: "2", ProfBonus: 2,
		Skills: []string{"Medicine +4", "Religion +4"},
	}
}

func wardenStatBlock() *domain.StatBlock {
	sb := mustSRD("guard")
	sb.MaxHP = 58
	sb.CR = "3"
	return &sb
}

func scoutStatBlock() *domain.StatBlock {
	sb := mustSRD("bandit")
	sb.Skills = append(sb.Skills, "Survival +4", "Stealth +4", "Perception +3")
	sb.CR = "1/2"
	return &sb
}

func banditLordStatBlock() *domain.StatBlock {
	sb := mustSRD("bandit")
	sb.MaxHP = 45
	sb.CR = "2"
	return &sb
}
