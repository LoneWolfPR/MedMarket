# MedMarket

Prescription price comparison platform. Monorepo with Go backend, React frontend, mock external services.

## Implementation Plan

See implementation-plan.md at repo root for the full day-by-day plan.

## Current Progress

In addition to the notes below check your most recent memories to make sure you have all context from previous conversations.

The plan is to have claude do basic scaffolding and boilerplat that I will review, while I do the coding in areas I'm trying to learn. All local technology has been verified except for Terraform which will be added when needed.

**Day 1 — complete (checkpoint passed 2026-07-01).** Scaffolding is in place and reviewed: top-level monorepo skeleton (with `.gitkeep` placeholders for `services/`, `workflows/`, `worker/`, `k8s/`, `terraform/`, `.github/workflows/`); root `go.work` (workspace root, `use ./backend`); `backend/` module (`github.com/LoneWolfPR/MedMarket/backend`, go 1.26) with `cmd/server/main.go` serving `GET /api/health`; multi-stage backend Dockerfile (golang:1.26 → distroless); `frontend/` React 19 + Vite 6 + TS skeleton (App.tsx fetches `/api/health` as a self-verifying checkpoint); frontend dev Dockerfile (node:24-alpine); `docker-compose.yml` (traefik + backend + frontend + postgres on the `medmarket` network). Backend builds + vets clean.

`docker compose up --build` verified: `http://localhost` shows "Backend health: ok", `/api/health` returns JSON, and the Traefik dashboard is at `:8080`. Two issues surfaced and were fixed during bring-up: (1) Docker socket permission — user added to the `docker` group and re-logged in; (2) `traefik:v3.3` hard-pins Docker API 1.24, which Engine 29.6.1 rejects (min API 1.40) — bumped to `traefik:v3.6`; (3) local Postgres already on host `5432`, so compose Postgres remapped to host **5433** (container-internal still 5432 — point Atlas/tooling at `localhost:5433`). `.gitignore` reviewed. Day 1 committed + pushed.

**Day 2 — in progress (user vertical slice; HTTP handlers + JWT middleware + most of `main.go` wiring done; final strict-handler wiring next).** Approach: build the `user` feature as a full vertical slice (domain → ports → adapter → HTTP → JWT); the `prescription`/`order`/`pharmacy` domains are deferred to their feature days (Days 3/4/6) — implementation-plan.md updated to match. Split refined this day: the user hand-writes the domain, ports, **the adapters + app service, and the HTTP handlers/middleware/wiring** (the whole learning surface); Claude does pure plumbing (dependency adds, Ent codegen, Atlas/tooling config), walkthroughs of new patterns, doc-comment/lint cleanup, and reviews.

Done + committed: hexagonal dirs under `backend/internal/`; `domain/user` (`Email` + `PasswordHash` value objects with validating constructors, `IsZero`, `String`; `NewUser` with invariants; `NewUserParams`) and `domain/shared` (`Address`); `ports/outbound.UserRepository` (`Create`/`GetByID`/`GetByEmail` + `ErrUserNotFound` sentinel) and `ports/inbound.UserService` (`Register`/`Login`/`GetProfile` + `RegisterInput`; `Login` returns an opaque token **string**, not a JWT-library type); Ent schema for `User` (uuid id: client-side `uuid.New` default mirrored by DB-level `gen_random_uuid()` via an `entsql` annotation — every Ent default also gets a DB-level one; `email` unique; `password_hash` `Sensitive`; `Address` flattened into columns); generated Ent client; first Atlas migration (`init`) generated + applied to Postgres (`localhost:5433`), verified in pgAdmin.

Tooling added: **go-task** (`Taskfile.yml` root + `backend/Taskfile.yml`; `task up`/`down`/`logs`, `task backend:check`/`generate`/`migrate-diff`/`migrate-apply`); **golangci-lint** v2 + `.golangci.yml`; `README.md`. Module path is `github.com/LoneWolfPR/MedMarket/backend`. Toolchain gotcha: Ent codegen and Atlas's `ent://` loader shell out with `-mod=mod`, which Go forbids under the `go.work` workspace, so those task targets set `GOWORK=off`.

Done since (user-written, not yet committed): `domain/user.Password` value object (`NewPassword` with length + character-class rules); Postgres adapter (`adapters/outbound/postgres/user_repository.go` — implements `UserRepository`, maps Ent↔domain, `ent.IsNotFound`→`ErrUserNotFound`, `ent.IsConstraintError`→new `ErrEmailTaken` sentinel); app service (`internal/app/user_service.go` — `Register`/`Login`/`GetProfile`, maps auth failures to `inbound.ErrInvalidCredentials`); bcrypt `PasswordHasher` (`adapters/outbound/bcrypt`) + JWT `TokenIssuer` (`adapters/outbound/jwt`, HS256, TTL) with their outbound ports. Ent schema gained `created_at`/`updated_at` (client + DB-level defaults; `updated_at` app-side refresh via `UpdateDefault`), regenerated client + `20260702191156_add_user_timestamps` migration.

