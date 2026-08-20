# MedMarket — Frontend Design Spec

The visual spec for the frontend. Written as a design handoff: it states intent
and values, not implementation. Translating these into Tailwind class strings is
the implementation work.

Tailwind's default palette and spacing scale are the source of truth for values,
so everything below names tokens (`slate-200`, `teal-600`) rather than hex codes.

---

## 1. Design tokens

### Color

| Role | Token | Used for |
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

Rule: **the page is `slate-50`, content sits on `white` cards with a
`slate-200` border.** That contrast is what gives the UI structure — avoid
white-on-white.

### Type

| Role | Size / weight |
| --- | --- |
| Page title | `text-2xl` `font-semibold` `tracking-tight` `slate-900` |
| Section heading | `text-lg` `font-semibold` `slate-900` |
| Body | `text-sm` `slate-600` |
| Label | `text-sm` `font-medium` `slate-700` |
| Helper / meta | `text-xs` `slate-400` |
| Wordmark | `text-lg` `font-semibold` `tracking-tight` `slate-900` |

### Spacing & shape

- Content column: **`max-w-5xl`, horizontally centered**. Header content and page
  content use the *same* max width so the columns align.
- Gutters: `px-4` on small screens, `px-6` from `sm:` up.
- Vertical rhythm: `py-8` around main content; `gap-4` between sibling cards.
- Corners: `rounded-lg` on cards and buttons; `rounded-md` on inputs.
- Elevation: `shadow-sm` on cards. Nothing heavier — this is a utility app.

---

## 2. App shell

### Header

- Full-bleed `white` bar with a `slate-200` bottom border. **No rounding, no
  side borders, no shadow** — a full-width bar with rounded corners reads as a
  mistake.
- Height ~`h-16`, content vertically centered.
- Inner container is width-constrained and centered, matching the content column.
- Left: wordmark "MedMarket", links to `/`.
- Right: nav links in a row, `gap-6`.

### Nav links

| State | Appearance |
| --- | --- |
| Default | `text-sm` `font-medium` `slate-600` |
| Hover | `slate-900` |
| Active (current route) | `teal-600` |

### Main

- Page background `slate-50`, and the shell should fill the viewport height so
  the background doesn't stop short on short pages.
- Content width-constrained and centered, same as the header, with `py-8`.

---

## 3. Components (as they arrive)

### Button — primary

`teal-600` background, white text, `text-sm` `font-medium`, `px-4 py-2`,
`rounded-lg`. Hover `teal-700`. Focus: visible ring in `teal-600` with a small
offset. Disabled: `slate-300` background, `slate-500` text, not-allowed cursor.

### Button — secondary

`white` background, `slate-200` border, `slate-700` text. Hover `slate-50`.
Same size and radius as primary.

**Surface caveat.** This variant assumes the `slate-50` page background, where
white reads as raised. On a **white card** it flattens into a hairline outline —
there, use a subtle fill instead (`slate-100`, hover `slate-200`, no border).
Check what a button is sitting on before reaching for this one.

### Input / textarea / select

Full width, `white`, `slate-300` border, `rounded-md`, `px-3 py-2`, `text-sm`.
Focus: `teal-600` border plus matching ring, and **remove the default browser
outline only if you replace it** — never leave a control with no focus
indicator. Error state: `red-600` border, message below in `text-xs` `red-600`.

### Card

`white`, `slate-200` border, `rounded-lg`, `shadow-sm`, `p-4` (or `p-6` for a
form panel).

### Prescription card

A wide, low-density row. One card per prescription, stacked in a single column
at the full width of the content column — no grid.

Internally it's a two-part row: **identity on the left, action on the right**,
pushed apart, vertically centered. On small screens let the action drop below
the identity block rather than crushing it.

Left block, three lines, tight vertical rhythm (`gap-1`):

| Line | Content | Style |
| --- | --- | --- |
| Title | Med name + strength as one string — "Lisinopril 20mg" | `text-base` `font-medium` `slate-900` |
| Detail | "Prescribed by {physician}" | `text-sm` `slate-600` |
| Meta | "Qty {quantity}" | `text-xs` `slate-400` |

