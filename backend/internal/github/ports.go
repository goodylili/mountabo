package github

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/goodylili/mountabo/internal/usecase"
	gogithub "github.com/google/go-github/v88/github"
	"gopkg.in/yaml.v3"
)

var _ usecase.PortDetector = (*Client)(nil)

// composeNames are the Compose filenames mountabo looks for, in preference
// order, when detecting a repo's ports. Matched case-insensitively.
var composeNames = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
}

// DetectPorts reads the published ports from a repo's container configuration.
// It prefers a Compose file in the target directory; absent one, it falls back
// to a Dockerfile's EXPOSE lines. A repo (or directory) with nothing to read
// yields an empty slice rather than an error, so the UI simply shows no ports.
func (c *Client) DetectPorts(ctx context.Context, t usecase.Token, ref usecase.RepoRef) ([]usecase.ServicePort, error) {
	api, err := gogithub.NewClient(gogithub.WithAuthToken(t.AccessToken))
	if err != nil {
		return nil, fmt.Errorf("build github client: %w", err)
	}

	dir := strings.Trim(ref.Dir, "/")
	opt := &gogithub.RepositoryContentGetOptions{Ref: ref.Ref}

	// One listing of the target directory tells us which container files exist,
	// so we fetch only the file we'll actually parse. A missing directory (empty
	// repo, wrong root) is "no ports", not a failure.
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

	for _, name := range composeNames {
		actualName, ok := present[name]
		if !ok {
			continue
		}
		content, err := fetchFile(ctx, api, ref, join(dir, actualName), opt)
		if err != nil {
			return nil, err
		}
		ports, err := parseComposePorts(content)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", actualName, err)
		}
		return ports, nil
	}

	// No Compose file: any "Dockerfile*" gives us EXPOSE ports to show read-only.
	for lower, real := range present {
		if strings.HasPrefix(lower, "dockerfile") {
			content, err := fetchFile(ctx, api, ref, join(dir, real), opt)
			if err != nil {
				return nil, err
			}
			return parseDockerfileExpose(content), nil
		}
	}

	return nil, nil
}

// fetchFile reads a single file's decoded contents from the repo.
func fetchFile(ctx context.Context, api *gogithub.Client, ref usecase.RepoRef, path string, opt *gogithub.RepositoryContentGetOptions) (string, error) {
	file, _, _, err := api.Repositories.GetContents(ctx, ref.Owner, ref.Name, path, opt)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", path, err)
	}
	if file == nil {
		return "", fmt.Errorf("%s is not a file", path)
	}
	content, err := file.GetContent()
	if err != nil {
		return "", fmt.Errorf("decode %s: %w", path, err)
	}
	return content, nil
}

func isNotFound(err error) bool {
	var resp *gogithub.ErrorResponse
	return errors.As(err, &resp) && resp.Response != nil && resp.Response.StatusCode == 404
}

func join(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

// composeFile is the slice of a Compose document mountabo reads: each service's
// ports list. Items are kept as raw nodes so we can handle both the short
// string form ("8080:80") and the long mapping form ({target, published}).
type composeFile struct {
	Services map[string]struct {
		Ports []yaml.Node `yaml:"ports"`
	} `yaml:"services"`
}

func parseComposePorts(content string) ([]usecase.ServicePort, error) {
	var f composeFile
	if err := yaml.Unmarshal([]byte(content), &f); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(f.Services))
	for name := range f.Services {
		names = append(names, name)
	}
	sort.Strings(names) // deterministic order regardless of map iteration

	var out []usecase.ServicePort
	for _, name := range names {
		for _, node := range f.Services[name].Ports {
			if sp, ok := portFromNode(name, node); ok {
				out = append(out, sp)
			}
		}
	}
	return out, nil
}

// portFromNode reads one entry of a service's ports list, in either Compose
// syntax, into a ServicePort.
func portFromNode(service string, node yaml.Node) (usecase.ServicePort, bool) {
	switch node.Kind {
	case yaml.ScalarNode:
		return parseShortPort(service, node.Value)
	case yaml.MappingNode:
		var long struct {
			Target    yaml.Node `yaml:"target"`
			Published yaml.Node `yaml:"published"`
		}
		if err := node.Decode(&long); err != nil {
			return usecase.ServicePort{}, false
		}
		return buildPort(service, long.Published.Value, long.Target.Value), true
	default:
		return usecase.ServicePort{}, false
	}
}

// parseShortPort parses the short string form: "[ip:][host:]container[/proto]".
func parseShortPort(service, raw string) (usecase.ServicePort, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return usecase.ServicePort{}, false
	}
	raw = strings.SplitN(raw, "/", 2)[0] // drop any "/tcp" protocol suffix

	segs := splitPortSegments(raw)
	var host, container string
	switch len(segs) {
	case 1:
		container = segs[0] // container-only: host port is auto-assigned
	case 2:
		host, container = segs[0], segs[1]
	case 3:
		host, container = segs[1], segs[2] // ip:host:container
	default:
		return usecase.ServicePort{}, false
	}
	return buildPort(service, host, container), true
}

// splitPortSegments splits a short-form port string on its colons, but treats
// colons inside a "${...}" variable reference as literal so "${PORT:-3000}:3000"
// splits into ["${PORT:-3000}", "3000"] rather than three pieces.
func splitPortSegments(s string) []string {
	var segs []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch {
		case strings.HasPrefix(s[i:], "${"):
			depth++
			i++ // skip the '{' so it isn't reconsidered
		case s[i] == '}' && depth > 0:
			depth--
		case s[i] == ':' && depth == 0:
			segs = append(segs, s[start:i])
			start = i + 1
		}
	}
	return append(segs, s[start:])
}

// buildPort assembles a ServicePort, detecting whether the host side is an
// environment variable mountabo can set ("${PORT:-3000}") or a fixed literal.
func buildPort(service, host, container string) usecase.ServicePort {
	sp := usecase.ServicePort{Service: service, Container: strings.TrimSpace(container)}
	if envVar, def, ok := hostEnvVar(host); ok {
		sp.EnvVar, sp.Host, sp.Editable = envVar, def, true
	} else {
		sp.Host = strings.TrimSpace(host)
	}
	return sp
}

// portVarRe matches a host port written as a shell variable reference:
// "${NAME}", "${NAME:-default}", "${NAME-default}", or bare "$NAME".
var portVarRe = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)(?::?-(.*))?\}$|^\$([A-Za-z_][A-Za-z0-9_]*)$`)

// hostEnvVar reports whether the host token is a variable reference, returning
// the variable name and its default value when so.
func hostEnvVar(tok string) (name, def string, ok bool) {
	m := portVarRe.FindStringSubmatch(strings.TrimSpace(tok))
	if m == nil {
		return "", "", false
	}
	if m[1] != "" {
		return m[1], m[2], true // ${NAME} / ${NAME:-default}
	}
	return m[3], "", true // bare $NAME
}

// parseDockerfileExpose reads EXPOSE lines into read-only container ports. A
// Dockerfile has no host mapping, so these are informational only.
func parseDockerfileExpose(content string) []usecase.ServicePort {
	var out []usecase.ServicePort
	sc := bufio.NewScanner(strings.NewReader(content))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 || !strings.EqualFold(fields[0], "EXPOSE") {
			continue
		}
		for _, f := range fields[1:] {
			port := strings.SplitN(f, "/", 2)[0]
			if port != "" {
				out = append(out, usecase.ServicePort{Container: port})
			}
		}
	}
	return out
}
