# MedMarket — Frontend Design Spec

The visual spec for the frontend. Written as a design handoff, and specific
enough to build from without guessing.

**Every style directive here is a literal Tailwind utility.** `text-teal-600`,
not "teal-600"; `gap-3`, not "a small gap". Where a value is genuinely the
implementer's call, this document says so outright. Anything ambiguous in here is
a bug in the spec — say so and it gets fixed.

Two Tailwind traps this convention exists to prevent: a **bare color name is not
a class** (`teal-600` emits nothing; the property prefix — `text-`, `bg-`,
`border-`, `ring-` — is what makes it real), and a **modifier is inert without
its base utility** (`gap-*` needs `flex` or `grid`, `border-slate-200` needs
`border`, `focus:ring-teal-600` needs `focus:ring-2`). Both fail silently: the
markup looks right in review and renders wrong.

Tailwind's default palette and spacing scale are the source of truth for values,
so nothing here uses hex codes or arbitrary values.

---

## 1. Design tokens

### Color

This table names color *roles*. The utility depends on the property you're
setting — `text-teal-600` for text, `bg-teal-600` for a fill, `border-teal-600`
for a border, `ring-teal-600` for a ring.

| Role | Color | Used for |
| --- | --- | --- |
| Primary | `teal-600` | Buttons, active nav, links, focus rings |
| Primary (hover) | `teal-700` | Hover state of anything primary |
| Primary (subtle) | `teal-50` | Selected-row / active-card background tint |
| Page background | `slate-50` | The `<body>`-level surface |
| Surface | `white` | Cards, header bar, form panels |
| Border | `slate-200` | All hairlines, dividers, card outlines |
| Text (primary) | `slate-900` | Headings, wordmark, key values |
| Text (secondary) | `slate-600` | Body copy, labels, inactive nav |
| Text (muted) | `slate-400` | Placeholders, timestamps, helper text |
| Danger | `red-600` | Errors, destructive actions |
| Success | `emerald-600` | Confirmations, delivered status |
| Warning | `amber-600` | Expiring offers, pending states |

Rule: **the page is `bg-slate-50`, content sits on `bg-white` cards with
`border border-slate-200`.** That contrast is what gives the UI structure — avoid
white-on-white.

### Type

Full class strings — copy them as written.

| Role | Classes |
| --- | --- |
| Page title | `text-2xl font-semibold tracking-tight text-slate-900` |
| Section heading | `text-lg font-semibold text-slate-900` |
| Body | `text-sm text-slate-600` |
| Label | `text-sm font-medium text-slate-700` |
| Helper / meta | `text-xs text-slate-400` |
| Wordmark | `text-lg font-semibold tracking-tight text-slate-900` |

### Spacing & shape

- Content column: `max-w-5xl mx-auto`. Header content and page content use the
  *same* max width so the columns align.
- Gutters: `px-4 sm:px-6`.
- Vertical rhythm: `py-8` around main content; `gap-4` between sibling cards.
- Corners: `rounded-lg` on cards and buttons; `rounded-md` on inputs.
- Elevation: `shadow-sm` on cards. Nothing heavier — this is a utility app.

---

## 2. App shell

### Header

- Outer bar: `bg-white border-b border-slate-200`. Full-bleed. **No rounding, no
  side borders, no shadow** — a full-width bar with rounded corners reads as a
  mistake.
- Inner container: `h-16 max-w-5xl mx-auto px-4 sm:px-6 flex items-center
  justify-between`.
- Left: wordmark "MedMarket", a `Link` to `/`, styled `text-lg font-semibold
  tracking-tight text-slate-900`.
- Right: `<nav>` holding the links in a row — `flex items-center gap-6`.

### Nav links

Base on every state: `text-sm font-medium`.

| State | Classes |
| --- | --- |
| Default | `text-slate-600` |
| Hover | `hover:text-slate-900` |
| Active (current route) | `text-teal-600` |

`NavLink` decides active/default via its render-prop `className`; hover applies
in both cases.

### Main

- Shell: `min-h-screen bg-slate-50` on the outermost wrapper, so the background
  doesn't stop short on short pages.
