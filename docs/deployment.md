# Running the server (issue #36, Phase D)

`thaimaturgy-server` serves the JSON API (`/api/…`) and the embedded web UI (`/`)
over the same `internal/appservice` core the desktop app uses. It operates on the
same data (adventures, sessions, config, roster).

## On the OS

```bash
make build-server            # → bin/thaimaturgy-server
bin/thaimaturgy-server        # binds 127.0.0.1:8765 by default
# open http://127.0.0.1:8765
```

Flags / environment:

| Setting | Flag | Env | Default |
|---------|------|-----|---------|
| Listen address | `--addr host:port` | `THAIM_ADDR` | `127.0.0.1:8765` |
| Auth token | `--token …` | `THAIM_SERVER_TOKEN` | (none) |
| Data directory | — | `THAIM_DATA_DIR` | `~/.thaimaturgy` |

Provider credentials are read exactly as the desktop app does
(`THAIM_ANTHROPIC_API_KEY` / `OPENAI_API_KEY` / …, or a reused local login).

### Exposing it beyond localhost

The server **refuses to start** when bound to a non-loopback address (including
the wildcard `:8765`) without a token — that would hand session control, deletion,
config, and billed oracle calls to anyone who can reach it. Set a token:

```bash
THAIM_SERVER_TOKEN=$(openssl rand -hex 24) bin/thaimaturgy-server --addr 0.0.0.0:8765
```

Clients send it as `Authorization: Bearer <token>`; the web UI has a token field.
Terminate TLS with a reverse proxy (nginx/Caddy/Traefik) in front — the server
speaks plain HTTP.

## With Docker

The image is static (distroless, non-root) and stores data on a volume. Because
the container binds the wildcard address, a token is required.

```bash
docker build -t thaimaturgy-server .
docker run --rm -p 127.0.0.1:8765:8765 \
  -e THAIM_SERVER_TOKEN=$(openssl rand -hex 24) \
  -e THAIM_ANTHROPIC_API_KEY=sk-... \
  -v thaimaturgy-data:/data \
  thaimaturgy-server
```

Or with compose (put `THAIM_SERVER_TOKEN=…` and any provider keys in a `.env`
file next to `docker-compose.yml`):

```bash
THAIM_SERVER_TOKEN=$(openssl rand -hex 24) docker compose up --build
```

The named volume `thaimaturgy-data` is mounted at `/data` (`THAIM_DATA_DIR`), so
adventures, sessions, config, and the roster persist across `docker run`. To
import an adventure into a container, mount your module or use the API's import
endpoint.

## Clients

- **Web UI** — served at `/` by the server (Phase C); a full decoupled client.
- **Desktop app (remote mode)** — launch the desktop GUI as a client of a server:

  ```bash
  bin/thaimaturgy --server http://127.0.0.1:8765 --token <token>
  # or THAIM_SERVER=… THAIM_SERVER_TOKEN=… bin/thaimaturgy
  ```

  In remote mode the library, sessions, oracle/commands, party, and the live log
  (over SSE) come from the server via `internal/apiclient`. Without `--server`
  the app runs locally against the in-process core exactly as before.
- **Go client** — `internal/apiclient` is a typed client for the API (used by the
  remote desktop mode and available for a CLI).

The Telegram bot continues to run locally against the core.
