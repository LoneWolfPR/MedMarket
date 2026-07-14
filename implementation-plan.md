# MedMarket Platform — Implementation Plan

## Project Overview

A web platform for an imaginary company that helps users find the best prescription medication prices across partner pharmacies. Users upload prescriptions, the system searches multiple pharmacy APIs for pricing, facilitates purchase with a markup, places orders with the selected pharmacy, and tracks shipping via webhook-driven status updates.

This plan is structured as a 14-day daily project designed to build software architecture skills progressively, targeting the weakest skill areas earliest while maintaining momentum toward a deployable system.

---

## Architecture Summary

### System Components (Local Docker Compose)

| Container | Purpose |
|-----------|---------|
| `traefik` | Reverse proxy, routes requests to frontend/backend by path |
| `frontend` | React + Vite app (dev server locally, nginx in prod) |
| `backend` | Go API server, hexagonal architecture |
| `postgres` | Primary database for all application data |
| `temporal` | Temporal server (workflow orchestration) |
| `temporal-db` | Postgres instance dedicated to Temporal |
| `temporal-ui` | Temporal's web UI for workflow visibility |
| `mock-pharmacy-a` | Mock pharmacy API — its own API shape, pricing/latency behavior |
| `mock-pharmacy-b` | Mock pharmacy API — a deliberately *different* API shape, catalog/error behavior |
| `mock-shipping` | Mock shipping service with webhook callbacks |
| `minio` | S3-compatible object storage for prescription uploads |
| `mailpit` | Local email capture with web UI |

### Monorepo Structure

```
medmarket/
├── docker-compose.yml
├── docker-compose.prod.yml
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── deploy.yml
├── terraform/
│   ├── main.tf
│   ├── variables.tf
│   ├── environments/
│   │   ├── staging.tfvars
│   │   └── production.tfvars
│   └── modules/
│       ├── gke/
│       ├── artifact-registry/
│       └── networking/
├── k8s/
│   ├── base/
│   └── overlays/
│       ├── staging/
│       └── production/
├── backend/
│   ├── go.mod
│   ├── go.work (workspace root)
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   ├── internal/
│   │   ├── domain/          # Pure domain types and business logic
│   │   │   ├── user/
│   │   │   ├── prescription/
│   │   │   ├── order/
│   │   │   └── pharmacy/
│   │   ├── ports/            # Interface definitions
│   │   │   ├── inbound/      # Service interfaces (what the app offers)
│   │   │   └── outbound/     # Repository/external service interfaces
│   │   ├── adapters/
│   │   │   ├── inbound/
│   │   │   │   └── http/     # HTTP handlers (REST API)
│   │   │   └── outbound/
│   │   │       ├── postgres/  # Ent-based repository implementations
│   │   │       ├── pharmacy/  # Pharmacy API client adapters
│   │   │       ├── storage/   # MinIO/GCS file storage adapter
│   │   │       ├── email/     # Email sending adapter
│   │   │       └── shipping/  # Shipping service adapter
│   │   ├── app/              # Application services (orchestration)
│   │   └── config/           # Configuration loading
│   ├── ent/
│   │   └── schema/           # Ent schema definitions
│   └── migrations/           # Atlas migration files
├── frontend/
│   ├── package.json
│   ├── vite.config.ts
│   ├── src/
│   │   ├── components/
│   │   ├── pages/
│   │   ├── hooks/
│   │   ├── api/
│   │   └── context/
│   ├── e2e/                  # Playwright tests
│   └── Dockerfile
├── services/                 # Throwaway mocks — Express/Node, not Go
│   ├── mock-pharmacy-a/       # REST/JSON API
│   │   ├── package.json
│   │   ├── server.js
│   │   ├── catalog.json
│   │   └── Dockerfile
│   ├── mock-pharmacy-b/       # GraphQL API (deliberately different shape)
│   │   ├── package.json
│   │   ├── server.js
│   │   ├── catalog.json
│   │   └── Dockerfile
│   └── mock-shipping/
│       ├── package.json
│       ├── server.js
│       └── Dockerfile
├── workflows/                # Temporal workflow definitions
│   ├── go.mod
│   ├── price_search.go
│   ├── order_placement.go
│   └── shipping_tracker.go
└── worker/                   # Temporal worker process
    ├── go.mod
    ├── cmd/
    │   └── worker/
    │       └── main.go
    └── Dockerfile
```

### Hexagonal Architecture — Conceptual Map

```
                    ┌─────────────────────────────┐
    HTTP Request ──▶│  Inbound Adapter (HTTP/REST) │
                    └──────────┬──────────────────┘
                               │ calls
                    ┌──────────▼──────────────────┐
                    │    Inbound Port (Interface)   │
                    │  e.g. PrescriptionService     │
                    └──────────┬──────────────────┘
                               │ implemented by
                    ┌──────────▼──────────────────┐
                    │    Application Service        │
                    │  (orchestrates domain logic)  │
                    └──────────┬──────────────────┘
                               │ uses
                    ┌──────────▼──────────────────┐
                    │    Domain Layer               │
                    │  (pure business rules,        │
                    │   no external dependencies)   │
                    └──────────┬──────────────────┘
                               │ depends on
                    ┌──────────▼──────────────────┐
                    │   Outbound Port (Interface)   │
                    │  e.g. PharmacyClient,          │
                    │       UserRepository          │
                    └──────────┬──────────────────┘
                               │ implemented by
                    ┌──────────▼──────────────────┐
                    │   Outbound Adapter            │
                    │  (Ent/Postgres, MinIO, HTTP   │
                    │   clients for pharmacies)     │
                    └─────────────────────────────┘
```