- `<main>`: `max-w-5xl mx-auto px-4 sm:px-6 py-8`.

---

## 3. Components (as they arrive)

### Button — primary

```
bg-teal-600 text-white text-sm font-medium px-4 py-2 rounded-lg
hover:bg-teal-700
focus:outline-hidden focus:ring-2 focus:ring-teal-600 focus:ring-offset-2
disabled:bg-slate-300 disabled:text-slate-500 disabled:cursor-not-allowed
```

`focus:ring-offset-2` is what keeps the ring from sitting flush against the
fill, where it reads as a smudge rather than a ring.

### Button — secondary

```
bg-white border border-slate-200 text-slate-700 text-sm font-medium px-4 py-2 rounded-lg
hover:bg-slate-50
focus:outline-hidden focus:ring-2 focus:ring-teal-600 focus:ring-offset-2
disabled:bg-slate-100 disabled:text-slate-400 disabled:cursor-not-allowed
```

**Surface caveat.** This variant assumes the `bg-slate-50` page background, where
white reads as raised. On a **white card** it flattens into a hairline outline —
there, swap the first line for a subtle fill and drop the border:

```
bg-slate-100 text-slate-700 text-sm font-medium px-4 py-2 rounded-lg
hover:bg-slate-200
```

Check what a button is sitting on before reaching for this one.

### Input / textarea / select

```
w-full bg-white border border-slate-300 rounded-md px-3 py-2 text-sm
focus:outline-hidden focus:ring-2 focus:ring-teal-600 focus:border-teal-600
```

`focus:outline-hidden` removes the browser's default outline, which is only
acceptable *because* the ring replaces it — never leave a control with no focus
indicator. Note this is Tailwind v4's rename of v3's `outline-none`, and it keeps
a transparent outline for forced-colors users.

Error state: add `border-red-600`, and put the message below the input at
`text-xs text-red-600`. Change the border **color** only — changing its width on
error shifts the layout.

This exact string lives in `src/pages/sharedClasses.ts` as `inputClass`; use that
rather than retyping it.

### Card

```
bg-white border border-slate-200 rounded-lg shadow-sm p-4
```

Use `p-6` instead of `p-4` for a form panel.

### Prescription card

A wide, low-density row. One card per prescription, stacked in a single column
at the full width of the content column — no grid.

Card element: the standard **Card** classes above at `p-4`.

Internally it's a two-part row: **identity on the left, actions on the right**.
On the card: `flex flex-col gap-3 sm:flex-row sm:items-center
sm:justify-between` — stacked on small screens, pushed apart and centered from
`sm:` up.

Left block: `flex flex-col gap-1`.

| Line | Content | Classes |
| --- | --- | --- |
| Title | Med name + strength as one string — "Lisinopril 20mg" | `text-base font-medium text-slate-900` |
| Detail | "Prescribed by {physician}" | `text-sm text-slate-600` |
| Meta | "Qty {quantity}" | `text-xs text-slate-400` |

Strength is `strengthValue` and `strengthUnit` concatenated with **no space** —
that matches how the backend's `MedStrength.String()` renders it.

Right block: `flex items-center gap-3`, with the **primary action last** —
rightmost is where the eye finishes, and it should finish on the action the page
exists for. Because the parent flips to `sm:flex-row`, this block stays a row at
every width; it simply sits below the identity block on small screens.

**View document** — a text link, not a button. A button per row for it reads
heavier than the action deserves.

```
text-sm font-medium text-teal-600 underline
hover:text-teal-700
focus:outline-hidden focus:ring-2 focus:ring-teal-600 focus:ring-offset-2 rounded-sm
```

It opens a presigned URL on another origin, so it takes `target="_blank"` and
`rel="noopener noreferrer"`. (`rounded-sm` exists only so the focus ring has a
radius to follow — an inline element has none by default.)

**Find prices** — the primary button classes, verbatim. This is what the list is
*for*; the whole app exists to get from a prescription to a price.

