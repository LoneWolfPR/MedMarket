# MedMarket

Prescription price comparison platform. Monorepo with Go backend, React frontend, mock external services. Built as a learning project — see "How we work" below, which governs everything.

## Documentation map

- `implementation-plan.md` (repo root) — the day-by-day plan.
- `docs/progress-log.md` — **the full chronological build log.** Every session, decision, and gotcha, in detail. Not loaded into context; read it (grep it) when you need the reasoning behind a past decision. **New session notes are appended there, not here** — this file holds only what still governs work, and its "Current state" below gets a couple of lines only when the headline changes.
- `docs/hexagonal.md` — the architecture rules, ratified in the 2026-08-04 consolidation review.
- `docs/design.md` — the visual spec Claude writes and the user implements.
- `docs/deploy.md` — first-bring-up runbook. `README.md` has environment setup + deliberate simplifications.
- `docs/interview-cheatsheet.md` — historical, from the 2026-08-11 interview prep.

Also check your most recent memories for context from previous conversations.

**Do not record commit status in this file** ("uncommitted", "not yet committed"). It is checked in, so any such claim is self-falsifying, and git answers the question authoritatively.

## How we work — the split

Claude does scaffolding and boilerplate that the user reviews; the user writes the code in the areas they are learning. The line is **"does it teach the thing I'm here to learn?"**, not "which directory is it in."

**Backend (Go):** the user writes the learning surface — domain, ports, adapters, app services, HTTP handlers, workflows, wiring. Claude does plumbing (dependency adds, Ent codegen, Atlas config, tooling), tests, walkthroughs, and reviews.

**Frontend (React/TS/Tailwind):** the user writes components, hooks, state, data flow, and types. Claude owns repetitive JSX/Tailwind/copy churn, accessibility wiring, config, and tests. (Amended 2026-08-20: the user handed back `aria-*` markup and required-field styling — "I'm not learning any React from this stuff.")

**Infra YAML (k8s/Terraform):** inverted — Claude writes each file, then explains it. Comprehension, not authorship, is the value there.

**Walkthroughs are just-in-time** — explain each genuinely-new concept as its bit of work starts, one paragraph at a time, then stop and ask. Prose and pointers, **never copy-pasteable implementation** for learning-surface code.

**Never edit a learning-surface file**, even reversibly, even to add an `export`. Ask. Every change should arrive as a reviewable editor diff.

**Design rationale does not go in source comments** — `docs/design.md` is where reasoning lives; markup just implements it. Comments in Claude's own test files are wanted.

## Current state

**Backend: feature-complete and deployed.** Auth, prescriptions (upload → GCS/MinIO → Postgres), pharmacy search via Temporal workflow, the full order saga (Stripe authorize → place → capture, shipping webhooks via signal, live status via query with Postgres fallback), OpenTelemetry traces + logs, Swagger UI, graceful shutdown, request logging. Hexagonal consolidation review complete. Deployed to GKE staging via keyless CI/CD (Workload Identity Federation); cluster is parked at 0 nodes when idle.

**Frontend: in progress.** Done: app shell + routing, auth context with JWT `exp` checking, protected routes, login, registration (RHF + Zod), prescriptions list + upload, TanStack Query, a not-found route, `formatCents`, Vitest suite (91 tests).

**In flight — price search → order placement** (`feat/search-and-order-updates`), the headline vertical slice and the last major gap in the UI. `/prescriptions/:id/search` holds a three-step state (`quotes` → `confirm` → `placed`). The quotes step is done; the confirm and placed steps are next, along with the profile query behind the address gate. Screen specs are in `docs/design.md`; the session detail is in `docs/progress-log.md`.

**Deferred / known work**

- **CI — path-filtered checks.** Backend and frontend jobs should only run when their own paths change. Use `dorny/paths-filter` inside an always-running job and gate the expensive steps on its output; **not** workflow-level `on.pull_request.paths`, which leaves a required status check pending forever on protected `main` and makes the PR unmergeable.
- **Frontend — `openapi-typescript` generation** from `backend/api/api.yaml` to replace the hand-written `src/api/types.ts`.
- **Frontend — `<Field>` component extraction** (eleven near-identical label/input/error blocks; unblocked now that the field anatomy has settled), and `Login`'s aria wiring, which waits on it.
- **Frontend — test coverage is starter-level.** `useAuthedApi` (including its 401 branch) and `Prescriptions.tsx` have none.
- **Nice-to-have after everything else:** a local logs-with-trace-id UI (Loki/Grafana in compose) so a fresh clone gets the full three-pillar experience.

## Architecture

- Hexagonal architecture in `backend/internal/` — domain and ports never import adapters. `docs/hexagonal.md` is authoritative.
- The **worker is a second hexagon**: driving side is the Temporal SDK, core is `OrderWorkflow`, same outbound ports. `workflows/` is core, so the no-adapter-imports rule binds it too. The boundary between hexagons is `outbound.OrderStarter`.
- Temporal for workflow orchestration (price search, order saga, shipping tracking).
- Traefik reverse proxy routes `/api/` to backend, `/` to frontend locally; the GKE Ingress does the same in prod — so the frontend uses **relative API URLs** and there is no CORS.

## Stack

- Go 1.26, Ent ORM, Atlas migrations, PostgreSQL 17
- React 19 + Vite + TypeScript + Tailwind v4 + react-router + TanStack Query + React Hook Form/Zod; Vitest + Testing Library
- Temporal for durable workflows; Stripe test mode for payments
- MinIO locally / GCS in prod for file storage, Mailpit for local email
- Docker Compose for local dev; GKE + Terraform + Kustomize for deployment

## Commands

