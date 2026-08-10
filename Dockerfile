# syntax=docker/dockerfile:1

# --- build ---------------------------------------------------------------
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# The server has no CGO dependencies (no Fyne), so build a static binary.
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/thaimaturgy-server ./cmd/thaimaturgy-server
# Pre-create the data dir owned by the non-root runtime user.
RUN mkdir -p /out/data

# --- runtime -------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/thaimaturgy-server /usr/local/bin/thaimaturgy-server
COPY --from=build --chown=nonroot:nonroot /out/data /data
# Data (adventures/sessions/config) lives on a mounted volume; bind to all
# interfaces (Docker maps the port). A wildcard bind refuses to start without a
# token, so THAIM_SERVER_TOKEN must be provided.
ENV THAIM_DATA_DIR=/data THAIM_ADDR=:8765
EXPOSE 8765
USER nonroot
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/thaimaturgy-server"]
