# Skills.md — Agent Guide to Mountabo

This file is the entry point for any AI agent working in this repository. Read it before touching code. It is exhaustive on purpose: layout, conventions, commands, gotchas. When the rules here conflict with general training instincts, this file wins.

## 1. What Mountabo is

Mountabo is a **local-first deploy tool**. A user runs it on their laptop, connects GitHub, registers a VPS, and configures a repo once. Pushes to the chosen branch then deploy through a generated GitHub Actions workflow straight to the user's server. Mountabo does not sit in the request path of deploys; it only sets them up and observes them.

Surfaces:
- **Backend** — Go HTTP server on `127.0.0.1:7778` (hexagonal architecture).
- **Frontend** — Next.js 16 + React 19 + Tailwind v4 on `http://localhost:4321`.
- Single root `.env` feeds both processes (`godotenv` upward search for Go, Next `loadEnvConfig` with `forceReload`).

Run everything together: `make mountabo` (from repo root). Stop with Ctrl-C.

## 2. Repo layout

```
mountabo/
  Makefile               # `mountabo`, `backend`, `frontend`, `deps`
  PRD.md                 # product spec, source of truth for intent
  README.md              # user-facing docs
  skills-lock.json       # pinned Vercel/Next agent skills (do not hand-edit)
  backend/
    CLAUDE.md            # hexagonal rules (READ THIS BEFORE BACKEND WORK)
    Makefile             # `build`, `test`, `fmt`, `lint`
    cmd/server/          # composition root: main.go wires concrete structs
    internal/
      usecase/           # application core; owns ports (interfaces)
      adapter/
        http/            # handlers, router, server (NOT internal/http)
        repository/      # in-memory persistence (SQLite later)
      ai/                # Anthropic helper for /terminal AI suggestions
      config/            # Load() -> *Config
      docker/ github/ keychain/ nginx/ ssh/ workflow/
                         # concrete adapters; grow as structs first
  frontend/
    CLAUDE.md + AGENTS.md  # Next.js 16 has breaking changes vs training data
    src/
      app/               # Next App Router: api/, configure/, connect/, deployments/, terminal/
      components/        # React components (kebab-case .tsx)
      lib/               # client/server helpers (data fetchers, previews)
  docs/screenshots/      # README assets
```

## 3. Backend conventions (hexagonal)

The load-bearing rule, from `backend/CLAUDE.md`: **accept interfaces, return structs**. Every backend change must honor it.

1. Constructors return concrete `*T`, never an interface.
2. Dependencies cross layer boundaries as interface parameters on the constructor.
3. **Interfaces are defined where they are consumed (`usecase`), not where they are implemented.** A repository must not import `usecase` to grab an interface, and must not define the port itself.
4. Do not pre-extract interfaces. Wait for a second implementation or a real test seam. Empty stub packages (`config`, `github`, `ssh`, `nginx`, `docker`, `workflow`) start as concrete structs.
5. Interfaces stay small (1–3 methods), `-er` suffix where natural. Compose, do not widen.
6. Pin the contract in the implementing package: `var _ usecase.UserStore = (*UserMemory)(nil)`.

Layer rules:
- `internal/usecase` — pure application logic. No `net/http`, no SQL, no SSH, no GitHub SDK types may leak in. Tests live next to it (`*_test.go`).
- `internal/adapter/repository` — concrete persistence, satisfies usecase ports.
- `internal/adapter/http` — handlers, router, server. Construct from usecase `*Service` values; expose `*Handler`, `*Server`.
- `internal/config` — `Load()` returns `*Config`.
- `cmd/server/main.go` — the **only** composition root. New wiring goes here.

Dependency direction always points inward toward `usecase`. If you find yourself importing `adapter/...` from `usecase`, stop and refactor.

### Backend commands (run from `backend/`)
- `make build`
- `make test`
- `make fmt`
- `make lint`

Run these before declaring backend work complete. `go run ./cmd/server` is what `make backend` does at the root.

### Go skills to consult
Before writing Go, invoke the matching `golang-*` skill via the Skill tool. At minimum check: `golang-project-layout`, `golang-naming`, `golang-error-handling`, `golang-testing`, `golang-code-style`, `golang-concurrency`, `golang-context`, `golang-dependency-management`, `golang-lint`, `golang-documentation`. If multiple apply, apply all.

## 4. Frontend conventions

This repo runs **Next.js 16 + React 19 + Tailwind v4**. The frontend `CLAUDE.md` is blunt: this is *not* the Next.js your training data describes. Before touching frontend code, read the relevant guide under `frontend/node_modules/next/dist/docs/`. Heed deprecation notices.

- App Router only. Routes live under `src/app/<segment>/page.tsx` and API handlers under `src/app/api/<segment>/route.ts`.
- Server vs client components: prefer server components; mark client code with `"use client"` only when needed (state, effects, browser APIs).
- Components: kebab-case file names (`server-domains.tsx`), default-exported React functions.
- Shared client/server logic lives in `src/lib/*.ts`; one file per feature area.
- TypeScript strict; use existing types in `src/lib` before inventing new ones.
- Tailwind v4 uses the new postcss plugin — no `tailwind.config.js` of the old shape. Don't add one.
- Lint with `npm run lint` in `frontend/`. Dev server: `npm run dev` (port 4321). Build: `npm run build`.

