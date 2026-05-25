# Mountabo — Product Requirements Document

## What this is

Mountabo is a tool you run on your own laptop. It helps you deploy your applications to a server you own. You set it up once for each server and each repository; from then on, every push to GitHub deploys the new version automatically.

Mountabo lives on your machine, holds onto your credentials there, and only runs when you tell it to. The deploys themselves happen between GitHub and your server directly — Mountabo is not sitting in the middle of them.

## Who it's for

Developers who want their own server (a VPS from Hetzner, Contabo, DigitalOcean, etc.) but don't want to spend an afternoon configuring it every time. Developers who would rather not trust a third-party deployment service with their credentials. People who already use the `gh` CLI, Terraform, or similar local-first tools.

## The user journey

1. **Install Mountabo.** One binary download per operating system. Running `mountabo` from the terminal opens a local web interface in your browser (something like `http://localhost:7777`). You can also drive everything through the CLI if you prefer (`mountabo server add`, `mountabo deploy`, etc.).

2. **Connect GitHub.** Click "Connect GitHub". Mountabo opens a GitHub login page; you approve access. Mountabo now has a token to read your repositories and write the small files it needs to. The token lives in your operating system's keychain — macOS Keychain, Windows Credential Manager, or libsecret on Linux.

3. **Add a server.** Paste your server's IP address and root password. Mountabo connects, runs the setup script, throws the password away, and remembers the server.

4. **Connect a repository.** Pick a repository, a branch, and the server you just added. Mountabo:
    - Generates a fresh SSH key for this server.
    - Adds the public key as a deploy key on your GitHub repository.
    - Writes a small workflow file to `.github/workflows/mountabo-deploy.yml`.
    - Sets three secrets on the repository's Actions: server IP, username, SSH private key.

5. **Push code.** From here on, every push to that branch triggers GitHub Actions. Actions SSHes into your server and updates the running app. No Mountabo involvement.

6. **Use Mountabo when you want to.** View deploy history, add another server, connect another repo, change configuration, roll back a previous version. Otherwise, leave it closed.

## Components

### CLI and Web UI

The two ways you interact with Mountabo. Same logic, different surface:

- **CLI** for power users and scripting: `mountabo server add`, `mountabo repo connect`, etc.
- **Web UI** served on localhost when you run `mountabo`: friendlier for browsing servers, viewing history, picking from lists.

Both call into the same core.

### Core

Where the decisions happen. What to do when you add a server, what gets written to a repo when you connect it, what to roll back to. Pure Go, no network of its own — it asks the SSH and GitHub modules to do anything outside the machine.

### Local state

Mountabo remembers things between runs: which servers you've added, which repos are connected, deploy history. Stored in a single SQLite file on your machine (typically `~/.mountabo/state.db`).

No external database, no network calls to look up your own data, nothing to back up to the cloud. If you want backups, copy the file.

### Secret store

Wherever Mountabo holds something sensitive — SSH private keys, GitHub tokens — it goes through this layer. The layer hands the secret to your operating system's native credential store:

- **macOS**: Keychain
- **Windows**: Credential Manager
- **Linux**: libsecret (GNOME Keyring, KWallet)

So secrets aren't sitting in the SQLite file as plain bytes. The OS handles encryption-at-rest, screen-lock protection, and (on macOS/Windows) per-app access control.

### SSH module

Talks to remote servers. Used during initial server setup (with the user-provided root password) and for any "let me poke at the server" features (view container logs, restart a service from the dashboard).

After setup, day-to-day deploys don't go through this module — they happen via GitHub Actions directly. Mountabo doesn't need to be running.

Built on `golang.org/x/crypto/ssh`.

### GitHub module

Talks to GitHub on your behalf. Three things it does most often:

1. Adds an SSH deploy key to a repository.
2. Writes the deploy workflow file to a repository.
3. Sets the Actions secrets the workflow needs.

Authenticated using the OAuth token from the connection step. Token stays in the keychain; the GitHub module reads it on demand.

