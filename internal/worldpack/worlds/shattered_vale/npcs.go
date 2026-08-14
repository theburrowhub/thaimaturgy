package shattered_vale

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func buildNPCs(p *worldpack.Pack) {
	guardSB := mustSRD("guard")
	banditSB := mustSRD("bandit")

	npcs := []worldpack.WorldNPC{
		{
			ID: "npc_eldric_vane", Name: "Mayor Eldric Vane", Role: "quest giver",
			Appearance:  "Silver-haired human in League regalia; signet ring taps the table when thinking.",
			Personality: "Measured, pragmatic, allergic to chaos.",
			Motivations: "Keep Millhaven prosperous and independent from Ironhold.",
			Secrets:     "Secretly funds Dawn expeditions into Undercrypt to appease the temple bloc.",
			Voice:       "Dry baritone, legal metaphors.",
			Knowledge:   []string{"League tariff schedules", "Council votes", "Smuggler routes (partial)"},
			SampleDialogue: []string{
				"The League sells stability by the bushel — don't confuse price with virtue.",
				"If you bring me Cassian's signet, I'll waive your dock fees for a tenday.",
			},
			Disposition: "neutral", FactionID: "merchants_league", DefaultLocation: "millhaven_town_hall",
			Tags:         []string{"civic", "noble"},
			ToolBindings: []worldpack.NPCToolBinding{{ToolID: "get_npc", Parameters: map[string]any{"npc_id": "npc_eldric_vane"}}},
		},
		{
			ID: "npc_mira_thorne", Name: "Captain Mira Thorne", Role: "authority",
			Appearance:  "Half-elf with scarred cheek; town guard tabard always crisp.",
			Personality: "Direct, fair, zero tolerance for Red Hand graffiti.",
			Motivations: "Protect Millhaven without becoming the League's private army.",
			Secrets:     "Has a informant in Cutpurse Alley (Sable Quinn).",
			Voice:       "Clipped commands; softens slightly for recruits.",
			StatBlock:   &guardSB,
			Disposition: "lawful", FactionID: "order_of_dawn", DefaultLocation: "millhaven_barracks",
			Tags: []string{"military", "guard"},
		},
		{
			ID: "npc_tomas_gull", Name: "Tomas Gull", Role: "informant",
			Appearance:     "Broad-shouldered human; apron and anchor tattoo.",
			Personality:    "Jovial surface, sharp ears.",
			Motivations:    "Keep the Gilded Anchor neutral ground.",
			Knowledge:      []string{"Dock schedules", "Which captains take bribes", "Undercroft entrances"},
			SampleDialogue: []string{"Ale first, questions second — house rule."},
			Disposition:    "friendly", DefaultLocation: "the_gilded_anchor",
			Tags: []string{"tavern", "commoner"},
		},
		{
			ID: "npc_lyra_dawn", Name: "Priestess Lyra Dawn", Role: "healer",
			Appearance:  "Human woman in white and gold; dawn-symbol tattoo on wrist.",
			Personality: "Compassionate but steel when undead are involved.",
			Motivations: "Close Undercrypt breaches before a death knight rises.",
			StatBlock:   priestessStatBlock(),
			Disposition: "good", FactionID: "order_of_dawn", DefaultLocation: "temple_of_dawn",
			Tags: []string{"cleric", "holy"},
		},
		{
			ID: "npc_brick_holt", Name: "Sergeant Brick Holt", Role: "trainer",
			Appearance:      "Stocky dwarf with braided beard and dented helm.",
			Personality:     "Gruff mentor energy.",
			Motivations:     "Turn green guards into soldiers.",
			StatBlock:       &guardSB,
			DefaultLocation: "millhaven_barracks",
			Tags:            []string{"military"},
		},
		{
			ID: "npc_fenn_reed", Name: "Dockmaster Fenn Reed", Role: "merchant",
			Appearance:      "Sun-leathered human with rope-calloused hands.",
			Personality:     "Everything has a price, including silence.",
			Motivations:     "Maximize dock fees; minimize League audits.",
			DefaultLocation: "river_docks", FactionID: "merchants_league",
			Tags: []string{"trade", "dock"},
		},
		{
			ID: "npc_sable_quinn", Name: "Sable Quinn", Role: "fence",
			Appearance:  "Hooded half-elf with ink-stained fingers.",
			Personality: "Wry, never surprised.",
			Motivations: "Profit without open war with Captain Thorne.",
			Secrets:     "Red Hand lieutenant but plays both sides.",
			StatBlock:   &banditSB,
			Disposition: "neutral evil", FactionID: "red_hand", DefaultLocation: "cutpurse_alley",
			Tags: []string{"criminal", "rogue"},
		},
		{
			ID: "npc_gareth_ironhold", Name: "Warden Gareth", Role: "authority",
			Appearance:      "Human veteran with iron-gray hair and siege-burn scars.",
			Personality:     "Soldier first, politician never.",
			Motivations:     "Hold the pass; feed the empire's forges.",
			StatBlock:       wardenStatBlock(),
			DefaultLocation: "ironhold_keep",
			Tags:            []string{"military", "noble"},
		},
		{
			ID: "npc_helga_stone", Name: "Helga Stone", Role: "merchant",
			Appearance:      "Muscular dwarf woman; burn scars on forearms.",
			Personality:     "Proud craftswoman; hates shoddy steel.",
			Motivations:     "Forge a blade worthy of legend.",
			DefaultLocation: "ironhold_smithy",
			Tags:            []string{"craft", "commoner"},
		},
		{
			ID: "npc_jessa_marrow", Name: "Scout-Captain Jessa Marrow", Role: "guide",
			Appearance:      "Lean human in mottled cloak; shortbow always strung.",
			Personality:     "Dry humor; trusts maps more than people.",
			Motivations:     "Keep Thornwall alive through the next winter.",
			StatBlock:       scoutStatBlock(),
			DefaultLocation: "thornwall_gatehouse",
			Tags:            []string{"ranger", "frontier"},
		},
		{
			ID: "npc_alden_cross", Name: "Merchant Prince Alden Cross", Role: "merchant",
			Appearance:  "Opulent robes; rings on every finger.",
			Personality: "Charming predator.",
			Motivations: "Corner the grain market.",
			FactionID:   "merchants_league", DefaultLocation: "millhaven_market",
			Tags: []string{"trade", "noble"},
		},
		{
			ID: "npc_cassian_red", Name: "Cassian of the Red Hand", Role: "villain",
			Appearance:  "Crimson cloak; hand brand visible on neck.",
			Personality: "Charismatic bandit king.",
			Motivations: "Control the moor road and ransom Thornwall.",
			StatBlock:   banditLordStatBlock(),
			Secrets:     "Hides at Caer Mor ruins between raids.",
			FactionID:   "red_hand", DefaultLocation: "northern_marches_ruins",
			Tags: []string{"villain", "bandit"},
		},
		{
			ID: "npc_nim_willow", Name: "Nim Willow", Role: "healer",
			Appearance:      "Gnome with moss-green hair and satchel of herbs.",
			Personality:     "Talks to plants; ignores social rank.",
			Motivations:     "Protect Whisperwood from logging expeditions.",
			DefaultLocation: "whisperwood_grove",
			Tags:            []string{"druid", "herbalist"},
		},
		{
			ID: "npc_durn_kettle", Name: "Durn Kettle", Role: "enforcer",
			Appearance:  "Orc with League badge he didn't earn.",
			Personality: "Bullying coward when alone.",
			StatBlock:   mustSRDPtr("orc"),
			FactionID:   "merchants_league", DefaultLocation: "millhaven_market",
			Tags: []string{"thug"},
		},
		{
			ID: "npc_mortis", Name: "Brother Mortis", Role: "cultist",
			Appearance:      "Hollow-eyed acolyte in crown sigil robes.",
			Personality:     "Whispers; believes undeath is mercy.",
			Motivations:     "Awaken the Hollow Crown regent.",
			DefaultLocation: "undercrypt_entrance",
			Tags:            []string{"cult", "villain"},
		},
		{
			ID: "npc_old_pel", Name: "Old Pel", Role: "hermit",
			Appearance:      "Bent human with cataract eyes; smells of fish oil.",
			Personality:     "Rambling storyteller.",
			Knowledge:       []string{"Tide charts", "Smuggler signals", "Sea cave layout"},
			DefaultLocation: "sunlit_lighthouse",
			Tags:            []string{"commoner", "guide"},
		},
	}
	for _, n := range npcs {
		worldpack.AddNPC(p, n)
	}
}
