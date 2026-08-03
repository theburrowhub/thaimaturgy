package domain

import "strings"

type ProviderType string
type Language string

const (
	ProviderOpenAI    ProviderType = "openai"
	ProviderAnthropic ProviderType = "anthropic"
	ProviderGemini    ProviderType = "gemini"
	// ProviderClaudeCLI runs inference through the official Claude Code CLI on the
	// user's machine (the sanctioned client), rather than calling the API directly.
	// Authentication is handled by the CLI's own login; no key/token is stored here.
	ProviderClaudeCLI ProviderType = "claude-cli"
)

const (
	LangEnglish Language = "en"
	LangSpanish Language = "es"
)

type TTSVoice string

const (
	TTSVoiceAlloy   TTSVoice = "alloy"
	TTSVoiceEcho    TTSVoice = "echo"
	TTSVoiceFable   TTSVoice = "fable"
	TTSVoiceOnyx    TTSVoice = "onyx"
	TTSVoiceNova    TTSVoice = "nova"
	TTSVoiceShimmer TTSVoice = "shimmer"
)

type TTSConfig struct {
	Enabled bool     `json:"enabled"`
	Voice   TTSVoice `json:"voice"`
	Model   string   `json:"model"`
	Speed   float64  `json:"speed"`
}

type Config struct {
	Provider    ProviderType `json:"provider"`
	Model       string       `json:"model"`
	Temperature float64      `json:"temperature"`
	Language    Language     `json:"language"`

	// Per-frontend model overrides. When non-empty they take precedence over Model
	// for the player/oracle (RunModel) and the editor/import (EditModel), so the two
	// apps can use different models — including under the claude-cli backend.
	RunModel  string `json:"run_model,omitempty"`
	EditModel string `json:"edit_model,omitempty"`

	OpenAIAPIKey    string `json:"openai_api_key,omitempty"`
	AnthropicAPIKey string `json:"anthropic_api_key,omitempty"`
	GeminiAPIKey    string `json:"gemini_api_key,omitempty"`

	// OAuth tokens reused from local logins (Claude Code, Gemini CLI). Never
	// persisted to disk.
	AnthropicOAuthToken string `json:"-"`
	GeminiOAuthToken    string `json:"-"`

	// AuthSource is a human description of how the active credential was
	// obtained (e.g. "Claude Code login"). Not persisted.
	AuthSource string `json:"-"`

	SystemPrompt string `json:"system_prompt,omitempty"`

	MaxTokens     int    `json:"max_tokens"`
	ShowScanlines bool   `json:"show_scanlines"`
	BorderStyle   string `json:"border_style"`

	DefaultSetting   string `json:"default_setting"`
	AutoSave         bool   `json:"auto_save"`
	AutoSaveInterval int    `json:"auto_save_interval"`

	// Oracle tunables.
	OracleMaxToolIterations int `json:"oracle_max_tool_iterations"`
	OracleRecentTimeline    int `json:"oracle_recent_timeline"`
	OracleSummarizeAfter    int `json:"oracle_summarize_after"`
	RequestTimeoutSeconds   int `json:"request_timeout_seconds"`

	// AI import tunables (PDF / images → module).
	ImportVisionMaxImages  int `json:"import_vision_max_images"`
	ImportVisionMaxImageMB int `json:"import_vision_max_image_mb"`
	ImportMaxDocChars      int `json:"import_max_doc_chars"`
	ImportMaxOutputTokens  int `json:"import_max_output_tokens"`

	// ImportLanguage is the language adventures are authored in during AI import,
	// independent of the source document's language (e.g. import an English PDF as
	// a Spanish module). A language name or code ("Spanish", "es", "French"). When
	// empty, import follows the UI Language.
	ImportLanguage string `json:"import_language,omitempty"`

	TTS TTSConfig `json:"tts"`
}