Built on `github.com/google/go-github`.

### Bootstrap script

A bash script that runs on a freshly added server. Creates a user, hardens SSH, installs Docker and Caddy, sets up the firewall. Same content as the standalone bootstrap guide.

Embedded in Mountabo's binary via Go's `embed` package, so there's no separate file to ship. Mountabo fills in user-specific values (username, the generated SSH public key) at runtime and uploads it before running.

### Workflow template

The GitHub Actions YAML that Mountabo writes to the user's repo. Also embedded. Mountabo fills in the branch name and repo-specific details before writing. The workflow itself contains the actual deploy logic — SSH to the server, run `git pull`, restart containers.

Mountabo isn't involved in any individual deploy. It only created the workflow once.

## How components connect

Three flows worth describing in plain English:

### Setting up a server

1. The user types into the UI: IP, root password, a name for the server.
2. The UI passes it to Core.
3. Core writes a "pending" server to SQLite.
4. Core asks the SSH module to connect with the root password.
5. SSH uploads the bootstrap script, runs it, streams the output back to the UI in real time.
6. The script creates a new user on the server and installs an SSH public key Mountabo generated.
7. Mountabo stores the private half of that key via the Secret store.
8. Core marks the server "ready" in SQLite.
9. The root password is discarded from memory. Mountabo never asks for it again.

### Connecting a repository

1. The user picks a repo, branch, and server in the UI.
2. Core asks the GitHub module to add the server's SSH key as a deploy key on the repo.
3. Core asks the GitHub module to write the workflow file.
4. Core asks the GitHub module to set the Actions secrets.
5. Core stores the connection in SQLite.

### A push triggering a deploy

1. The user pushes code to GitHub.
2. GitHub Actions reads the workflow file, fetches the secrets, SSHes into the server.
3. The workflow runs `git pull` and restarts the containers on the server.
4. **Mountabo is not involved.** It can be closed. Your laptop can be asleep.

## What's in version one

- macOS, Linux, Windows binaries.
- GitHub connection (one account).
- Add servers (one at a time, manually).
- Connect repositories.
- View deploy history (pulled from GitHub Actions runs on demand).
- Web UI + CLI.

## What's not in version one

- Multiple GitHub accounts.
- Importing existing, already-configured servers (assume fresh).
- Live log tailing during deploys (link out to the GitHub Actions logs instead).
- Server health checks or background monitoring.
- Automatic backups of the SQLite file.
- Custom build steps beyond what's in the user's Dockerfile.
- Multi-user / team sharing.
- A hosted version.

## Open questions

- **Distribution.** Single binary download from a release page? Homebrew / Scoop / Snap? Auto-update?
- **Live deploys without GitHub Actions.** Some users may not want every deploy gated on GitHub Actions (private repo minutes, CI cold-start delays). Should Mountabo offer a mode where it watches GitHub for new commits and deploys directly over SSH? Probably not v1, but worth flagging.
- **Background server checks.** Mountabo isn't running most of the time, so it can't do background health checks. Is that a feature (lower resource use, no surprise daemons) or a missing feature (need a separate uptime monitor)?
- **Cross-machine sync.** If a developer works on both their laptop and a desktop, today they'd have to set up Mountabo on each. Optional cloud-synced state file? Encrypted, of course.

---

## Glossary

- **VPS** — Virtual Private Server. A computer rented from a hosting provider that you have full control over.
- **SSH** — A way to connect to a remote computer and run commands on it.
- **Docker / Docker Compose** — Tools for packaging applications so they run the same way on any computer. Compose describes a group of containers that work together.
- **GitHub Actions** — GitHub's built-in tool for running tasks when things happen in a repository, like running tests on every push or deploying to a server.
- **Deploy key** — An SSH key registered on a single GitHub repository, granting just enough access to read or write that repo. Safer than a personal SSH key because it's scoped to one repository.
- **Keychain / Credential Manager / libsecret** — The operating system's secure store for passwords and credentials. Apps store secrets there instead of in their own files.