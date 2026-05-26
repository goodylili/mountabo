package github

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/goodylili/mountabo/internal/usecase"
	gogithub "github.com/google/go-github/v88/github"
	"golang.org/x/crypto/nacl/box"
)

// SetSecret creates or updates a GitHub Actions secret on owner/repo. GitHub
// requires secret values to be encrypted client-side with the repository's
// public key using a libsodium sealed box, exactly what box.SealAnonymous
// produces, so the plaintext never reaches GitHub's API in the clear.
func (c *Client) SetSecret(ctx context.Context, t usecase.Token, owner, repo, name, value string) error {
	api, err := gogithub.NewClient(gogithub.WithAuthToken(t.AccessToken))
	if err != nil {
		return fmt.Errorf("build github client: %w", err)
	}

	pubKey, _, err := api.Actions.GetRepoPublicKey(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("get actions public key for %s/%s: %w", owner, repo, err)
	}

	encrypted, err := sealSecret(pubKey.GetKey(), value)
	if err != nil {
		return fmt.Errorf("encrypt secret %q: %w", name, err)
	}

	if _, err := api.Actions.CreateOrUpdateRepoSecret(ctx, owner, repo, &gogithub.EncryptedSecret{
		Name:           name,
		KeyID:          pubKey.GetKeyID(),
		EncryptedValue: encrypted,
	}); err != nil {
		return fmt.Errorf("set actions secret %q on %s/%s: %w", name, owner, repo, err)
	}
	return nil
}

// sealSecret encrypts value against the repository's base64-encoded public key
// with a libsodium sealed box, returning the base64 ciphertext GitHub expects.
func sealSecret(publicKeyB64, value string) (string, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return "", fmt.Errorf("decode public key: %w", err)
	}
	if len(keyBytes) != 32 {
		return "", fmt.Errorf("unexpected public key length %d (want 32)", len(keyBytes))
	}
	var recipient [32]byte
	copy(recipient[:], keyBytes)

	sealed, err := box.SealAnonymous(nil, []byte(value), &recipient, rand.Reader)
	if err != nil {
		return "", fmt.Errorf("seal secret: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sealed), nil
}
