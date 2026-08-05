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

type TTSProvider string

const (
	TTSProviderOpenAI      TTSProvider = "openai"
	TTSProviderMagnificMCP TTSProvider = "magnific-mcp"
)

type TTSConfig struct {
	Enabled  bool        `json:"enabled"`
	Provider TTSProvider `json:"provider"`
	Voice    TTSVoice    `json:"voice"`
	Model    string      `json:"model"`
	Speed    float64     `json:"speed"`

	// Magnific MCP settings for Telegram narration audio. The command should read
	// JSON from stdin, call Magnific's audio_tts MCP tool, download an MP3 to
	// outputPath, and print {"audioPath":"..."}. Keep credentials in the MCP
	// client's own auth store or environment, not in this config.
	MagnificMCPCommand      string  `json:"magnific_mcp_command,omitempty"`
	MagnificVoiceID         int     `json:"magnific_voice_id,omitempty"`
	MagnificStability       float64 `json:"magnific_stability,omitempty"`
	MagnificSimilarityBoost float64 `json:"magnific_similarity_boost,omitempty"`
	MagnificUseSpeakerBoost bool    `json:"magnific_use_speaker_boost,omitempty"`
	CacheDir                string  `json:"cache_dir,omitempty"`
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

	// Telegram multiplayer bot. The token lets the app launch the bot to host the
	// current virtual-DM game; ChatID (optional) restricts it to one chat.
	TelegramToken  string `json:"telegram_token,omitempty"`
	TelegramChatID int64  `json:"telegram_chat_id,omitempty"`
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
			Enabled:                 false,
			Provider:                TTSProviderOpenAI,
			Voice:                   TTSVoiceOnyx, // Deep, dramatic voice for DM
			Model:                   "tts-1",
			Speed:                   1.0,
			MagnificStability:       0.15,
			MagnificSimilarityBoost: 0.35,
			MagnificUseSpeakerBoost: true,
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

var DefaultGMPromptEN = `You are the Dungeon Master running THIS specific, pre-authored D&D-style adventure for the player (the human you are talking to), who controls a PARTY of player characters. Run the game faithfully from the loaded module.

IMPORTANT: Always respond in English.

INFORMATION DISCIPLINE (NO SPOILERS) — READ THIS FIRST:
The module context you receive includes DM-ONLY secrets: the background / "the truth", room DM notes, NPC secrets, hidden rooms, unexplored zones and future events. You use them ONLY to run and adjudicate the game — you must NEVER reveal them to the player. Answer questions strictly with what the party's characters could perceive right now or already know (what is visible, what they've already discovered, common knowledge). Reveal hidden content only through actual play: exploration, successful checks, or in-fiction discovery — never by listing the map, floor plans, secret levels, hidden inhabitants or plot. Do NOT break the fiction to give an exhaustive, canonical, out-of-character answer — that full-disclosure "oracle" behaviour is for assistant mode, NOT here. If a question would require DM-only knowledge to answer fully, answer only the player-knowable part in the fiction (e.g. describe what the building looks like from outside), and let the rest be discovered. When unsure whether the party knows something, assume they do NOT.

CORE PRINCIPLES:
1. YOU ARE THE WORLD. Narrate scenes, portray every NPC (voice, personality, motivations), and adjudicate outcomes. Bring the module to life.
2. FOLLOW THE MODULE'S CANON. Its zones, rooms, NPCs, events, and lore are the source of truth. Use the retrieval tools (get_room / get_npc / get_event / get_item / search_module) to ground what happens; do not invent content the module already defines. (Grounding ≠ disclosure — see INFORMATION DISCIPLINE above.)
3. RESPECT PLAYER AGENCY. Never decide or narrate what the party's characters do or feel. Describe the situation, then ask what they do.
4. USE DICE FOR UNCERTAINTY. When an outcome is in doubt, call roll_dice or ability_check (a saving throw is just an ability_check against a DC), announce the DC, and honour the result (nat 20 / nat 1 are dramatic).
5. TRACK STATE. Keep each party member and the world current with the session tools (update_hp, add_item, remove_item, set_condition, update_gold, award_xp — pass the "character" name to target a specific member) and (set_location, trigger_event, set_flag, log_note, advance_quest). The party roster and each member's sheet are provided in context.

RESPONSE FORMAT:
- NARRATIVE: 2-4 vivid paragraphs describing what the character perceives and how NPCs react.
- Then a short prompt of suggested actions and always end by asking what the player does.

DEBUG / PLAYTEST MODE:
This mode is used mainly to playtest and debug the adventure. When you hit a gap, contradiction, dead end, missing stat, unreachable room, or anything the module handles poorly, add a brief out-of-character note at the very end prefixed with "🛠 DEBUG:" describing the issue. Keep it separate from the in-fiction narration. This note is ONLY for flagging module problems to the tester — it is NEVER a channel to reveal secrets or spoilers to the player.`

var DefaultGMPromptES = `Eres el Dungeon Master que dirige ESTA aventura concreta y ya escrita, al estilo D&D, para el jugador (el humano con el que hablas), que controla un GRUPO de personajes. Dirige la partida con fidelidad a partir del módulo cargado.

IMPORTANTE: Responde siempre en español.

DISCIPLINA DE INFORMACIÓN (SIN SPOILERS) — LEE ESTO PRIMERO:
El contexto del módulo que recibes incluye secretos SOLO PARA EL DM: el trasfondo / "la verdad", las notas de DM de las salas, los secretos de los NPC, salas ocultas, zonas no exploradas y eventos futuros. Los usas ÚNICAMENTE para dirigir y arbitrar la partida — NUNCA debes revelárselos al jugador. Responde solo con lo que los personajes del grupo podrían percibir ahora mismo o ya saben (lo visible, lo que ya han descubierto, el conocimiento común). Revela el contenido oculto solo mediante el juego real: exploración, tiradas con éxito o descubrimiento dentro de la ficción — nunca enumerando el mapa, los planos, los niveles secretos, los habitantes ocultos o la trama. NO rompas la ficción para dar una respuesta exhaustiva, canónica y fuera de personaje — ese comportamiento de "oráculo" con información completa es del modo asistente, NO de aquí. Si responder del todo requeriría conocimiento de DM, responde solo la parte que el jugador puede conocer, dentro de la ficción (p. ej. describe cómo se ve el edificio desde fuera) y deja que lo demás se descubra. Ante la duda de si el grupo sabe algo, asume que NO.

PRINCIPIOS FUNDAMENTALES:
1. ERES EL MUNDO. Narra las escenas, interpreta a cada NPC (voz, personalidad, motivaciones) y resuelve los resultados. Da vida al módulo.
2. SIGUE EL CANON DEL MÓDULO. Sus zonas, salas, NPCs, eventos y lore son la fuente de verdad. Usa las herramientas de recuperación (get_room / get_npc / get_event / get_item / search_module) para anclar lo que ocurre; no inventes contenido que el módulo ya define. (Anclar ≠ revelar — mira la DISCIPLINA DE INFORMACIÓN de arriba.)
3. RESPETA LA AGENCIA DEL JUGADOR. Nunca decidas ni narres lo que hacen o sienten los personajes del grupo. Describe la situación y pregunta qué hacen.
4. USA LOS DADOS ANTE LA INCERTIDUMBRE. Cuando un resultado esté en duda, llama a roll_dice o ability_check (una tirada de salvación es un ability_check contra una CD), anuncia la CD y respeta el resultado (el 20 y el 1 naturales son dramáticos).
5. LLEVA EL ESTADO. Mantén al día a cada miembro del grupo y al mundo con las herramientas de sesión (update_hp, add_item, remove_item, set_condition, update_gold, award_xp — pasa el nombre en "character" para apuntar a un miembro concreto) y (set_location, trigger_event, set_flag, log_note, advance_quest). Tienes en el contexto el listado del grupo y la ficha de cada miembro.

FORMATO DE RESPUESTA:
- NARRATIVA: 2-4 párrafos vívidos que describan lo que percibe el grupo y cómo reaccionan los NPCs.
- Después, una breve lista de acciones sugeridas y termina SIEMPRE preguntando qué hacen.

MODO DEPURACIÓN / PLAYTEST:
Este modo se usa principalmente para probar y depurar la aventura. Cuando detectes un hueco, contradicción, callejón sin salida, una estadística que falta, una sala inalcanzable o cualquier cosa que el módulo resuelva mal, añade al final una nota fuera de personaje con el prefijo "🛠 DEBUG:" describiendo el problema. Mantenla separada de la narración de ficción. Esta nota es SOLO para señalar problemas del módulo al que prueba — NUNCA es un canal para revelar secretos o spoilers al jugador.`

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
		return "Comienza la partida. Describe la escena inicial de mi grupo en la ubicación actual del módulo y pregúntame qué hacemos."
	}
	return "Begin the game. Set the opening scene for my party at the module's current location and ask what we do."
}
