# Backend conventions

Go module `github.com/goodylili/mountabo`. Hexagonal layout: `usecase` (application
core) depends on `adapter` implementations only through interfaces it defines itself.

## Primitive: accept interfaces, return structs

This is the load-bearing rule for the whole backend. Apply it everywhere; the points
below are how it lands in each layer.

1. **Constructors return concrete types — never interfaces.**
   `func NewUserService(store UserStore) *UserService`. The caller gets the full
   struct API and can assign it to an interface itself if it ever needs to. A
   constructor that returns an interface is wrong.

2. **Accept interfaces as dependencies.** A struct's dependencies that cross a layer
   boundary (a store, a remote client, a clock) come in as interface parameters on the
   constructor, so tests can pass a stub.

3. **Define interfaces where they're consumed — not where they're implemented.**
   The port lives in `usecase`. Example: `usecase` declares
   `type UserStore interface { ... }` and `internal/adapter/repository` exports a
   concrete `*UserMemory` that happens to satisfy it. The repository package does NOT
   import `usecase` for an interface, and does NOT define `UserStore` itself.

4. **Don't create interfaces prematurely.** Wait for a *second* implementation or a
   real test seam before extracting one. A single-implementation interface sitting next
   to its only impl is indirection with no payoff — start concrete and extract later.
   The empty stub packages (`config`, `github`, `ssh`, `nginx`, `docker`, `workflow`)
   should grow as concrete structs first; add a port in `usecase` only when `usecase`
   actually consumes them.

5. **Keep interfaces small (1–3 methods)** and name them for behaviour with the `-er`
   suffix where it reads naturally (`Sender`, `KeyStore`, `Deployer`). Compose larger
   contracts from small ones rather than declaring one wide interface.

6. **Pin the contract at compile time in the implementing package:**
   `var _ usecase.UserStore = (*UserMemory)(nil)` next to the type. Costs nothing,
   fails the build the moment the impl drifts.

## Layer responsibilities

- `internal/usecase` — application logic. Owns the **ports** (interfaces) it needs.
  Constructors take those interfaces, return `*Service` structs. No HTTP, SQL, SSH, or
  GitHub types leak in here.
- `internal/adapter/repository` — concrete persistence (in-memory now, SQLite later).
  Exports structs (`*UserMemory`), satisfies usecase ports.
- `internal/adapter/http` — handlers, router, server. Construct from usecase structs;
  return `*Handler`, `*Server`.
- `internal/config` — `Load()` returns a concrete `*Config`.

`cmd/server/main.go` is the composition root: it builds concrete structs and wires them
together. Dependency direction always points inward, toward `usecase`.

## Build / test

`make build`, `make test`, `make fmt`, `make lint` (run from `backend/`).