The key rule: **domain and ports never import adapters.** Dependencies always point inward. The domain layer has zero knowledge of Ent, HTTP, MinIO, or any external technology. This is what makes the architecture testable and swappable.

### Networking (Local)

Traefik sits at the front on ports 80/443 and routes by path prefix:

- `/` → frontend (React dev server on port 5173)
- `/api/` → backend (Go server on port 8080)
- `/temporal/` → Temporal UI (port 8233)
- `/mail/` → Mailpit UI (port 8025)
- `/minio/` → MinIO console (port 9001)

All inter-service communication happens over a shared Docker network. Backend reaches mock services, Temporal, Postgres, MinIO, and Mailpit by their Docker Compose service names (e.g., `http://mock-pharmacy-a:8080`).

---

## Daily Implementation Plan

### Day 1 — Project Scaffolding + Docker Compose Foundation

**Goal:** A running Docker Compose stack with Traefik routing to a hello-world Go API and a skeleton React app.

**Skills targeted:** Docker (2→3), Traefik (1→2), Vite (1→2)

**Tasks:**

1. Initialize the monorepo: `git init`, create the top-level directory structure, `go.work` workspace file
2. Create a minimal Go HTTP server in `backend/cmd/server/main.go` — a single `/api/health` endpoint returning `{"status": "ok"}`
3. Scaffold the React app with `npm create vite@latest` in the `frontend/` directory (TypeScript template)
4. Write Dockerfiles:
   - Backend: simple Go build (`FROM golang:1.26 AS builder` → `FROM gcr.io/distroless/static-debian12`). Use multi-stage even now — learn it early
   - Frontend dev: `FROM node:24-alpine`, runs `npm run dev` with Vite's `--host` flag
5. Write `docker-compose.yml`:
   - Traefik with dashboard enabled, Docker provider auto-discovery
   - Backend service with Traefik labels for `/api/` routing
   - Frontend service with Traefik labels for `/` routing
   - A shared `medmarket` Docker network
6. Add Postgres service (official `postgres:17` image), environment variables for credentials, a named volume for data persistence
7. Verify: `docker compose up` and confirm you can hit the React app at `http://localhost` and the Go health endpoint at `http://localhost/api/health`

