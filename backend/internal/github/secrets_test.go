package github

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"golang.org/x/crypto/nacl/box"
)

// TestSealSecret verifies sealSecret produces a libsodium sealed box that the
// matching private key can open back to the original value — i.e. the exact
// contract GitHub Actions relies on to decrypt secrets.
func TestSealSecret(t *testing.T) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	pubB64 := base64.StdEncoding.EncodeToString(pub[:])

	const plaintext = "multi-line value\nwith symbols !@#$"
	sealedB64, err := sealSecret(pubB64, plaintext)
	if err != nil {
		t.Fatalf("sealSecret: %v", err)
	}

	sealed, err := base64.StdEncoding.DecodeString(sealedB64)
	if err != nil {
		t.Fatalf("decode sealed: %v", err)
	}
	opened, ok := box.OpenAnonymous(nil, sealed, pub, priv)
	if !ok {
		t.Fatal("OpenAnonymous failed — ciphertext not decryptable with the keypair")
	}
	if string(opened) != plaintext {
		t.Errorf("roundtrip mismatch: got %q", string(opened))
	}
}

func TestSealSecret_RejectsBadKey(t *testing.T) {
	if _, err := sealSecret("not-base64!!", "x"); err == nil {
		t.Error("expected error for non-base64 public key")
	}
	if _, err := sealSecret(base64.StdEncoding.EncodeToString([]byte("short")), "x"); err == nil {
		t.Error("expected error for wrong-length public key")
	}
}
