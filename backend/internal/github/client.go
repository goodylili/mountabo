package github

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/goodylili/mountabo/internal/usecase"
	gogithub "github.com/google/go-github/v88/github"
	"golang.org/x/sync/errgroup"
)

// Client reads from the GitHub API using a user-to-server token. It is stateless
// with respect to credentials: each call builds an authenticated client from the
// token it is given, so one Client can serve any connected account.
type Client struct{}

var (
	_ usecase.AccountFetcher = (*Client)(nil)
	_ usecase.RepoLister     = (*Client)(nil)
)

// NewClient returns a GitHub API client.
func NewClient() *Client {
	return &Client{}
}

// Account returns the GitHub account the token authenticates as. The empty
// username asks the API for the authenticated user behind the token.
func (c *Client) Account(ctx context.Context, t usecase.Token) (usecase.Account, error) {
	api, err := gogithub.NewClient(gogithub.WithAuthToken(t.AccessToken))
	if err != nil {
		return usecase.Account{}, fmt.Errorf("build github client: %w", err)
	}

	user, _, err := api.Users.Get(ctx, "")
	if err != nil {
		return usecase.Account{}, fmt.Errorf("get authenticated user: %w", err)
	}
	return usecase.Account{Login: user.GetLogin()}, nil
}

// List returns every repository the token can access, public and private. To
// miss nothing it unions two sources and de-dupes by full name: GET /user/repos
// (owned, collaborator, organization) and the GitHub App's installation repos
// (which can surface org/private repos /user/repos omits). Each source is
// best-effort, if one fails but the other returns repos, the listing still
// succeeds; only when both fail and nothing came back is it an error. The UI
// ordering (most recently pushed first) is restored after merging.
func (c *Client) List(ctx context.Context, t usecase.Token) ([]usecase.Repo, error) {
	api, err := gogithub.NewClient(gogithub.WithAuthToken(t.AccessToken))
	if err != nil {
		return nil, fmt.Errorf("build github client: %w", err)
	}

	seen := map[string]bool{}
	var repos []usecase.Repo
	add := func(list []*gogithub.Repository) {
		for _, r := range list {
			full := r.GetFullName()
			if full == "" || seen[full] {
				continue
			}
			seen[full] = true
			repos = append(repos, toRepo(r))
		}
	}

	userRepos, userErr := listAuthenticatedUserRepos(ctx, api)
	add(userRepos)
	installRepos, installErr := listInstallationRepos(ctx, api)
	add(installRepos)

	if len(repos) == 0 {
		if userErr != nil {
			return nil, userErr
		}
		if installErr != nil {
			return nil, installErr
		}
	}

	// Most recently pushed first.
	sort.Slice(repos, func(i, j int) bool { return repos[i].PushedAt.After(repos[j].PushedAt) })

	annotateDocker(ctx, api, repos)
	return repos, nil
}