Done this session (user-written unless noted, not yet committed): **OpenAPI/HTTP layer** (oapi-codegen strict server; `api/api.yaml` is source of truth). `AuthHandler` implements the generated `StrictServerInterface` — `RegisterUser`/`LoginUser`/`GetProfile` (`internal/adapters/inbound/http/auth_handler.go`); handlers are thin translators (DTO↔service input, `errors.Is` on inbound sentinels → typed response objects; expected errors → response objects, unexpected → `return nil, err` for a global 500). **Error normalization pattern:** the app service normalizes domain/outbound errors into a small inbound vocabulary — `inbound` now exports `ErrValidation` + `ErrEmailTaken` (alongside `ErrInvalidCredentials`); `Register` maps validation/duplicate cases, `Login` maps malformed-email → `ErrInvalidCredentials`, `GetProfile` maps `outbound.ErrUserNotFound` → `ErrInvalidCredentials` (deleted-user-with-valid-token → 401). Handlers only ever `errors.Is` against `inbound.*`. Removed the login `400` from `api.yaml` + regenerated (login is now 200/401 only; malformed-JSON 400 is a cross-cutting transport concern, handled globally, not per-op — same treatment as 500). **JWT auth middleware** (`auth_middleware.go`): Option A — a `StrictMiddlewareFunc` keyed on a `protected` operationID set; factory closure `newAuthMiddleware(ti, protected)`; guard-clause parse of the `Bearer` header (`strings.Cut` + `EqualFold`) then `ti.Verify`; failures write a 401 via `writeJSONError` and `return nil, nil` (short-circuit). **Context contract** (`context.go`): unexported `ctxKey struct{}` + `setUserIDKeyValue`/`getUserIDKeyValue` (uuid in/out of ctx; getter does the comma-ok assertion). **Helpers** (`helpers.go`, `respond.go`): `MapToSharedAddress`/`MapToOAPIAddress` (ptr-based, `omitempty` — empty→nil at the adapter boundary), `toUserResponse`, `writeJSONError`. New **`internal/ptr`** package (generic `To`/`Deref`; the "empty string ⇄ nil" convention stays in adapters, not in `ptr`). Claude-written plumbing: added `lll` (line-length 120, tab-width 4) to `.golangci.yml`; re-added `pgx/v5` as a direct dep (blank import `_ ".../pgx/v5/stdlib"`); doc comments on the exported handler methods. User cleanup done: dropped the unused plaintext `Password` field from `domain/user.User`/`NewUserParams`.

`cmd/server/main.go` **wiring — in progress.** Refactored to the `run() error` pattern (main is a shell; defers run before `os.Exit`). Done: logger (`slog` JSON + `SetDefault`); `loadConfig()` from env (`constants.go`: `DATABASE_URL`/`JWT_SECRET`/`JWT_TTL`/`PORT`; required-fatal, TTL defaults on empty but errors on malformed); DB open (`sql.Open("pgx", …)` → `PingContext` 5s fail-fast → `entsql.OpenDB(dialect.Postgres, db)` → `ent.NewClient`, deferred close logs its error); outbound adapters (repo/hasher/tokenIssuer), `app.NewUserService`, and `httpapi.NewAuthHandler` (note: inbound `http` package is imported **aliased** as `httpapi` to avoid the stdlib `net/http` collision).

**Remaining Day 2:** finish `main.go` **step 6** — build the auth middleware + strict handler and mount routes. Wrinkle to resolve first: `newAuthMiddleware` and the `protected` operationID set are **unexported** in the `http` package, so `main` can't reach them via the alias — need an exported seam (e.g. an exported `NewAPI`/assembly func in the `http` package that builds the `StrictMiddlewareFunc` and the strict handler internally, keeping the context key/middleware private). Then `NewStrictHandlerWithOptions(authHandler, []{authMW}, opts)` with a custom `ResponseErrorHandlerFunc` (JSON 500s) + `RequestErrorHandlerFunc` (JSON 400s), `HandlerFromMux`, mount. Deferred: request-logging middleware (logs every request outcome incl. 4xx — the "end of chain" log; 500 detail logged in `ResponseErrorHandlerFunc`); graceful shutdown (signal `context.NotifyContext` at top of `run`); add `DATABASE_URL`/`JWT_SECRET`/`JWT_TTL` to the backend compose service env for local testing (in-container DB host is `postgres:5432`, not host `5433`); possible rename of the `http` package → `httpapi`/`rest` to drop the import alias. Lint currently shows 3 `unused` warnings (`newAuthMiddleware`/`setUserIDKeyValue`/`writeJSONError`) — expected, they clear once the middleware is wired in `main.go`. **Checkpoint:** register → login (JWT) → protected `/api/auth/profile`.

**Design decision (2026-07-02, applies Day 3+):** mock pharmacies must expose **different** API shapes, not a shared contract — one standard `PharmacyClient` port, a separate adapter per pharmacy translating its API. implementation-plan.md updated to match.

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
- Idiomatic Go is an explicit goal — the user is learning Go from a JS/TS/PHP/Java (OO) background. Favor the idiomatic Go construct (first-class functions/closures where a thing is essentially one behavior, e.g. middleware; small interfaces; struct+constructor only when a type holds dependencies *and* exposes multiple behaviors or satisfies an interface). Principle: reach for the smallest construct that fits, not "structs everywhere." When a choice diverges from OO-language habits, call it out and explain the Go reasoning rather than silently picking.

## Conversation Rules

- Keep all answers brief, including only necessary information. If I need more detail I'll ask for it.