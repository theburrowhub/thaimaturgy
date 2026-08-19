# Provider credentials for the server

The `thaimaturgy-server` (used by the web UI and the remote desktop GUI) reaches
the LLM the same three ways the desktop app does. In order of simplicity:

1. **Environment API keys** — works on the default image, no mounts.
2. **Reused local logins (OAuth)** — mount a Claude Code / Gemini CLI login; works
   on the default image.
3. **The `claude-cli` backend** — drive your Claude Code login/subscription through
   the `claude` CLI; needs the larger `Dockerfile.claude-cli` image.

The active credential the server auto-detected is shown read-only in **Settings**
(“Detected credential”) in both the web UI and the remote GUI.

---

## 1. Environment API keys (simplest)

Pass any of these to the container (e.g. in `docker-compose.yml` or a `.env`):

```
THAIM_OPENAI_API_KEY     # or OPENAI_API_KEY
THAIM_ANTHROPIC_API_KEY  # or ANTHROPIC_API_KEY
THAIM_GEMINI_API_KEY     # or GEMINI_API_KEY / GOOGLE_API_KEY
THAIM_PROVIDER           # openai | anthropic | gemini | claude-cli
THAIM_MODEL              # a model id
```

Providers openai/anthropic/gemini are plain HTTPS, so this works on the default
distroless image as-is. Keys are never written to disk.

## 2. Reuse a local login (OAuth) — default image

The server can reuse a **Claude Code** or **Gemini CLI** login the same way the
desktop app does: it reads the portable credential files (the macOS Keychain path
is desktop-only and skipped on Linux). Mount them into the container user’s home:

| Login       | Host file                        | Mount target (container)                 |
|-------------|----------------------------------|------------------------------------------|
| Claude Code | `~/.claude/.credentials.json`    | `/home/nonroot/.claude/.credentials.json`|
| Gemini CLI  | `~/.gemini/oauth_creds.json`     | `/home/nonroot/.gemini/oauth_creds.json` |

The default image sets `HOME=/home/nonroot` so `os.UserHomeDir()` resolves. Example
`docker-compose.override.yml`:

```yaml
services:
  thaimaturgy:
    volumes:
      - ${HOME}/.thaimaturgy:/data
      - ${HOME}/.claude/.credentials.json:/home/nonroot/.claude/.credentials.json:ro
```

Then set the matching provider (`anthropic` / `gemini`) **without** an API key so
the OAuth token is used. The OAuth provider hits the normal API with a bearer
token — no extra binary needed.

> **Caveat — token expiry.** These are short-lived access tokens. The server reads
> them at startup and when the config is saved; it does **not** refresh them or
> re-read the file periodically. When the token expires, re-log in on the host and
> restart the container (or re-save the config). For long-running/unattended
> servers, prefer an environment API key (option 1).

## 3. The `claude-cli` backend — `Dockerfile.claude-cli`

`provider=claude-cli` drives the actual **Claude Code CLI** (`claude -p`) instead of
calling the API directly. This needs the `claude` binary (an npm package, so
Node.js) on `PATH`, which the default distroless image does not have. Use the
dedicated image, which ships Node + the CLI and runs the server (the server binary
answers the `__mcp-tools` subcommand the backend re-execs to expose session tools):

```
docker compose -f docker-compose.yml -f docker-compose.claude.yml up -d --build
```

`docker-compose.claude.yml` builds `Dockerfile.claude-cli`, sets
`THAIM_PROVIDER=claude-cli`, and mounts your `~/.claude` login (read-write, so the
CLI can refresh its token) plus your data dir. Requirements:

- A **Claude Code login on the host** (`~/.claude`) — or an API key in the
  environment: set either `ANTHROPIC_API_KEY` or `THAIM_ANTHROPIC_API_KEY` and the
  compose file forwards it to `ANTHROPIC_API_KEY`, which the CLI reads.
- Outbound HTTPS to the Anthropic API.

> **Native Linux — file ownership.** Bind mounts preserve host ownership, but the
> image runs as the `node` user (UID 1000). If your host user isn’t UID 1000, the
> mounted data dir won’t be writable and `~/.claude` won’t be readable, so storage
> init or CLI auth fails. Run the container as yourself — uncomment the `user:`
> line in `docker-compose.claude.yml` and export your ids first
> (`export UID; export GID=$(id -g)`). On Docker Desktop (macOS/Windows) this is
> handled for you and no change is needed.

Pin the CLI version for a reproducible image:

```
docker compose -f docker-compose.yml -f docker-compose.claude.yml build \
  --build-arg CLAUDE_VERSION=x.y.z
```

> This image is **much larger** than the default distroless one and has a broader
> attack surface (Node + the CLI). Use it only when you specifically want the
> claude-cli backend; options 1–2 cover OpenAI/Anthropic/Gemini on the lean image.

### Running the server natively (no Docker)

The `claude-cli` backend also works when you run `./bin/thaimaturgy-server` directly
on a machine that already has `claude` on `PATH` and is logged in — no image needed.
Set the provider to `claude-cli` in Settings or via `THAIM_PROVIDER=claude-cli`.
