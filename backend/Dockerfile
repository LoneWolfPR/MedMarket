# Multi-stage build: compile with the full Go toolchain, ship only the binary.
FROM golang:1.26 AS builder
WORKDIR /src

# Cache module downloads independently of source changes.
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

# distroless/static has no shell or package manager — minimal attack surface.
FROM gcr.io/distroless/static-debian12
COPY --from=builder /out/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
