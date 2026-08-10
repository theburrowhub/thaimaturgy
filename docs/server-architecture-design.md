# Client/server architecture — design & phased plan (issue #36)

Scoping deliverable for the backlog refactor: run thAImaturgy as a **server**
(directly on the OS or via Docker) with a **web interface** mirroring the GUI,
then adapt the desktop GUI to talk to that server — separating client from
business logic. This document proposes the architecture and a phased plan with
the open decisions called out, so implementation can proceed (and be steered)
without re-litigating the shape mid-way.

## 1. Starting point (what helps)

- **The core is already UI-agnostic.** `internal/` (domain, engine, storage,
  providers, srd, dmbook, …) holds all logic; the desktop GUI (`cmd/thaimaturgy`)
  and the Telegram bot (`internal/tgbot`) are thin frontends over it. This is the
  right seam to expose over HTTP/WS.
- **A shared action model exists.** `engine.CommandHandler`
  (`ParseCommand`/`Execute` → `CommandResult`) already abstracts UI actions for
  parity (#20); it maps naturally onto request/response endpoints.
- **`feat/wails-local-web`** (reverted PR #12) has a **web frontend** replicating
  the GUI (library, session/oracle, editor, settings) plus art/audio, with
  bindings to the core. Although Wails ships a *local* binary, its web assets and
  the action mapping are a strong starting point for the server's frontend.
- **Storage is filesystem-based** under `~/.thaimaturgy` (adventures, sessions,
  config, characters), which maps cleanly to a Docker **volume**.

## 2. Target layering

```
core/service (internal/*, unchanged)        ← business logic, no transport
        ▲
service facade (new: internal/appservice)   ← use-cases: list/open/play/edit,
        ▲                                       run oracle turn, host, roster…
transport (new: internal/httpapi)           ← REST + WebSocket, auth, sessions
        ▲
clients:  web UI  ·  desktop GUI (as client) ·  (Telegram bot unchanged)
```

- **Service facade** (`internal/appservice`): a transport-agnostic API expressing
  the app's use-cases (the same operations the GUI performs today), returning
  plain data + errors. Both the HTTP layer and — during migration — the desktop
  GUI call it. This is where session lifecycle and concurrency live.
- **Transport** (`internal/httpapi`): thin HTTP handlers + a WebSocket for the
  streaming oracle/DM turn and live session/log updates. Serves the embedded web
  UI. No business logic.
- **Clients**: the web UI (served by the server) and the desktop GUI refactored
  to call the service — locally in-process (default) or over HTTP to a remote
  server (configurable). The Telegram bot keeps using the core directly, or
  moves behind the facade too (optional).

## 3. API surface (sketch)

REST (JSON) for CRUD/state; WebSocket for streaming and push:

- `GET /api/adventures`, `GET /api/adventures/{id}`, `POST /api/import`,
  `DELETE /api/adventures/{id}`
- `GET /api/sessions`, `POST /api/sessions` (new from adventure),
  `GET/PUT/DELETE /api/sessions/{name}`, `POST /api/sessions/{name}/rename`
- `POST /api/sessions/{name}/command` — run a shared `CommandHandler` action
  (parity path); returns `CommandResult`
- `POST /api/sessions/{name}/oracle` — a DM/oracle turn; **WS** variant streams
  tokens and tool activity
- `GET /api/roster`, `POST /api/roster`, `DELETE /api/roster/{id}` (#33)
- `GET /api/config`, `PUT /api/config`; `GET /api/adventures/{id}/assets/...`
  (images/maps/art), respecting the same zip-slip-safe resolution as today
- `WS /api/sessions/{name}/events` — pushes log/timeline/party/combat updates so
  every connected client stays in sync
- media/art and the DM book/PDF export reuse existing `internal` renderers

Streaming the oracle over WS matches the existing background-goroutine turn model
and lets the web UI and GUI render narration as it arrives.

## 4. Session concurrency & state ownership

Today the GUI owns one live `*domain.Session` in memory and autosaves it. On a
server the live session must live **server-side**:

- the service holds a registry of open sessions (`map[name]*Session` under a
  mutex), each with its existing internal locking; the FIFO autosave worker (#33)
  moves server-side.
- **Open decision — concurrency model:**
  - **Single-user / single active session (recommended first):** one session
    open at a time, one writer; simplest and matches current behavior. Multiple
    read-only viewers via the events WS are fine.
  - **Multi-user later:** multiple concurrent sessions and per-session
    subscribers; needs a broadcast hub and careful turn serialization (only one
    oracle turn per session at a time — already true).
  Recommendation: build the registry to *support* multiple sessions but ship
  single-active-session semantics first.

## 5. Web frontend

Reuse `feat/wails-local-web`'s assets and action mapping, but decouple from Wails
bindings: replace the Wails `Call`/binding layer with `fetch`/WebSocket against
the REST/WS API above. The server **embeds** the built web assets
(`go:embed`) so a single binary serves the UI. Feature scope mirrors the GUI:
library, session (oracle + virtual DM, dice, party/roster, combat view when #22
lands), editor, settings.

## 6. Desktop GUI as a client

Refactor the Fyne GUI so its handlers call the **service facade** instead of the
core directly. Two run modes behind one interface:

- **Embedded (default):** the GUI hosts an in-process service — same
  single-binary UX as today, just with presentation/business separated.
- **Remote:** the GUI points at a server URL and uses the HTTP/WS client. Same
  screens, remote data.

The refactor is incremental: introduce the facade, move GUI calls onto it screen
by screen, then add the remote client behind the same interface.

## 7. Docker & deployment

- Multi-stage image: build the web assets + Go binary, ship a small runtime image
  exposing the server port.
- **Volume** for `~/.thaimaturgy` (or a configurable data dir) so adventures /
  sessions / config / roster persist across `docker run`; document `docker run`
  and a `compose` example.
- Config/paths become server-relative and env-overridable (the config layer
  already reads env — extend for data-dir and bind address).
- Provider credentials via env/secrets (never baked into the image); the existing
  auth/config precedence carries over.

## 8. Auth & security

- **localhost default:** no auth when bound to `127.0.0.1` (parity with a local
  app).
- **Exposed:** require a token/simple auth when bound to a non-loopback address;
  fail closed (mirror the Telegram allow-list philosophy from #34). Note that the
  DM-only content discipline (#28) applies to any player-facing client too.
- HTTPS termination is left to a reverse proxy; document it.

## 9. Relationship to the Wails app

**Decision: the server supersedes the Wails local-web app.** Wails' value was a
local web GUI; the server provides that plus remote/Docker. Keep
`feat/wails-local-web` only as the source of frontend assets to port. The desktop
Fyne GUI stays (as an embedded-or-remote client); Wails is not maintained as a
third frontend.

## 10. Phased implementation plan

- **Phase 0 (this doc).** Architecture + API + decisions.
- **Phase A — service facade.** Extract `internal/appservice` with the app's
  use-cases and the session registry + server-side autosave; the desktop GUI is
  refactored to call it in-process. No transport yet, no behavior change. *This
  is the enabling refactor and the biggest, most careful step.*
- **Phase B — HTTP/WS server.** `internal/httpapi` over the facade; REST for
  CRUD/state, WS for the streaming oracle and event push; `cmd/thaimaturgy-server`
  binary. Serves a minimal/static UI to validate end-to-end.
- **Phase C — web UI.** Port the `feat/wails-local-web` frontend onto the REST/WS
  API; embed the built assets in the server binary.
- **Phase D — Docker & remote GUI.** Dockerfile + compose with a data volume and
  deployment docs; add the GUI's "remote server" mode behind the facade
  interface. Basic auth when non-loopback.

Each phase is independently shippable; Phase A changes no behavior, and the
Telegram bot keeps working throughout.

## 11. Required changes (summary)

- **new** `internal/appservice` (use-cases + session registry + autosave),
  `internal/httpapi` (REST/WS + embedded web), `cmd/thaimaturgy-server`.
- **refactor** `cmd/thaimaturgy` to call the facade (embedded or remote).
- **port** the `feat/wails-local-web` frontend to the API; drop Wails bindings.
- **ops** Dockerfile + compose + data volume + deployment docs; config gains
  data-dir/bind-address/auth.

Recommended first step to implement: **Phase A** (the `appservice` facade), as its
own issue/PR.