// listAuthenticatedUserRepos pages GET /user/repos with a STABLE sort
// (full_name): "pushed" reorders the instant any repo is pushed, so concurrent
// page windows could shift and silently drop a repo at a boundary. With a stable
// key the windows are fixed, so every repo is fetched exactly once. Page 1 is
// fetched first to learn the count, the rest concurrently (bounded).
func listAuthenticatedUserRepos(ctx context.Context, api *gogithub.Client) ([]*gogithub.Repository, error) {
	opt := gogithub.RepositoryListByAuthenticatedUserOptions{
		Visibility:  "all",
		Affiliation: "owner,collaborator,organization_member",
		Sort:        "full_name",
		Direction:   "asc",
		ListOptions: gogithub.ListOptions{PerPage: 100, Page: 1},
	}

	first, resp, err := api.Repositories.ListByAuthenticatedUser(ctx, &opt)
	if err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}

	pages := make([][]*gogithub.Repository, max(resp.LastPage, 1))
	pages[0] = first

	if resp.LastPage > 1 {
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(6)
		for p := 2; p <= resp.LastPage; p++ {
			pageOpt := opt
			pageOpt.Page = p
			idx := p - 1
			g.Go(func() error {
				repos, _, err := api.Repositories.ListByAuthenticatedUser(gctx, &pageOpt)
				if err != nil {
					return fmt.Errorf("list repositories page %d: %w", pageOpt.Page, err)
				}
				pages[idx] = repos
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err
		}
	}

	var out []*gogithub.Repository
	for _, page := range pages {
		out = append(out, page...)
	}
	return out, nil
}

// listInstallationRepos enumerates repositories across every GitHub App
// installation the user can reach (their account plus orgs the app is installed
// on), which catches repos /user/repos can omit. It returns an empty slice (no
// error) when the user has no installations.
func listInstallationRepos(ctx context.Context, api *gogithub.Client) ([]*gogithub.Repository, error) {
	var installations []*gogithub.Installation
	opt := &gogithub.ListOptions{PerPage: 100}
	for {
		page, resp, err := api.Apps.ListUserInstallations(ctx, opt)
		if err != nil {
			return nil, fmt.Errorf("list installations: %w", err)
		}
		installations = append(installations, page...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	var out []*gogithub.Repository
	for _, inst := range installations {
		repoOpt := &gogithub.ListOptions{PerPage: 100}
		for {
			result, resp, err := api.Apps.ListUserRepos(ctx, inst.GetID(), repoOpt)
			if err != nil {
				return nil, fmt.Errorf("list repositories for installation %d: %w", inst.GetID(), err)
			}
			out = append(out, result.Repositories...)
			if resp.NextPage == 0 {
				break
			}
			repoOpt.Page = resp.NextPage
		}
	}
	return out, nil
}

// annotateDocker sets each repo's container Kind ("compose"/"docker"/"none")
// and HasDocker by inspecting its root directory. It fans the per-repo lookups
// out with bounded concurrency (one GitHub call per repo), writing only to its
// own slice element. A failed lookup (empty repo, missing branch, transient
// error) leaves Kind "none" rather than failing the whole listing, so it is
// best-effort and never blocks the repo list.
func annotateDocker(ctx context.Context, api *gogithub.Client, repos []usecase.Repo) {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(8) // bound concurrent GitHub calls
	for i := range repos {
		// each goroutine writes only its own slice element, so no element is shared
		g.Go(func() error {
			kind := containerKind(gctx, api, repos[i])
			repos[i].Kind = kind
			repos[i].HasDocker = kind != "none"
			return nil // best-effort: never abort the group on one repo
		})
	}
	_ = g.Wait()
}

// containerKind reports how a repo containerizes from its root directory: a
// Compose file wins ("compose"), else a Dockerfile ("docker"), else "none". It
// reads the root listing in a single call against the default branch; errors
// (including a 404 on an empty repo) report "none".
func containerKind(ctx context.Context, api *gogithub.Client, r usecase.Repo) string {
	opt := &gogithub.RepositoryContentGetOptions{Ref: r.DefaultBranch}
	_, dir, _, err := api.Repositories.GetContents(ctx, r.Owner, r.Name, "", opt)
	if err != nil {
		return "none"
	}
	hasDockerfile := false
	for _, entry := range dir {
		name := strings.ToLower(entry.GetName())
		if isComposeName(name) {
			return "compose" // compose takes precedence over a bare Dockerfile
		}
		if strings.HasPrefix(name, "dockerfile") {
			hasDockerfile = true
		}
	}
	if hasDockerfile {
		return "docker"
	}
	return "none"
}

// isComposeName reports whether a root filename is one of the Compose files
// mountabo recognises (shares the list with port detection).
func isComposeName(lowerName string) bool {
	for _, n := range composeNames {
		if lowerName == n {
			return true
		}
	}
	return false
}

func toRepo(r *gogithub.Repository) usecase.Repo {
	return usecase.Repo{
		Owner:         r.GetOwner().GetLogin(),
		Name:          r.GetName(),
		FullName:      r.GetFullName(),
		Private:       r.GetPrivate(),
		DefaultBranch: r.GetDefaultBranch(),
		Language:      r.GetLanguage(),
		PushedAt:      r.GetPushedAt().Time,
	}
}
