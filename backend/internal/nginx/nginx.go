// Package nginx renders the nginx reverse-proxy configuration that fronts a
// deployed app on its own domain with HTTPS. Generation is pure (config in, text
// out): HTTPConfig and TLSConfig render the two virtual hosts, and Script renders
// the shell script that installs nginx and certbot, writes those configs, obtains
// a Let's Encrypt certificate over the http-01 challenge, and reloads.
//
// It terminates TLS for a domain and proxies to a local app port; DNS for the
// domain must already point at the server for the http-01 challenge to pass.
package nginx

import (
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed templates/app.conf.tmpl templates/ssl.conf.tmpl
var templates embed.FS

// The templates are embedded constants, so a parse failure is a programmer error
// caught at startup, not a runtime condition to handle.
var (
	httpTmpl = template.Must(template.ParseFS(templates, "templates/app.conf.tmpl"))
	tlsTmpl  = template.Must(template.ParseFS(templates, "templates/ssl.conf.tmpl"))
)

// defaultUpstream is the local app port assumed when Config.Upstream is blank.
const defaultUpstream = "3000"

// Config is everything the generated nginx config and setup script derive from.
// Domain is the primary server name (e.g. "app.example.com"); Aliases are extra
// names served by the same vhost and certificate (e.g. "www.example.com").
// Upstream is the host port the app listens on locally; Email is the contact
// address Let's Encrypt uses for expiry notices (optional). Staging requests the
// certificate from Let's Encrypt's staging CA, which is untrusted by browsers but
// has generous rate limits, useful while testing DNS.
type Config struct {
	Domain   string
	Aliases  []string
	Upstream string
	Email    string
	Staging  bool
}

// names returns the primary domain followed by the aliases, dropping blanks. The
// primary is first because certbot stores the certificate under that name.
func (c Config) names() []string {
	out := make([]string, 0, 1+len(c.Aliases))
	if d := strings.TrimSpace(c.Domain); d != "" {
		out = append(out, d)
	}
	for _, a := range c.Aliases {
		if a = strings.TrimSpace(a); a != "" {
			out = append(out, a)
		}
	}
	return out
}

// primary is the first server name; "" when no domain is set.
func (c Config) primary() string {
	if names := c.names(); len(names) > 0 {
		return names[0]
	}
	return ""
}

// serverNames is the space-joined list for an nginx `server_name` line.
func (c Config) serverNames() string {
	return strings.Join(c.names(), " ")
}

// upstream is the chosen local port, or the default when blank.
func (c Config) upstream() string {
	if u := strings.TrimSpace(c.Upstream); u != "" {
		return u
	}
	return defaultUpstream
}

// siteData is the view passed to the templates; it exposes only the fields they
// reference so the templates stay simple.
type siteData struct {
	ServerNames string
	Upstream    string
	Domain      string
}

func (c Config) data() siteData {
	return siteData{
		ServerNames: c.serverNames(),
		Upstream:    c.upstream(),
		Domain:      c.primary(),
	}
}

// SitePath is where the vhost config is written on the server, named after the
// primary domain so each domain gets its own file.
func SitePath(c Config) string {
	return "/etc/nginx/sites-available/" + c.primary() + ".conf"
}

// HTTPConfig renders the HTTP (port 80) virtual host. It serves the ACME http-01
// challenge from the certbot webroot and redirects everything else to HTTPS. This
// vhost goes up first, before any certificate exists, so certbot can validate the
// domain; it stays in place afterwards to keep serving renewals and the redirect.
func HTTPConfig(c Config) (string, error) {
	return render(httpTmpl, c)
}

// TLSConfig renders the HTTPS (port 443) virtual host that terminates TLS with the
// Let's Encrypt certificate and proxies to the local app. It can only be loaded
// once the certificate exists, so Script writes it only after certbot succeeds.
func TLSConfig(c Config) (string, error) {
	return render(tlsTmpl, c)
}

func render(t *template.Template, c Config) (string, error) {
	var b strings.Builder
	if err := t.Execute(&b, c.data()); err != nil {
		return "", fmt.Errorf("render nginx config: %w", err)
	}
	return b.String(), nil
}

// Script renders the shell script that points the domain at the app end to end:
// install nginx and certbot, bring up the HTTP vhost, obtain the certificate over
// the http-01 challenge, then add the HTTPS vhost and reload. It is idempotent —
// rerunning it renews the certificate and rewrites the config in place — so the
// caller can stream it over SSH the same way bootstrap and option scripts run.
func Script(c Config) (string, error) {
	httpConf, err := HTTPConfig(c)
	if err != nil {
		return "", err
	}
	tlsConf, err := TLSConfig(c)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, `#!/usr/bin/env bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
log() { echo "==> $*"; }

DOMAIN=%q
SITE=%q
WEBROOT="/var/www/certbot"

log "serving %s over HTTPS, proxying to localhost:%s"
apt-get update -y
apt-get install -y nginx certbot
mkdir -p "$WEBROOT"

`, c.primary(), SitePath(c), c.serverNames(), c.upstream())

	// Step 1: HTTP vhost so certbot's http-01 challenge can be served. A quoted
	// heredoc delimiter keeps nginx's own $variables out of shell expansion.
	b.WriteString("# Step 1: HTTP virtual host that serves the ACME challenge.\n")
	b.WriteString("cat > \"$SITE\" <<'MOUNTABO_NGINX_HTTP'\n")
	b.WriteString(httpConf)
	b.WriteString("MOUNTABO_NGINX_HTTP\n")
	b.WriteString("ln -sf \"$SITE\" /etc/nginx/sites-enabled/\n")
	b.WriteString("rm -f /etc/nginx/sites-enabled/default\n")
	b.WriteString("nginx -t\n")
	b.WriteString("systemctl reload nginx || systemctl restart nginx\n\n")

	// Step 2: obtain (or renew) the certificate over the http-01 challenge.
	b.WriteString("# Step 2: obtain the Let's Encrypt certificate over http-01.\n")
	b.WriteString(certbotCommand(c))
	b.WriteString("\n\n")

	// Step 3: rewrite the site file with the HTTP vhost plus the HTTPS vhost now
	// the certificate exists, and reload.
	b.WriteString("# Step 3: add the HTTPS virtual host now the certificate exists.\n")
	b.WriteString("cat > \"$SITE\" <<'MOUNTABO_NGINX_TLS'\n")
	b.WriteString(httpConf)
	b.WriteByte('\n')
	b.WriteString(tlsConf)
	b.WriteString("MOUNTABO_NGINX_TLS\n")
	b.WriteString("nginx -t\n")
	b.WriteString("systemctl reload nginx || systemctl restart nginx\n")
	b.WriteString("log \"done, https://$DOMAIN is live\"\n")

	return b.String(), nil
}

