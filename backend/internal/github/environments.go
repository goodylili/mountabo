package github

import (
	"context"
	"fmt"

	"github.com/goodylili/mountabo/internal/usecase"
	gogithub "github.com/google/go-github/v88/github"
)

var (
	_ usecase.EnvManager      = (*Client)(nil)
	_ usecase.EnvSecretSetter = (*Client)(nil)
)

// EnsureEnvironment creates the named deployment environment on owner/repo, or
// leaves it as-is if it already exists (the API is upsert). No protection rules
// are set, mountabo only needs the environment to scope its secrets.
func (c *Client) EnsureEnvironment(ctx context.Context, t usecase.Token, owner, repo, name string) error {
	api, err := gogithub.NewClient(gogithub.WithAuthToken(t.AccessToken))
	if err != nil {
		return fmt.Errorf("build github client: %w", err)
	}
	if _, _, err := api.Repositories.CreateUpdateEnvironment(ctx, owner, repo, name, &gogithub.CreateUpdateEnvironment{}); err != nil {
		return fmt.Errorf("create environment %q on %s/%s: %w", name, owner, repo, err)
	}
	return nil
}

// SetEnvSecrets stores secrets on owner/repo's environment. GitHub's env-secret
// API is keyed by repo id and requires each value sealed against the
// environment's public key, so it resolves the id and key once, then seals and
// uploads each secret. Values never appear in errors or logs.
func (c *Client) SetEnvSecrets(ctx context.Context, t usecase.Token, owner, repo, environment string, secrets []usecase.NamedSecret) error {
	if len(secrets) == 0 {
		return nil
	}
	api, err := gogithub.NewClient(gogithub.WithAuthToken(t.AccessToken))
	if err != nil {
		return fmt.Errorf("build github client: %w", err)
	}

	repository, _, err := api.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("look up %s/%s: %w", owner, repo, err)
	}
	repoID := int(repository.GetID())

	pubKey, _, err := api.Actions.GetEnvPublicKey(ctx, repoID, environment)
	if err != nil {
		return fmt.Errorf("get public key for environment %q on %s/%s: %w", environment, owner, repo, err)
	}

	for _, secret := range secrets {
		encrypted, err := sealSecret(pubKey.GetKey(), secret.Value)
		if err != nil {
			return fmt.Errorf("encrypt secret %q: %w", secret.Name, err)
		}
		if _, err := api.Actions.CreateOrUpdateEnvSecret(ctx, repoID, environment, &gogithub.EncryptedSecret{
			Name:           secret.Name,
			KeyID:          pubKey.GetKeyID(),
			EncryptedValue: encrypted,
		}); err != nil {
			return fmt.Errorf("set secret %q on environment %q of %s/%s: %w", secret.Name, environment, owner, repo, err)
		}
	}
	return nil
}
