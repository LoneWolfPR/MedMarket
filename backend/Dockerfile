# Multi-stage build: compile with the full Go toolchain, ship only the binary.
# CMD selects which cmd/ entrypoint to build (server or worker) so the API and
# the Temporal worker share one Dockerfile and the module-download cache layer.
FROM golang:1.26 AS builder
WORKDIR /src

# Cache module downloads independently of source changes.
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
ARG CMD=server
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/app ./cmd/${CMD}

# distroless/static has no shell or package manager — minimal attack surface.
FROM gcr.io/distroless/static-debian12
COPY --from=builder /out/app /app
EXPOSE 8080
ENTRYPOINT ["/app"]
