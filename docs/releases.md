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
  tagged `X.Y.Z`, `X.Y`, and `latest`.

Everyday testing (build + `go test -race` on every push/PR) lives in
`.github/workflows/ci.yml`.

## Cut a release

```bash
git tag v1.2.3
git push origin v1.2.3
```

The workflow also has a manual `workflow_dispatch` trigger.

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
