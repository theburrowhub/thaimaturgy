package caribdus

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func buildNPCs(p *worldpack.Pack) {
	npcs := []worldpack.WorldNPC{
		{
			ID: "npc_valeria_storm", Name: "Capitán Valeria Storm", Role: "privateer captain",
			Appearance:  "Lean woman, storm-gray coat, silver earring shaped like a lightning bolt.",
			Personality: "Bold, theatrical, hates Crown hypocrisy.",
			Motivations: "Build a fleet that answers to no flag but her own.",
			Secrets:     "Owes Marisela a blood debt from a saved crew.",
			Voice:       "Laughs mid-sentence; calls everyone 'mate' until they earn a name.",
			Knowledge:   []string{"Reef passages", "Corsair Council votes", "Storm omens"},
			SampleDialogue: []string{
				"I don't sail for kings or coffers — I sail because the sea asked nicely.",
				"Puerto Sombrío's customs man takes bribes in pearls. I take his pride for free.",
			},
			Disposition: "chaotic good", FactionID: "free_corsair_council", DefaultLocation: "taberna_ancla_podrida",
			StatBlock: ptrStatBlock(captainStatBlock()),
			Tags:      []string{"captain", "wild_card"},
		},
		{
			ID: "npc_marisela_bruma", Name: "Marisela la Bruma", Role: "sea witch",
			Appearance:  "Salt-white hair; kelp bracelets; eyes like tide pools.",
			Personality: "Patient, ominous, never wastes a word.",
			Motivations: "Bind the trench's waking leviathan back to sleep.",
			Secrets:     "Sold calm weather to the Crown once; regrets it.",
			Voice:       "Speaks as if every sentence is a prophecy.",
			Knowledge:   []string{"Ghost Shoals paths", "Curse breaking", "Deluge glyphs"},
			SampleDialogue: []string{
				"The sea remembers every oath you break. I merely read the minutes.",
				"Bring salt, blood, and silence. Questions come after.",
			},
			Disposition: "neutral", FactionID: "coven_tides", DefaultLocation: "choza_bruja_mar",
			StatBlock: ptrStatBlock(seaWitchStatBlock()),
			Tags:      []string{"sea-witch", "spellcaster"},
		},
		{
			ID: "npc_mateo_almirez", Name: "Gobernador Mateo Almirez", Role: "colonial governor",
			Appearance:  "Iron-gray uniform, gout limp, signet ring of the Crown.",
			Personality: "Polished cruelty; believes order is mercy.",
			Motivations: "Hang enough pirates to make an example; fill the treasury.",
			Secrets:     "Secretly trades prisoners to the coven for storm warnings.",
			Voice:       "Soft Spanish cadence; threats wrapped in etiquette.",
			Disposition: "lawful evil", FactionID: "crown_armada", DefaultLocation: "palacio_gobernador",
			Tags: []string{"noble", "authority"},
		},
		{
			ID: "npc_garfio_reyes", Name: "Garfio Reyes", Role: "smuggler",
			Appearance:  "Hook-hand prosthetic (iron, not silver); shark-tooth necklace.",
			Personality: "Grins through danger; loyal to coin.",
			Motivations: "Control the Sombrío smuggling routes.",
			Secrets:     "Informant for Valeria Storm when Crown tariffs rise.",
			Voice:       "Whispers like a knife sliding from a sheath.",
			StatBlock:   ptrStatBlock(pirateStatBlock()),
			Disposition: "neutral", FactionID: "free_corsair_council", DefaultLocation: "cala_contrabandistas",
			Tags: []string{"criminal", "smuggler"},
		},
		{
			ID: "npc_toro_contramaestre", Name: "Contramaestre Toro", Role: "shipwright",
			Appearance:      "Massive shoulders, tar-stained beard, voice like a capstan.",
			Personality:     "Blunt, honest, drinks too much.",
			Motivations:     "Finish the Blackfin sloop before hurricane season.",
			DefaultLocation: "astillero_negro",
			Tags:            []string{"craft", "commoner"},
		},
		{
			ID: "npc_coral_priestess", Name: "Sacerdotisa Coral", Role: "tide priestess",
			Appearance:      "Coral rosary; bare feet; algae-green shawl.",
			Personality:     "Gentle in public; steel when rites are mocked.",
			Motivations:     "Keep Perla Azul neutral in the Crown–Corsair war.",
			Knowledge:       []string{"Tide omens", "Pearl diving rites", "Coven politics (partial)"},
			DefaultLocation: "templo_mareas", FactionID: "coven_tides",
			Tags: []string{"holy", "sea-witch"},
		},
		{
			ID: "npc_almirante_ribera", Name: "Almirante Ribera", Role: "naval commander",
			Appearance:  "Braid-heavy uniform; powder burns on left cheek.",
			Personality: "By-the-book; despises pirates and witchcraft equally.",
			Motivations: "Sink Valeria Storm's squadron before the next moon.",
			StatBlock:   ptrStatBlock(marineOfficerStatBlock()),
			Disposition: "lawful", FactionID: "crown_armada", DefaultLocation: "cuartel_almirez",
			Tags: []string{"military", "naval"},
		},
		{
			ID: "npc_cuervo_salazar", Name: "Cuervo Salazar", Role: "pirate captain",
			Appearance:  "Raven-feather tricorn; pistol bandolier.",
			Personality: "Mocking, reckless, superstitious about crows.",
			Motivations: "Claim Puerto Sombrío's docks for the Council.",
			StatBlock:   ptrStatBlock(captainStatBlock()),
			FactionID:   "free_corsair_council", DefaultLocation: "taberna_ancla_podrida",
			Tags: []string{"pirate", "villain"},
		},
		{
			ID: "npc_paco_tabernero", Name: "Paco el Tabernero", Role: "innkeeper",
			Appearance:      "Barrel-chested; apron; missing two fingers.",
			Personality:     "Fatherly until you break house rules.",
			Motivations:     "Keep the Ancla Podrida neutral ground.",
			SampleDialogue:  []string{"No blades drawn inside — that's what the alley's for."},
			DefaultLocation: "taberna_ancla_podrida",
			Tags:            []string{"tavern", "informant"},
		},
		{
			ID: "npc_cartografo_mir", Name: "Tomás Mir", Role: "cartographer",
			Appearance:      "Ink-stained fingers; brass dividers on a chain.",
			Personality:     "Obsessive about accuracy.",
			Motivations:     "Chart the trench without Crown censorship.",
			Knowledge:       []string{"Hidden reefs", "Ghost Shoals drift", "Trench depth soundings"},
			DefaultLocation: "malecon_faro",
			Tags:            []string{"scholar", "navigation"},
		},
		{
			ID: "npc_isla_marinera", Name: "Isla Cortavientos", Role: "deckhand",
			Appearance:      "Teen sailor; shaved head; knife in boot.",
			Personality:     "Quick, curious, fearless to a fault.",
			Motivations:     "Earn a berth on Valeria's crew.",
			DefaultLocation: "taberna_ancla_podrida",
			Tags:            []string{"sailor", "commoner"},
		},
		{
			ID: "npc_envoy_sombrio", Name: "Envoy Lucía Mendoza", Role: "customs official",
			Appearance:      "Crown livery; spectacles; ledger always open.",
			Personality:     "Corrupt but polite.",
			Motivations:     "Maximize bribes while appearing loyal to Almirez.",
			Secrets:         "Keeps a list of smugglers for blackmail.",
			DefaultLocation: "aduana_sombrio", FactionID: "crown_armada",
			Tags: []string{"civic", "colonial"},
		},
	}
	for _, npc := range npcs {
		worldpack.AddNPC(p, npc)
	}
}
