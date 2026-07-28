package usecase

import (
	"sync"
	"time"
)

// DeployLocker serializes deploys per target: at most one deploy operation may
// hold a given key at a time. It is an in-process, in-memory registry — the
// right scope for mountabo's local-first, single-process model. A process
// restart clears all locks, which is correct: no deploy is actually running
// locally after a restart. It is safe for concurrent use.
//
// The lock guards the local configure-and-trigger operation (DeployService.Deploy),
// which is minutes long (SSH, repo commits, secret uploads); it does not track
// the remote GitHub Actions run. Overlapping Actions runs are serialized
// separately by the workflow's concurrency group.
type DeployLocker struct {
	mu   sync.Mutex
	held map[string]lockInfo
}

// lockInfo is the metadata kept for a held lock, used to surface lock state
// (e.g. "locked 2m ago") in the UI.
type lockInfo struct {
	Since time.Time
}

// NewDeployLocker returns an empty locker ready for use.
func NewDeployLocker() *DeployLocker {
	return &DeployLocker{held: map[string]lockInfo{}}
}

// lockKey is the deploy-lock key for a deployment target, its full identity
// (owner, repo, environment) — the same key the storage layer uses, so two
// deployments never share a lock just because they share an app name. The NUL
// separators can't appear in the values, so keys never collide by concatenation.
func lockKey(owner, repo, environment string) string {
	return owner + "\x00" + repo + "\x00" + environment
}

// TryAcquire marks key as held and returns true, or returns false when it is
// already held (the caller should reject the duplicate operation). Never blocks.
func (l *DeployLocker) TryAcquire(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.held[key]; ok {
		return false
	}
	l.held[key] = lockInfo{Since: time.Now().UTC()}
	return true
}

// Release frees key. Releasing a key that is not held is a no-op, so it is safe
// to defer unconditionally after a successful TryAcquire.
func (l *DeployLocker) Release(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.held, key)
}

// Clear force-releases key regardless of who holds it, for the "clear stuck
// lock" recovery action, and reports whether a lock was actually held. Note:
// clearing a lock whose operation is still genuinely running lets a new deploy
// start alongside it — this is an operator escape hatch for a wedged deploy.
func (l *DeployLocker) Clear(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, ok := l.held[key]
	delete(l.held, key)
	return ok
}

// Held reports whether key is locked and, if so, since when — for surfacing
// lock state to the UI.
func (l *DeployLocker) Held(key string) (time.Time, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	info, ok := l.held[key]
	return info.Since, ok
}
