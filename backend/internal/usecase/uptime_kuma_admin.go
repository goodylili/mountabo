package usecase

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// UptimeKumaAdmin is the operator-facing credential pair for an Uptime Kuma
// instance mountabo manages. The password is shown to the operator exactly
// once (regenerate to get a new one); both are persisted in the OS keychain
// so a reload of the deployments page still has them.
type UptimeKumaAdmin struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// UptimeKumaAdminService manages the admin account on an Uptime Kuma instance
// running on one of the operator's servers: it generates a credential pair,
// writes it straight into Uptime Kuma's SQLite database from inside the
// container (bcrypt hashed via UK's own bundled bcryptjs), and stores the
// pair in the keychain so the dashboard panel can show it. UK has no public
// HTTP setup endpoint, hence the in-container seeding.
type UptimeKumaAdminService struct {
	servers ServerStore
	vault   SecretVault
	runner  RootRunner
}

// NewUptimeKumaAdminService wires the service to its ports.
func NewUptimeKumaAdminService(servers ServerStore, vault SecretVault, runner RootRunner) *UptimeKumaAdminService {
	return &UptimeKumaAdminService{servers: servers, vault: vault, runner: runner}
}

// Get returns the stored admin credentials for id's Uptime Kuma, or false when
// none have been generated yet (the operator hasn't clicked "set up admin").
func (s *UptimeKumaAdminService) Get(id string) (UptimeKumaAdmin, bool, error) {
	raw, err := s.vault.LoadSecret(uptimeKumaAdminKey(id))
	if err != nil {
		// The vault returns a sentinel for "not found"; treat any error as
		// not-yet-set since the panel just shows the "set up admin" CTA.
		return UptimeKumaAdmin{}, false, nil //nolint:nilerr // not-found is expected here
	}
	var admin UptimeKumaAdmin
	if err := json.Unmarshal([]byte(raw), &admin); err != nil {
		return UptimeKumaAdmin{}, false, fmt.Errorf("decode stored uptime kuma admin: %w", err)
	}
	return admin, true, nil
}

// Reset generates fresh credentials, seeds them into Uptime Kuma's SQLite from
// inside the container (the only reliable path: UK has no setup HTTP route),
// and persists them in the keychain. Returns the new credentials so the UI can
// show them once. The server must be set up and Uptime Kuma must be running.
func (s *UptimeKumaAdminService) Reset(ctx context.Context, id string) (UptimeKumaAdmin, error) {
	server, err := s.servers.Get(id)
	if err != nil {
		return UptimeKumaAdmin{}, err
	}
	if server.Status != StatusReady {
		return UptimeKumaAdmin{}, ErrToolNotInstalled
	}
	hasKuma := false
	for _, o := range server.Options {
		if o == "uptime-kuma" {
			hasKuma = true
			break
		}
	}
	if !hasKuma {
		return UptimeKumaAdmin{}, ErrToolNotInstalled
	}

	key, err := s.vault.LoadSecret(privateKeyKey(id))
	if err != nil {
		return UptimeKumaAdmin{}, err
	}
	target := SSHTarget{
		Host:        server.IP,
		Port:        server.SSHPort,
		User:        BootstrapUser,
		PrivateKey:  key,
		Fingerprint: server.Fingerprint,
	}

	password, err := randomPassword(20)
	if err != nil {
		return UptimeKumaAdmin{}, fmt.Errorf("generate password: %w", err)
	}
	admin := UptimeKumaAdmin{Username: "mountabo", Password: password}

	// Seed the user directly in UK's SQLite. UK ships bcryptjs and the DB at
	// /app/data/kuma.db; the active column is on the row (UK 1.x). Username
	// and password ride in as env vars so neither needs shell escaping. A
	// final container restart is harmless and clears any session cache.
	const script = `set -e
docker exec -e UK_USER -e UK_PWD uptime-kuma node -e '
  const Database = require("better-sqlite3");
  const bcrypt = require("bcryptjs");
  const db = new Database("/app/data/kuma.db");
  const hash = bcrypt.hashSync(process.env.UK_PWD, 10);
  const row = db.prepare("SELECT id FROM user LIMIT 1").get();
  if (row) {
    db.prepare("UPDATE user SET username = ?, password = ?, active = 1, twofa_status = 0 WHERE id = ?").run(process.env.UK_USER, hash, row.id);
  } else {
    db.prepare("INSERT INTO user (username, password, active, twofa_status) VALUES (?, ?, 1, 0)").run(process.env.UK_USER, hash);
  }
'
docker restart uptime-kuma >/dev/null
`
	scriptWithEnv := fmt.Sprintf("export UK_USER=%s\nexport UK_PWD=%s\n%s",
		shellQuote(admin.Username), shellQuote(admin.Password), script)

	if err := s.runner.RunAsRoot(ctx, target, scriptWithEnv, io.Discard); err != nil {
		return UptimeKumaAdmin{}, fmt.Errorf("seed uptime kuma admin: %w", err)
	}

	// G117 flags this for marshaling a Password field, but the keychain is
	// exactly where this secret belongs and the encoded blob is written
	// straight into SaveSecret on the next line.
	enc, err := json.Marshal(admin) //nolint:gosec // stored in the OS keychain by design
	if err != nil {
		return UptimeKumaAdmin{}, fmt.Errorf("encode admin: %w", err)
	}
	if err := s.vault.SaveSecret(uptimeKumaAdminKey(id), string(enc)); err != nil {
		return UptimeKumaAdmin{}, fmt.Errorf("store admin: %w", err)
	}
	return admin, nil
}

// uptimeKumaAdminKey is the keychain entry holding the JSON-encoded admin
// credentials for a server's Uptime Kuma instance.
func uptimeKumaAdminKey(serverID string) string {
	return "mountabo/uptime-kuma/admin/" + serverID
}

// randomPassword returns n url-safe-alphabet characters from the system CSPRNG.
// Length 20 gives ~120 bits of entropy, well above what the dashboard needs.
func randomPassword(n int) (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	out := make([]byte, n)
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i, b := range buf {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out), nil
}

// shellQuote single-quotes s for safe bash inclusion (every single quote inside
// is closed, escaped, and reopened), so an attacker cannot inject shell from
// a value that ends up in the seed script.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