**Find prices navigates, so it must be a `Link`** styled as a button — never a
`<button>` with an `onClick` that calls `navigate`. A real link gives you
middle-click, open-in-new-tab, the context menu, a status-bar preview, and
correct Back behavior for free, and screen readers announce it as a link rather
than promising an in-page action. Looking like a button is a styling choice;
*being* one is a semantic claim, and here it would be false.

On small screens the two actions drop below the identity block and sit on their
own row together, rather than stacking vertically.

### List page states

Every list page has the same four-state skeleton under its page title. The title
is always rendered — it never disappears into a loading branch.

| State | Classes |
| --- | --- |
| Loading | `text-sm text-slate-600` — "Loading prescriptions…". Skeletons aren't worth it at this scale. |
| Error | `text-sm text-red-600`, generic copy. Never render a raw exception message. |
| Empty | Panel: `border border-dashed border-slate-300 rounded-lg p-8 flex flex-col items-center gap-1 text-center`. Inside: one line at `text-sm text-slate-600` explaining the list is empty, then one at `text-xs text-slate-400` pointing at the next action. |
| Loaded | The list itself — `<ul>` at `flex flex-col gap-4`, one `<li>` per card. |

The empty state is a designed state, not a blank page — see rule 5.

### Form layout

Single column, `gap-4` between fields. Label above input, `gap-1.5` between
them. Submit button full-width on mobile, auto width from `sm:` up.

**Panel width.** A standalone form panel is centered with `mx-auto` and sized to
its content, not to the viewport. A short single-column form — login, a
confirmation — is `max-w-sm`. A form that earns multiple columns, like register
with its name row and its city/state/zip row, is `max-w-2xl`: wide enough that a
two-column grid doesn't crush its fields, narrow enough that a name input isn't
stretched across the full page column. Forms embedded in a page take the page
column's width.

| Content | Width |
| --- | --- |
| Single-column form | `max-w-sm` – `max-w-md` |
| Multi-column form panel | `max-w-2xl` |
| Prose | `max-w-prose` (65ch) |
| Page / app content column | `max-w-5xl` |

**Panel heading.** The panel's `h1` is a sibling of the `<form>`, so the form's
`gap-4` doesn't apply to it — give the panel itself a flex column with the same
`gap-4`, or the heading sits flush against the first field.

Form-level error messages sit above the first field: `text-sm text-red-600`.
Field-level errors sit below their input: `text-xs text-red-600`.

A form-level **notice** — a success or informational message carried in from
another page, like "Account created — please sign in" — occupies that same slot
above the first field, `text-sm text-emerald-600`. One slot, so the form doesn't
reflow depending on which message appears. A notice and an error are mutually
exclusive: the notice says what to do next, the error says the last attempt
failed, and once there's an error the notice has been overtaken. Show the error
and drop the notice.

**Field hints.** A rule the user needs *before* typing — a password policy, an
accepted format — goes between the label and the input, `text-xs text-slate-400`.
Above the input, not below the error: the constraint is guidance, so it should
be read on the way into the field rather than discovered underneath it, and it
keeps the error message directly under the control it belongs to. Order is
always **label → hint → input → error**.

**That ordering assumes one field per row.** A hint inside a grid cell pushes
that cell's input down, and the field beside it no longer lines up — errors are
exempt because they sit *below* the input, so the row grows without moving
anything. When a hinted field shares a row, lift the hint out and put it above
the whole row as a group hint, so every cell keeps the same
label → input → error shape. Point each affected control's `aria-describedby` at
it; a rule stated once above a pair still governs both of them.

**A group hint must be bound to its group by spacing.** Dropped straight into a
form on the form's own `gap-4`, it has equal air above and below and reads as
belonging to neither the section before it nor the one after — a floating
sentence. Wrap the hint and the fields it describes in a container at `gap-1.5`,
the same distance a label sits from its input, and let that whole unit take the
form's normal spacing. Where the group has its own internal rhythm, nest it:
outer wrapper at `gap-1.5` holding the hint plus an inner container at `gap-4`
holding the fields. Proximity is the only thing telling a reader what a hint
belongs to, so it has to be closer to its group than the group is to its
neighbors.