Common commands are wrapped with [go-task](https://taskfile.dev) — run `task --list` to see everything.

- `task up` / `down` / `logs` — the full local Docker stack
- `task restart -- <service>` — recreate one service. Uses `-V` to drop anonymous volumes, which is the point: the frontend's `node_modules` lives in one, so a host-side `npm install` never reaches the container otherwise. Named volumes (postgres/minio/temporal-db) are untouched.
- `task backend:check` — build + vet + test + lint. Also `generate`, `migrate-diff`, `migrate-apply`, `test-integration`, `smoke`.
- `task frontend:check` — format:check + typecheck + lint + test. Also `dev`, `build`, `test-watch`.

**Toolchain gotchas**

- Codegen tools are `tool` directives in `go.mod` (`go tool ent generate ./schema`) — add future ones the same way, never `go run -mod=mod`, which deposits CLI deps into `go.sum` and busts the Docker cache layer.
- Atlas's `ent://` loader still shells out with `-mod=mod`, which Go forbids under the workspace, so `backend:migrate-*` sets `GOWORK=off`.
- **`GOWORK=off go build ./...` is what Docker does.** Run it before a container build — `go.work` masks missing `go.sum` entries and a plain `go build` will pass where the image build fails.
- Compose Postgres is on host **5433** (native Postgres holds 5432); point Atlas and pgAdmin there. In-container it's still `postgres:5432`.
- On a fresh machine, apply pending migrations before the first `task up`.
- `STRIPE_SECRET_KEY` lives in a gitignored `.env` via `${...}` compose substitution. **Never read `.env` or any secret file**, even to grep a variable name — verify wiring from committed sources.

## Conventions

**Go**

- Idiomatic Go is an explicit goal — the user is learning Go from a JS/TS/PHP/Java background. Reach for the smallest construct that fits, not "structs everywhere": closures where a thing is one behavior, small interfaces, struct+constructor only when a type holds dependencies *and* exposes multiple behaviors. When a choice diverges from OO habits, call it out and explain the Go reasoning rather than silently picking.
- All external dependencies go through port interfaces in `ports/outbound/`; adapters implement them and are wired in `cmd/server/main.go` (and `cmd/worker/main.go`).
- **Every sentinel lives beside its port.** An adapter may declare its own only while it and its tests are the sole users; once a caller branches on it, it is a term of the port. Cross-side name repeats (`ErrUserNotFound` on both sides) are deliberate — different vocabularies, the service translates.
- **Error normalization:** app services map domain/outbound errors into a small `inbound` vocabulary; handlers only ever `errors.Is` against `inbound.*`. Expected errors become typed response objects; unexpected ones `return nil, err` for a global 500.
- **Secrets enter only at the composition root**, injected into adapter constructors. Never `os.Getenv` inside an adapter.
- HTTP is **spec-first** via oapi-codegen; `backend/api/api.yaml` is the source of truth. Note oapi-codegen has **two** error seams — body binding (`StrictHTTPServerOptions.RequestErrorHandlerFunc`) and path/query binding (`StdHTTPServerOptions.ErrorHandlerFunc`); `NewAPI` wires both, which is why route mounting lives inside the http package.
- **List endpoints return an envelope** (`{"orders":[...]}`), never a bare array.

**Persistence**

- Every Ent `Default(...)` also gets a DB-level default via an `entsql` annotation.
- Relations get **real foreign keys** via edges bound to explicit uuid fields. The edges exist purely to emit the FK — nothing traverses them; the domain still references other aggregates by bare uuid. **Always index FK columns explicitly** — Postgres does not index the referencing side.
- **Constraints in the DB (declarative), behavior in the app (imperative).** That's the line that says yes to FKs, CHECKs, and partial unique indexes, and no to stored procedures.

**Temporal**

- The user knows Temporal well — skip fundamentals, discuss project-specific design at peer level.
- Workflow/activity inputs and outputs are **exported DTOs**. Domain VOs have unexported fields and serialize to `{}` across the wire; reconstruct the VO inside the activity via its constructor.
- Shared activity options live in `options.go`; get-modify-set for per-activity overrides. Retryability is expressed by the error type, not by a config flag.
- **Workflow vs. app service:** logic that must survive a process restart goes in the workflow; logic that only needs to outlive a request goes in the app service.

**Testing**

- Tests are kept in pace, never deferred to the end. Claude generates them; the user reviews the cases and assertions.
- Unit = one unit with collaborators faked; integration = cross-layer, using testcontainers-go where Postgres is needed. **Don't test third-party library behavior** — test only our own logic.
- Assertions use testify (`require`/`assert`), table-driven with `t.Run` subtests.
- Frontend: Vitest 4 + jsdom + Testing Library, explicit imports (globals are off).
- `golangci-lint` must pass before committing.

**Git**

- The user runs **all** git themselves — never run `add`/`commit`/`push`, not even `status` before a commit.
- Work lands through **PRs from feature branches**; branch protection gates `main` with CI plus the Claude GitHub-app review.
- Commit messages: very brief, single short sentence, conventional-commit format, no body unless asked.

**Process**

- The user reviews file changes in the VS Code editor, not the console — keep the IDE bridge connected and flag if it drops.
- Act without asking on Claude-owned paths (`services/**`, `*_test.go`) and report after; that scope excludes `ent/` and infra configs.
- Walk through config-touching commands before running them — the user often handles machine config themselves.
- Permission allowlist rules go in `settings.local.json` (gitignored), never `settings.json`.
- Review by reading; save build/vet/lint for occasional checkpoints rather than every review.
- American English throughout — docs, comments, identifiers, prose.
- The user leans toward flexibility "in case requirements change." Don't answer with bare YAGNI; argue from constraints-being-reversible and tight-types-helping-when-churning.

## Conversation Rules

- Keep all answers brief, including only necessary information. If I need more detail I'll ask for it.
