# Bootstrap fails on pre-existing broken apt repositories

Status: **open** — deferred, not yet fixed.
Filed: 2026-07-22.

## Summary

Server bootstrap fails at the very first `apt-get update -y` on servers that are
not freshly provisioned. Any single broken/stale apt repository — even one
mountabo never added — makes `apt-get update` abort with exit status 100, which
aborts the whole bootstrap. The UI shows only a bare `Process exited with status
100`, giving the operator nothing to act on.

## Where it fails

`backend/internal/ssh/scripts/bootstrap.sh.tmpl`, line 37-38:

```bash
log "updating package lists"
apt-get update -y
```

This runs immediately after the user/SSH-key setup and **before** the Docker
install step. So failures here have nothing to do with Docker, the compose
plugin, or any later step — the script dies on the package-list refresh.

## Root cause

`apt-get update` exits non-zero if *any* configured repository is unreachable or
inconsistent, and `set -euo pipefail` then aborts the bootstrap. The bootstrap
header assumes "fresh Ubuntu → Docker-ready", but on an older, previously-managed
server there are leftover third-party repos mountabo neither created nor needs.

Two real instances observed on the same server (`dev-test-server`, 164.92.116.48):

1. **Docker repo with mismatched signing keys**
   ```
   E: Conflicting values set for option Signed-By regarding source
      https://download.docker.com/linux/ubuntu/ jammy:
      /etc/apt/keyrings/docker.asc != /etc/apt/keyrings/docker.gpg
   E: The list of sources could not be read.
   ```
   Two source entries for the Docker repo, each pinned to a different keyring
   (`docker.gpg` from an old manual install, `docker.asc` from a later
   `get.docker.com` run). apt refuses to read sources with conflicting
   `Signed-By`. Note: mountabo's own scripts never write either keyring — it
   relies entirely on `get.docker.com` — so the `docker.gpg` entry came from
   outside mountabo.

2. **Retired NodeSource repo**
   ```
   E: The repository 'https://deb.nodesource.com/node_18.x jammy Release'
      no longer has a Release file.
   ```
   NodeSource retired the `node_18.x` per-distro repo (Node 18 EOL). mountabo
   never installs Node; this is pure leftover state.

Both are the *same class* of failure: a stale third-party apt source the operator
(or the base image) left behind, tripping the first `apt-get update`.

## Manual remediation (current workaround)

Inspect and remove the offending source on the server, then re-run setup:

```bash
# find which repo apt is complaining about
grep -rn '<offending-host>' /etc/apt/sources.list /etc/apt/sources.list.d/ 2>/dev/null

# Docker keyring conflict:
sudo rm -f /etc/apt/sources.list.d/docker.list /etc/apt/sources.list.d/docker.sources
sudo rm -f /etc/apt/keyrings/docker.gpg /etc/apt/keyrings/docker.asc

# retired NodeSource repo:
sudo rm -f /etc/apt/sources.list.d/nodesource.list

sudo apt-get update
```

`get.docker.com` re-adds a single clean Docker entry on the next run.

## Decided fix (for later): fail fast with guidance

Chosen posture (over auto-healing repos or a tolerant/non-fatal update): keep
`apt-get update` strict, but catch its failure, surface the offending repos and
the exact remediation commands, then exit. Safest and most transparent; it does
not touch the operator's apt config, it just turns an opaque `status 100` into an
actionable message.

Rejected alternatives:
- **Auto-heal** (disable failing third-party repos and retry): convenient but
  modifies operator config; risk of masking a genuinely broken base mirror.
- **Tolerant update** (`apt-get update || true`): leaves broken repos in place to
  keep erroring on the operator's future apt commands; hides real failures.

### Proposed patch to `bootstrap.sh.tmpl` (replaces line 37-38)

```bash
log "updating package lists"
# apt-get update aborts (status 100) if ANY configured repository is broken,
# including stale third-party repos mountabo never added (e.g. a removed
# NodeSource distro, a Docker repo with mismatched signing keys). On a non-fresh
# server that is the most common bootstrap failure, and a bare "status 100" tells
# the operator nothing. Catch it and surface the offending repos + how to fix.
if ! apt-get update -y >/tmp/mountabo-apt-update.log 2>&1; then
  cat /tmp/mountabo-apt-update.log
  echo "==> ERROR: apt-get update failed on a pre-existing apt repository." >&2
  echo "==> mountabo did not add these; they are leftover on this server and must" >&2
  echo "==> be fixed before setup can continue. Offending repositories:" >&2
  grep -iE "^(E:|W:|Err:)" /tmp/mountabo-apt-update.log >&2 || true
  echo "==> Find the source file (use the host shown above):" >&2
  echo "==>   grep -rn '<host>' /etc/apt/sources.list /etc/apt/sources.list.d/" >&2
  echo "==> Then remove or disable the stale source and re-run setup, e.g.:" >&2
  echo "==>   sudo rm /etc/apt/sources.list.d/<offending>.list && sudo apt-get update" >&2
  exit 100
fi
```

### Verification when picked up
- Stub the Go template directives and `bash -n` the rendered script:
  `sed -E 's/\{\{[^}]*\}\}/x/g' bootstrap.sh.tmpl > /tmp/bs.sh && bash -n /tmp/bs.sh`
- Confirm the existing bootstrap-render Go tests still pass (`go test ./internal/ssh/...`).
- Reproduce on a server with a stale NodeSource/Docker source and confirm the
  panel now shows the offending repo + remediation instead of a bare status 100.

## Notes / follow-ups
- Consider whether the "fresh Ubuntu" assumption in the bootstrap header should be
  softened, since the tool is clearly used on non-fresh servers.
- A future, broader option is a preflight check that reports *all* broken repos at
  once (rather than failing on the first), so the operator fixes them in one pass.