State the rule in prose, not as a pattern dump. "8–32 characters with an
uppercase letter, a lowercase letter, a number, and a symbol. No spaces." tells
someone what to do; an enumerated list of every legal punctuation mark does not.
If a set is small and surprising, name it; if it's large and unsurprising,
describe it and name only the exclusion.

**Required fields.** Mark them with an asterisk at the end of the label, and put
a `* Required` key above the first field — between the page title and the form,
`text-xs text-slate-500`. The key goes *above* because a legend explaining a notation is no use
after the reader has already met the notation and guessed at it. Keep the key
tight to the title (`gap-1`) rather than on the panel's own rhythm, so it reads
as an annotation on the heading and not as a section of its own.

The asterisk is **`text-slate-500`, not `text-red-600`**. Red is reserved for errors and
destructive actions; if required markers are red, a form with nothing wrong
still reads as a form full of problems. The marker's job is to be catchable on a
scan, not to signal danger. Use the same `text-slate-500` for the key.

The asterisk is decoration, so the *state* has to be carried separately: put
`required` on the control and `aria-hidden="true"` on the asterisk. A bare `*` is
either skipped or read aloud as "star," so on its own it tells a screen-reader
user nothing. Pair it with `noValidate` on the form — the attribute then exposes
the requirement to assistive tech without handing validation back to the browser,
which keeps the schema the single source of truth.

**Conditionally-required groups** don't fit this convention and shouldn't be
forced into it. Where a set of fields is optional as a whole but mandatory once
any one of them is touched — a postal address, a payment method — an asterisk on
each is a lie and no asterisk is misleading. State it once at the group instead,
as a hint above the group's first control: the group is optional, and entering
any of it means entering all of it apart from the genuinely optional members.
Leave the individual labels unmarked and let the validator enforce the rule.

#### Field accessibility

Every control carries `aria-invalid` reflecting its error state, and
`aria-describedby` pointing at whatever text explains it — the hint, the error,
or both. `aria-describedby` takes a space-separated list read in order, so a
field with a persistent hint keeps that id and appends the error id only while
the error is showing. When there is nothing to describe, omit the attribute
rather than pointing at an id that isn't rendered.

Field errors do **not** get `role="alert"`. A failed submit moves focus to the
first invalid control, and its `aria-describedby` is announced on arrival, so an
alert role would say the same thing twice. The form-level error is the exception:
it belongs to no control and would otherwise never be announced, so it takes
`role="alert"`.

#### Field rows

A form is a single column **by default**, but short fields shouldn't be
stretched to the full page column — a quantity box at 1024px wide reads as a
mistake. Group short, related fields into a row: one column on small screens,
`sm:grid-cols-*` from `sm:` up, with the same `gap-4` as the vertical rhythm so
the spacing stays uniform in both axes.

Width should follow the *content*, not the container. Long free text (names,
addresses) takes the full width; a number, a unit, or a short code shares a row.

Each cell in the row is a complete field group — **label above input, always**,
exactly as in a stacked form. Nothing about being in a row changes it. Labels
beside inputs would break alignment the moment one of them wrapped, and a form
that mixes both placements reads as unfinished. Keep row labels short enough not
to wrap in a narrow column ("Strength", "Unit", "Quantity").

#### File input

Don't hide or fake the native control — a custom "upload" widget that isn't a
real `<input type="file">` breaks keyboard access and drag-and-drop for no gain.
Style the native one instead, and leave the filename text beside it at `text-sm
text-slate-600`.

The button gets a **subtle fill**, not the secondary-button treatment. Style it
through the `file:` variant:

```
file:mr-3 file:rounded-md file:border-0 file:bg-slate-100 file:px-3 file:py-2
file:text-sm file:font-medium file:text-slate-700 hover:file:bg-slate-200
```

Two reasons it differs from `Button — secondary`: it sits on a **white card**,
where a white button reduces to a hairline border at the smallest size in the
form; and it must stay visually subordinate to the submit button, since choosing
a file is a step, not the action. Two equally prominent buttons in one form is
the failure on the other side.

Note that Tailwind's preflight zeroes this pseudo-element's padding, border, and
cursor, so every one of those has to be stated explicitly — nothing carries over
from the button styles elsewhere in the system.

