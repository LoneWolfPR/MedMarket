# Hexagonal Architecture in MedMarket

Most diagrams online show `adapter → port → hexagon` on both sides and stop
there. That hides the one asymmetry that actually explains where the
**application service** fits. This doc captures it, grounded in the `user`
vertical slice.

## The vocabulary (all the synonyms)

| Diagram side | Also called | What it means |
|---|---|---|
| **inbound** | **driving** / primary | Actors that *drive* the app (HTTP, CLI, a Temporal activity) |
| **outbound** | **driven** / secondary | Things the app *drives* (Postgres, bcrypt, JWT, pharmacy APIs) |

A **port** is always an interface owned by the core. An **adapter** is always
the concrete, technology-specific thing on the boundary. Dependencies always
point inward: the concrete adapter depends on the core's interface, never the
reverse.

## The asymmetry the online diagrams omit

The interface (port) always belongs to the core. What flips between the two
sides is **whether the core is the *implementer* or the *caller*** of that port:

| | Interface (port) | Implementer | Caller |
|---|---|---|---|
| **Outbound / driven** | `outbound.UserRepository` | `postgres.UserRepository` (adapter) | `app.UserService` (core) |
| **Inbound / driving**  | `inbound.UserService`    | `app.UserService` (core)            | `http.AuthHandler` (adapter) |

- On the **outbound** side the core is the *caller*, so something outside must
  *implement* the port → the **driven adapter** implements it.
- On the **inbound** side the core is the *implementer* — and that implementer
  is exactly the **application service**. There's nothing left for the driving
  adapter to implement; it's just a *caller*.

This is why a **driven adapter implements its port but a driving adapter does
not.** It's not an inconsistency — the service already filled the "implementer"
role on the inbound side.

## Where the service fits

Notice the service appears in **both** rows above: it *implements* the inbound
port and *calls* the outbound ports. That dual role is what "the center of the
hexagon" means.

```
                 ┌────────────────────── the hexagon (core) ──────────────────────┐
                 │                                                                  │
  HTTP request   │   ┌─────────────────┐        ┌──────────────────┐               │
 ──────────────▶ │   │  inbound port   │◀───────│  app.UserService │───────┐       │
 http.AuthHandler│   │ inbound.        │ implements  (THE SERVICE)  │       │       │
 (driving        │   │  UserService    │        └──────────────────┘       │ calls │
  adapter)       │   └─────────────────┘                                    ▼       │
      │  calls   │                                          ┌─────────────────────┐ │
      └──────────┼─────────────────────────────────────────│   outbound ports    │ │
                 │                                          │ UserRepository,     │ │
                 │                                          │ PasswordHasher,     │ │
                 │                                          │ TokenIssuer         │ │
                 │                                          └──────────┬──────────┘ │
                 └─────────────────────────────────────────────────── │ ───────────┘
                                                              implements│
                                                    ┌───────────────────▼───────────────────┐
                                                    │ postgres.UserRepository, bcrypt.*, jwt.*│
                                                    │            (driven adapters)            │
                                                    └─────────────────────────────────────────┘
```

Read it as: the **driving adapter calls** the inbound port; the **service
implements** the inbound port and **calls** the outbound ports; the **driven
adapters implement** the outbound ports.

## Service vs. adapter

Near-opposites, easy to conflate because a driving adapter *calls* the service:

| | Application service (`app.UserService`) | Adapter (`postgres.*`, `http.AuthHandler`) |
|---|---|---|
| Job | Orchestrate a use case | Translate to/from one specific technology |
| Knows about | Business flow + domain objects | HTTP, Ent/SQL, bcrypt, JWT — one concrete tech |
| Business logic? | Yes (the *sequence* + error normalization) | No — pure translation |
| Position | Inside the hexagon | On the boundary |

`UserService.Register` is the textbook example: it sequences the flow
(validate → hash → persist) and normalizes domain/outbound errors into the
`inbound.ErrValidation` / `ErrEmailTaken` vocabulary. That orchestration has no
home in the HTTP adapter (which just maps DTOs) or the Postgres adapter (which
just maps Ent↔domain).

A service earns its place only when there's real orchestration across ports. A
straight pass-through to a single repository wouldn't need one.

## A framework wrinkle: `openapi.StrictServerInterface`

`http.AuthHandler` *does* implement an interface —
`openapi.StrictServerInterface` (generated by oapi-codegen). Don't let that
reintroduce the confusion: that generated interface is **transport plumbing**
so the router can dispatch HTTP requests to the handler's methods. It is **not**
the application's inbound port. The real inbound port is `inbound.UserService`,
which the handler *calls*.

- `AuthHandler` **implements** `StrictServerInterface` → satisfies the HTTP framework.
- `AuthHandler` **calls** `inbound.UserService` → drives the application core.

Only the second is the hexagonal boundary.