Strength is `strengthValue` and `strengthUnit` concatenated with **no space** —
that matches how the backend's `MedStrength.String()` renders it.

Right: the document link — a text link, not a button. A button per row in a list
reads as heavier than the action deserves. `text-sm` `font-medium` `teal-600`,
hover `teal-700` with an underline. It opens a presigned URL on another origin,
so it targets a new tab and carries `rel="noopener noreferrer"`.

### List page states

Every list page has the same four-state skeleton under its page title. The title
is always rendered — it never disappears into a loading branch.

| State | Treatment |
| --- | --- |
| Loading | Body copy, `slate-600` — "Loading prescriptions…". Skeletons aren't worth it at this scale. |
| Error | `text-sm` `red-600`, generic copy. Never render a raw exception message. |
| Empty | Centered inside a dashed `slate-300` bordered panel, `rounded-lg`, `p-8`: one line of `slate-600` explaining the list is empty, and a `slate-400` `text-xs` line pointing at the next action. |
| Loaded | The list itself. |

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

Form-level error messages sit above the first field: `text-sm` `red-600`.
Field-level errors sit below their input: `text-xs` `red-600`.

A form-level **notice** — a success or informational message carried in from
another page, like "Account created — please sign in" — occupies that same slot
above the first field, `text-sm` `emerald-600`. One slot, so the form doesn't
reflow depending on which message appears. A notice and an error are mutually
exclusive: the notice says what to do next, the error says the last attempt
failed, and once there's an error the notice has been overtaken. Show the error
and drop the notice.

**Field hints.** A rule the user needs *before* typing — a password policy, an
accepted format — goes between the label and the input, `text-xs` `slate-400`.
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
`text-xs`. The key goes *above* because a legend explaining a notation is no use
after the reader has already met the notation and guessed at it. Keep the key
tight to the title (`gap-1`) rather than on the panel's own rhythm, so it reads
as an annotation on the heading and not as a section of its own.

The asterisk is **`slate-500`, not `red-600`**. Red is reserved for errors and
destructive actions; if required markers are red, a form with nothing wrong
still reads as a form full of problems. The marker's job is to be catchable on a
scan, not to signal danger. Use the same `slate-500` for the key.

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
Style the native one instead, and leave the filename text beside it at `text-sm`
`slate-600`.

The button gets a **subtle fill**, not the secondary-button treatment:
`slate-100` background, hover `slate-200`, `slate-700` text, `rounded-md`,
`text-sm` `font-medium`, and its own padding. No border.

Two reasons it differs from `Button — secondary`: it sits on a **white card**,
where a white button reduces to a hairline border at the smallest size in the
form; and it must stay visually subordinate to the submit button, since choosing
a file is a step, not the action. Two equally prominent buttons in one form is
the failure on the other side.

Note that Tailwind's preflight zeroes this pseudo-element's padding, border, and
cursor, so every one of those has to be stated explicitly — nothing carries over
from the button styles elsewhere in the system.

Below it, a `text-xs` `slate-400` helper line stating what's accepted — file
inputs are the one control where users genuinely can't guess the constraint.

### Prescription upload panel

Sits directly above the list on the prescriptions page. A **Card** at `p-6`,
taking the full page-column width (it's embedded in a page, so the `max-w-sm`
standalone-panel rule does not apply).

Structure, top to bottom:

| Element | Treatment |
| --- | --- |
| Heading | Section heading — `text-lg` `font-semibold` `slate-900` |
| Status line | Error `text-sm` `red-600`, success `text-sm` `emerald-600`. Reserve one slot above the fields; only one shows at a time. |
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

Inline pill: `rounded-full`, `px-2 py-0.5`, `text-xs` `font-medium`, tinted
background with a darker text of the same hue — `emerald` delivered, `teal`
confirmed/shipped, `amber` pending, `red` failed, `slate` unknown.

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
