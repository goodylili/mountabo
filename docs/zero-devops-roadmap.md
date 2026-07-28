# Zero DevOps Roadmap

Mountabo is already close to zero DevOps for solo developers: local app, GitHub OAuth, VPS bootstrap, Docker deployments, nginx domains, monitoring, logs, terminal access, and optional hardening.

The immediate goal is not the whole platform vision. The goal is to ship the features that make Mountabo useful for startup teams and tech agencies while keeping the implementation safe, secure, and compatible with the current local first architecture.

## Architectural Starting Point (read first)

Every feature below has to survive contact with what Mountabo actually is today. Design against these facts, not around them:

- **Single user, single machine.** The backend binds `127.0.0.1` and runs under one GitHub OAuth identity. There is no auth boundary inside the app.
- **State is split and machine-local.** Servers live in a JSON file, deployments and an append-only `deploy_events` log live in local SQLite, and tokens/secrets live in the OS keychain. None of it is portable, shareable, or reconciled across machines today.
- **Deployment is Actions-driven.** Deploying commits a generated GitHub Actions workflow into the user's repo; the workflow SSHes into a root VPS and runs Docker or Compose. Mountabo does not run the deploy itself, so it cannot assume it holds a lock or sees every trigger.
- **GitHub Actions secrets are write-only.** Mountabo can set them but cannot read them back or reliably enumerate them. Any "which secrets are configured" view is Mountabo's own record, not ground truth.

Two rules fall out of this and govern the whole roadmap:

1. **Do not ship access boundaries the architecture cannot enforce.** Roles and multi-tenant workspaces that render in the UI but enforce nothing are worse than absent: they communicate a security boundary that does not exist.
2. **Capture the data before building the screen.** Several features below depend on fields the current schema does not store. The schema change is the real first task, not the UI.

## Priority Scope

Focus on six features, sequenced so the load-bearing data and safety work lands before anything built on top of it:

1. Release History with Artifact References.
2. Deploy Concurrency and Locking.
3. Environment Management.
4. Secrets and Configuration UI.
5. Rollback and Redeploy.
6. Migrations and Release Hooks.

