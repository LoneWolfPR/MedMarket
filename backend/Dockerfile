# Multi-stage build: compile with the full Go toolchain, ship only the binary.
# CMD selects which cmd/ entrypoint to build (server or worker) so the API and
# the Temporal worker share one Dockerfile and the module-download cache layer.
FROM golang:1.26 AS builder
WORKDIR /src

# Cache module downloads independently of source changes.
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
ARG CMD=server
# The module cache must be mounted here too: with a cache mount above, the
# downloads live in the cache rather than in this image's layers, so dropping it
# would make the build re-fetch every module. The build cache keeps compiled
# package objects (the Temporal SDK dominates) across builds, so a source change
# recompiles only what changed instead of the whole dependency tree.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o /out/app ./cmd/${CMD}

# distroless/static has no shell or package manager — minimal attack surface.
FROM gcr.io/distroless/static-debian12
COPY --from=builder /out/app /app
EXPOSE 8080
ENTRYPOINT ["/app"]
