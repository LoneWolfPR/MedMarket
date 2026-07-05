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

**Day 2 — COMPLETE (checkpoint passed 2026-07-05).** Route wiring done and the full auth vertical slice runs end-to-end. The exported-seam wrinkle is resolved: **`NewAPI(h *AuthHandler, ti outbound.TokenIssuer) openapi.ServerInterface`** in `internal/adapters/inbound/http/api.go` (new file) is the http package's single exported assembly point — builds the `protected` operationID set (`{"GetProfile"}`), `newAuthMiddleware(ti, protected)`, and a `StrictHTTPServerOptions` with `RequestErrorHandlerFunc` (JSON 400 via `writeJSONError`) + `ResponseErrorHandlerFunc` (logs the real error via `h.logger`, returns a **generic** `msgInternalServerError` — no `err` leak to the client), then returns `openapi.NewStrictHandlerWithOptions(h, []{authMW}, opts)`. No `error` return (nothing inside can fail — deliberately diverges from the `NewXxxParams`+error house pattern, which exists only to *validate*; `NewAPI` validates nothing). `newAuthMiddleware`/`ctxKey`/`writeJSONError` stay unexported; `main` only touches `NewAPI`. `cmd/server/main.go` step 6: `api := httpapi.NewAPI(authHandler, tokenIssuer)` then `openapi.HandlerFromMux(api, mux)` mounts all three auth routes onto the same mux as `GET /api/health` (`HandlerFromMux` generates one `HandleFunc` per spec operation; auth gating is separate, via the middleware's `protected` check).

Two bugs surfaced + fixed during bring-up: (1) **Postgres race** — backend does a fail-fast `PingContext` (5s) and exits on refusal, but `depends_on: postgres` only waits for container *start*; on `up` the backend raced Postgres, died, and Traefik dropped its route so `/api/*` fell through to the frontend (GET → SPA HTML/200, POST → 404). Fixed with a `pg_isready` **healthcheck** on the postgres service + `depends_on: { postgres: { condition: service_healthy } }` on backend. (2) **Missing `Logger`** in the `postgres.NewUserRepository` call in `main.go` (constructor validates it non-nil) — added `Logger: logger`. Compose backend env also got `DATABASE_URL` (in-container host `postgres:5432`, **not** host `5433`), `JWT_SECRET` (local-dev placeholder), `JWT_TTL=24h`. **Checkpoint verified:** register `201` → login `200` (JWT, `sub` = new user id) → profile-with-token `200`; profile no-token `401`; profile bad-token `401`. **Deferred (not blocking):** unit tests (see next-session note below); request-logging middleware (end-of-chain log incl. 4xx); graceful shutdown (`signal.NotifyContext` at top of `run`); optional compile-time `var _ openapi.StrictServerInterface = (*AuthHandler)(nil)` assertion; possible `http`→`httpapi`/`rest` package rename to drop the main import alias.

**Next session — unit tests first.** Before moving to Day 3 features, stand up unit tests for everything built so far (domain value objects + invariants, app service error-normalization, adapters, HTTP handlers/middleware). Testing is **not** deferred to the end. Workflow: **Claude generates the tests, the user reviews the test cases + assertions** (the cases/assertions are the learning-review surface here, not hand-writing every test). Integration tests needing Postgres use testcontainers-go per the conventions below.

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