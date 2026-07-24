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
- [Deployment](#deployment)

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

### Deliberate simplifications

- **Synchronous tracking id at order placement.** The mock pharmacies return a
  shipping tracking id in the `PlaceOrder` response, so the order saga can
  register the shipping webhook immediately. In a real medication-fulfillment
  API, placement and shipment creation are decoupled: the pharmacy confirms the
  order synchronously and a tracking id is assigned *later*, delivered by an
  async callback once the shipment exists. The saga already accommodates that
  model — because the **order id** (not the tracking id) is the workflow anchor
  and webhook address, the tracking id would simply arrive as another inbound
  signal handled in the selector loop, with no change to addressing or state.

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
| Payments              | [Stripe](https://stripe.com) (test mode — no real transactions)   |
| Local orchestration   | Docker Compose                                                    |
| Deployment            | Google Kubernetes Engine (GKE), Terraform                        |
| Tooling               | [go-task](https://taskfile.dev) (task runner), golangci-lint      |
| Testing               | [testify](https://github.com/stretchr/testify), [testcontainers-go](https://golang.testcontainers.org) (integration) |

## Repository Structure

```
.
├── backend/        Go API — hexagonal architecture (domain, ports, adapters)
├── frontend/       React + Vite + TypeScript web app
├── services/       Mock external services (pharmacy A/B, shipping) — Node/Express
├── workflows/      Temporal workflow & activity definitions — placeholder
├── worker/         Temporal worker process — placeholder
├── k8s/            Kubernetes manifests (Kustomize base + staging/production overlays)
├── terraform/      Infrastructure as code (GKE, Artifact Registry, GCS, Workload Identity)
├── .github/        CI/CD workflows (PR checks + deploy on merge to main)
├── docker-compose.yml   Full local stack (Traefik, backend, frontend, Postgres, mock services)
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

**Environment.** The stack reads secrets from a gitignored `.env` at the repo
root, which Compose substitutes into `docker-compose.yml`. The one you must set
yourself is `STRIPE_SECRET_KEY` — a Stripe **test-mode secret** key
(`sk_test_…`, *not* a publishable `pk_…` key); the payment worker fails to start
without it. Every other local credential is a dev placeholder baked into
`docker-compose.yml`.

```sh
# .env (repo root) — no quotes, no `export`, no spaces around `=`
STRIPE_SECRET_KEY=sk_test_your_key_here
```

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

### Testing

The backend is tested in three tiers, cheapest first:

| Tier        | Command                        | Needs           | Covers |
| ----------- | ------------------------------ | --------------- | ------ |
| Unit        | `task backend:test`            | nothing         | Domain logic, app services (with faked ports), and adapters. Fast and Docker-free. |
| Integration | `task backend:test-integration`| Docker          | Repositories against a real Postgres (testcontainers), the HTTP stack, and a **composition-root boot test** that serves the real `main` graph against a throwaway DB. Build tag `integration`. |
| Smoke       | `task backend:smoke`           | a running stack | The **full customer journey** (register → login → upload → search → order → status) against the live compose stack through Traefik — real Temporal, MinIO, mock pharmacies, and Stripe test mode. Build tag `smoke`. |

Only the unit tier runs in `task backend:check`, keeping it fast and
Docker-free; integration and smoke are run on their own. The smoke test is
end-to-end and has prerequisites:

- The stack must be up (`task up`) with `STRIPE_SECRET_KEY` set.
- It registers a throwaway user (unique email) each run, so it is safe to
  re-run and leaves no fixtures to clean up.
- Override the target with `SMOKE_BASE_URL` (defaults to `http://localhost`).

## Deployment

The application deploys to **Google Kubernetes Engine**. Infrastructure is
managed with Terraform (`terraform/`) and workloads with Kustomize (`k8s/`). A
step-by-step first-bring-up runbook lives in [`docs/deploy.md`](./docs/deploy.md);
this section is the model.

### Architecture

- **Keyless auth via Workload Identity.** No service-account keys exist anywhere
  (the org policy forbids them). In-cluster, pods impersonate Google service
  accounts through Workload Identity — the backend signs GCS presigned URLs via
  IAM `signBlob`, and the OpenTelemetry Collector writes to Cloud Trace / Cloud
  Logging — all without a key file. CI authenticates the same way, exchanging a
  GitHub OIDC token for short-lived GCP credentials (Workload Identity
  Federation).
- **Kustomize base + overlays.** `k8s/base/` holds every environment-agnostic
  resource; `k8s/overlays/staging/` layers on the namespace, Artifact Registry
  image names, the Workload Identity annotations, and the generated Secret.
  `k8s/overlays/production/` is a stub that proves the structure scales to a
  second environment (staging is the working target).
- **Self-hosted infra.** Postgres and Temporal's database run as StatefulSets
  with persistent disks; Temporal, the mock services, and Mailpit run in-cluster.
  Prescription files go to **GCS** (the keyless adapter replaces local MinIO);
  traces/logs go to **Cloud Trace / Cloud Logging** (replacing local Jaeger).
- **Ingress.** A single GKE HTTP load balancer routes `/api` to the backend and
  everything else to the frontend, replacing Traefik (which is local-dev only).

### Secrets

Secrets never enter git. The staging Kustomize overlay's `secretGenerator` reads
a gitignored `.env` (template: `k8s/overlays/staging/.env.example`), so the
source of the values depends on who runs the apply:

- **Manual / first bring-up** — a local `k8s/overlays/staging/.env` you fill in.
- **CI** — the values live in **GitHub Actions Secrets**; the deploy workflow
  reconstructs the `.env` on the ephemeral runner, then applies. Set:
  `POSTGRES_PASSWORD`, `DATABASE_URL`, `JWT_SECRET`, `STRIPE_SECRET_KEY`,
  `PHARMACY_A_SECRET`, `PHARMACY_B_SECRET`.

Do **not** reuse the local-dev placeholder values from `docker-compose.yml` —
they are committed and insecure. Generate fresh values for `JWT_SECRET` and
`POSTGRES_PASSWORD` in particular (e.g. `openssl rand -base64 48`).

> **Production-grade upgrade (not implemented).** At scale the source of truth
> would be **GCP Secret Manager**, synced into Kubernetes Secrets by the
> **External Secrets Operator** (or the Secret Manager CSI driver) — no `.env`,
> no secrets in CI, centralized rotation. The `.env` / GitHub-Secrets flow here
> is the pragmatic equivalent for a single-environment project.

### CI/CD

Two GitHub Actions workflows, split by trust boundary:

- **`ci.yml` (pull requests)** — deterministic, **credential-free** gates: Go
  build/vet/test/lint, Kustomize render, and `terraform validate`. These are the
  **required status checks** for branch protection on `main`, and they pair with
  a review from the Claude GitHub app: CI enforces the mechanical gates, the
  review covers judgment. A PR touches no cloud resources.
- **`deploy-staging.yml` (push to `main`)** — builds and pushes the six images to
  Artifact Registry (tagged with the commit SHA), pins those tags into the
  overlay, and applies to the staging namespace. Because direct pushes to `main`
  are blocked, this runs only after a reviewed PR merges. Only this workflow
  holds cloud credentials.