func DefaultConfig() *Config {
	return &Config{
		Provider:         ProviderOpenAI,
		Model:            "gpt-4o-mini",
		Temperature:      0.8,
		Language:         LangEnglish,
		MaxTokens:        2048,
		ShowScanlines:    false,
		BorderStyle:      "rounded",
		DefaultSetting:   "fantasy",
		AutoSave:         true,
		AutoSaveInterval: 300,

		OracleMaxToolIterations: 6,
		OracleRecentTimeline:    15,
		OracleSummarizeAfter:    20,
		RequestTimeoutSeconds:   90,

		ImportVisionMaxImages:  10,
		ImportVisionMaxImageMB: 4,
		ImportMaxDocChars:      200000,
		ImportMaxOutputTokens:  64000,

		TTS: TTSConfig{
			Enabled: false,
			Voice:   TTSVoiceOnyx, // Deep, dramatic voice for DM
			Model:   "tts-1",
			Speed:   1.0,
		},
	}
}

func (c *Config) GetActiveAPIKey() string {
	switch c.Provider {
	case ProviderOpenAI:
		return c.OpenAIAPIKey
	case ProviderAnthropic:
		return c.AnthropicAPIKey
	case ProviderGemini:
		return c.GeminiAPIKey
	}
	return ""
}

// IsConfigured reports whether the active provider has a usable credential,
// either an API key or a reused local OAuth token.
func (c *Config) IsConfigured() bool {
	switch c.Provider {
	case ProviderOpenAI:
		return c.OpenAIAPIKey != ""
	case ProviderAnthropic:
		return c.AnthropicAPIKey != "" || c.AnthropicOAuthToken != ""
	case ProviderGemini:
		return c.GeminiAPIKey != "" || c.GeminiOAuthToken != ""
	case ProviderClaudeCLI:
		return true // the CLI carries its own login; binary presence is checked when building the provider
	}
	return false
}

// DefaultModel returns a sensible default model id for a provider.
func DefaultModel(p ProviderType) string {
	switch p {
	case ProviderOpenAI:
		return "gpt-4o"
	case ProviderAnthropic:
		return "claude-sonnet-5"
	case ProviderGemini:
		return "gemini-2.5-flash"
	case ProviderClaudeCLI:
		return "claude-sonnet-5"
	}
	return ""
}

func (c *Config) GetSystemPrompt() string {
	if c.SystemPrompt != "" {
		return c.SystemPrompt
	}
	if c.Language == LangSpanish {
		return DefaultSystemPromptES
	}
	return DefaultSystemPromptEN
}

// LanguageName returns the English display name of a UI language.
func LanguageName(l Language) string {
	if l == LangSpanish {
		return "Spanish"
	}
	return "English"
}

// ImportLanguageName returns the human-readable target language that adventures
// are authored in during AI import. An empty ImportLanguage follows the UI
// Language; otherwise common codes/names are normalized and anything else is
// passed through verbatim (e.g. "French", "Deutsch").
func (c *Config) ImportLanguageName() string {
	s := strings.TrimSpace(c.ImportLanguage)
	if s == "" {
		return LanguageName(c.Language)
	}
	switch strings.ToLower(s) {
	case "en", "eng", "english":
		return "English"
	case "es", "spa", "spanish", "español", "espanol", "castellano":
		return "Spanish"
	}
	return s
}

// ImportLanguageCode returns a best-effort language code for the import target,
// suitable for the adventure's "language" field ("en", "es", or a lower-cased
// form of a free-text language for anything else).
func (c *Config) ImportLanguageCode() string {
	switch c.ImportLanguageName() {
	case "English":
		return "en"
	case "Spanish":
		return "es"
	default:
		return strings.ToLower(strings.TrimSpace(c.ImportLanguage))
	}
}

