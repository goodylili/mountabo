package usecase

import (
	"context"
	"errors"
	"testing"
)

// fakeExchanger, fakeFetcher, fakeRepoLister, fakePortDetector, and fakeStore
// are in-memory stand-ins for the connector's ports, letting the flow be tested
// without GitHub or a keychain.
type fakeExchanger struct {
	token Token
	err   error
}

func (f fakeExchanger) Exchange(context.Context, string, string) (Token, error) {
	return f.token, f.err
}

type fakeFetcher struct {
	account Account
	err     error
}

func (f fakeFetcher) Account(context.Context, Token) (Account, error) {
	return f.account, f.err
}

type fakeRepoLister struct {
	repos []Repo
	err   error
}

func (f fakeRepoLister) List(context.Context, Token) ([]Repo, error) {
	return f.repos, f.err
}

type fakePortDetector struct {
	ports []ServicePort
	err   error
}

func (f fakePortDetector) DetectPorts(context.Context, Token, RepoRef) ([]ServicePort, error) {
	return f.ports, f.err
}

type fakeStore struct {
	saved   *Token
	deleted bool
	loadTok Token
	loadErr error
	saveErr error
}

func (f *fakeStore) Save(t Token) error { f.saved = &t; return f.saveErr }
func (f *fakeStore) Load() (Token, error) {
	return f.loadTok, f.loadErr
}
func (f *fakeStore) Delete() error { f.deleted = true; return nil }

func TestConnect_StoresTokenAndReturnsAccount(t *testing.T) {
	store := &fakeStore{}
	c := NewGitHubConnector(
		fakeExchanger{token: Token{AccessToken: "gho_abc"}},
		fakeFetcher{account: Account{Login: "octocat"}},
		fakeRepoLister{},
		fakePortDetector{},
		store,
	)

	account, err := c.Connect(context.Background(), "code-123", "http://localhost/cb")
	if err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	if account.Login != "octocat" {
		t.Errorf("login = %q, want octocat", account.Login)
	}
	if store.saved == nil || store.saved.AccessToken != "gho_abc" {
		t.Errorf("token not persisted, got %+v", store.saved)
	}
}

func TestConnect_ExchangeFailureDoesNotStore(t *testing.T) {
	store := &fakeStore{}
	c := NewGitHubConnector(
		fakeExchanger{err: errors.New("bad code")},
		fakeFetcher{account: Account{Login: "octocat"}},
		fakeRepoLister{},
		fakePortDetector{},
		store,
	)

	if _, err := c.Connect(context.Background(), "code", "uri"); err == nil {
		t.Fatal("expected error, got nil")
	}
	if store.saved != nil {
		t.Error("token should not be stored when exchange fails")
	}
}

func TestConnect_AccountFailureDoesNotStore(t *testing.T) {
	store := &fakeStore{}
	c := NewGitHubConnector(
		fakeExchanger{token: Token{AccessToken: "gho_abc"}},
		fakeFetcher{err: errors.New("unauthorized")},
		fakeRepoLister{},
		fakePortDetector{},
		store,
	)

	if _, err := c.Connect(context.Background(), "code", "uri"); err == nil {
		t.Fatal("expected error, got nil")
	}
	if store.saved != nil {
		t.Error("token should not be stored when the account lookup fails")
	}
}

func TestStatus_NotConnectedPropagates(t *testing.T) {
	c := NewGitHubConnector(
		fakeExchanger{},
		fakeFetcher{},
		fakeRepoLister{},
		fakePortDetector{},
		&fakeStore{loadErr: ErrNotConnected},
	)

	_, err := c.Status(context.Background())
	if !errors.Is(err, ErrNotConnected) {
		t.Fatalf("err = %v, want ErrNotConnected in chain", err)
	}
}

func TestRepositories_ReturnsListedRepos(t *testing.T) {
	c := NewGitHubConnector(
		fakeExchanger{},
		fakeFetcher{},
		fakeRepoLister{repos: []Repo{{FullName: "octocat/hello"}, {FullName: "octocat/secret", Private: true}}},
		fakePortDetector{},
		&fakeStore{loadTok: Token{AccessToken: "gho_abc"}},
	)

	repos, err := c.Repositories(context.Background())
	if err != nil {
		t.Fatalf("Repositories returned error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("got %d repos, want 2", len(repos))
	}
}

func TestRepositories_NotConnectedPropagates(t *testing.T) {
	c := NewGitHubConnector(
		fakeExchanger{},
		fakeFetcher{},
		fakeRepoLister{},
		fakePortDetector{},
		&fakeStore{loadErr: ErrNotConnected},
	)

	if _, err := c.Repositories(context.Background()); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("err = %v, want ErrNotConnected in chain", err)
	}
}

func TestDetectPorts_ReturnsDetectedPorts(t *testing.T) {
	want := []ServicePort{
		{Service: "web", EnvVar: "FRONTEND_PORT", Host: "3000", Container: "3000", Editable: true},
		{Service: "db", Host: "5432", Container: "5432"},
	}
	c := NewGitHubConnector(
		fakeExchanger{},
		fakeFetcher{},
		fakeRepoLister{},
		fakePortDetector{ports: want},
		&fakeStore{loadTok: Token{AccessToken: "gho_abc"}},
	)

	got, err := c.DetectPorts(context.Background(), RepoRef{Owner: "octocat", Name: "hello"})
	if err != nil {
		t.Fatalf("DetectPorts returned error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d ports, want %d", len(got), len(want))
	}
	if got[0].EnvVar != "FRONTEND_PORT" || !got[0].Editable || got[1].Editable {
		t.Errorf("ports not passed through faithfully: %+v", got)
	}
}

func TestDetectPorts_NotConnectedPropagates(t *testing.T) {
	c := NewGitHubConnector(
		fakeExchanger{},
		fakeFetcher{},
		fakeRepoLister{},
		fakePortDetector{},
		&fakeStore{loadErr: ErrNotConnected},
	)

	if _, err := c.DetectPorts(context.Background(), RepoRef{}); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("err = %v, want ErrNotConnected in chain", err)
	}
}

func TestDisconnect_DeletesToken(t *testing.T) {
	store := &fakeStore{}
	c := NewGitHubConnector(fakeExchanger{}, fakeFetcher{}, fakeRepoLister{}, fakePortDetector{}, store)

	if err := c.Disconnect(); err != nil {
		t.Fatalf("Disconnect returned error: %v", err)
	}
	if !store.deleted {
		t.Error("Disconnect did not delete the stored token")
	}
}
