package config

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestTurnCredentialsMatchCoturnRESTFormula(t *testing.T) {
	secret := "test-shared-secret"
	expiry := time.Unix(1_800_000_000, 0)
	username, credential, err := turnCredentials(secret, expiry)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(username, "1800000000:") {
		t.Fatalf("unexpected username %q", username)
	}

	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write([]byte(username))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if credential != want {
		t.Fatalf("credential mismatch: got %q want %q", credential, want)
	}
}

func TestLoadDefaultEphemeralVAPIDKeys(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.VAPIDKeys == nil {
		t.Fatal("expected VAPIDKeys to be generated")
	}
	if cfg.VAPIDPublicKey == "" || cfg.VAPIDPrivateKey == "" {
		t.Fatalf("expected non-empty vapid public/private keys: pub=%q, priv=%q", cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey)
	}
}

func TestLoadExplicitVAPIDKeys(t *testing.T) {
	t.Setenv("VAPID_PUBLIC_KEY", "BN7k_G4G293c66gO_Z9N0d9a6c4F_2Z1d293c66gO_Z9N0d9a6c4F_2Z1d293c66gO_Z9N0d9a6c4F_2Z1d293c66gO_Z9N0d9a6c4F8=")
	t.Setenv("VAPID_PRIVATE_KEY", "invalid-key")
	if _, err := Load(); err == nil {
		t.Fatal("expected error with invalid private key")
	}
}

