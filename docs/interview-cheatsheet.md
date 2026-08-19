# Interview cheat sheet — Tue 2026-08-11, 3pm

Target codebase: Nx monorepo, NestJS + TypeORM + Postgres API (`:3001`), React 19 + Vite SPA (`:3000`),
TanStack Query, React Router v6, React Hook Form + Zod, Tailwind.

**Calibration.** The stated requirements are React, TypeScript, and Node. Zod, NestJS, and TypeORM are
their stack, not your claims — they were never on your resume and never discussed, so nobody is
grading recall on them. What's actually being assessed is how you orient in an unfamiliar codebase and
whether your React/TS reasoning holds up under questions. Everything below is calibrated to that:
enough of their libraries to move confidently and name what you're looking at, real depth only where
you're expected to have it. See section 5 for how to say "I haven't used this" and gain ground.

## Morning schedule (~2 hours)

| Time | Item |
| --- | --- |
| 0:00–0:10 | Read "Their conventions" (§3) and §5. No code. |
| 0:10–0:55 | **Hands on: RHF + Zod.** Convert `frontend/src/pages/Login.tsx` to `useForm` + `zodResolver` + `useMutation`. |
| 0:55–1:20 | NestJS anatomy (§2). Trace how you'd add `GET /users?type=patient`. Shape, not memorization. |
| 1:20–1:35 | Deltas: router v6, Tailwind v3 config, TypeORM shapes. |
| 1:35–1:50 | Say the §4 discussion answers out loud. Retrieval, not reading. |
| 1:50–2:00 | Re-read §5 and the habits list. That's what you carry in. |

Cut, deliberately: `drill.tsx` cold rewrite, `PrescriptionCard` extraction, writing TypeORM migrations.

The RHF block stays large even though Zod isn't a requirement. Typing it once is the difference
between "the schema is the source of truth and the type is inferred off it" sounding like
comprehension versus sounding like you're reading the file for the first time.

---

## 1. React Hook Form + Zod — the whole API you need

The one real gap. Their "new patient creation" page is built from exactly this.

### Mental model

RHF is **uncontrolled by default**. `register` hands the input a `ref` and lets the DOM own the value —
the opposite of your `value={email} onChange={...}`. That's why it re-renders far less than
`useState`-per-field. Validation runs on submit (by default), Zod produces the errors, RHF stores them.

### Canonical shape

```tsx
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'

const schema = z.object({
  email: z.string().email('Enter a valid email'),
  password: z.string().min(8, 'At least 8 characters'),
})

type FormValues = z.infer<typeof schema>   // types derived FROM the schema, never hand-written

function Login() {
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
    reset,
    setError,
  } = useForm<FormValues>({ resolver: zodResolver(schema) })

  const onSubmit = async (values: FormValues) => { /* values are validated + typed */ }

  return (
    <form onSubmit={handleSubmit(onSubmit)}>
      <input {...register('email')} />
      {errors.email && <p>{errors.email.message}</p>}
      <button disabled={isSubmitting}>Sign in</button>
    </form>
  )
}
```

### Things to actually remember

- `handleSubmit(onSubmit)` is a **wrapper** — it validates first, calls `preventDefault` for you, and
  only calls `onSubmit` if the schema passes. You never write `e.preventDefault()`.
- `{...register('email')}` spreads `name`, `ref`, `onChange`, `onBlur`. Don't also pass `value`.
- `errors.email?.message` is the string from the Zod message. Field name is the object key.
- `isSubmitting` replaces your `isPending` state while `onSubmit` is awaited.
- Server-side errors: `setError('email', { message: 'Already taken' })`, or `setError('root', {...})`
  for a form-level error, read back as `errors.root?.message`.
- `reset()` clears the form after a successful submit.
- `defaultValues: { email: '' }` in `useForm` — needed for edit forms (pre-fill from a query).
- `mode: 'onBlur'` / `'onChange'` if they ask for live validation. Default is `'onSubmit'`.
- Non-native inputs (a custom `<Select>`, a date picker) can't take `register` — use
  `<Controller name="x" control={control} render={({ field }) => <Select {...field} />} />`.
  Know this exists; you probably won't need it.

### Zod bits worth knowing

```ts
z.string().min(1, 'Required')
z.string().email()
z.coerce.number().int().positive()          // "12" from an input → 12
z.enum(['patient', 'physician', 'advocate', 'internal'])
z.string().optional()                        // string | undefined
z.string().nullable()                        // string | null
z.object({...}).refine(v => v.a === v.b, { message: '...', path: ['b'] })  // cross-field
schema.parse(data)      // throws
schema.safeParse(data)  // { success, data } | { success, error }
```

