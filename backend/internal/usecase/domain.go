package usecase

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/goodylili/mountabo/internal/nginx"
)

// hostname matches a fully-qualified domain name (at least one dot, valid label
// characters). It is deliberately strict: the value flows into an nginx
// server_name and certbot -d flag, so we reject anything that is not a plain
// host before it reaches a shell.
var hostname = regexp.MustCompile(`^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,63}$`)

// Domain is a custom domain fronted by nginx + Let's Encrypt HTTPS on a server,
// proxying to a local app port. Host is the primary name; Aliases are extra
// names served by the same vhost and certificate (e.g. the www variant).
type Domain struct {
	Host      string    `json:"host"`
	Aliases   []string  `json:"aliases,omitempty"`
	Upstream  string    `json:"upstream"`
	Email     string    `json:"email,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// DomainInput is what the operator supplies to point a domain at an app. Staging
// requests an untrusted certificate from Let's Encrypt's staging CA, useful while
// testing DNS without burning the production rate limit.
type DomainInput struct {
	Host     string
	Aliases  []string
	Upstream string
	Email    string
	Staging  bool
}

// DomainArtifacts is exactly what configuring a domain will write and run on the
// server: the two nginx vhost configs, the setup script, and where the config
// lives. Generated purely from a DomainInput so the UI can preview it.
type DomainArtifacts struct {
	SitePath   string `json:"sitePath"`
	HTTPConfig string `json:"httpConfig"`
	TLSConfig  string `json:"tlsConfig"`
	Script     string `json:"script"`
}

// normalize trims and lower-cases the host and aliases (DNS is case-insensitive),
// drops blank/duplicate aliases and any alias equal to the host, and defaults the
// upstream port. It does not validate; call validateDomain for that.
func (in DomainInput) normalize() DomainInput {
	in.Host = strings.ToLower(strings.TrimSpace(in.Host))
	in.Upstream = strings.TrimSpace(in.Upstream)
	in.Email = strings.TrimSpace(in.Email)

	seen := map[string]bool{in.Host: true}
	aliases := make([]string, 0, len(in.Aliases))
	for _, a := range in.Aliases {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" || seen[a] {
			continue
		}
		seen[a] = true
		aliases = append(aliases, a)
	}
	in.Aliases = aliases
	return in
}

// validateDomain rejects anything that is not a clean FQDN, a valid port, or a
// plausible email before it reaches the server's shell.
func validateDomain(in DomainInput) error {
	if !hostname.MatchString(in.Host) {
		return fmt.Errorf("invalid domain %q", in.Host)
	}
	for _, a := range in.Aliases {
		if !hostname.MatchString(a) {
			return fmt.Errorf("invalid domain %q", a)
		}
	}
	if u := in.Upstream; u != "" {
		port, err := strconv.Atoi(u)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid app port %q", u)
		}
	}
	if in.Email != "" && (!strings.Contains(in.Email, "@") || strings.ContainsAny(in.Email, " \t\n")) {
		return fmt.Errorf("invalid email %q", in.Email)
	}
	return nil
}

// config maps a normalized DomainInput onto the nginx generator's config.
func (in DomainInput) config() nginx.Config {
	return nginx.Config{
		Domain:   in.Host,
		Aliases:  in.Aliases,
		Upstream: in.Upstream,
		Email:    in.Email,
		Staging:  in.Staging,
	}
}

// toDomain records the input as a stored Domain (without the staging flag, which
// only affects the one-off certbot run).
func (in DomainInput) toDomain() Domain {
	return Domain{
		Host:      in.Host,
		Aliases:   in.Aliases,
		Upstream:  in.Upstream,
		Email:     in.Email,
		CreatedAt: time.Now().UTC(),
	}
}

// PreviewDomain renders the nginx config and setup script for a domain from the
// input alone, no server, no SSH, no side effects, so the UI can show exactly
// what configuring the domain would do.
func (s *ServerService) PreviewDomain(in DomainInput) (DomainArtifacts, error) {
	in = in.normalize()
	if err := validateDomain(in); err != nil {
		return DomainArtifacts{}, err
	}
	cfg := in.config()
	httpConf, err := nginx.HTTPConfig(cfg)
	if err != nil {
		return DomainArtifacts{}, err
	}
	tlsConf, err := nginx.TLSConfig(cfg)
	if err != nil {
		return DomainArtifacts{}, err
	}
	script, err := nginx.Script(cfg)
	if err != nil {
		return DomainArtifacts{}, err
	}
	return DomainArtifacts{
		SitePath:   nginx.SitePath(cfg),
		HTTPConfig: httpConf,
		TLSConfig:  tlsConf,
		Script:     script,
	}, nil
}

// AddDomain points a domain at a local app port on a ready server: it installs
// nginx and certbot, writes the vhost, obtains a Let's Encrypt certificate over
// the http-01 challenge, and reloads, streaming the live log to out. On success
// the domain is recorded on the server (replacing any earlier entry for the same
// host, so re-running updates it in place). DNS for the domain must already point
// at the server, that is the operator's responsibility and the http-01 challenge
// fails loudly if it does not.
func (s *ServerService) AddDomain(ctx context.Context, id string, in DomainInput, out io.Writer) error {
	in = in.normalize()
	if err := validateDomain(in); err != nil {
		return err
	}

	server, key, err := s.lockReadyServer(id)
	if err != nil {
		return err
	}
	defer s.unlock(id)

	script, err := nginx.Script(in.config())
	if err != nil {
		return err
	}

	target := SSHTarget{Host: server.IP, Port: server.SSHPort, User: BootstrapUser, PrivateKey: key, Fingerprint: server.Fingerprint}
	if runErr := s.runner.RunAsRoot(ctx, target, script, out); runErr != nil {
		return fmt.Errorf("configure domain: %w", runErr)
	}

	server.Domains = upsertDomain(server.Domains, in.toDomain())
	if err := s.store.Save(server); err != nil {
		return fmt.Errorf("save server: %w", err)
	}
	return nil
}

// RemoveDomain tears a domain's nginx vhost and certificate back down on the
// server and drops it from the server's record, streaming progress to out.
// Removing a host that is not (or no longer) configured still succeeds, the
// teardown script tolerates missing files.
func (s *ServerService) RemoveDomain(ctx context.Context, id, host string, out io.Writer) error {
	host = strings.ToLower(strings.TrimSpace(host))
	if !hostname.MatchString(host) {
		return fmt.Errorf("invalid domain %q", host)
	}

	server, key, err := s.lockReadyServer(id)
	if err != nil {
		return err
	}
	defer s.unlock(id)

	target := SSHTarget{Host: server.IP, Port: server.SSHPort, User: BootstrapUser, PrivateKey: key, Fingerprint: server.Fingerprint}
	if runErr := s.runner.RunAsRoot(ctx, target, nginx.RemoveScript(nginx.Config{Domain: host}), out); runErr != nil {
		return fmt.Errorf("remove domain: %w", runErr)
	}

	server.Domains = removeDomain(server.Domains, host)
	if err := s.store.Save(server); err != nil {
		return fmt.Errorf("save server: %w", err)
	}
	return nil
}

// lockReadyServer claims the per-server work lock, loads the server, checks it is
// ready, and loads mountabo's key for the SSH connection. The caller must defer
// s.unlock(id). It mirrors the guard ApplyOptions uses so domain changes never
// run concurrently with a bootstrap, an apply, or each other.
func (s *ServerService) lockReadyServer(id string) (Server, string, error) {
	s.mu.Lock()
	if s.settingUp[id] {
		s.mu.Unlock()
		return Server{}, "", ErrSetupInProgress
	}
	s.settingUp[id] = true
	s.mu.Unlock()

	server, err := s.store.Get(id)
	if err != nil {
		s.unlock(id)
		return Server{}, "", err
	}
	if server.Status != StatusReady {
		s.unlock(id)
		return Server{}, "", fmt.Errorf("server must be set up before managing domains")
	}
	key, err := s.vault.LoadSecret(privateKeyKey(id))
	if err != nil {
		s.unlock(id)
		return Server{}, "", fmt.Errorf("load mountabo key: %w", err)
	}
	return server, key, nil
}

func (s *ServerService) unlock(id string) {
	s.mu.Lock()
	delete(s.settingUp, id)
	s.mu.Unlock()
}

// upsertDomain replaces an existing entry for the same host (case-insensitive) or
// appends a new one, so configuring a domain twice updates it rather than
// duplicating it.
func upsertDomain(domains []Domain, d Domain) []Domain {
	for i, existing := range domains {
		if strings.EqualFold(existing.Host, d.Host) {
			domains[i] = d
			return domains
		}
	}
	return append(domains, d)
}

// removeDomain drops the entry for host (case-insensitive), preserving order.
func removeDomain(domains []Domain, host string) []Domain {
	out := make([]Domain, 0, len(domains))
	for _, d := range domains {
		if !strings.EqualFold(d.Host, host) {
			out = append(out, d)
		}
	}
	return out
}