**Key concepts to understand:**
- Traefik label-based routing (`traefik.http.routers.*`, `traefik.http.services.*`)
- Docker Compose networks (why services can reach each other by name)
- Multi-stage Docker builds (why the final image doesn't need the Go toolchain)
- Volume mounts for local development (so code changes reflect without rebuilding)

**Checkpoint:** Browser shows the Vite default page at `/`, JSON health response at `/api/health`, Traefik dashboard accessible at `localhost:8080` (Traefik's default dashboard port, not to be confused with your backend — configure accordingly).

---

### Day 2 — Database Layer + Hexagonal Architecture Skeleton

**Goal:** Deliver the `user` domain as a full **vertical slice** — domain → ports → Ent/Postgres adapter → HTTP → JWT — to validate the hexagonal wiring end-to-end and reach a working registration/login endpoint. The `prescription`, `order`, and `pharmacy` domains are **deferred to the days their features land** (Days 3–6), so we don't model them before we understand their flows (avoids premature modeling — `Order` in particular gets reshaped by the Day 6 saga work). Each deferred domain has an explicit task on its day (see Days 3, 4, 6).

**Skills targeted:** Hexagonal architecture (1→2), Atlas migrations (2→3)

**Tasks:**

1. Set up the hexagonal architecture directory structure under `backend/internal/` — `domain/`, `ports/inbound/`, `ports/outbound/`, `adapters/inbound/http/`, `adapters/outbound/postgres/`, `app/`
   - **Project tooling:** add a root `Taskfile.yml` (go-task) that consolidates common commands, with per-module Taskfiles pulled in via `includes` (e.g. `task up`, `task backend:lint`). Grow it with `generate` and `migrate:*` targets as Ent/Atlas land.
2. Define the `user` domain types (plain Go structs, no adapter dependencies):
   - `domain/user/` — User entity; value objects for email + password hash (each with a validating constructor + `IsZero`); `NewUser` constructor enforcing entity invariants
   - `domain/shared/` — cross-cutting value objects (e.g. Address)
   - Other domains (`prescription`, `order`, `pharmacy`) are deferred to their feature days (Day 3+) — see Goal
3. Define outbound port interfaces:
   - `ports/outbound/user_repository.go` — `Create`, `GetByID`, `GetByEmail`
   - Keep other repository interfaces as stubs for now
4. Define inbound port interfaces:
   - `ports/inbound/user_service.go` — `Register`, `Login`, `GetProfile`
5. Create the Ent schema for `User` only (`ent/schema/`); other entities are added on their feature days
   - Run `go generate ./ent` to generate the Ent client code
6. Create the first Atlas migration (`atlas migrate diff init --env local`) — just the `users` table for now; later domains get their own migrations as they land
   - Configure Atlas to diff against your Ent schema
   - Apply the migration to your Dockerized Postgres (host `localhost:5433` — remapped from 5432 to avoid the local Postgres clash)
7. Implement the Postgres adapter for the user repository using Ent
8. Implement the application service for user registration (hash password with bcrypt, call repository)
9. Wire up the HTTP adapter **spec-first with OpenAPI**: define routes/schemas in `backend/api/api.yaml`, generate models + a strict server interface with `oapi-codegen` (`task backend:generate-api`), then implement the generated `StrictServerInterface` in the `AuthHandler` — `POST /api/auth/register`, `POST /api/auth/login` (returns a JWT), `GET /api/auth/profile`. Generated code lives in its own `internal/adapters/inbound/http/openapi` package; hand-written handlers map generated DTOs ↔ domain and map domain/service errors to the typed response objects (no route wiring by hand).
10. Add JWT middleware that validates tokens and injects user info into the request context — modeled as the `bearerAuth` security scheme in the spec and implemented as a `StrictMiddlewareFunc`; requires a `Verify` method on the `TokenIssuer` outbound port.

**Key concepts to understand:**
- Why interfaces are defined in `ports/` and implementations live in `adapters/` — the domain dictates what it needs, adapters fulfill it
- Dependency injection: the `main.go` wires everything together, creating adapters and passing them to application services
- Ent's code generation model — schemas define the graph, `go generate` creates the typed client
- Atlas declarative migrations vs. Ent's built-in auto-migration (Atlas gives you version-controlled, reviewable migration files)
- Spec-first HTTP: the OpenAPI `api.yaml` is the source of truth; routes/types/server interface are generated (like Ent/Atlas), handlers only supply the business logic — DTO↔domain mapping and error→response-type decisions

**Checkpoint:** You can register a user via `curl -X POST localhost/api/auth/register`, log in to get a JWT, and hit a protected `/api/auth/profile` endpoint with the token.

---

### Day 3 — Mock Pharmacy + Shipping Services

**Goal:** Two mock pharmacy APIs and one mock shipping service running as separate containers, reachable from the backend.

**Skills targeted:** Docker (2→3), hexagonal architecture (1→2), API-shape design

**Note (deviation from original plan):** the mocks are **Express/Node services**, not Go. They exist only to give the backend something to query, so they're kept as simple as possible: a single `server.js` each with a static JSON catalog (no DB). The real work — and the hexagonal payoff — is the per-pharmacy Go **adapters** (Day 4/5) that translate each distinct API onto the one `PharmacyClient` port. Claude implemented all three mocks; the user reviews.

**Tasks:**

1. Build `mock-pharmacy-a` — an Express **REST/JSON** service:
   - `POST /api/v1/search` accepts `{ drug, strength }`, returns quotes with money as **integer cents**; simulated 200-500ms delay
   - `POST /api/v1/order` accepts `{ sku, quantity, shipping }`, returns a confirmation with a tracking ID
   - `X-Api-Key` header auth; fixed catalog of ~16 common medications with pricing randomized within realistic ranges per request
   - Simulates occasional errors (~5% of requests return HTTP 500)
2. Build `mock-pharmacy-b` — a **deliberately different API shape**: an Express **GraphQL** service (`POST /graphql`). Real pharmacies don't share a contract; each backend adapter's job is to translate its pharmacy's API into the standard `PharmacyClient` port, and the mocks must differ enough to make that translation real:
   - GraphQL, not REST: `medications(name, strength)` query + `placeOrder(input)` mutation
   - Money as **decimal-string dollars** (`"12.99"`), not cents; `Bearer` token auth, not an API key
   - Failures surface as GraphQL `errors` (~10%), not HTTP 500 — a different latency profile (100-300ms)
   - Slightly different catalog (overlapping but not identical to A)
3. Build `mock-shipping` service (Express):
   - `POST /api/register-webhook` registers `{ trackingId, callbackUrl }` (the backend calls this after an order is placed — the pharmacy and shipping mocks stay decoupled; the backend orchestrates)
   - On registration, walks a compressed timeline (picked_up → in_transit → out_for_delivery → delivered over ~60s, interval env-tunable for tests) POSTing a webhook to the callback URL at each stage
   - `GET /api/status/:trackingId` returns current status + history
4. Write Dockerfiles for all three services (node:24-alpine)
5. Add all three to `docker-compose.yml` with Traefik labels. They're only accessed by the backend over the Docker network (`http://mock-pharmacy-a:8080` etc.); the Traefik `Host(...)` rules (`pharmacy-a.localhost`, `pharmacy-b.localhost`, `shipping.localhost`) are only for host-side inspection — Host rules, not path prefixes, so they don't collide with the backend's `PathPrefix(/api)`
6. Define the `pharmacy` domain in the backend (Pharmacy entity, `PriceQuote` value object — **deferred from Day 2**), then define the **single** outbound port interfaces: `PharmacyClient` (its methods return `PriceQuote`), `ShippingClient`. One port; each pharmacy gets its own adapter (Day 4) that maps its distinct API onto this port.
7. Test each mock service independently with curl to verify behavior

**Key concepts to understand:**
- Adapters absorb API differences — the two pharmacy mocks expose *different* APIs on purpose; the single `PharmacyClient` port is what lets the service stay pharmacy-agnostic while a per-pharmacy adapter translates each upstream shape. This is the core hexagonal payoff.
- Why mock services should simulate real-world failure modes (timeouts, errors, partial data) rather than always returning happy-path responses
- Docker Compose service discovery — your backend will call `http://mock-pharmacy-a:8080` and `http://mock-pharmacy-b:8080`

**Checkpoint:** You can curl each mock service from your host (via Traefik) and see different prices/behaviors. The shipping mock sends webhook callbacks to a test endpoint.

---

### Day 4 — MinIO, Mailpit, and File/Email Adapters

**Goal:** Prescription file upload working end-to-end (upload → stored in MinIO → metadata in Postgres), and email sending captured by Mailpit.

**Skills targeted:** Hexagonal architecture (2→3), Docker (2→3)

**Tasks:**

1. Add MinIO and Mailpit containers to `docker-compose.yml`
   - MinIO: expose API port (9000) and console port (9001), configure default credentials via environment variables, create a `prescriptions` bucket on startup
   - Mailpit: expose SMTP port (1025) and web UI port (8025)
2. Define the `FileStorage` outbound port interface:
   - `Upload(ctx, bucketName, objectName, reader, size) (string, error)` — returns the object URL/key
   - `GetPresignedURL(ctx, bucketName, objectName, expiry) (string, error)` — for download links
   - `Delete(ctx, bucketName, objectName) error`
3. Implement the MinIO adapter for `FileStorage` using the `minio-go` SDK
4. Define the `EmailSender` outbound port interface:
   - `Send(ctx, to, subject, htmlBody) error`
5. Implement the SMTP adapter for `EmailSender` (standard Go `net/smtp` package, pointed at Mailpit)
6. Build the prescription upload flow:
   - First define the `prescription` domain (Prescription entity — **deferred from Day 2**)
   - `POST /api/prescriptions/upload` — multipart form upload, protected by JWT middleware
   - Application service validates the file type (images and PDFs only), uploads to MinIO, creates a Prescription record in Postgres with the storage key
   - `GET /api/prescriptions` — lists user's prescriptions with presigned download URLs
7. Add Ent schema updates and a new Atlas migration for the Prescription entity if not already complete
8. Test the full flow: upload a test image via curl, verify it appears in MinIO console, verify the Postgres record exists, verify the presigned URL downloads the file

**Key concepts to understand:**
- Presigned URLs — why you generate temporary download links rather than proxying file content through your API
- The adapter pattern in action — your application service calls `fileStorage.Upload()` without knowing whether it's MinIO, S3, or GCS behind the interface
- Why email goes through an interface even locally — in production you'll swap the SMTP adapter for a SendGrid/Mailgun adapter without changing any application code

**Checkpoint:** Upload a file at `/api/prescriptions/upload`, see it in MinIO console at `localhost/minio/`, list prescriptions and download via presigned URL. Send a test email and see it in Mailpit at `localhost/mail/`.

---

### Day 5 — Temporal Integration: Price Search Workflow

**Goal:** Temporal server running, a price search workflow that fans out to all pharmacy adapters and returns aggregated results.

**Skills targeted:** Temporal (3→4), hexagonal architecture (2→3)

**Tasks:**

1. Add Temporal to `docker-compose.yml`:
   - `temporalio/server` with auto-setup (uses its own Postgres instance)
   - `temporalio/ui` for the web dashboard
   - A separate `temporal-db` Postgres container (Temporal needs its own database)
   - Route Temporal UI through Traefik at `/temporal/`
2. Set up the Go workspace: `workflows/` module for workflow/activity definitions, `worker/` module for the worker process
3. Implement the price search workflow:
   - Input: medication name, dosage, user ID
   - Activities: `SearchPharmacy` — calls a single pharmacy adapter and returns a quote (or error)
   - Workflow logic: fan out `SearchPharmacy` activities to all registered pharmacies concurrently, collect results with a timeout (e.g., 5 seconds — if a pharmacy doesn't respond, proceed without it), sort by price, return aggregated results
   - Handle partial failures gracefully — if one pharmacy errors, still return results from the others
4. Implement **one pharmacy adapter per mock** for the backend — a separate HTTP client for pharmacy-a and pharmacy-b, each translating its pharmacy's distinct API into the standard `PharmacyClient` outbound port
5. Build the Temporal worker: registers workflows and activities, connects to Temporal server
6. Add the worker as a container in `docker-compose.yml`
7. Wire up the API endpoint: `POST /api/prescriptions/:id/search` triggers the workflow, returns a workflow ID. `GET /api/prescriptions/:id/search/:workflowId` polls for results (or use a simple synchronous wait with timeout for now)
8. Test: trigger a price search, watch it execute in Temporal UI, verify you get prices back from both pharmacy mocks

**Key concepts to understand:**
- Temporal's execution model — workflows are durable, deterministic functions; activities are where side effects happen
- Why you fan out via activities rather than goroutines — Temporal tracks each activity's state, retries failures, and provides visibility
- Activity retry policies — configure max attempts, backoff, non-retryable error types
- The worker is just a host process that polls Temporal for tasks — it doesn't receive inbound requests

**Checkpoint:** Trigger a price search, see the workflow in Temporal UI with individual activity completions for each pharmacy, get aggregated price results back through the API.

---

### Day 6 — Order Placement Workflow (Saga Pattern)

**Goal:** A complete order workflow: charge user → place pharmacy order → register shipping webhook → handle compensation on failure.

**Skills targeted:** Temporal (3→4), saga pattern, hexagonal architecture (2→3)

**Tasks:**

1. Design the order placement saga with explicit compensation:
   - Step 1: Validate and reserve the order (create Order record with `pending` status)
   - Step 2: Authorize payment via **Stripe test mode** — create + confirm a manual-capture PaymentIntent (places a hold; no funds captured yet). Real Stripe API, test keys, no real money moves. See "Payments" note below.
   - Step 3: Place order with the selected pharmacy API
   - Step 4: Register webhook with the shipping service
   - Step 5: **Capture** the Stripe PaymentIntent (settle the authorized hold), update order status to `confirmed`, send confirmation email
   - Compensation: if step 3 or 4 fails → **cancel the PaymentIntent** (void the authorization — cheaper/cleaner than a refund since nothing was captured), update order to `failed`. If a failure occurs *after* capture, fall back to a Stripe **Refund**.
2. Implement each step as a Temporal activity
3. Implement the saga workflow with compensation logic
4. Define the `order` domain (Order entity, `OrderStatus` enum, OrderItem — **deferred from Day 2**), then add order-related Ent schemas and Atlas migration (Order, OrderItem with relations to User, Prescription, selected pharmacy)
5. Build the API endpoints:
   - `POST /api/orders` — accepts prescription ID, selected pharmacy quote, and a Stripe PaymentMethod id (test-mode, e.g. `pm_card_visa`); starts the order workflow
   - `GET /api/orders` — list user's orders
   - `GET /api/orders/:id` — order detail with current status
6. Implement the shipping webhook receiver on the backend:
   - `POST /api/webhooks/shipping` — receives status updates from the mock shipping service
   - Uses Temporal signals to notify the shipping tracker workflow (which you'll build on Day 7, but stub the signal receiver now)
   - Updates order status in the database
   - Sends status update email to user via the email adapter
7. Test the happy path end-to-end: search → select pharmacy → place order → watch saga complete in Temporal UI → receive confirmation email in Mailpit
8. Test failure scenarios: make one pharmacy return errors, verify compensation executes correctly

**Key concepts to understand:**
- Saga pattern — a sequence of transactions where each step has a compensating action for rollback
- Why sagas exist instead of distributed transactions — in a microservices world you can't wrap multiple services in a single database transaction
- Temporal signals — external events (like webhook callbacks) that influence a running workflow
- Idempotency — your activities should be safe to retry (Temporal may replay them)

**Payments (design decision, 2026-07-14):** Use **Stripe test mode** (real Stripe API, `sk_test_…` keys) instead of a pure mock — test transactions are free, unlimited, and move no real money; a free Stripe account with no business activation is enough. Modeled hexagonally: a `PaymentGateway` outbound port (`Authorize`/`Capture`/`Cancel`/`Refund`) with a `stripe` adapter, structurally identical to `PharmacyClient` — the domain never imports Stripe. `STRIPE_SECRET_KEY` injected as a per-adapter env var at the composition root (same pattern as pharmacy secrets → GKE Secret later). **All Stripe calls happen inside Temporal activities, never the workflow** (side effects + nondeterminism), and **every activity passes a Stripe idempotency key** — Temporal retries activities, so the key is what prevents a retry from double-charging (the Temporal-retry ↔ Stripe-idempotency pairing is a deliberate learning target). Auth-then-capture (manual-capture PaymentIntent) is chosen over charge-then-refund so saga compensation is a cheap authorization *void*, not a settled-money reversal.

**Checkpoint:** Full order flow works. Force a failure at step 3, verify the Stripe authorization gets voided (check the Stripe test dashboard) and order marked failed. Check Mailpit for confirmation and status emails.

---

### Day 7 — Shipping Workflow + Remaining Backend Polish

**Goal:** Shipping tracking as a long-running Temporal workflow, and clean up any rough edges in the backend.

**Skills targeted:** Temporal (3→4), Go testing (2→3)

**Tasks:**

1. Implement the shipping tracker workflow:
   - Starts when an order is confirmed
   - Listens for signals from the webhook receiver (status updates)
   - Has a timeout — if no update received within X minutes (compressed for demo), trigger an alert/notification
   - On final `delivered` status, send delivery confirmation email, update order
   - Workflow can run for days in production (minutes in your demo) — this is Temporal's sweet spot
2. Connect the webhook endpoint to signal this workflow
3. Write unit tests for domain logic:
   - Price comparison and markup calculation
   - Order validation rules
   - User registration validation
4. Write integration tests for the database adapters:
   - Use a test Postgres container (testcontainers-go or a separate compose profile)
   - Test CRUD operations for each repository
5. Write integration tests for the Temporal workflows:
   - Use Temporal's test framework (`go.temporal.io/sdk/testsuite`)
   - Test the price search workflow with mocked activities
   - Test the order saga with simulated failures at each step
6. Set up `golangci-lint` configuration (`.golangci.yml`) — enable useful linters beyond the defaults: `errcheck`, `govet`, `staticcheck`, `unused`, `gosimple`, `ineffassign`, plus style linters like `gofmt`, `goimports`
7. Fix any linting issues across the codebase
8. Document the API — the OpenAPI spec (`backend/api/api.yaml`, spec-first since Day 2) is the source of truth; ensure every endpoint added since is defined there, and optionally serve it (Swagger UI / Redoc) or export it for consumers

**Checkpoint:** Shipping flow works end-to-end — place order, watch shipping status progress through webhook callbacks, see status emails in Mailpit, verify final delivery status. All tests pass, linter is clean.

---

### Day 8 — React Frontend: Auth + Layout

**Goal:** Working React app with authentication flow, route protection, and the main application shell.

**Skills targeted:** React (3, refreshing), Vite (1→2)

**Tasks:**

1. Set up the React project structure:
   - Install dependencies: `react-router-dom`, a lightweight HTTP client (native `fetch` with a wrapper is fine), and a component library of your choice (or go headless/minimal — up to you)
   - Set up Vite proxy config to forward `/api` requests to the backend through Traefik (or directly, depending on your compose networking)
2. Build the auth layer:
   - `AuthContext` provider wrapping the app — stores JWT, user info, provides `login`/`register`/`logout` functions
   - Token stored in memory (not localStorage for security, but localStorage is fine for a learning project — just know the tradeoff)
   - Axios/fetch interceptor that attaches the JWT to all API requests
3. Build the pages:
   - Login page with form
   - Registration page with form
   - Protected route wrapper component that redirects to login if no token
4. Build the main application shell:
   - Navigation bar with user info and logout
   - Sidebar or top-nav with links: Dashboard, Prescriptions, Orders, Profile
   - Layout component that wraps all authenticated pages
5. Set up ESLint and Vitest:
   - ESLint config (you're at 5 here, so this should be quick)
   - Vitest config in `vite.config.ts`
   - Write a few component tests to verify the testing setup works
6. Verify hot module replacement works through the Docker/Traefik setup — editing a component should reflect in the browser without a full reload

**Checkpoint:** You can register and log in through the React UI. Authenticated routes are protected. Navigation works. HMR works through the Docker stack.

---

### Day 9 — React Frontend: Core Features

**Goal:** All user-facing features implemented: prescription upload, price search, order placement, order tracking.

**Skills targeted:** React (3→4), REST API consumption

**Tasks:**

1. Prescription management:
   - Upload page with drag-and-drop file input (images/PDFs)
   - Prescription list page showing uploaded prescriptions with download links
   - Prescription detail view
2. Price search flow:
   - From a prescription, trigger a price search
   - Loading state while the Temporal workflow runs
   - Results display: pharmacy name, price, estimated delivery, comparison view sorted by price
   - Highlight the best deal, show your markup transparently
3. Order placement:
   - Select a pharmacy quote, enter mock payment info (fake credit card form — no real payment processing)
   - Order confirmation page
   - Handle loading/error states during the saga
4. Order tracking:
   - Order list page with status badges
   - Order detail page showing current shipping status, status history timeline
   - Poll for updates or implement a simple polling mechanism (WebSockets would be better but polling is fine for the timeline)
5. User profile page:
   - View/edit basic profile information
   - Prescription history
   - Order history
6. Write Vitest tests for critical components — at minimum the auth context, the price comparison display logic, and the order flow

**Checkpoint:** Complete user journey works through the UI: register → login → upload prescription → search prices → place order → track shipping. All states (loading, error, empty, populated) are handled in the UI.

---

### Day 10 — End-to-End Testing + CI Pipeline

**Goal:** Playwright tests covering the critical path, GitHub Actions CI running tests and linting on every push.

**Skills targeted:** Playwright (1→2), GitHub Actions, testing strategy

**Tasks:**

1. Set up Playwright:
   - Install and configure Playwright in the `frontend/e2e/` directory
   - Configure it to run against the full Docker Compose stack
   - Write a `docker-compose.test.yml` override that starts the stack in test mode (seeded database, deterministic mock service behavior, faster shipping simulation)
2. Write E2E tests for the critical user journey:
   - Test 1: Registration → Login → Verify dashboard loads
   - Test 2: Upload prescription → Verify it appears in list
   - Test 3: Search prices → Verify results from both pharmacies appear
   - Test 4: Place order → Verify order confirmation → Verify status updates appear
   - Keep these tests focused on the happy path — E2E tests are expensive and brittle when they try to cover edge cases
3. Set up GitHub Actions CI pipeline (`.github/workflows/ci.yml`):
   - Trigger on push to `main` and on pull requests
   - Job 1: Backend — run `golangci-lint`, run Go unit tests, run Go integration tests (needs a Postgres service container)
   - Job 2: Frontend — run ESLint, run Vitest
   - Job 3: E2E — build all Docker images, start the full compose stack, run Playwright, upload test artifacts (screenshots, traces) on failure
   - Use GitHub Actions service containers for Postgres in the backend job
   - Cache Go modules and npm packages across runs
4. Push to GitHub and verify the full CI pipeline passes
5. Optional: add a simple PR template that includes a checklist (tests pass, linter clean, migration reviewed)

**Key concepts to understand:**
- The testing pyramid — unit tests (many, fast, cheap) → integration tests (moderate, slower) → E2E tests (few, slow, expensive). Your E2E tests should cover workflows, not individual UI behaviors
- CI as a gatekeeper — nothing merges without green CI
- Why E2E tests run against the full Docker Compose stack — they're testing the system, not individual components

**Checkpoint:** All tests pass locally. Push to GitHub, CI pipeline runs all three jobs successfully. Green check on the commit.

---

### Day 11 — Production Docker Builds + Artifact Registry

**Goal:** Production-optimized Docker images built and pushed to Google Artifact Registry.

**Skills targeted:** Docker (2→4), GCP services

**Tasks:**

1. Optimize your Dockerfiles for production:
   - Backend: multi-stage build, compile with CGO_DISABLED=1, copy binary into `distroless/static` or `scratch` — the final image should be ~15-20MB
   - Frontend: multi-stage build — stage 1 runs `npm run build`, stage 2 copies the built static files into an `nginx:alpine` image with a custom nginx config that handles SPA routing (all non-file routes serve `index.html`)
   - Mock services: same pattern as the backend
   - Worker: same base as the backend (it's just another Go binary)
2. Create `docker-compose.prod.yml` as an override — uses built images instead of volume mounts, no Vite dev server, production environment variables, no debug ports exposed
3. Set up Google Artifact Registry:
   - Enable the Artifact Registry API in your GCP project
   - Create a Docker repository (e.g., `us-central1-docker.pkg.dev/YOUR_PROJECT/medmarket`)
   - Configure Docker auth: `gcloud auth configure-docker us-central1-docker.pkg.dev`
4. Tag and push your images:
   - Establish a tagging convention: `us-central1-docker.pkg.dev/YOUR_PROJECT/medmarket/backend:v0.1.0` and `:latest`
   - Push all images: backend, frontend, worker, mock-pharmacy-a, mock-pharmacy-b, mock-shipping
5. Test locally using the production images: `docker compose -f docker-compose.yml -f docker-compose.prod.yml up` — verify everything works with production builds
6. Document image sizes and build times — understanding your build efficiency matters architecturally

**Key concepts to understand:**
- Why multi-stage builds matter — a Go build environment is ~1GB, your production image should be <20MB
- Distroless images — minimal attack surface, no shell, no package manager
- Container registry as the bridge between CI and deployment — CI builds and pushes images, Kubernetes pulls them
- Image tagging strategy — why `:latest` alone is insufficient for production (no rollback, no auditability)

**Checkpoint:** All production images built, pushed to Artifact Registry, and verified running locally via the production compose override.

---

### Day 12 — Terraform + GKE Infrastructure

**Goal:** GKE cluster and supporting infrastructure provisioned via Terraform.

**Skills targeted:** Terraform (1→3), GKE (1→2)

**Tasks:**

1. Set up Terraform:
   - Install Terraform locally if not already present
   - Initialize the `terraform/` directory with provider configuration (Google provider)
   - Configure the GCP backend for state storage (a GCS bucket — create this manually first as a one-time bootstrap, or use local state for simplicity given the timeline)
2. Define the GKE module:
   - Cluster with 2 e2-medium nodes (matching what you already have, or modify to match)
   - Node pool configuration: autoscaling 0-2 nodes (scale to zero when not in use to save credits)
   - Workload Identity enabled (best practice for pod-to-GCP-service auth)
   - Network policy enabled
3. Define the Artifact Registry module (or just reference the one you created manually on Day 11)
4. Define the networking module:
   - VPC network for the cluster
   - Firewall rules
5. Create environment-specific `.tfvars` files:
   - `staging.tfvars` and `production.tfvars` — for a single-cluster setup these mainly differ in namespace names and resource limits, but structuring them separately teaches you the pattern
6. Run `terraform plan` → review → `terraform apply`
7. Connect kubectl to the new cluster: `gcloud container clusters get-credentials`
8. Verify the cluster is running: `kubectl get nodes`

**Key concepts to understand:**
- Terraform state — what it tracks, why it matters, why remote state storage exists
- The plan/apply cycle — always review what Terraform intends to do before applying
- Infrastructure modules — reusable, composable units of infrastructure
- Environment separation via variables — same modules, different configuration

**Checkpoint:** GKE cluster running, kubectl connected, nodes healthy. Terraform state is clean and matches reality.

---

### Day 13 — Kubernetes Manifests + Deployment Pipeline

**Goal:** Application deployed to GKE in staging and production namespaces via a GitHub Actions workflow.

**Skills targeted:** GKE (1→3), Kubernetes, CI/CD, Terraform (1→3)

**Tasks:**

1. Create Kubernetes manifests in `k8s/base/`:
   - Deployments: backend, frontend, worker, mock-pharmacy-a, mock-pharmacy-b, mock-shipping
   - Services: ClusterIP for each deployment
   - ConfigMaps: application configuration per service
   - Secrets: database credentials, JWT secret, MinIO credentials (use kubectl create secret for now; in production you'd use Secret Manager)
   - Postgres StatefulSet with a PersistentVolumeClaim (remember: this is for learning, not production-grade database hosting)
   - Temporal StatefulSet with its own Postgres (or use Temporal Cloud free tier if available — otherwise self-host in-cluster)
   - Ingress resource for external traffic routing (GKE's built-in ingress controller or install Traefik via Helm for consistency with local dev)
2. Create Kustomize overlays in `k8s/overlays/`:
   - `staging/`: staging namespace, reduced replicas (1 each), resource limits tuned for small nodes
   - `production/`: production namespace, same replicas for now but structured for future scaling
   - Environment-specific ConfigMap patches
3. Deploy staging manually first:
   - `kubectl create namespace staging`
   - `kubectl apply -k k8s/overlays/staging/`
   - Debug any issues (image pull errors, resource constraints, networking)
   - Verify all pods are running: `kubectl get pods -n staging`
   - Test the application via the ingress IP/hostname
4. Build the GitHub Actions deployment workflow (`.github/workflows/deploy.yml`):
   - Trigger: manual dispatch with environment selection (staging/production), or auto-deploy to staging on merge to main
   - Steps: authenticate to GCP (Workload Identity Federation or service account key), build and push Docker images to Artifact Registry, apply Kustomize overlay to the target namespace
   - Add the deployment workflow to the repository
5. Deploy production:
   - `kubectl create namespace production`
   - Deploy via the GitHub Actions workflow or manually apply the production overlay
6. Verify both environments are accessible and functional

**Key concepts to understand:**
- Kubernetes resource model — Deployments manage ReplicaSets which manage Pods
- Services and service discovery — how pods find each other within the cluster
- Kustomize — layered configuration without templating (as opposed to Helm's template approach)
- Namespaces — logical isolation within a single cluster
- Resource limits — critical on small nodes to prevent OOM kills and pod eviction
- Ingress — how external traffic reaches your services

**Checkpoint:** Application running in both staging and production namespaces. GitHub Actions workflow can deploy to either environment. You can access the app via the GKE ingress IP.

---

### Day 14 — Integration Testing on GKE, Polish, and Retrospective

**Goal:** Verify everything works in the cloud, fix issues, document what you built and what you learned.

**Skills targeted:** All areas — this day is about integration and reflection

**Tasks:**

1. Run through the full user journey on the deployed staging environment:
   - Register, login, upload a prescription, search prices, place an order, receive shipping updates, receive emails (note: email in GKE requires a real SMTP service or you skip it in cloud — document this as an architectural decision)
2. Fix any deployment-specific issues:
   - Database connectivity between pods
   - Inter-service networking
   - Ingress routing
   - Resource constraints causing OOM or pod eviction
   - Image pull issues
3. Load test lightly — even just running the E2E flow 5-10 times concurrently helps surface issues that don't appear in single-user testing
4. Document the architecture:
   - Write a comprehensive `README.md` at the repo root
   - Include: architecture diagram (the one from this plan, refined), setup instructions for local development, deployment instructions, technology choices and rationale, what you'd do differently, what's missing for real production readiness
5. Write a personal retrospective — this is for your own learning:
   - What was harder than expected?
   - What concepts clicked during implementation vs. during planning?
   - Where did the hexagonal architecture help? Where did it feel like overhead?
   - What would you change about the Temporal workflow design?
   - If you were building this for real, what would you add (observability, rate limiting, caching, CDN, managed database, secrets management)?
6. Clean up GKE resources to stop spending credits:
   - Either `terraform destroy` to tear down the cluster entirely
   - Or scale the node pool to zero: `gcloud container clusters resize medmarket-cluster --num-nodes=0`
   - Verify in the GCP console that no compute resources are running

**Checkpoint:** Working application deployed and verified in the cloud. Documentation complete. GKE resources cleaned up. You have a portfolio-ready project demonstrating end-to-end architectural skills.

---

## Things Intentionally Left Out (and Why)

These are all valuable in production systems but would bloat a two-week learning project past its useful scope:

- **Observability (Prometheus, Grafana, OpenTelemetry):** Adds 3-4 containers and significant configuration. Worth learning separately. You already get some observability for free through Temporal's UI.
- **API Gateway features (rate limiting, API keys, request validation):** Traefik can do some of this, but configuring it properly is a rabbit hole. Not where you want to spend time on this project.
- **HTTPS/TLS locally:** Traefik supports it with self-signed certs, but debugging cert issues locally wastes time. Use HTTP locally, TLS terminates at the GKE ingress in cloud.
- **Secrets management (HashiCorp Vault, GCP Secret Manager):** The right way to handle secrets in Kubernetes. Using Kubernetes Secrets directly is the pragmatic choice here.
- **Database connection pooling (PgBouncer):** Important at scale, irrelevant with your traffic level.
- **CDN/static asset optimization:** Your frontend is tiny. Not worth the complexity.
- **Live payment processing (real money):** Out of scope, but payments are **not** mocked — Day 6 uses **Stripe test mode** (real API, test keys, no real funds) so the saga exercises a genuine external payment gateway. See the Day 6 Payments note.
- **Horizontal pod autoscaling:** Your 2-node cluster doesn't have the headroom to demonstrate this meaningfully.

Each of these would be a strong follow-up project once this foundation is solid.

---

## Daily Time Estimate

Most days involve 4-6 hours of focused work. Days 5, 6, and 13 are the heaviest and may push to 8 hours depending on how quickly you work through debugging. If you find yourself behind, the first things to cut are: the second Kustomize overlay (just do staging), comprehensive Playwright tests (keep only the registration + login flow), and the shipping tracker workflow (make it a simple status poller instead of a long-running Temporal workflow). The things you should never cut: the hexagonal architecture structure (it's the architectural core of the project), Temporal for at least the price search workflow, and the full CI/CD pipeline to GKE.

---

## Getting Started

On Day 1, start with:

```bash
mkdir medmarket && cd medmarket
git init
```

And build from there. Refer back to this plan daily — each day's section tells you exactly what to build, what concepts to focus on, and what the checkpoint looks like before moving on.