`z.infer<typeof schema>` is the point: **the schema is the source of truth, the type is derived.**
If asked "why Zod and not just a TS interface" — TS types vanish at runtime; a Zod schema validates
data arriving from the network, and the type falls out for free. Same schema can validate an API
response, not just a form.

### Your exercise

Convert `frontend/src/pages/Login.tsx`. It currently has four `useState`s
(`email`, `password`, `isPending`, `formError`) and a hand-rolled `handleSubmit`. Target:

- `schema` + `z.infer` type at module level.
- One `useForm` call, `zodResolver`, `register` on both inputs.
- `useMutation({ mutationFn: ({email, password}) => login(email, password) })`, submit calls
  `mutation.mutateAsync(values)` inside a `try/catch` so `setError('root', ...)` can show the 401 copy.
- Keep `isValidRedirect` + `navigate(dest, { replace: true })` exactly as they are.
- Delete all four `useState`s. If any survive, you've missed something.

You'll need `npm i react-hook-form @hookform/resolvers zod` (installs are fine; it's your repo).

---

## 2. NestJS anatomy

You already know this pattern — it's constructor injection at a composition root, i.e. your
`cmd/server/main.go` with decorators doing the wiring instead of you.

### Mapping from what you know

| MedMarket (Go) | NestJS |
| --- | --- |
| `main.go` wiring | `@Module({ providers, controllers, imports })` |
| `inbound/http` handler | `@Controller('users')` class |
| `app.UserService` | `@Injectable()` service class |
| port interface | usually just the service class (Nest uses classes as DI tokens) |
| OpenAPI request struct | DTO class + `class-validator` decorators |
| Ent schema | TypeORM `@Entity()` class |

### The four files

```ts
// user.entity.ts — the table
@Entity('users')
export class User {
  @PrimaryGeneratedColumn('uuid') id: string
  @Column() email: string
  @Column({ type: 'enum', enum: UserType }) type: UserType
  @Column({ type: 'jsonb', nullable: true }) metadata: Record<string, unknown>
  @CreateDateColumn({ name: 'created_at' }) createdAt: Date
  @UpdateDateColumn({ name: 'updated_at' }) updatedAt: Date
}

// dto/create-user.dto.ts — the request body shape
export class CreateUserDto {
  @IsEmail() email: string
  @IsEnum(UserType) type: UserType
}

// users.service.ts — the business logic
@Injectable()
export class UsersService {
  constructor(@InjectRepository(User) private readonly repo: Repository<User>) {}

  findAll(type?: UserType) {
    return this.repo.find({ where: type ? { type } : {} })
  }

  async findOne(id: string) {
    const user = await this.repo.findOne({ where: { id } })
    if (!user) throw new NotFoundException(`User ${id} not found`)
    return user
  }

  create(dto: CreateUserDto) {
    return this.repo.save(this.repo.create(dto))
  }
}

// users.controller.ts — the HTTP surface
@Controller('users')
export class UsersController {
  constructor(private readonly users: UsersService) {}

  @Get()      findAll(@Query('type') type?: UserType) { return this.users.findAll(type) }
  @Get(':id') findOne(@Param('id') id: string)        { return this.users.findOne(id) }
  @Post()     create(@Body() dto: CreateUserDto)      { return this.users.create(dto) }
  @Patch(':id') update(@Param('id') id: string, @Body() dto: UpdateUserDto) { ... }
  @Delete(':id') remove(@Param('id') id: string)      { return this.users.remove(id) }
}
```

### Points that matter

