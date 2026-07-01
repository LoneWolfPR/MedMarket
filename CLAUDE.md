# MedMarket

Prescription price comparison platform. Monorepo with Go backend, React frontend, mock external services.

## Implementation Plan

See implementation-plan.md at repo root for the full day-by-day plan.

## Current Progress

In addition to the notes below check your most recent memories to make sure you have all context from previous conversations.

The plan is to have claude do basic scaffolding and boilerplat that I will review, while I do the coding in areas I'm trying to learn. All local technology has been verified except for Terraform which will be added when needed.

**Day 1 — complete (checkpoint passed 2026-07-01).** Scaffolding is in place and reviewed: top-level monorepo skeleton (with `.gitkeep` placeholders for `services/`, `workflows/`, `worker/`, `k8s/`, `terraform/`, `.github/workflows/`); root `go.work` (workspace root, `use ./backend`); `backend/` module (`github.com/LoneWolfPR/MedMarket/backend`, go 1.26) with `cmd/server/main.go` serving `GET /api/health`; multi-stage backend Dockerfile (golang:1.26 → distroless); `frontend/` React 19 + Vite 6 + TS skeleton (App.tsx fetches `/api/health` as a self-verifying checkpoint); frontend dev Dockerfile (node:24-alpine); `docker-compose.yml` (traefik + backend + frontend + postgres on the `medmarket` network). Backend builds + vets clean.

`docker compose up --build` verified: `http://localhost` shows "Backend health: ok", `/api/health` returns JSON, and the Traefik dashboard is at `:8080`. Two issues surfaced and were fixed during bring-up: (1) Docker socket permission — user added to the `docker` group and re-logged in; (2) `traefik:v3.3` hard-pins Docker API 1.24, which Engine 29.6.1 rejects (min API 1.40) — bumped to `traefik:v3.6`; (3) local Postgres already on host `5432`, so compose Postgres remapped to host **5433** (container-internal still 5432 — point Atlas/tooling at `localhost:5433`). `.gitignore` reviewed. **Not yet git-committed** (user commits Day 1 manually).

## Architecture

- Hexagonal architecture in backend/internal/ — domain and ports never import adapters
- Temporal for workflow orchestration (price search, order saga, shipping tracking)
- Traefik reverse proxy routes /api/ to backend, / to frontend

## Stack

- Go 1.26, Ent ORM, Atlas migrations, PostgreSQL 17
- React + Vite + TypeScript frontend
- Temporal for durable workflows
- MinIO for file storage (S3-compatible), Mailpit for local email
- Docker Compose for local dev, GKE for deployment

## Commands

Common commands are wrapped with [go-task](https://taskfile.dev) — run `task` (or `task --list`) to see everything. Key targets:

- `task up` / `task down` / `task logs` — manage the full local Docker stack
- `task backend:build` / `vet` / `lint` / `test` / `tidy` — backend Go workflow
- `task backend:check` — build + vet + lint in one shot

Not yet wrapped (task targets will be added as Ent/Atlas/frontend land):

- `go generate ./ent` — regenerate Ent client after schema changes
- `atlas migrate diff <name> --env local` — create new migration
- `npm run dev` — frontend dev server (runs inside container)

## Conventions

- All external dependencies accessed through port interfaces in ports/outbound/
- Adapters implement port interfaces, wired together in cmd/server/main.go
- Tests use testcontainers-go for integration tests needing Postgres
- golangci-lint must pass before committing