Below it, a `text-xs text-slate-400` helper line stating what's accepted — file
inputs are the one control where users genuinely can't guess the constraint.

### Prescription upload panel

Sits directly above the list on the prescriptions page. A **Card** at `p-6`,
taking the full page-column width (it's embedded in a page, so the `max-w-sm`
standalone-panel rule does not apply).

Structure, top to bottom:

Panel element: `flex flex-col gap-4` on the card, so the heading, status slot,
and form are on one rhythm.

| Element | Treatment |
| --- | --- |
| Heading | `text-lg font-semibold text-slate-900` |
| Status line | Error `text-sm text-red-600`, success `text-sm text-emerald-600`. Reserve one slot above the fields; only one shows at a time. |
| Medication name | Full width |
| Physician name | Full width |
| Strength value / unit / quantity | One field row, three-up from `sm:` |
| Document | File input + helper line |
| Submit | Primary button, full width on mobile, `sm:w-auto`. Pending label "Uploading…", disabled while in flight. |

On success, clear every field including the file input, and show the
confirmation line. The list refetching below is itself the strongest feedback —
the confirmation is there to explain *why* the form went blank, so it can be
brief.

### Page composition — panel above list

When a page carries both a write affordance and the list it writes to, stack
them in one column with `gap-8` between — noticeably more than the `gap-4`
between sibling cards, so the two regions read as separate concerns rather than
as three cards in a row.

The page title sits above both. The list keeps its own four-state skeleton
underneath, and its **empty state should point at the panel above it** rather
than at some other page.

### Status badge

Inline pill. Base on every variant:

```
inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium
```

Then one tint pair — a light background with darker text of the same hue:

| Meaning | Classes |
| --- | --- |
| Delivered, best price | `bg-emerald-50 text-emerald-700` |
| Confirmed, shipped | `bg-teal-50 text-teal-700` |
| Pending, placed | `bg-amber-50 text-amber-700` |
| Failed, canceled | `bg-red-50 text-red-700` |
| Unknown | `bg-slate-100 text-slate-700` |

### Money

Always rendered from cents, never from a float. `$12.99` — two decimal places
always, including `$12.00`, and a thousands separator above `$999`. One shared
formatter; no per-component arithmetic.

A **total** is heavier than the unit price it derives from: `text-lg
font-semibold text-slate-900` against `text-sm text-slate-600`. The reader should
be able to find the number they're committing to without reading the label.

`formatCents` in `src/api/money.ts` is the one formatter — no per-component
arithmetic.

### Quote card

Same wide, low-density row as the prescription card, and for the same reason —
one card per quote, single column, full content width. The backend returns them
cheapest first; **preserve that order**, and never re-sort client-side.

Card element: standard **Card** classes at `p-4`, then `flex flex-col gap-3
sm:flex-row sm:items-center sm:justify-between` — same shape as the prescription
card.

Left block: `flex flex-col gap-1`.

| Line | Content | Classes |
| --- | --- | --- |
| Title row | Pharmacy name, then the badge | Row: `flex items-center gap-2`. Name: `text-base font-medium text-slate-900` |
| Detail | "{unit price} each × {qty}" | `text-sm text-slate-600` |

Right block: `flex items-center gap-4` holding the total, then the Order button.

- **Total** — `text-lg font-semibold text-slate-900`, per the Money section.
- **Order** — the primary button classes, verbatim. A button, not a text link:
  the prescription card's document link is a side errand, whereas this is the
  action the page exists for, and the weight should say so.

The first card carries a **`Best price` badge** — the emerald status-badge
variant (`bg-emerald-50 text-emerald-700`) — inline after the pharmacy name. It's
only honest because cheapest-first is a backend guarantee; if that stops being
true, the badge goes.

### Order confirmation panel

Replaces the quote list in place (see "Page composition — a step within a page").
Card classes at `p-6`, plus `max-w-2xl mx-auto flex flex-col gap-6`.

The summary is a **description list** (`<dl>`), not a form and not a table. On
the `<dl>`: `flex flex-col gap-3`. Each row: `flex justify-between gap-4`, with
`<dt>` at `text-sm text-slate-600` and `<dd>` at `text-sm text-slate-900
text-right`.