Application Blueprints and Team Workspaces are still in scope for the near term but move out of the critical path; see [Adjacent Work](#adjacent-work). These features turn Mountabo from a repo to server deployment helper into a small team deployment workspace, and they build on existing GitHub, server, deployment, workflow, and UI flows.

## 1. Release History with Artifact References

This is the foundation. Rollback, redeploy, and environment history all depend on it, and the data it needs does not exist yet.

The current `deploy_events` table records that a deploy happened but stores **no commit SHA and no image reference**. Before any UI work, extend the release record to capture what actually ran:

- commit SHA
- **image digest or image tag of the built artifact** (the field that makes rollback real)
- branch
- environment
- deployment time
- deployment status
- workflow run link
- actor when available
- health check result when available
- previous release reference

Treat a release as an immutable record keyed to an artifact, not a pointer to "whatever building this branch produces now." Mountabo should show deployments as releases, not just GitHub Actions runs.

Deliverable for this phase: the schema migration on `deploy_events` (or a new `releases` table), the deploy path populating it, and a read-only release history view. No rollback actions yet, they come in phase 5 once artifacts are retained.

## 2. Deploy Concurrency and Locking

A safety prerequisite that must land before environments and rollback, because both make the race worse.

Deploys are triggered by `git push` to Actions to SSH into a shared directory on the box. Two pushes race on the same target with nothing serializing them, and a rollback racing an in-flight deploy is how production ends up half-updated.

For this phase:

- Add a per-target deploy lock (per `repo` + `environment` + `server`).
- Serialize deploys to the same target; reject or queue a second deploy while one is in flight.
- Use an Actions concurrency group for the generated workflow so GitHub itself cancels or queues overlapping runs.
- Surface lock state in the UI, and provide an explicit "clear stuck lock" action for recovery.

## 3. Environment Management

Add first class environments so one repository can have separate deployment targets.

Initial environments:

```text
production
staging
```

**Environment is a first-class entity, not a branch.** The current deployments table is keyed by `(owner, repo, branch)`. Environment owns its own configuration and is keyed by `(repo, environment)`, with branch as one *attribute* of the environment. This is a primary-key change that touches every deploy path, so do it deliberately and do not let "environment equals branch" leak back into the schema.

Each environment should own:

- branch
- server
- deploy directory
- domain
- secrets (per environment, see phase 4)
- port mappings
- release history (phase 1, scoped per environment)
- migration and hook settings (phase 6)

The core workflow should become:

```text
Choose repository -> choose environment -> configure deployment -> deploy
```

For startup teams, staging and production separation is the minimum bar. For agencies, environments make it easier to give clients staging links while keeping production stable.

**Preview is deferred, not stubbed.** A preview "environment type" with no create/destroy lifecycle only leaks ports, domains, and containers on the VPS. Do not ship the word "preview" until the per pull request lifecycle in [Adjacent Work](#adjacent-work) exists. Keep phase one to `production` and `staging`.

## 4. Secrets and Configuration UI

Add a proper configuration surface for environment variables and deployment settings, designed around the write-only reality of GitHub Actions secrets.

Core capabilities:

- edit environment variables per environment
- detect missing variables from `.env.example`
- sync secrets to GitHub Actions
- validate required variables before deployment
- compare staging and production variable **names** without exposing values
- show which secrets Mountabo has configured, without showing values

**Source of truth and drift, decided up front.** Because Actions secrets cannot be read back, "which secrets are configured" is Mountabo's own record and *will* drift the moment someone edits a secret in the GitHub UI. This phase must include:

- an explicit statement of which store is authoritative for each class of value,
- a detection/reconciliation path that flags when Mountabo's record and GitHub disagree, rather than silently trusting the local record.

**Deploy-key rotation is a distributed transaction, treat it as one.** Rotating a deploy key touches three systems: the VPS `authorized_keys`, the repo deploy key, and the Actions secret. Design it with an explicit order of operations and a rollback path for partial failure. It is not a single button; it gets its own small design note before implementation. Defer rotation to the end of this phase if it threatens the timeline.

The UI should separate:

```text
Plain configuration
Sensitive secrets
Generated GitHub Actions secrets
Server level values
Environment level values
```

Prioritize `.env.example` detection and GitHub Actions secret sync over advanced secret sharing.

## 5. Rollback and Redeploy

Now that releases carry an artifact reference (phase 1) and deploys are serialized (phase 2), rollback can be real.

**Rollback re-points to a retained artifact; it does not rebuild an old commit.** Rebuilding an old SHA is slow and non-deterministic: floating base images and unpinned dependencies mean the rebuilt "previous release" is not the artifact that was running. Real rollback requires immutable, retained artifacts. This phase must therefore include an artifact retention decision:

- tag and push each build to a registry keyed by digest, **or**
- keep the last N images on the box,

and rollback re-points the running container to that stored digest.

Required actions:

```text
View release details
Redeploy current release
Rollback to previous release
Rollback to selected release
```

Users must not need to SSH into the server or edit GitHub Actions to recover. The UI should make rollback explicit and confirm the target release before running it, and rollback must respect the deploy lock from phase 2.

**Health as a gate, not a note.** Phase 1 records a health check result. Here, wire it in: a deploy whose health check fails should surface clearly and offer one-click rollback. Automatic rollback on failed health check can follow once the manual path is trusted.

## 6. Migrations and Release Hooks

Add structured deployment hooks so users do not need to edit generated YAML manually.

Supported hooks:

```text
pre_build
post_build
pre_deploy
post_deploy
migration
health_check
rollback
```

Common migration examples:

```bash
php artisan migrate --force
python manage.py migrate
rails db:migrate
npm run db:migrate
go run ./cmd/migrate
```

**Confront the expand/contract problem.** A `migration` hook and a `rollback` hook are not symmetric: an applied migration usually cannot be undone by redeploying old code (a dropped column, a changed type). The docs and UI must be explicit that rolling back code does not roll back the database, and should steer users toward backward-compatible (expand/contract) migrations. Do not imply that "rollback" reverses a migration.

**Specify the production confirmation gate concretely.** "Production requires confirmation" is meaningless when the actor is a `git push`. Define: *where* the human confirms (a Mountabo action, not the push), and *what blocks the Actions run* while it waits (a manual-approval environment gate or a hold step). Until that is designed, production migrations should default to not running automatically.

Each environment can override hook commands. Hook settings should include:

- command
- working directory
- when to run
- whether failure blocks deployment
- whether production requires confirmation

For this phase, keep hooks command based. Do not build a plugin system yet.

## Adjacent Work

Still near-term and valuable, but moved off the critical path because they either depend on the phases above or introduce boundaries the architecture cannot yet enforce.

### State Portability (prerequisite for any team feature)

Before workspaces, the data has to be able to leave one laptop. Today it is trapped across a local JSON file, local SQLite, and the OS keychain.

- Export/import a workspace definition (servers, environments, deployment config).
- Reference secrets by name/location rather than embedding values, so a shared definition carries no plaintext secrets.
- Define how two machines reconcile divergent state.

This is the real unblock for teams, and it is honest about what local first can offer before there is a shared backend.

### Application Blueprints

Low risk, ship whenever it fits; it does not block the critical path.

Blueprints help users configure a repository faster by choosing a known stack instead of filling every deployment detail from scratch. Start with practical templates: Next.js, Laravel, Django, Rails, Node API, Go API, Docker Compose, static frontend plus API.

Each blueprint provides **editable seed defaults** (not hidden config) for: deploy strategy, app port, health check path, root directory hints, build and start command hints, environment variable examples, migration command examples, recommended release hooks. Users select a blueprint, review the generated settings, and edit before saving.

Treat baked-in env and migration examples as seeds that go stale, present them as starting points, not guarantees. Keep this scoped to presets inside Mountabo: no marketplace, remote registry, or provider specific provisioning yet.

### Team Workspaces

Deliberately last, and gated on a backend that can enforce a role. Shipping roles that render in the UI but enforce nothing communicates a security boundary that does not exist; on a single-tenant local app with root SSH keys and a GitHub token in one keychain, that is a liability, not a feature.

The near-term, honest version of "workspaces" is organizational language and grouping on top of [State Portability](#state-portability-prerequisite-for-any-team-feature), not access control:

```text
Workspace
  Client
    Project
      App
        Environment
```

This lets Mountabo organize work the way agencies think, clients, projects, staging links, production links, and handoffs, using fields like workspace name, workspace type (startup, agency, personal), and client/project labels. **Roles with real enforcement (Owner, Admin, Developer, Viewer, Client Viewer) wait for hosted collaboration**, below, where there is a server to enforce them.

### One Click Server Provisioning

Add provider integrations later: Hetzner, DigitalOcean, AWS Lightsail, Vultr, Linode, Contabo.

```text
Choose provider -> choose size -> choose region -> Mountabo creates VPS -> bootstraps it -> deploys app
```

### Managed Databases and Add Ons

Add VPS hosted add ons later: PostgreSQL, MySQL, Redis, MinIO, Meilisearch, Uptime Kuma, Plausible or Umami, n8n. Mountabo should eventually install, secure, expose internally, back up, and inject connection strings into GitHub Actions secrets.

### Preview Deployments

Full pull request preview deployments (the prerequisite for a real `preview` environment): create temporary deployment, assign preview domain, run migrations optionally, destroy automatically after merge or close, comment the URL back on the pull request.

### Backups and Restore

Add scheduled backups later for databases, app volumes, uploaded files, and Mountabo local state. Destinations: S3 compatible storage, Backblaze B2, Cloudflare R2, local download.

### Health, Alerts, and Self Healing

Add operational safety later: HTTP health checks, container status checks, disk usage alerts, memory alerts, SSL expiry alerts, failed deployment alerts, backup failure alerts, domain DNS mismatch alerts. Then safe remediation: restart failed container, renew certificate, prune old Docker images, rotate logs, clear failed deployment lock, run disk cleanup.

### Hosted Collaboration and Sync

True multi user collaboration, and the home for enforceable roles, comes after local workspace support: encrypted workspace sync, cloud backed organizations, audit logs, shared deployment history, **shared role based access control**, client handoff pages, monthly infrastructure reports.

## Recommended Build Order

The sequence is the point: each step unblocks the next, and the risky data and safety work lands before anything is built on it.

1. **Release records with artifact references** — schema migration on `deploy_events` to capture SHA, image digest, status, health, actor, and previous-release reference. Smallest change, highest leverage; everything else leans on it.
2. **Deploy concurrency and locking** — per-target lock plus an Actions concurrency group. Safety prerequisite for environments and rollback.
3. **Environment Management** — environment as a first-class entity (fix the primary key), production and staging only.
4. **Secrets and Configuration UI** — per environment, with an explicit source-of-truth and drift-detection design; deploy-key rotation designed as a distributed transaction.
5. **Rollback and Redeploy** — against retained artifacts from step 1, gated by the lock from step 2, with health as a rollback trigger.
6. **Migrations and Release Hooks** — with an explicit expand/contract stance and a concretely specified production confirmation gate.
7. **Application Blueprints** — editable seed presets; ship anytime, off the critical path.
8. **State Portability** — export/import and secret references; the real prerequisite for teams.
9. **Team Workspace grouping** — organizational language on top of portable state. Enforceable roles wait for hosted collaboration.

This sequence keeps the release grounded in the existing product, fixes the three load-bearing problems (real artifacts, captured data, deploy serialization) before building on them, and avoids shipping access boundaries the architecture cannot yet enforce.
