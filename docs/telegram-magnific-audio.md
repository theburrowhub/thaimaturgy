# Telegram DM audio with Magnific MCP

thAImaturgy can send Telegram DM narration as text first, then best-effort MP3 audio generated through Magnific MCP. If audio generation or upload fails, the bot keeps the text response and logs the audio error.

## Prerequisites

Configure Magnific in Hermes on the same machine that runs the bot:

```bash
hermes mcp add magnific --url https://mcp.magnific.com --auth oauth
hermes mcp login magnific
hermes config set mcp_servers.magnific.enabled true
hermes mcp test magnific
```

## Config

In `config.yaml` or via the Settings screen:

```yaml
tts:
  enabled: true
  provider: magnific-mcp
  model: eleven_v3
  speed: 0.95
  magnific_mcp_command: "python3 /path/to/thaimaturgy/scripts/magnific_tts_from_hermes.py"
  magnific_voice_id: 554  # 554 = Álvaro Serrano
  magnific_stability: 0.15
  magnific_similarity_boost: 0.35
  magnific_use_speaker_boost: true
```

Environment overrides are also supported:

```bash
export THAIM_TTS_ENABLED=true
export THAIM_TTS_PROVIDER=magnific-mcp
export THAIM_MAGNIFIC_MCP_COMMAND="python3 /path/to/thaimaturgy/scripts/magnific_tts_from_hermes.py"
export THAIM_MAGNIFIC_VOICE_ID=554
export THAIM_MAGNIFIC_STABILITY=0.15
```

Secrets stay in Hermes/Magnific OAuth storage or the environment. The app passes only text, voice/model parameters, and an output path to the command.

Security note: `magnific_mcp_command` is executed as an operator-supplied shell command. Treat write access to `config.yaml` as equivalent to local code execution, and point it only at scripts you trust.
