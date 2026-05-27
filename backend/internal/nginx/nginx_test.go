package nginx

import (
	"strings"
	"testing"
)

func base() Config {
	return Config{
		Domain:   "app.example.com",
		Aliases:  []string{"www.example.com"},
		Upstream: "3000",
		Email:    "ops@example.com",
	}
}

func TestHTTPConfig_ServesChallengeAndRedirects(t *testing.T) {
	conf, err := HTTPConfig(base())
	if err != nil {
		t.Fatalf("HTTPConfig: %v", err)
	}
	for _, want := range []string{
		"listen 80;",
		"server_name app.example.com www.example.com;",
		"location /.well-known/acme-challenge/",
		"return 301 https://$host$request_uri;",
	} {
		if !strings.Contains(conf, want) {
			t.Errorf("HTTP config missing %q", want)
		}
	}
}

func TestTLSConfig_TerminatesAndProxies(t *testing.T) {
	conf, err := TLSConfig(base())
	if err != nil {
		t.Fatalf("TLSConfig: %v", err)
	}
	for _, want := range []string{
		"listen 443 ssl;",
		"ssl_certificate     /etc/letsencrypt/live/app.example.com/fullchain.pem;",
		"proxy_pass http://127.0.0.1:3000;",
	} {
		if !strings.Contains(conf, want) {
			t.Errorf("TLS config missing %q", want)
		}
	}
}

func TestUpstreamDefaultsWhenBlank(t *testing.T) {
	c := base()
	c.Upstream = ""
	conf, err := TLSConfig(c)
	if err != nil {
		t.Fatalf("TLSConfig: %v", err)
	}
	if !strings.Contains(conf, "proxy_pass http://127.0.0.1:3000;") {
		t.Error("blank upstream should default to 3000")
	}
}

func TestScript_FullFlow(t *testing.T) {
	s, err := Script(base())
	if err != nil {
		t.Fatalf("Script: %v", err)
	}
	for _, want := range []string{
		"apt-get install -y nginx certbot",
		`SITE="/etc/nginx/sites-available/app.example.com.conf"`,
		"certbot certonly --webroot",
		"-d app.example.com",
		"-d www.example.com",
		`--email "ops@example.com"`,
		"systemctl reload nginx",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("script missing %q", want)
		}
	}
	// HTTP vhost is written before the certificate, then both vhosts after.
	if strings.Count(s, "location /.well-known/acme-challenge/") != 2 {
		t.Error("the HTTP vhost should be written in both step 1 and step 3")
	}
	// Default Config has Staging false, so the staging flag must be absent.
	if strings.Contains(s, "--staging") {
		t.Error("staging flag should be absent when Staging is false")
	}
}

func TestCertbotCommand_NoEmailOptsOut(t *testing.T) {
	c := base()
	c.Email = ""
	cmd := certbotCommand(c)
	if !strings.Contains(cmd, "--register-unsafely-without-email") {
		t.Error("blank email should register without an email")
	}
	if strings.Contains(cmd, "--email") {
		t.Error("blank email should not emit --email")
	}
}

func TestRemoveScript_TearsDownSiteAndCert(t *testing.T) {
	s := RemoveScript(base())
	for _, want := range []string{
		`DOMAIN="app.example.com"`,
		`SITE="/etc/nginx/sites-available/app.example.com.conf"`,
		`rm -f "/etc/nginx/sites-enabled/${DOMAIN}.conf" "$SITE"`,
		`certbot delete --cert-name "$DOMAIN"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("remove script missing %q", want)
		}
	}
}

func TestStaging_AddsFlag(t *testing.T) {
	c := base()
	c.Staging = true
	if !strings.Contains(certbotCommand(c), "--staging") {
		t.Error("Staging should add the --staging flag")
	}
}
