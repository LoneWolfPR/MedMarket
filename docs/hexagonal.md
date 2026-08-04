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

## The second hexagon: the worker

`cmd/worker` is not an appendage of the backend's hexagon — it is its own,
sharing the same ports and driven adapters but with a different driving side:

| | backend (`cmd/server`) | worker (`cmd/worker`) |
|---|---|---|
| Driving adapter | `http.OrderHandler` | the Temporal SDK dispatching a workflow task |
| Core | `app.OrderService` | `OrderWorkflow` |
| Outbound ports | `internal/ports/outbound` | the same interfaces |
| Driven adapters | postgres, temporal starter, s3/gcs, … | pharmacya/b, postgres, stripe, mailer, shipping |
| Composition root | `cmd/server/main.go` | `cmd/worker/main.go` |

The boundary between them is `outbound.OrderStarter`. Its adapter
(`internal/adapters/outbound/temporal/order_starter.go`) is where the backend's
hexagon ends; the workflow runs on the far side, in another process.

**`workflows/order` is core, not an adapter.** It holds real business rules —
$0 orders skip authorize/capture, a failed capture leaves `PricePaid` nil rather
than failing the order, a re-quote divergence is logged but the offer amount is
honored, placement-rejected voids the auth while outcome-unknown parks and
touches nothing. That is a use case, not a coordinator, so the "core never
imports adapters" rule binds it exactly as it binds `internal/app`.

**`Activities` is not a hexagonal concept.** It's the workflow's outbound-port
call sites, split into separately-schedulable functions because determinism
forbids a workflow doing I/O itself. That's a Temporal constraint wearing a
hexagonal-looking hat — don't read `Activities` as a layer.

### Workflow or application service?

Both `pricesearch` and `order` have an app service *and* a workflow, but the
work is divided differently, and the rule that produces both is:

> Logic that must survive a process restart goes in the workflow. Logic that
> only needs to outlive a request goes in the application service.

`app.PriceSearchService` owns its use case — ownership check, criteria, name
resolution, totals, offer persistence — and its workflow is a computation over
ports (fan out, skip failures, sort). Offer persistence belongs there precisely
because a dropped HTTP request *should* lose it. The order workflow is the
opposite: the void-on-rejection must not be lost, so the use case lives in the
workflow and `app.OrderService` only does the pre-work before starting it.

**Known accepted exception:** `workflows/order/workflow.go` composes the
shipping-update email body inline. Deciding *that* a status change notifies the
customer is orchestration; deciding *how the sentence reads* is presentation and
belongs behind the port. Moving it means a notifier port plus an adapter — a
cost not worth paying for one message today. It is a deliberate exception, not
an oversight.

## What crosses an inbound port

Domain values are welcome in inbound views. Ports speak the core's language, and
driving adapters are *allowed* to know the domain — `helpers.go` commits to this
already, with `toUserResponse(u *user.User)` and `MapToOAPIAddress(shared.Address)`
taking domain types directly. The DTO-at-every-boundary instinct comes from
layered/Clean architecture, not this one.

> A view exists to **compose**, not to insulate. Return the bare entity when
> there is nothing to compose.

| Inbound type | Shape | Why |
|---|---|---|
| `UserService → *user.User` | bare entity | the use case's output *is* the entity |
| `PrescriptionView` | entity + `DocumentURL` | one entity plus a derived field |
| `QuoteView` | `PriceQuote` + offer id, name, total | composition across four sources |
| `OrderView`, `OrderStatusView` | flat fields | composition across order, prescription, workflow query |

The test for whether a type belongs in `ports/inbound` rather than the domain:
**would this type exist if there were no API?**

**`OrderView` narrows deliberately.** `Order` has eight fields; the view exposes
four, withholding `PharmacyOrderID`, `TrackingID`, `PricePaid`, and `OfferID`.
That means response-shaping happens at the port here and in the handler
elsewhere — accepted as defense-in-depth for PHI-adjacent data, at the cost of
the contract living in two layers depending on which endpoint you're reading.

## Error vocabulary

Each boundary has its own vocabulary, and the **application service translates**
between them. A driving adapter only ever matches `inbound.*`; it never sees a
domain or outbound error identity.

```
domain / library error
  → outbound.Err* or a typed *outbound.PlaceOrderError
    → the app service normalizes
      → inbound.Err*
        → the handler maps to a status code
```

Names repeating across the two sides — `ErrUserNotFound`, `ErrOfferNotFound`,
`ErrOrderNotFound`, `ErrPrescriptionNotFound`, `ErrOrderWorkflowNotFound` — are
deliberate, not duplication. They are different vocabularies that happen to
describe the same fact, and the translation between them is where the meaning
changes (`app/shipping_service.go` is the clearest example).

**Where a sentinel lives:** in the file declaring the port it belongs to.
`ports/inbound/errors.go` holds only sentinels that belong to no single port —
today just `ErrValidation`, which spans two services and two handlers.

**When an adapter may declare its own sentinel:** only while that adapter and
its tests are the sole users. The moment a caller branches on it, it has become
a term of the port and belongs beside the interface. `ErrShippingRejected` and
`ErrInvalidMessage` both started in their adapters and moved once the order
activities began classifying retryability with them.

**A message is not an error.** If nothing ever returns it and nothing matches it
with `errors.Is`, it is response copy — it belongs with `msgUnauthorized` in the
handler package, not in `ports/inbound`.

## Authorization lives in the application service

Ownership is a business rule, so it sits in the core, not in a repository and
not in middleware (the auth middleware verifies the token; it cannot know what
the token's subject owns).

`OrderService.ownedOffer` is the worked example. It walks offer → prescription →
compare `UserID`, and **collapses every way that chain can fail into a single
not-found sentinel** — missing offer, missing prescription, or a prescription
belonging to someone else all answer identically, so a caller can never probe
for records it has no right to see. Callers translate that one sentinel into
their own vocabulary (`GetOrderStatus` reports it as `ErrOrderNotFound`).

Because foreign keys make the missing-prescription case impossible, the service
logs it at error level before hiding it: the client still gets a 404, but the
data-integrity problem doesn't vanish with it.
