# MedMarket

A prescription price comparison platform. Users upload a prescription, compare
real-time prices across multiple pharmacies, place an order, and track shipping
— orchestrated by durable workflows and built on a hexagonal (ports & adapters)
architecture.

This repository is a monorepo housing the backend API, the web frontend, mock
external services, workflow definitions, and deployment configuration.

> **Status:** Under active development, built in phases. See
> [`implementation-plan.md`](./implementation-plan.md) for the roadmap. Several
> top-level directories are scaffolding that fills in as the build progresses
> (noted below).

## Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Tech Stack](#tech-stack)
- [Repository Structure](#repository-structure)
- [Local Development](#local-development)

## Overview

MedMarket lets a user:

1. **Upload a prescription** — stored as a file with metadata persisted in Postgres.
2. **Compare prices** — a workflow fans out to multiple pharmacy services
   concurrently and returns aggregated, sorted quotes.
3. **Place an order** — a saga coordinates payment, pharmacy ordering, and
   shipping registration, with automatic compensation on failure.
4. **Track shipping** — a long-running workflow follows the order from dispatch
   to delivery via webhook callbacks.

## Architecture

- **Hexagonal (ports & adapters).** The backend's domain and port interfaces
  (`backend/internal/domain`, `backend/internal/ports`) depend on nothing
  outward; adapters (Postgres, HTTP, external clients) implement those ports and
  are wired together at startup. This keeps business logic isolated from
  infrastructure.
- **Durable workflows (Temporal).** Price search, the order saga, and shipping
  tracking run as Temporal workflows — durable, observable, and resilient to
  partial failure.
- **Reverse proxy (Traefik).** Routes `/api` to the backend and `/` to the
  frontend, with service discovery via Docker labels.

## Tech Stack

| Area                  | Technology                                                        |
| --------------------- | ----------------------------------------------------------------- |
| Backend               | Go 1.26, [Ent](https://entgo.io) (ORM), [Atlas](https://atlasgo.io) (migrations) |
| Database              | PostgreSQL 17                                                      |
| Workflow orchestration| [Temporal](https://temporal.io)                                   |
| Frontend              | React 19, Vite, TypeScript                                         |
| Reverse proxy         | Traefik                                                           |
| Object storage (local)| MinIO (S3-compatible)                                             |
| Email (local)         | Mailpit (SMTP capture)                                            |
| Local orchestration   | Docker Compose                                                    |
| Deployment            | Google Kubernetes Engine (GKE), Terraform                        |
| Tooling               | [go-task](https://taskfile.dev) (task runner), golangci-lint      |
| Testing               | [testify](https://github.com/stretchr/testify), [testcontainers-go](https://golang.testcontainers.org) (integration) |

## Repository Structure

```
.
├── backend/        Go API — hexagonal architecture (domain, ports, adapters)
├── frontend/       React + Vite + TypeScript web app
├── services/       Mock external services (pharmacy, shipping) — placeholder
├── workflows/      Temporal workflow & activity definitions — placeholder
├── worker/         Temporal worker process — placeholder
├── k8s/            Kubernetes manifests (base + overlays) — placeholder
├── terraform/      Infrastructure as code for GKE — placeholder
├── .github/        CI/CD workflows — placeholder
├── docker-compose.yml   Full local stack (Traefik, backend, frontend, Postgres)
├── go.work         Go workspace tying the Go modules together
├── Taskfile.yml    Task runner entry point (see `task --list`)
└── .golangci.yml   Linter configuration (applies across Go modules)
```

Directories marked *placeholder* currently hold a `.gitkeep` and are populated
as their corresponding phase of the build lands.

## Local Development

> A full getting-started guide will expand here as the stack matures. The
> essentials that work today:

**Prerequisites:** Docker + Docker Compose, Go 1.26, Node 24, and
[go-task](https://taskfile.dev).

Start the full local stack:

```sh
task up          # docker compose up --build
```

Then:

- Frontend — <http://localhost>
- Backend health check — <http://localhost/api/health>
- Traefik dashboard — <http://localhost:8080>
- PostgreSQL — `localhost:5433` (remapped from 5432 to avoid clashing with a
  local Postgres)

Common tasks (run `task --list` for the full set):

```sh
task down                       # stop the stack
task backend:check              # build + vet + test + lint the backend
task backend:test               # run backend unit tests
task backend:test-integration   # run integration tests (needs Docker; uses testcontainers)
```
