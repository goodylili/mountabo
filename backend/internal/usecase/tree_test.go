package usecase

import (
	"context"
	"errors"
	"testing"
)

type fakeTreeLister struct {
	entries []TreeEntry
	err     error
}

func (f fakeTreeLister) Tree(context.Context, Token, string, string, string) ([]TreeEntry, error) {
	return f.entries, f.err
}

func TestTreeService_ListsWithStoredToken(t *testing.T) {
	store := &fakeStore{loadTok: Token{AccessToken: "tok"}}
	lister := fakeTreeLister{entries: []TreeEntry{{Path: "apps", Dir: true}, {Path: "apps/web", Dir: true}, {Path: "README.md"}}}
	svc := NewTreeService(store, lister)

	got, err := svc.Tree(context.Background(), "acme", "shop", "main")
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	if len(got) != 3 || got[0].Path != "apps" || !got[0].Dir || got[2].Dir {
		t.Errorf("unexpected entries: %+v", got)
	}
}

func TestTreeService_RequiresOwnerRepoRef(t *testing.T) {
	svc := NewTreeService(&fakeStore{loadTok: Token{AccessToken: "tok"}}, fakeTreeLister{})
	if _, err := svc.Tree(context.Background(), "acme", "", "main"); err == nil {
		t.Fatal("expected an error when repo is missing")
	}
}

func TestTreeService_NotConnected(t *testing.T) {
	svc := NewTreeService(&fakeStore{loadErr: ErrNotConnected}, fakeTreeLister{})
	if _, err := svc.Tree(context.Background(), "acme", "shop", "main"); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("want ErrNotConnected, got %v", err)
	}
}