// RemoveScript renders the script that tears a domain back down: drop the vhost
// from sites-enabled and sites-available, reload nginx (only if the remaining
// config still tests clean), and delete the Let's Encrypt certificate. Every
// destructive step tolerates the domain already being gone, so removing a domain
// that was never fully set up still exits cleanly.
func RemoveScript(c Config) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
log() { echo "==> $*"; }

DOMAIN=%q
SITE=%q

log "removing the HTTPS site for %s"
rm -f "/etc/nginx/sites-enabled/${DOMAIN}.conf" "$SITE"
if nginx -t; then
  systemctl reload nginx || systemctl restart nginx
fi
certbot delete --cert-name "$DOMAIN" -n >/dev/null 2>&1 || true
log "done, $DOMAIN removed"
`, c.primary(), SitePath(c), c.serverNames())
}

// certbotCommand builds the `certbot certonly --webroot` invocation: one -d per
// server name, an email registration (or an explicit opt-out when none is given),
// non-interactive agreement, and the staging flag when requested.
func certbotCommand(c Config) string {
	var b strings.Builder
	b.WriteString("certbot certonly --webroot -w \"$WEBROOT\"")
	for _, name := range c.names() {
		fmt.Fprintf(&b, " \\\n  -d %s", name)
	}
	if email := strings.TrimSpace(c.Email); email != "" {
		fmt.Fprintf(&b, " \\\n  --email %q", email)
	} else {
		b.WriteString(" \\\n  --register-unsafely-without-email")
	}
	b.WriteString(" \\\n  --agree-tos --no-eff-email -n")
	if c.Staging {
		b.WriteString(" \\\n  --staging")
	}
	return b.String()
}