var DefaultSystemPromptEN = `You are an expert assistant to a human Dungeon Master who is running THIS specific, pre-authored D&D-style adventure. You are NOT the DM and you do NOT control the players. Your job is to help the DM run the adventure that is loaded.

IMPORTANT: Always respond in English.

CORE PRINCIPLES:
1. GROUND EVERY ANSWER IN THE MODULE. The adventure's canon (its zones, rooms, NPCs, events, and lore) is the source of truth. Prefer authored content over invention.
2. USE RETRIEVAL TOOLS. When you need details you don't have in context (another room, NPC, event, or item), call get_room / get_npc / get_event / get_item / search_module instead of guessing.
3. TRACK THE TABLE. When the DM tells you what the players did, record it with the session tools (set_location, mark_npc_met, trigger_event, set_flag, log_note, advance_quest, update_party_member).
4. LABEL IMPROVISATION. If the module doesn't cover something and you must improvise, clearly mark it as a SUGGESTION consistent with the tone — never present invention as canon.
5. RESPECT PLAYER AGENCY. Offer options and consequences; never dictate what the player characters do.

WHAT THE DM WANTS FROM YOU:
- What should happen here / what the module intends.
- Read-aloud (boxed) text to deliver to the players.
- Roleplay support: an NPC's voice, personality, motivations, secrets, and lines of dialogue.
- Mechanics: relevant stat blocks, DCs, and quick dice rolls (roll_dice, ability_check).
- Inspiration and consistent options when players go off-script.

STYLE:
- Be concise and scannable. Separate "read-aloud" text from "DM notes" clearly.
- Cite the ID of rooms/NPCs/events you reference so the DM can look them up.`

var DefaultSystemPromptES = `Eres un asistente experto para un Dungeon Master humano que está dirigiendo ESTA aventura concreta y ya escrita, al estilo D&D. NO eres el DM y NO controlas a los jugadores. Tu trabajo es ayudar al DM a dirigir la aventura cargada.

IMPORTANTE: Responde siempre en español.

PRINCIPIOS FUNDAMENTALES:
1. ANCLA TODA RESPUESTA EN EL MÓDULO. El canon de la aventura (sus zonas, salas, NPCs, eventos y lore) es la fuente de verdad. Prefiere el contenido escrito frente a la invención.
2. USA LAS HERRAMIENTAS DE RECUPERACIÓN. Cuando necesites detalles que no tengas en contexto (otra sala, NPC, evento u objeto), llama a get_room / get_npc / get_event / get_item / search_module en vez de suponer.
3. REGISTRA LA MESA. Cuando el DM te cuente lo que hicieron los jugadores, regístralo con las herramientas de sesión (set_location, mark_npc_met, trigger_event, set_flag, log_note, advance_quest, update_party_member).
4. ETIQUETA LA IMPROVISACIÓN. Si el módulo no cubre algo y debes improvisar, márcalo claramente como SUGERENCIA coherente con el tono; nunca presentes lo inventado como canon.
5. RESPETA LA AGENCIA DEL JUGADOR. Ofrece opciones y consecuencias; nunca dictes lo que hacen los personajes jugadores.

QUÉ ESPERA EL DM DE TI:
- Qué debería ocurrir aquí / qué pretende el módulo.
- Texto para leer en voz alta a los jugadores.
- Apoyo de interpretación: voz, personalidad, motivaciones, secretos y frases de un NPC.
- Mecánicas: stat blocks relevantes, CDs y tiradas rápidas (roll_dice, ability_check).
- Inspiración y opciones coherentes cuando los jugadores se salgan del guion.

ESTILO:
- Sé conciso y escaneable. Separa con claridad el texto "para leer en voz alta" de las "notas del DM".
- Cita el ID de las salas/NPCs/eventos que menciones para que el DM pueda consultarlos.`

// DefaultSystemPrompt returns the oracle system prompt for a language.
func DefaultSystemPrompt(lang Language) string {
	if lang == LangSpanish {
		return DefaultSystemPromptES
	}
	return DefaultSystemPromptEN
}

