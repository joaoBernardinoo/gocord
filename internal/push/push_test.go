package push

import (
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestVAPIDKeyGenerationAndLoading(t *testing.T) {
	keys, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatalf("generate vapid keys: %v", err)
	}

	pubB64 := keys.PublicKeyBase64()
	privB64 := keys.PrivateKeyBase64()

	if pubB64 == "" || privB64 == "" {
		t.Fatalf("empty key string: pub=%q, priv=%q", pubB64, privB64)
	}

	loaded, err := LoadVAPIDKeys(pubB64, privB64)
	if err != nil {
		t.Fatalf("load vapid keys: %v", err)
	}

	if loaded.PublicKeyBase64() != pubB64 {
		t.Errorf("public key mismatch: got %q, want %q", loaded.PublicKeyBase64(), pubB64)
	}
	if loaded.PrivateKeyBase64() != privB64 {
		t.Errorf("private key mismatch: got %q, want %q", loaded.PrivateKeyBase64(), privB64)
	}

	// Load with only private key
	loadedPrivOnly, err := LoadVAPIDKeys("", privB64)
	if err != nil {
		t.Fatalf("load vapid keys with priv only: %v", err)
	}
	if loadedPrivOnly.PublicKeyBase64() != pubB64 {
		t.Errorf("derived public key mismatch: got %q, want %q", loadedPrivOnly.PublicKeyBase64(), pubB64)
	}
}

func TestVAPIDSignJWT(t *testing.T) {
	keys, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}

	audience := "https://fcm.googleapis.com"
	subject := "mailto:admin@example.com"
	expiry := time.Now().Add(time.Hour)

	token, err := keys.SignJWT(audience, subject, expiry)
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("jwt parts = %d, want 3", len(parts))
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	if header.Alg != "ES256" || header.Typ != "JWT" {
		t.Errorf("unexpected header: %+v", header)
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	var claims jwtClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	if claims.Aud != audience || claims.Sub != subject || claims.Exp != expiry.Unix() {
		t.Errorf("unexpected claims: %+v", claims)
	}

	// Verify ECDSA signature
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode sig: %v", err)
	}
	if len(sigBytes) != 64 {
		t.Fatalf("sig length = %d, want 64", len(sigBytes))
	}

	r := new(big.Int).SetBytes(sigBytes[:32])
	s := new(big.Int).SetBytes(sigBytes[32:])
	signingInput := parts[0] + "." + parts[1]
	hash := sha256.Sum256([]byte(signingInput))

	pubKeyBytes := keys.PublicKeyBytes()
	curve := elliptic.P256()
	x, y := elliptic.Unmarshal(curve, pubKeyBytes)
	ecdsaPub := &ecdsa.PublicKey{Curve: curve, X: x, Y: y}

	if !ecdsa.Verify(ecdsaPub, hash[:], r, s) {
		t.Fatal("jwt signature verification failed")
	}
}

func TestRFC8291EncryptionDecryptionRoundtrip(t *testing.T) {
	// Generate recipient (User Agent) keys
	curve := ecdh.P256()
	uaPriv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	uaPubBytes := uaPriv.PublicKey().Bytes()

	authSecret := make([]byte, 16)
	if _, err := rand.Read(authSecret); err != nil {
		t.Fatal(err)
	}

	testCases := []string{
		"Hello, Web Push!",
		`{"title":"Call from Alice","url":"https://example.com/join/room123#sec999"}`,
		strings.Repeat("Long payload text with Unicode 🚀📞 ", 50),
	}

	for _, tc := range testCases {
		encrypted, err := EncryptPayload(uaPubBytes, authSecret, []byte(tc))
		if err != nil {
			t.Fatalf("encrypt failed for %q: %v", tc, err)
		}

		decrypted, err := DecryptPayload(uaPriv, authSecret, encrypted)
		if err != nil {
			t.Fatalf("decrypt failed for %q: %v", tc, err)
		}

		if string(decrypted) != tc {
			t.Errorf("roundtrip mismatch: got %q, want %q", string(decrypted), tc)
		}
	}
}

func TestEncryptPayloadInvalidKeys(t *testing.T) {
	auth := make([]byte, 16)
	pub := make([]byte, 64) // invalid: 64 instead of 65
	if _, err := EncryptPayload(pub, auth, []byte("test")); err == nil {
		t.Error("expected error for invalid public key length")
	}

	pubValid := make([]byte, 65)
	pubValid[0] = 0x04
	authShort := make([]byte, 10)
	if _, err := EncryptPayload(pubValid, authShort, []byte("test")); err == nil {
		t.Error("expected error for invalid auth length")
	}
}

