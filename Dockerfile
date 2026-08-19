# syntax=docker/dockerfile:1

# --- build ---------------------------------------------------------------
FROM golang:1.25 AS build
# Version stamp injected into internal/buildinfo (the image shows it at
# /api/version and in the web badge). .dockerignore excludes .git, so there is no
# VCS fallback in-image — pass VERSION (the semver tag) via --build-arg / compose.
ARG VERSION=
ARG COMMIT=
ARG DATE=
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# The server has no CGO dependencies (no Fyne), so build a static binary.
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X github.com/theburrowhub/thaimaturgy/internal/buildinfo.Version=${VERSION} -X github.com/theburrowhub/thaimaturgy/internal/buildinfo.Commit=${COMMIT} -X github.com/theburrowhub/thaimaturgy/internal/buildinfo.Date=${DATE}" \
    -o /out/thaimaturgy-server ./cmd/thaimaturgy-server
# Pre-create the data dir owned by the non-root runtime user.
RUN mkdir -p /out/data

# --- runtime -------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/thaimaturgy-server /usr/local/bin/thaimaturgy-server
COPY --from=build --chown=nonroot:nonroot /out/data /data
# Data (adventures/sessions/config) lives on a mounted volume; bind to all
# interfaces (Docker maps the port). A wildcard bind refuses to start without a
# token, so THAIM_SERVER_TOKEN must be provided.
# HOME is set so os.UserHomeDir() resolves for the nonroot user: mounting a host
# ~/.claude/.credentials.json or ~/.gemini/oauth_creds.json into /home/nonroot
# lets the server reuse a Claude Code / Gemini CLI login (see docs/server-credentials.md).
ENV THAIM_DATA_DIR=/data THAIM_ADDR=:8765 HOME=/home/nonroot
EXPOSE 8765
USER nonroot
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/thaimaturgy-server"]
