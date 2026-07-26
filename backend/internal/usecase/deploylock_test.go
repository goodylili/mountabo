package usecase

import "testing"

func TestDeployLocker_AcquireRejectsSecondThenReleaseFrees(t *testing.T) {
	l := NewDeployLocker()

	if !l.TryAcquire("shop") {
		t.Fatal("first acquire should succeed")
	}
	if l.TryAcquire("shop") {
		t.Error("second acquire of a held key should be rejected")
	}
	// A different key is independent.
	if !l.TryAcquire("blog") {
		t.Error("acquiring a different key should succeed")
	}

	l.Release("shop")
	if !l.TryAcquire("shop") {
		t.Error("acquire after release should succeed")
	}
}

func TestDeployLocker_HeldReportsState(t *testing.T) {
	l := NewDeployLocker()

	if _, ok := l.Held("shop"); ok {
		t.Error("unheld key should report not held")
	}
	l.TryAcquire("shop")
	since, ok := l.Held("shop")
	if !ok {
		t.Fatal("held key should report held")
	}
	if since.IsZero() {
		t.Error("held key should carry a since time")
	}
}

func TestDeployLocker_ClearForceReleases(t *testing.T) {
	l := NewDeployLocker()

	if l.Clear("shop") {
		t.Error("clearing an unheld key should report false")
	}
	l.TryAcquire("shop")
	if !l.Clear("shop") {
		t.Error("clearing a held key should report true")
	}
	if _, ok := l.Held("shop"); ok {
		t.Error("key should be free after clear")
	}
	if !l.TryAcquire("shop") {
		t.Error("acquire after clear should succeed")
	}
}

// Release of an unheld key must be a no-op so a deferred Release is always safe.
func TestDeployLocker_ReleaseUnheldIsNoOp(t *testing.T) {
	l := NewDeployLocker()
	l.Release("never-held") // must not panic
	if _, ok := l.Held("never-held"); ok {
		t.Error("releasing an unheld key should not create it")
	}
}