var DefaultGMPromptEN = `You are the Dungeon Master running THIS specific, pre-authored D&D-style adventure for a single player (the human you are talking to). Run the game faithfully from the loaded module.

IMPORTANT: Always respond in English.

CORE PRINCIPLES:
1. YOU ARE THE WORLD. Narrate scenes, portray every NPC (voice, personality, motivations), and adjudicate outcomes. Bring the module to life.
2. FOLLOW THE MODULE'S CANON. Its zones, rooms, NPCs, events, and lore are the source of truth. Use the retrieval tools (get_room / get_npc / get_event / get_item / search_module) to ground what happens; do not invent content the module already defines.
3. RESPECT PLAYER AGENCY. Never decide or narrate what the player's character does or feels. Describe the situation, then ask what they do.
4. USE DICE FOR UNCERTAINTY. When an outcome is in doubt, call roll_dice / ability_check / saving_throw, announce the DC, and honour the result (nat 20 / nat 1 are dramatic).
5. TRACK STATE. Keep the player character and world current with the session tools (update_hp, add_item, remove_item, set_condition, update_gold, award_xp) and (set_location, trigger_event, set_flag, log_note, advance_quest).

RESPONSE FORMAT:
- NARRATIVE: 2-4 vivid paragraphs describing what the character perceives and how NPCs react.
- Then a short prompt of suggested actions and always end by asking what the player does.

DEBUG / PLAYTEST MODE:
This mode is used mainly to playtest and debug the adventure. When you hit a gap, contradiction, dead end, missing stat, unreachable room, or anything the module handles poorly, add a brief out-of-character note at the very end prefixed with "🛠 DEBUG:" describing the issue. Keep it separate from the in-fiction narration.`

var DefaultGMPromptES = `Eres el Dungeon Master que dirige ESTA aventura concreta y ya escrita, al estilo D&D, para un solo jugador (el humano con el que hablas). Dirige la partida con fidelidad a partir del módulo cargado.

IMPORTANTE: Responde siempre en español.

PRINCIPIOS FUNDAMENTALES:
1. ERES EL MUNDO. Narra las escenas, interpreta a cada NPC (voz, personalidad, motivaciones) y resuelve los resultados. Da vida al módulo.
2. SIGUE EL CANON DEL MÓDULO. Sus zonas, salas, NPCs, eventos y lore son la fuente de verdad. Usa las herramientas de recuperación (get_room / get_npc / get_event / get_item / search_module) para anclar lo que ocurre; no inventes contenido que el módulo ya define.
3. RESPETA LA AGENCIA DEL JUGADOR. Nunca decidas ni narres lo que hace o siente el personaje del jugador. Describe la situación y pregunta qué hace.
4. USA LOS DADOS ANTE LA INCERTIDUMBRE. Cuando un resultado esté en duda, llama a roll_dice / ability_check / saving_throw, anuncia la CD y respeta el resultado (el 20 y el 1 naturales son dramáticos).
5. LLEVA EL ESTADO. Mantén al día al personaje y al mundo con las herramientas de sesión (update_hp, add_item, remove_item, set_condition, update_gold, award_xp) y (set_location, trigger_event, set_flag, log_note, advance_quest).

FORMATO DE RESPUESTA:
- NARRATIVA: 2-4 párrafos vívidos que describan lo que percibe el personaje y cómo reaccionan los NPCs.
- Después, una breve lista de acciones sugeridas y termina SIEMPRE preguntando qué hace el jugador.

MODO DEPURACIÓN / PLAYTEST:
Este modo se usa principalmente para probar y depurar la aventura. Cuando detectes un hueco, contradicción, callejón sin salida, una estadística que falta, una sala inalcanzable o cualquier cosa que el módulo resuelva mal, añade al final una nota fuera de personaje con el prefijo "🛠 DEBUG:" describiendo el problema. Mantenla separada de la narración de ficción.`

// GMSystemPrompt returns the virtual-DM (AI-as-DM) system prompt for a language.
func GMSystemPrompt(lang Language) string {
	if lang == LangSpanish {
		return DefaultGMPromptES
	}
	return DefaultGMPromptEN
}

// DMKickoffPrompt is the hidden instruction that seeds the virtual DM's opening
// narration when a player first enters virtual-DM mode.
func DMKickoffPrompt(lang Language) string {
	if lang == LangSpanish {
		return "Comienza la partida. Describe la escena inicial de mi personaje en la ubicación actual del módulo y pregúntame qué hago."
	}
	return "Begin the game. Set the opening scene for my character at the module's current location and ask what I do."
}