func TestSenderDeliversNotification(t *testing.T) {
	keys, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}

	// Generate client UA keys
	uaPriv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authSecret := make([]byte, 16)
	rand.Read(authSecret)

	expectedMsg := `{"title":"Incoming Call","url":"https://call.example.com/join/room1#secret"}`
	var receivedBody []byte
	var receivedAuth string
	var receivedTTL string
	var receivedEnc string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedTTL = r.Header.Get("TTL")
		receivedEnc = r.Header.Get("Content-Encoding")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		receivedBody = body
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()

	sender := NewSender(keys, "mailto:admin@example.com", ts.Client())
	sender.SetAllowEndpoint(func(*url.URL) bool { return true }) // test server isn't a real push host

	sub := Subscription{
		Endpoint: ts.URL + "/push-endpoint",
		Keys: SubscriptionKeys{
			P256DH: base64.RawURLEncoding.EncodeToString(uaPriv.PublicKey().Bytes()),
			Auth:   base64.RawURLEncoding.EncodeToString(authSecret),
		},
	}

	err = sender.Send(context.Background(), sub, []byte(expectedMsg), 120)
	if err != nil {
		t.Fatalf("sender.Send failed: %v", err)
	}

	if !strings.HasPrefix(receivedAuth, "vapid t=") || !strings.Contains(receivedAuth, ", k="+keys.PublicKeyBase64()) {
		t.Errorf("unexpected Authorization header: %q", receivedAuth)
	}
	if receivedTTL != "120" {
		t.Errorf("TTL = %q, want 120", receivedTTL)
	}
	if receivedEnc != "aes128gcm" {
		t.Errorf("Content-Encoding = %q, want aes128gcm", receivedEnc)
	}

	decrypted, err := DecryptPayload(uaPriv, authSecret, receivedBody)
	if err != nil {
		t.Fatalf("failed to decrypt received body on simulated push server: %v", err)
	}
	if string(decrypted) != expectedMsg {
		t.Fatalf("decrypted msg = %q, want %q", string(decrypted), expectedMsg)
	}
}

func TestSenderHandlesExpiredSubscription(t *testing.T) {
	keys, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	uaPriv, _ := ecdh.P256().GenerateKey(rand.Reader)
	authSecret := make([]byte, 16)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone) // 410
	}))
	defer ts.Close()

	sender := NewSender(keys, "mailto:admin@example.com", ts.Client())
	sender.SetAllowEndpoint(func(*url.URL) bool { return true }) // test server isn't a real push host
	sub := Subscription{
		Endpoint: ts.URL + "/expired",
		Keys: SubscriptionKeys{
			P256DH: base64.RawURLEncoding.EncodeToString(uaPriv.PublicKey().Bytes()),
			Auth:   base64.RawURLEncoding.EncodeToString(authSecret),
		},
	}

	err = sender.Send(context.Background(), sub, []byte("test"), 60)
	if err != ErrSubscriptionExpired {
		t.Fatalf("expected ErrSubscriptionExpired, got %v", err)
	}
}

// The server relays notifications to any endpoint a caller supplies, so
// Send must refuse hosts outside the standard push services (and any non
// https scheme) rather than acting as an open relay/SSRF primitive.
func TestSenderRejectsUnrecognizedEndpoint(t *testing.T) {
	keys, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	uaPriv, _ := ecdh.P256().GenerateKey(rand.Reader)
	authSecret := make([]byte, 16)

	sender := NewSender(keys, "mailto:admin@example.com", nil)
	baseSub := Subscription{
		Keys: SubscriptionKeys{
			P256DH: base64.RawURLEncoding.EncodeToString(uaPriv.PublicKey().Bytes()),
			Auth:   base64.RawURLEncoding.EncodeToString(authSecret),
		},
	}

	for _, endpoint := range []string{
		"http://fcm.googleapis.com/fcm/send/abc",   // not https
		"https://internal.metadata.local/latest",   // not an allowlisted push host
		"https://fcm.googleapis.com.evil.com/send", // suffix trick, not the real host
	} {
		sub := baseSub
		sub.Endpoint = endpoint
		err := sender.Send(context.Background(), sub, []byte("test"), 60)
		if !errors.Is(err, ErrInvalidSubscription) {
			t.Fatalf("endpoint %q: expected ErrInvalidSubscription, got %v", endpoint, err)
		}
	}
}

func TestIsKnownPushHostAcceptsStandardServices(t *testing.T) {
	accepted := []string{
		"https://fcm.googleapis.com/fcm/send/abc",
		"https://updates.push.services.mozilla.com/wpush/v2/abc",
		"https://web.push.apple.com/abc",
		"https://channel123.notify.windows.com/abc",
	}
	for _, raw := range accepted {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if !isKnownPushHost(u) {
			t.Errorf("expected %q to be accepted", raw)
		}
	}
}
