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

**Panel width.** A standalone form panel (login, register) is `max-w-sm`,
horizontally centered with `mx-auto`. Forms embedded in a page take the page
column's width.

Width follows content, not the viewport:

| Content | Width |
| --- | --- |
| Single-column form | `max-w-sm` – `max-w-md` |
| Prose | `max-w-prose` (65ch) |
| Page / app content column | `max-w-5xl` |

Form-level error messages sit above the first field: `text-sm` `red-600`.
Field-level errors sit below their input: `text-xs` `red-600`.

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