### Vercel / web skills available
`vercel-react-best-practices`, `vercel-composition-patterns`, `vercel-optimize`, `vercel-react-view-transitions`, `web-design-guidelines`. Consult them for any non-trivial UI work.

## 5. UI copy rules (hard requirements)

These are recurring user corrections; agents have gotten them wrong repeatedly.

- **No dashes** in any UI copy — no em dash, en dash, or minus dash. This includes hardening descriptions served by the backend. Use commas, colons, or rewriting. Use `n/a` for missing placeholders.
- **Standard, full grammatical English.** "deployment", not "deploy"; "configuration", not "config" in user-visible text. Avoid clipped jargon. Backend strings shown in the UI follow the same rule.

When in doubt, re-read copy aloud: it should sound like a paragraph in a manual, not a CLI flag list.

## 6. Authoritative flows (do not regenerate elsewhere)

### Deploy generation is **backend-authoritative**
- The GitHub Actions workflow and `deploy.sh` are generated **only** in Go (`internal/workflow`) and exposed via `/api/deploy/preview`.
- The frontend **previews**, never regenerates. If the preview looks wrong, fix the Go generator; do not patch on the client.
- Two deploy strategies: single-Dockerfile and docker-compose. Both branches live in the Go generator.

### Custom domains
- Per-server nginx + Let's Encrypt logic lives in `internal/nginx`.
- Applied over SSH via the RootRunner.
- Persisted on the Server record as `Server.Domains`.
- UI mirrors a Vercel-style add/list/remove flow (`ServerDomains` component).

### Server bootstrap
- Add server (with SSH probe) → JSON persist → SSE-streamed bootstrap that creates the `mountabo` user and manages an ed25519 key lifecycle (generate / store / wipe / destroy).
- Keys are stored in the OS keychain (`internal/keychain`), never on disk.

### GitHub auth
- Classic OAuth App (scopes: `repo`, `workflow`) so every repo is visible. The earlier GitHub App approach was abandoned; do not re-introduce it.
- Tokens persisted in the keychain.

### Local env loading
- A single root `.env`. The Go process uses `godotenv` with upward search. Next picks it up via `loadEnvConfig` with `forceReload` and a matching env key. Don't add per-package `.env` files.
- Dropboy reserves port 7777; Mountabo backend uses 7778, frontend uses 4321.

## 7. AI helper (terminal page)

- Located at `/terminal`. Calls Anthropic directly from the Go backend (`internal/ai`).
- Requires `ANTHROPIC_API_KEY` in `.env`. Without it the terminal still works; the helper shows a hint.
- **Suggests only** — it must never auto-execute a command. The user clicks "use this command" to populate the input.
- Bring-your-own-key. Key never leaves the user's machine.

## 8. Commit and PR workflow

- After every major update, commit and push to `main`.
- Commit messages: short, `[tag]:` prefix. Tags seen in history: `[add]`, `[docs]`, `[fix]`, etc. Example: `[add]: filter the container logs to one service in a compose stack`.
- **Never** add a `Co-Authored-By: Claude ...` trailer or any self-attribution. Commits are authored solely by the user.
- Do not skip hooks. Do not force-push `main`.
- Stage specific files; avoid `git add -A` so `.env` and similar never sneak in.

## 9. Testing and verification

- Backend: `make test` from `backend/`. Several usecase files have companion `_test.go`; mirror that pattern when adding logic.
- Frontend: no test runner is configured. For UI changes, run the dev server and exercise the path manually (see the `verify` and `run` skills). If you cannot verify visually, say so explicitly — do not claim a UI change works because the build passed.
- Type-checking and linting verify code correctness, not feature correctness.

## 10. Skills to invoke proactively

Use the Skill tool when a task fits. High-relevance picks for this repo:
- Any Go work → relevant `golang-*` skill(s).
- Anthropic API integration in `internal/ai` → `claude-api`.
- Running or screenshotting the app → `run`.
- Verifying a change end-to-end → `verify`.
- Reviewing the diff before commit → `code-review` (or `simplify` to auto-apply).
- Settings / hooks / permissions changes → `update-config`.

Do not invent skill names. Only call skills listed by the harness.

## 11. Don't do this list

- Don't return interfaces from constructors.
- Don't define ports inside adapter packages.
- Don't put HTTP/SSH/GitHub SDK types in `usecase`.
- Don't regenerate the deploy workflow on the frontend.
- Don't introduce em/en/minus dashes into user-visible copy.
- Don't write `internal/http` — it's `internal/adapter/http`.
- Don't add per-package `.env` files; there is one root `.env`.
- Don't reintroduce the GitHub App flow.
- Don't add `Co-Authored-By: Claude` trailers.
- Don't add features, abstractions, or error handling beyond what the task requires.
- Don't write planning or summary markdown files unless the user asks.

## 12. When something is unclear

The PRD (`PRD.md`) is the source of truth for *intent*. `backend/CLAUDE.md` is the source of truth for *backend structure*. `frontend/CLAUDE.md` + `AGENTS.md` are the source of truth for *frontend conventions*. This file (`Skills.md`) summarizes them and adds the cross-cutting rules. If the three disagree, follow the more specific file and flag the conflict to the user.