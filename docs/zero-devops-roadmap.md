# Zero DevOps Roadmap

Mountabo is already close to zero DevOps for solo developers: local app, GitHub OAuth, VPS bootstrap, Docker deployments, nginx domains, monitoring, logs, terminal access, and optional hardening.

The immediate goal is not the whole platform vision. The goal is to ship the features that make Mountabo useful for startup teams and tech agencies while keeping the implementation safe, secure, and compatible with the current local first architecture.

## Priority Scope

Focus on six features:

1. Application Blueprints.
2. Environment Management.
3. Rollback and Release History.
4. Team Workspaces.
5. Secrets and Configuration UI.
6. Migrations and Release Hooks.

These features turn Mountabo from a repo to server deployment helper into a small team deployment workspace. They are also compatible with the current local first architecture because they can mostly build on existing GitHub, server, deployment, workflow, and UI flows.

## 1. Application Blueprints

Application blueprints should help users configure a repository faster by choosing a known stack instead of filling every deployment detail from scratch.

Start with practical templates:

- Next.js app
- Laravel app
- Django app
- Rails app
- Node API
- Go API
- Docker Compose app
- Static frontend plus API

Each blueprint should provide defaults for:

- deploy strategy
- app port
- health check path
- root directory hints
- build and start command hints
- environment variable examples
- migration command examples
- recommended release hooks

Blueprints should not hide the generated configuration. Users should be able to select a blueprint, review the resulting deployment settings, and edit them before saving.

For this phase, keep this scoped to presets inside Mountabo. Do not add a marketplace, remote template registry, or provider specific provisioning yet.

## 2. Environment Management

Add first class environments so one repository can have separate deployment targets.

Initial environments:

```text
production
staging
preview
```

Each environment should support:

- branch
- server
- deploy directory
- domain
- secrets
- port mappings
- release history
- migration and hook settings

The core workflow should become:

```text
Choose repository -> choose environment -> configure deployment -> deploy
```

For startup teams, staging and production separation is the minimum bar. For agencies, environments make it easier to give clients staging links while keeping production stable.

Preview environments can start as a named environment type without full per pull request automation. Automated pull request preview deployments can come later.

## 3. Rollback and Release History

Mountabo should show deployments as releases, not just GitHub Actions runs.

For each release, store or display:

- commit SHA
- branch
- environment
- deployment time
- deployment status
- workflow run link
- actor when available
- health check result when available
- previous release reference

Required actions:

```text
View release details
Redeploy current release
Rollback to previous release
Rollback to selected release
```

For this phase, rollback can be implemented through the existing deployment mechanism if that is the fastest safe path. The important product behavior is that users do not need to SSH into the server or manually edit GitHub Actions to recover.

The UI should make rollback explicit and confirm the target release before running it.

## 4. Team Workspaces

Add a workspace model that groups servers, repositories, deployments, and settings.

Initial model:

```text
Workspace
  Members
  Servers
  Repositories
  Deployments
  Environments
```

Suggested roles:

```text
Owner
Admin
Developer
Viewer
Client Viewer
```

For this phase, this can start as local workspace structure and UI, even if true multi user sync comes later. The goal is to make the product language fit teams and agencies immediately.

Useful workspace fields:

- workspace name
- workspace type: startup, agency, personal
- member name or email
- role
- client or project label

Agency friendly grouping:

```text
Workspace
  Client
    Project
      App
        Environment
```

This lets Mountabo organize work the way agencies already think: clients, projects, staging links, production links, and handoffs.

## 5. Secrets and Configuration UI

Add a proper configuration surface for environment variables and deployment settings.

Core capabilities:

- edit environment variables per environment
- detect missing variables from `.env.example`
- sync secrets to GitHub Actions
- rotate deploy keys
- validate required variables before deployment
- compare staging and production variable names without exposing values
- show which secrets are configured without showing secret values

The UI should separate:

```text
Plain configuration
Sensitive secrets
Generated GitHub Actions secrets
Server level values
Environment level values
```

This is one of the most important agency features because every client project has different environment variables, and mistakes here commonly break deployments.

Prioritize GitHub Actions secret sync and `.env.example` detection over advanced secret sharing.

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

Each environment should be able to override hook commands. Production may require manual confirmation for migrations while staging may run automatically.

Hook settings should include:

- command
- working directory
- when to run
- whether failure blocks deployment
- whether production requires confirmation

For this phase, keep hooks command based. Do not build a plugin system yet.

## Later

These are still valuable, but they can come after the priority scope.

### One Click Server Provisioning

Add provider integrations later:

- Hetzner
- DigitalOcean
- AWS Lightsail
- Vultr
- Linode
- Contabo

Target flow:

```text
Choose provider -> choose size -> choose region -> Mountabo creates VPS -> bootstraps it -> deploys app
```

### Managed Databases and Add Ons

Add VPS hosted add ons later:

- PostgreSQL
- MySQL
- Redis
- MinIO
- Meilisearch
- Uptime Kuma
- Plausible or Umami
- n8n

Mountabo should eventually install, secure, expose internally, back up, and inject connection strings into GitHub Actions secrets.

### Preview Deployments

Full pull request preview deployments can come later:

- create temporary deployment
- assign preview domain
- run migrations optionally
- destroy automatically after merge or close
- comment the URL back on the GitHub pull request

### Backups and Restore

Add scheduled backups later for:

- databases
- app volumes
- uploaded files
- Mountabo local state

Support destinations:

- S3 compatible storage
- Backblaze B2
- Cloudflare R2
- local download

### Health, Alerts, and Self Healing

Add operational safety later:

- HTTP health checks
- container status checks
- disk usage alerts
- memory alerts
- SSL expiry alerts
- failed deployment alerts
- backup failure alerts
- domain DNS mismatch alerts

Then add safe remediation:

- restart failed container
- renew certificate
- prune old Docker images
- rotate logs
- clear failed deployment lock
- run disk cleanup

### Hosted Collaboration and Sync

True multi user collaboration can come after local workspace support:

- encrypted workspace sync
- cloud backed organizations
- audit logs
- shared deployment history
- shared role based access
- client handoff pages
- monthly infrastructure reports

## Recommended Build Order

Recommended order:

1. Add the data model for workspaces, environments, releases, hooks, and blueprint selection.
2. Add Application Blueprints to speed up deployment configuration.
3. Add Environment Management for production and staging.
4. Add Secrets and Configuration UI per environment.
5. Add Migrations and Release Hooks to the generated workflow path.
6. Add Release History from existing deployment and GitHub Actions data.
7. Add Rollback actions against previous releases.
8. Add Team Workspace screens and role labels.

This sequence keeps the release grounded in the existing product while making Mountabo much more useful for small teams and agencies.
