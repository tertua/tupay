# Build for the target platform (buildx sets TARGETOS/TARGETARCH; plain
# `docker build` falls back to the host pair). $BUILDPLATFORM keeps the
# toolchain native and cross-compiles instead of emulating under qemu.
FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine AS builder

LABEL maintainer="Candro Aleandro <admin@tanet.eu.org> (https://tanet.eu.org/)"

# Move to working directory (/build).
WORKDIR /build

# Copy and download dependency using go mod.
COPY go.mod go.sum ./
RUN go mod download

# Copy the code into the container.
COPY . .

# Build the static API server (scratch has no libc; CGO off).
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o tupay .

# scratch has no shell, so the writable state dir for the non-root user
# (USER below) must arrive baked in with the right ownership. Defaults:
# SQLITE_PATH=./data/db/* and STORAGE_DIR=./data/uploads (cwd is /).
RUN mkdir -p /out/data

FROM scratch

# Copy the binary, single-source VERSION and the empty /data state dir.
# Secrets are never baked in: godotenv/autoload is a no-op when no .env file
# exists, so inject config at run time instead:
#   docker run --env-file .env tupay
# or per-var -e flags / orchestrator secrets. Anyone with the image
# (registry pull, docker export, layer history) must not recover keys.
COPY --from=builder ["/build/tupay", "/"]
COPY --from=builder ["/build/VERSION", "/VERSION"]
COPY --from=builder --chown=65532:65532 ["/out/data", "/data"]

# Non-root: scratch has no /etc/passwd, so use a numeric UID/GID. 65532 is
# the distroless "nonroot" convention (NOT nobody — that is 65534 on
# Debian/Alpine; the numeric ID is what matters, names don't exist here).
# Bind-mounted state must be owned by it on the host:
#   chown -R 65532:65532 data/
USER 65532:65532

# Probes (see pkg/routes/health_routes.go). The orchestrator, not the app,
# decides restart/unready based on these.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s CMD ["/tupay", "-healthcheck"]

# Command to run when starting the container.
ENTRYPOINT ["/tupay"]