- **DI is by class token.** `constructor(private readonly users: UsersService) {}` — Nest reads the
  parameter *type* and injects the instance. The provider must be listed in the module's `providers`,
  and the module must be listed in `imports` wherever it's used. "It says `Nest can't resolve
  dependencies of X`" → the provider isn't in `providers`, or its module isn't imported.
- `private readonly` in the constructor signature is TypeScript **parameter properties** — it declares
  and assigns the field in one line. Not a Nest thing.
- **Returning a value = 200 JSON.** `@Post` defaults to 201. Throw `NotFoundException`,
  `BadRequestException`, `ConflictException` for other statuses — Nest's exception filter maps them.
- **DTO validation** requires `app.useGlobalPipes(new ValidationPipe())` in `main.ts`. Add
  `{ whitelist: true }` to strip unknown properties. If the DTOs have `class-validator` decorators,
  this is already on — check `main.ts`.
- **Module structure:** `@Module({ imports: [TypeOrmModule.forFeature([User])], controllers: [UsersController], providers: [UsersService], exports: [UsersService] })`.
  `exports` is what makes a provider available to other modules that import this one.
- Global route prefix is often `app.setGlobalPrefix('api')` in `main.ts` — that's why the Vite proxy
  maps `/api/*` to `:3001`.

### TypeORM query bits

```ts
this.repo.find({ where: { type }, order: { createdAt: 'DESC' }, take: 20, skip: 0 })
this.repo.findOne({ where: { id }, relations: ['physician'] })
this.repo.create(dto)   // builds an entity instance, does NOT hit the DB
this.repo.save(entity)  // insert or update
this.repo.update(id, dto)
this.repo.delete(id)
ILike('%term%')  // case-insensitive search: where: { name: ILike(`%${q}%`) }
```

`create` then `save` (not `save(dto)`) so entity defaults and hooks run.

---

## 3. Deltas from what you built

### React Router v6 vs your v8

- Package is **`react-router-dom`**, not `react-router`. Import from there.
- Likely uses `createBrowserRouter([...])` + `<RouterProvider router={router} />` in `main.tsx`,
  with routes as a data array in `router.tsx` — not JSX `<Routes>`. Nested children under a
  `element: <RootLayout />` with `<Outlet />` — same idea you already built.
- `useNavigate`, `useParams`, `useLocation`, `<Link>`, `<NavLink>`, `<Navigate>` — identical to yours.
- `useParams()` returns `Record<string, string | undefined>` — the `id` from `/patients/:id` is
  `string | undefined`, so it needs narrowing before use.
- Data-router extras exist (`loader`, `action`, `useLoaderData`) but with TanStack Query in the stack
  they're probably unused. Don't reach for them.

### Tailwind v3 (config file) vs your v4 (CSS-first)

- Theme lives in `tailwind.config.js` under `theme.extend`, not `@theme` in CSS.
- `content: [...]` globs must include any new file path — irrelevant in v4.
- `outline-none` in v3 is v4's `outline-hidden`. Everything else you know carries over.

### Their frontend conventions (read `lib/` first thing)

- **`lib/api.ts`** — typed API client. Look for the function naming (`getPatients`, `createPatient`)
  and *follow it*. Don't inline `fetch` in a component; that's the pattern this codebase has already
  rejected.
- **Query key factory** — something like
  `export const patientKeys = { all: ['patients'] as const, list: (f) => [...patientKeys.all, 'list', f] as const, detail: (id) => [...patientKeys.all, 'detail', id] as const }`.
  Use it for both `useQuery` keys and `invalidateQueries`. Never hand-write a key literal.
  This is the "everything the queryFn closes over goes in the key" rule, factored out.
- **`lib/schemas.ts`** — Zod schemas live here, shared. Add yours here, not in the component.
- After a mutation: `queryClient.invalidateQueries({ queryKey: patientKeys.all })`. You did exactly
  this in the upload feature.

### Nx

`nx serve api`, `nx build web`, `nx graph`. `tsconfig.base.json` `paths` gives cross-project imports
like `@project/shared`. That's the whole of it — it's a task runner with a dependency graph and a
build cache. Don't let the word "monorepo" cost you any time.

---

## 4. Discussion answers — say these out loud

**The render model.** A component function isn't a constructor, it *is* the render. It re-runs top to
bottom on every render. The hook calls happen every render; the state persists across them. Each
render is a snapshot — the values in it never change, a new render produces new values. `useEffect`
during render only *registers* setup + deps; React compares deps after commit and runs cleanup then
setup.

**Keys.** A stable item ID, never the array index — the index is position wearing a costume, and it
changes exactly when keys matter. Unique among siblings only. Goes on the outermost element the
`.map` callback returns. `key` is consumed by React; it is not a prop.

**Effect deps / the infinite loop.** An effect that sets state included in its own deps loops. Also:
an object or array literal in the deps is a new reference every render, so the effect runs every
time. Fix by moving the value out, memoizing it, or depending on the primitive inside it.

**Controlled vs uncontrolled.** Controlled = React state is the source of truth (`value` + `onChange`).
Uncontrolled = the DOM owns it, you read it with a ref. A file input **cannot** be controlled — the
browser forbids setting `value` — so a ref is the only way to read `.files[0]` and the only way to
clear it. RHF's `register` is the uncontrolled approach, which is why it re-renders less.

**Why not Redux.** Most of what people put in Redux is *server* state — remote, async, cached,
shared, and stale the moment you have it. TanStack Query owns that: caching, deduping, background
refetch, invalidation. What's left is genuinely *client* state (auth token, a modal flag, a theme),
and that's small enough for context + `useReducer`. Redux is a fine answer when you have complex
client-side state with real cross-cutting transitions; it's the wrong tax for a CRUD app.

**`useMemo` / `useCallback`.** They cost something — a deps array, a comparison every render, and
real cognitive weight. Reach for them when (a) the computation is genuinely expensive, (b) the value
is a dependency of an effect or another hook, or (c) it's passed to a `React.memo`'d child where a new
reference would defeat the memo. Reflexive memoization is the wrong answer: it adds bugs (stale deps)
without measured wins. React Compiler is making most of it unnecessary anyway.

**TanStack Query vs Apollo's cache.** Apollo *normalizes* by `__typename` + `id`, so updating one
entity propagates everywhere it appears. TanStack caches opaque blobs keyed by `queryKey` and you
invalidate explicitly. Simpler model, transport-agnostic, manual coordination.

**Optimistic updates**, if it comes up: `onMutate` cancels in-flight queries, snapshots the cache,
writes the expected value; `onError` rolls back to the snapshot; `onSettled` invalidates.

---

## 5. When you hit something you haven't used

You are not expected to know Nest, TypeORM, or Zod. You *are* expected to be effective in a codebase
that uses them. Those are different skills, and the second one is more senior. Play it that way.

**The move, in three beats:** name what you don't know, state what you can infer from the code in
front of you, then proceed. Never stall, never bluff.

> "I haven't worked with TypeORM specifically, but this is the same repository pattern I've used with
> other ORMs — `create` builds the entity, `save` persists it. Let me check how the existing service
> does a filtered find so I match it."

> "Zod is new to me, but I can read what it's doing here — the schema validates at runtime and the TS
> type is inferred from it, so I only edit the schema and the type follows. I'd add the field there."

> "I've not used Nest, though the DI here looks like constructor injection with the module declaring
> the providers — same wiring I do by hand in Go. So to add an endpoint I'd touch the controller, the
> service, and the DTO. Is there anything else in this repo that a new endpoint usually needs?"

That last clause — asking what a new endpoint conventionally touches — is a strength move. It shows
you know codebases have unwritten conventions and that you'd rather match them than guess.

**Look before you write.** On any task, the first thing you do is find the nearest existing example
and read it: an existing controller before adding one, an existing page before adding a page, an
existing schema before adding a field. Say it out loud — "let me see how patients list does this" —
because the interviewer can't see you reading. Matching an existing pattern is nearly always the
correct answer, and it costs you nothing to not know the library.

**Where to spend your uncertainty budget.** Be relaxed about their libraries and precise about React,
TypeScript, and HTTP/data flow. If you're going to be firm about anything, be firm there — that's what
they asked for. "I'd have to look up the Zod API for that" is free. "I'm not sure why this re-renders"
is not.

**Don't volunteer weakness unprompted.** Lead with what you're doing, mention unfamiliarity only when
it's actually load-bearing. "I'll add the field to the schema and the type comes with it" is better
than "I don't really know Zod, but I think I'd add the field to the schema."

**If you get stuck, externalize.** Say what you're trying to do, what you expected, and what you're
seeing. That converts silence into a debugging conversation, which is exactly what pairing with you on
the job would look like. Asking one good question beats five minutes of quiet.

## Habits to carry in

- `fetch` does not reject on 4xx/5xx. Check `res.ok`; `catch` is only network failure.
- `res.json()` returns `any` — annotate at the boundary, once, and let inference do the rest.
- An unneeded cast or `?.` silences a question instead of answering it.
- Never bare-concatenate `className` strings. Template literal with a visible space.
- A Tailwind modifier is inert without its base: `gap` needs `flex`/`grid`, `flex-col` needs `flex`,
  `border-dashed` needs `border`, `focus:ring-*` color needs `focus:ring-2`.
- Derived state is an anti-pattern — compute it during render.
- Read the surrounding code and match it. On someone else's codebase that's most of the grade.
- Narrate what you're doing while you type. Silence reads as stuck; thinking out loud reads as senior.
