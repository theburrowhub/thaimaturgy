# Releases (CI/CD)

Releases are cut by pushing a **semver tag** (`vX.Y.Z`). The `Release` GitHub
Actions workflow (`.github/workflows/release.yml`) then publishes, all versioned
from the tag:

- **Static binaries** — `thaimaturgy-server`, `thaimaturgy-bot`, `thaimaturgy-novel`
  for linux/darwin/windows × amd64/arm64, bundled per platform and attached to a
  GitHub Release with SHA-256 checksums.
- **Desktop GUI** — `thaimaturgy` (CGO/Fyne) for linux (amd64) and macOS (arm64),
  built on native runners and attached to the same Release. *(Windows GUI packaging
  is a follow-up — Windows users get the static binaries above.)*
- **Docker images** — multi-arch (`linux/amd64,linux/arm64`) pushed to GHCR:
  - `ghcr.io/theburrowhub/thaimaturgy-server` (default, distroless)
  - `ghcr.io/theburrowhub/thaimaturgy-server-claude` (Node + the claude CLI, for the
    `claude-cli` provider — see [server-credentials.md](server-credentials.md))

Everyday testing (build + `go test -race` on every push/PR) lives in
`.github/workflows/ci.yml`.

### Tag policy (two-phase, concurrency-safe)

Publishing is split so moving tags stay correct even if releases run out of order:

1. **Immutable push** — each release builds and pushes only its own `X.Y.Z`
   image (or `X.Y.Z-pre`) and attaches its binaries to a GitHub Release. These
   jobs are independent and never serialized, so every release always ships.
2. **Promote** (stable releases only) — a full, idempotent reconciler. It re-reads
   and re-validates all tags and repoints `latest` **and every** `X.Y` at the
   highest stable image that is actually published:

| Moving tag | Points at |
|------------|-----------|
| `X.Y.Z`    | immutable, set once at build (incl. prereleases) |
| `X.Y`      | highest published stable patch in that `X.Y` series |
| `latest`   | highest published stable version in the whole repo |

Every run reconciles all moving tags to the true highest (never "self"), and falls
back past a failed higher build to an older published image — so order/overlap
don't matter and no serialization is needed; an older tag can't clobber a newer
one, and any single run leaves every moving tag correct. Prereleases publish only
their immutable image + a prerelease GitHub Release. **Build metadata**
(`v1.2.3+meta`) is rejected: Docker tag normalization drops it, so it would
collide with `v1.2.3`.

## Cut a release

```bash
git tag v1.2.3
git push origin v1.2.3
```

Only strict-semver `vX.Y.Z` tags publish; a malformed tag fails validation before
anything is pushed.

## Consume a release

Pull an image:

```bash
docker pull ghcr.io/theburrowhub/thaimaturgy-server:latest
# or a pinned version
docker pull ghcr.io/theburrowhub/thaimaturgy-server:1.2.3
```

Or download a binary bundle from the [GitHub Releases](https://github.com/theburrowhub/thaimaturgy/releases)
page and verify it:

```bash
sha256sum -c thaimaturgy_v1.2.3_linux_amd64.tar.gz.sha256
tar xzf thaimaturgy_v1.2.3_linux_amd64.tar.gz
```

## Version embedding

The build passes `-ldflags "-X main.Version=<tag> -X main.Commit=<sha> -X
main.BuildTime=<ts>"` (mirroring the Makefile). Wiring a `--version` flag that
prints these is a small follow-up.
