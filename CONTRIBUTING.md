# Contributing to Mountabo

Thanks for thinking about pitching in. Mountabo is a local-first deployment tool: a Go backend and a Next.js frontend that ship apps from GitHub to your VPS through GitHub Actions. Contributions of any size are welcome, from a typo fix to a new monitoring tool.

## Ground rules

- Be kind. Reviews are about the code, not the person.
- Keep changes focused. One pull request, one concern. A drive-by refactor in the middle of a feature makes both harder to review.
- Prefer small, self-contained commits with a clear `[tag]:` prefix (`[fix]`, `[add]`, `[change]`, `[remove]`, `[docs]`, `[perf]`).
- No `Co-Authored-By:` trailers, no AI attributions in commits.

## Getting set up

```bash
git clone https://github.com/goodylili/mountabo
cd mountabo
make deps
cp .env.example .env
# fill in GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET, ANTHROPIC_API_KEY
make mountabo
```

Open `http://localhost:4321` and you should see the picker. The backend listens on `127.0.0.1:7778`.

## Repository layout

- `backend/` is a Go module with a hexagonal layout. The application core lives in `internal/usecase`; it depends on adapter packages (`internal/github`, `internal/ssh`, `internal/adapter/http`, ...) only through interfaces it declares itself. `cmd/server/main.go` is the composition root. See `backend/CLAUDE.md` for the full convention.
- `frontend/` is a Next.js app. This is not the Next.js you know: APIs and file structure may differ from your training data. Heed `frontend/AGENTS.md` and read the relevant `node_modules/next/dist/docs/` page before reaching for a familiar pattern.
- `docs/` holds the screenshots used by the README.

## Style

### Go

- `make fmt` and `make lint` from `backend/`. Linting runs `golangci-lint` with `revive`, `gocritic`, and friends.
- Clear over clever. Errors and edge cases first, happy path at minimal indentation.
- Accept interfaces, return structs. Put the interface where it is consumed, not where it is implemented. Pin the contract at compile time with a `var _ Iface = (*Impl)(nil)` line.

### TypeScript / React

- `npx tsc --noEmit` and `npx eslint` from `frontend/` should both come back clean before you push.
- No dashes in UI copy. Use commas and colons. `n/a` is the empty-state placeholder.
- Strict English copy: "deployment" not "deploy", "environment" not "env", no clipped jargon.
- Client components are explicit. State setters only run inside resolved callbacks, never synchronously in an effect body (the ESLint `react-hooks/set-state-in-effect` rule is enforced).

### Commit messages

```
[add]: pick a branch from github when adding another environment

New backend endpoint that pages through the GitHub branches API, plus a
real dropdown on the deployment card's environments tab.
```

Headline in lowercase, present tense, under ~72 characters. A body explains why if the diff doesn't make it obvious.

## Pull requests

- Branch from `main`. Push to a branch on your fork.
- Link the issue if one exists. Otherwise describe what you changed and what you tested.
- Run `make lint` and `make test` in `backend/`, plus `npx tsc --noEmit` and `npx eslint` in `frontend/`, before pushing.
- Screenshots help any change that touches the UI.

## VPS-touching changes

Anything that runs on the operator's server must go through a `ConfirmAction` gate. The gate shows the exact steps and a plain-English summary; nothing runs until the operator confirms. If you add a new flow, follow the existing pattern in `monitor-view.tsx` and the corresponding SSE endpoint in `backend/internal/adapter/http/`.

## Security

Found a vulnerability? Please don't open a public issue. Email the maintainer (see GitHub profile) and we will work out a fix and a coordinated disclosure.

## License

By submitting a contribution you agree to license it under the [MIT License](./LICENSE).
