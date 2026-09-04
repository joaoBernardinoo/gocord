package push

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

var (
	ErrInvalidVAPIDKey = errors.New("invalid VAPID key format")
)

type VAPIDKeys struct {
	privateKey       *ecdsa.PrivateKey
	publicKeyBytes   []byte
	publicKeyBase64  string
	privateKeyBase64 string
}

// GenerateVAPIDKeys creates a new ephemeral P-256 ECDSA key pair for VAPID.
func GenerateVAPIDKeys() (*VAPIDKeys, error) {
	curve := elliptic.P256()
	priv, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate vapid key: %w", err)
	}
	return newVAPIDKeysFromPrivate(priv)
}

// LoadVAPIDKeys loads VAPID keys from base64-encoded strings (RawURLEncoding or standard).
// If only privKeyB64 is provided, the public key is derived.
func LoadVAPIDKeys(pubKeyB64, privKeyB64 string) (*VAPIDKeys, error) {
	privBytes, err := DecodeBase64Flexible(privKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode vapid private key: %w", err)
	}
	if len(privBytes) != 32 {
		return nil, fmt.Errorf("%w: private key must be 32 bytes, got %d", ErrInvalidVAPIDKey, len(privBytes))
	}

	curve := elliptic.P256()
	d := new(big.Int).SetBytes(privBytes)
	priv := new(ecdsa.PrivateKey)
	priv.PublicKey.Curve = curve
	priv.D = d
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(privBytes)

	keys, err := newVAPIDKeysFromPrivate(priv)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(pubKeyB64) != "" {
		expectedPubBytes, err := DecodeBase64Flexible(pubKeyB64)
		if err != nil {
			return nil, fmt.Errorf("decode vapid public key: %w", err)
		}
		if len(expectedPubBytes) != 65 || expectedPubBytes[0] != 0x04 {
			return nil, fmt.Errorf("%w: public key must be 65 bytes uncompressed", ErrInvalidVAPIDKey)
		}
		if string(expectedPubBytes) != string(keys.publicKeyBytes) {
			return nil, fmt.Errorf("%w: public key does not match private key", ErrInvalidVAPIDKey)
		}
	}

	return keys, nil
}

func newVAPIDKeysFromPrivate(priv *ecdsa.PrivateKey) (*VAPIDKeys, error) {
	curve := elliptic.P256()
	pubBytes := elliptic.Marshal(curve, priv.PublicKey.X, priv.PublicKey.Y) // 65 bytes uncompressed (0x04 || X || Y)
	if len(pubBytes) != 65 {
		return nil, fmt.Errorf("%w: invalid marshaled public key length", ErrInvalidVAPIDKey)
	}

	dBytes := priv.D.Bytes()
	if len(dBytes) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(dBytes):], dBytes)
		dBytes = padded
	} else if len(dBytes) > 32 {
		return nil, fmt.Errorf("%w: private key scalar is too large", ErrInvalidVAPIDKey)
	}

	return &VAPIDKeys{
		privateKey:       priv,
		publicKeyBytes:   pubBytes,
		publicKeyBase64:  base64.RawURLEncoding.EncodeToString(pubBytes),
		privateKeyBase64: base64.RawURLEncoding.EncodeToString(dBytes),
	}, nil
}

// PublicKeyBase64 returns the uncompressed P-256 public key in base64 URL raw encoding.
func (v *VAPIDKeys) PublicKeyBase64() string {
	return v.publicKeyBase64
}

// PrivateKeyBase64 returns the 32-byte private key scalar in base64 URL raw encoding.
func (v *VAPIDKeys) PrivateKeyBase64() string {
	return v.privateKeyBase64
}

// PublicKeyBytes returns the 65-byte uncompressed P-256 public key.
func (v *VAPIDKeys) PublicKeyBytes() []byte {
	return append([]byte(nil), v.publicKeyBytes...)
}

type jwtHeader struct {
	Typ string `json:"typ"`
	Alg string `json:"alg"`
}

type jwtClaims struct {
	Aud string `json:"aud"`
	Exp int64  `json:"exp"`
	Sub string `json:"sub"`
}

// SignJWT creates an ES256 JWT for VAPID authentication (RFC 8292).
func (v *VAPIDKeys) SignJWT(audience, subject string, expiry time.Time) (string, error) {
	if subject == "" {
		subject = "mailto:admin@localhost"
	}

	headerJSON, err := json.Marshal(jwtHeader{Typ: "JWT", Alg: "ES256"})
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(jwtClaims{
		Aud: audience,
		Exp: expiry.Unix(),
		Sub: subject,
	})
	if err != nil {
		return "", err
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := headerB64 + "." + claimsB64

	hash := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, v.privateKey, hash[:])
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}

	// Format signature as IEEE P1363 (32 bytes r + 32 bytes s)
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])

	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	return signingInput + "." + sigB64, nil
}

// DecodeBase64Flexible decodes base64 string tolerating URL-safe, Standard, and unpadded variants.
func DecodeBase64Flexible(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	// Try RawURLEncoding first
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	// Try URLEncoding
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	// Try RawStdEncoding
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	// Try StdEncoding
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return nil, errors.New("invalid base64 encoding")
}