Rows, in order: medication (name + strength), quantity, pharmacy, price each,
shipping address.

The **total** sits apart from the list — its own row after the `<dl>`, with
`border-t border-slate-200 pt-4 flex justify-between items-baseline`. Label at
`text-sm text-slate-600`, amount at `text-lg font-semibold text-slate-900`. It is
the one number the user is actually agreeing to, so it does not sit in the same
rhythm as the details.

The shipping address `<dd>` holds two lines — street, then
"{city}, {state} {zip}" — so it needs `flex flex-col` to stack them.

Actions row: `flex flex-col gap-3 sm:flex-row sm:justify-end` — full-width
stacked on small screens, right-aligned from `sm:` up. **Cancel** first,
**Confirm** last (primary rightmost, same reasoning as the prescription card).
Cancel takes the **white-card** secondary variant — `bg-slate-100`, hover
`bg-slate-200`, no border — per the surface caveat.

While the order is in flight the primary button shows a pending label and both
buttons disable — a double-submitted order is a real order.

### Blocking notice — action unavailable

When the user can't take an action until they fix something elsewhere, the
confirm affordance is **replaced** by a notice, not accompanied by a disabled
button. A disabled button with no explanation is a dead end; the notice explains
and offers the way out.

```
bg-amber-50 border border-amber-200 rounded-lg p-4 flex flex-col gap-2
```

Inside: one line at `text-sm text-amber-900` stating the problem in the user's
terms — not the API's — then a link to where it gets fixed, using the text-link
treatment from the prescription card (`text-sm font-medium text-teal-600
underline hover:text-teal-700` plus the focus ring).

The Cancel action stays, so the user is never trapped in the step.

Carries `role="alert"`: it appears in response to the user's action and changes
what they can do next.

The notice is a **client-side courtesy**. The server rejects the same case with a
`400` regardless, and that response still has to be handled — see rule 7.

### Page composition — a step within a page

When a flow has a second step that depends entirely on data the first step is
holding, render it as a step **within the same route**, not a new one. The list
is replaced by the step; the page title stays.

Use this when the step's data can't be re-fetched on its own — a confirmation
built from a search result that has no endpoint of its own can't survive a
refresh at a URL of its own, so giving it a URL promises something it can't keep.
Reach for a real route when the step is independently addressable.

Cancel returns to the previous step and nothing else — it is not a navigation.

### Not-found page

Any URL that matches no route gets a real page, not an empty shell. Wrapper:
`flex flex-col gap-4`. Inside: the page-title classes, one line at
`text-slate-600` explaining the page doesn't exist, and a `Link` back to `/`
using the text-link treatment. A blank content area under a working header reads
as a bug, because it usually is one.

---

## 4. Rules that hold everywhere

1. **Every interactive element has a visible focus state.** Keyboard users and
   screen-reader users are not optional, and it is the accessibility detail
   interviewers notice first.
2. **Mobile-first.** Unprefixed classes are the small-screen case; add `sm:` /
   `md:` to layer on larger layouts. Don't design desktop-down.
3. **Semantic elements over `div`** — `header`, `nav`, `main`, `section`,
   `button`, `label`. A `div` with an `onClick` is not a button.
4. **Don't invent values.** Stay on Tailwind's scale (`p-4`, not `p-[17px]`).
   Arbitrary values are an escape hatch, and needing one usually means the
   design is wrong, not the scale.
5. **Loading and empty states are part of the design**, not an afterthought.
   Every list has an empty state; every async action has a pending state.
6. **Never auto-redirect a user out of what they were doing.** When they can't
   proceed, say why and link to the fix. Bouncing someone mid-task discards
   their intent and their context, and it steals the Back button — they landed
   somewhere they didn't choose. This holds even once the destination page
   exists and the redirect would "work."
7. **A client-side check is UX; the server is the authority.** Checking before
   you call saves a pointless round-trip and gives a better message, but the
   server rejects the case regardless and that rejection still needs handling.
   Two code paths, deliberately — never delete the second because the first
   makes it hard to reach.
