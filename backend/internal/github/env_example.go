package github

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/goodylili/mountabo/internal/usecase"
	gogithub "github.com/google/go-github/v88/github"
)

var _ usecase.EnvExampleReader = (*Client)(nil)

// envExampleNames are the example env filenames mountabo looks for, in
// preference order. Matched case-insensitively.
var envExampleNames = []string{
	".env.example",
	".env.sample",
	".env.template",
	".env.dist",
	"example.env",
}

// EnvExampleKeys reads the variable names declared in a repo's example env file
// in the target directory, so the configure form can pre-fill the env var rows.
// It picks the first example file present (preferring .env.example) and returns
// its keys in file order; only the names are read, never the values. A repo or
// directory with no example file yields an empty slice, not an error.
func (c *Client) EnvExampleKeys(ctx context.Context, t usecase.Token, ref usecase.RepoRef) ([]string, error) {
	api, err := gogithub.NewClient(gogithub.WithAuthToken(t.AccessToken))
	if err != nil {
		return nil, fmt.Errorf("build github client: %w", err)
	}

	dir := strings.Trim(ref.Dir, "/")
	opt := &gogithub.RepositoryContentGetOptions{Ref: ref.Ref}

	// One listing of the target directory tells us which example file exists, so
	// we fetch only the one we'll parse. A missing directory (empty repo, wrong
	// root) is "no example", not a failure.
	_, entries, _, err := api.Repositories.GetContents(ctx, ref.Owner, ref.Name, dir, opt)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s/%s contents: %w", ref.Owner, ref.Name, err)
	}

	present := make(map[string]string, len(entries)) // lowercased name -> real name
	for _, e := range entries {
		if e.GetType() == "file" {
			present[strings.ToLower(e.GetName())] = e.GetName()
		}
	}

	for _, name := range envExampleNames {
		actualName, ok := present[name]
		if !ok {
			continue
		}
		content, err := fetchFile(ctx, api, ref, join(dir, actualName), opt)
		if err != nil {
			return nil, err
		}
		return parseEnvKeys(content), nil
	}
	return nil, nil
}

// parseEnvKeys extracts the variable names from an env file's contents, in file
// order with duplicates dropped. It accepts `KEY=value`, `export KEY=value`,
// blank lines, and `#` comments, mirroring the frontend's .env parser, and keeps
// only valid env identifiers. Values are ignored entirely.
func parseEnvKeys(content string) []string {
	var keys []string
	seen := map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(content))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if rest, ok := strings.CutPrefix(line, "export "); ok {
			line = strings.TrimSpace(rest)
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		if !isEnvName(key) || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys
}

// isEnvName reports whether s is a valid environment variable name: a leading
// letter or underscore, then letters, digits, or underscores.
func isEnvName(s string) bool {
	for i, r := range s {
		switch {
		case r == '_', r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return s != ""
}
