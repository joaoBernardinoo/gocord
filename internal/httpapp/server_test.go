package httpapp

import (
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"ipv6call/internal/config"
	"ipv6call/internal/push"
	"ipv6call/internal/rooms"
	"ipv6call/internal/signaling"
)

func newTestApp(t *testing.T, mutate func(*config.Config)) (*App, http.Handler) {
	t.Helper()
	cfg := config.Config{
		PublicBaseURL:     "https://call.example.com",
		RoomTTL:           time.Hour,
		EmptyRoomGrace:    time.Minute,
		MaxRooms:          100,
		RoomCreateRate:    0, // limiting off unless a test asks for it
		RoomCreateBurst:   5,
		PushRate:          0, // limiting off unless a test asks for it
		PushBurst:         5,
		TURNCredentialTTL: time.Hour,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	if cfg.VAPIDKeys == nil {
		keys, err := push.GenerateVAPIDKeys()
		if err != nil {
			t.Fatalf("generate vapid keys: %v", err)
		}
		cfg.VAPIDKeys = keys
		cfg.VAPIDPublicKey = keys.PublicKeyBase64()
		cfg.VAPIDPrivateKey = keys.PrivateKeyBase64()
	}
	manager := rooms.NewManager(cfg.RoomTTL, cfg.EmptyRoomGrace, cfg.MaxRooms)
	app, err := New(cfg, manager, signaling.NewHandler(manager, cfg.TrustProxyHeaders))
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	return app, app.Handler()
}

func do(t *testing.T, handler http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestCreateRoomReturnsInviteURL(t *testing.T) {
	_, handler := newTestApp(t, nil)
	rec := do(t, handler, http.MethodPost, "/api/rooms")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}

	var body struct {
		Room   string `json:"room"`
		Secret string `json:"secret"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Room == "" || body.Secret == "" {
		t.Fatalf("empty room or secret: %+v", body)
	}
	// The secret must ride in the fragment so it is never sent to the server.
	want := "https://call.example.com/join/" + body.Room + "#" + body.Secret
	if body.URL != want {
		t.Fatalf("url = %q, want %q", body.URL, want)
	}
}

func TestCreateRoomRateLimited(t *testing.T) {
	_, handler := newTestApp(t, func(c *config.Config) {
		c.RoomCreateRate = 0.01
		c.RoomCreateBurst = 2
	})
	for i := 0; i < 2; i++ {
		if rec := do(t, handler, http.MethodPost, "/api/rooms"); rec.Code != http.StatusCreated {
			t.Fatalf("request %d: status = %d, want 201", i, rec.Code)
		}
	}
	rec := do(t, handler, http.MethodPost, "/api/rooms")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("429 response should carry Retry-After")
	}
}

func TestCreateRoomAtCapacityReturns503(t *testing.T) {
	_, handler := newTestApp(t, func(c *config.Config) { c.MaxRooms = 1 })
	if rec := do(t, handler, http.MethodPost, "/api/rooms"); rec.Code != http.StatusCreated {
		t.Fatalf("first create: status = %d", rec.Code)
	}
	rec := do(t, handler, http.MethodPost, "/api/rooms")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestRoomStatusHidesUnknownRooms(t *testing.T) {
	_, handler := newTestApp(t, nil)
	rec := do(t, handler, http.MethodGet, "/api/rooms/does-not-exist")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestClientConfigIssuesTURNCredentials(t *testing.T) {
	const secret = "shared-secret"
	_, handler := newTestApp(t, func(c *config.Config) {
		c.STUNURLs = []string{"stun:turn.example.com:3478"}
		c.TURNURLs = []string{"turn:turn.example.com:3478?transport=udp"}
		c.TURNSharedSecret = secret
	})

	rec := do(t, handler, http.MethodGet, "/api/config")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatal("the TURN shared secret must never reach the browser")
	}

	var body struct {
		ICEServers []config.ICEServer `json:"iceServers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	var turn *config.ICEServer
	for i := range body.ICEServers {
		if body.ICEServers[i].Username != "" {
			turn = &body.ICEServers[i]
		}
	}
	if turn == nil {
		t.Fatal("no TURN server with credentials in response")
	}
	// The credential must be a coturn REST HMAC over the ephemeral username.
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write([]byte(turn.Username))
	if want := base64.StdEncoding.EncodeToString(mac.Sum(nil)); turn.Credential != want {
		t.Fatalf("credential = %q, want %q", turn.Credential, want)
	}
	if _, _, ok := strings.Cut(turn.Username, ":"); !ok {
		t.Fatalf("username %q should be <expiry>:<nonce>", turn.Username)
	}
}

func TestIndexServedForRoots(t *testing.T) {
	_, handler := newTestApp(t, nil)
	for _, path := range []string{"/", "/friends", "/contacts", "/join/abc123"} {
		rec := do(t, handler, http.MethodGet, path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("%s: content-type = %q", path, ct)
		}
	}
	if rec := do(t, handler, http.MethodGet, "/nope"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown path: status = %d, want 404", rec.Code)
	}
}

// The frontend carries no inline scripts or styles, so the CSP must not need to
// allow either. A regression here silently breaks whatever went inline.
func TestSecurityHeadersDisallowInlineCode(t *testing.T) {
	_, handler := newTestApp(t, nil)
	csp := do(t, handler, http.MethodGet, "/").Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("missing Content-Security-Policy")
	}
	for _, forbidden := range []string{"unsafe-inline", "unsafe-eval", "sha256-"} {
		if strings.Contains(csp, forbidden) {
			t.Errorf("CSP should not contain %q: %s", forbidden, csp)
		}
	}
	for _, required := range []string{"default-src 'self'", "object-src 'none'", "frame-ancestors 'none'"} {
		if !strings.Contains(csp, required) {
			t.Errorf("CSP missing %q: %s", required, csp)
		}
	}
}

func TestHSTSOnlyOverHTTPS(t *testing.T) {
	_, handler := newTestApp(t, nil)

	if got := do(t, handler, http.MethodGet, "/").Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("plain HTTP should not get HSTS, got %q", got)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	handler.ServeHTTP(rec, req)
	if got := rec.Header().Get("Strict-Transport-Security"); !strings.Contains(got, "max-age=") {
		t.Errorf("proxied HTTPS should get HSTS, got %q", got)
	}
}

func TestClientKeyIgnoresForwardedHeaderUnlessTrusted(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/rooms", nil)
	req.RemoteAddr = "[2001:db8::1]:44321"
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")

	// Untrusted: a spoofed header must not let a client mint fresh buckets.
	if got := clientKey(req, false); got != "2001:db8::1" {
		t.Errorf("untrusted key = %q, want the socket address", got)
	}
	// Trusted: the rightmost entry is the one our own proxy appended.
	if got := clientKey(req, true); got != "5.6.7.8" {
		t.Errorf("trusted key = %q, want 5.6.7.8", got)
	}
}

func TestRateLimiterRefillsOverTime(t *testing.T) {
	limiter := newRateLimiter(1, 1)
	now := time.Unix(1_700_000_000, 0)
	limiter.now = func() time.Time { return now }

	if !limiter.allow("k") {
		t.Fatal("first request should be allowed")
	}
	if limiter.allow("k") {
		t.Fatal("burst of 1 should be exhausted")
	}
	now = now.Add(2 * time.Second)
	if !limiter.allow("k") {
		t.Fatal("bucket should have refilled")
	}
}

func TestClientConfigIncludesVAPIDPublicKey(t *testing.T) {
	_, handler := newTestApp(t, nil)
	rec := do(t, handler, http.MethodGet, "/api/config")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body struct {
		VAPIDPublicKey string `json:"vapidPublicKey"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.VAPIDPublicKey == "" {
		t.Fatal("expected non-empty vapidPublicKey in /api/config")
	}
}

func TestFaviconAndAssetsServed(t *testing.T) {
	_, handler := newTestApp(t, nil)
	recFavicon := do(t, handler, http.MethodGet, "/favicon.ico")
	if recFavicon.Code != http.StatusMovedPermanently {
		t.Fatalf("/favicon.ico status = %d, want 301", recFavicon.Code)
	}

	for _, asset := range []string{"/assets/favicon.ico", "/assets/favicon.png", "/assets/bg-removed-logo.webp", "/assets/bg-removed-logo.png", "/assets/styles.css", "/assets/app.js"} {
		recAsset := do(t, handler, http.MethodGet, asset)
		if recAsset.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", asset, recAsset.Code)
		}
		if recAsset.Body.Len() == 0 {
			t.Fatalf("%s returned empty body", asset)
		}
	}
}

func TestOpenGraphAndTwitterTags(t *testing.T) {
	_, handler := newTestApp(t, nil)
	rec := do(t, handler, http.MethodGet, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	requiredTags := []string{
		`property="og:title"`,
		`property="og:description"`,
		`property="og:image" content="https://call.example.com/assets/favicon.png"`,
		`property="og:type" content="website"`,
		`name="twitter:card" content="summary"`,
		`name="twitter:title"`,
		`name="twitter:description"`,
		`name="twitter:image" content="https://call.example.com/assets/favicon.png"`,
	}
	for _, tag := range requiredTags {
		if !strings.Contains(body, tag) {
			t.Errorf("served HTML missing expected meta tag: %s", tag)
		}
	}
}

func TestServiceWorkerEndpoint(t *testing.T) {
	_, handler := newTestApp(t, nil)
	for _, path := range []string{"/sw.js", "/assets/sw.js"} {
		rec := do(t, handler, http.MethodGet, path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") {
			t.Errorf("%s: content-type = %q, want application/javascript", path, ct)
		}
		if swAllowed := rec.Header().Get("Service-Worker-Allowed"); swAllowed != "/" {
			t.Errorf("%s: Service-Worker-Allowed = %q, want /", path, swAllowed)
		}
		if !strings.Contains(rec.Body.String(), "addEventListener") {
			t.Errorf("%s: unexpected sw.js content", path)
		}
	}
}

func TestPushNotifySuccessAndExpired(t *testing.T) {
	var pushReceived bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "expired") {
			w.WriteHeader(http.StatusGone)
			return
		}
		pushReceived = true
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()

	app, handler := newTestApp(t, nil)
	sender := push.NewSender(app.cfg.VAPIDKeys, app.cfg.VAPIDSubject, ts.Client())
	sender.SetAllowEndpoint(func(*url.URL) bool { return true })
	app.pushSender = sender

	uaPriv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authBytes := make([]byte, 16)
	rand.Read(authBytes)

	// Valid push notification
	reqBody := map[string]any{
		"subscription": map[string]any{
			"endpoint": ts.URL + "/push",
			"keys": map[string]string{
				"p256dh": base64.RawURLEncoding.EncodeToString(uaPriv.PublicKey().Bytes()),
				"auth":   base64.RawURLEncoding.EncodeToString(authBytes),
			},
		},
		"payload": map[string]string{
			"title": "Incoming Call",
			"url":   "https://call.example.com/join/room1#secret",
		},
		"ttl": 60,
	}
	bodyJSON, _ := json.Marshal(reqBody)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/push/notify", strings.NewReader(string(bodyJSON)))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("push status = %d, want 200: body=%s", rec.Code, rec.Body.String())
	}
	if !pushReceived {
		t.Fatal("mock push server did not receive notification")
	}

	// Expired push subscription
	reqBody["subscription"].(map[string]any)["endpoint"] = ts.URL + "/expired"
	bodyJSON, _ = json.Marshal(reqBody)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/push/notify", strings.NewReader(string(bodyJSON)))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("expired push status = %d, want 410 Gone", rec.Code)
	}
}

func TestPushNotifyInvalidInput(t *testing.T) {
	_, handler := newTestApp(t, nil)

	// Missing keys
	badReq := `{"subscription":{"endpoint":"https://fcm.googleapis.com/test"}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/push/notify", strings.NewReader(badReq))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestPushNotifyRateLimited(t *testing.T) {
	_, handler := newTestApp(t, func(c *config.Config) {
		c.PushRate = 0.01
		c.PushBurst = 1
	})

	uaPriv, _ := ecdh.P256().GenerateKey(rand.Reader)
	authBytes := make([]byte, 16)
	rand.Read(authBytes)

	reqBodyMap := map[string]any{
		"subscription": map[string]any{
			"endpoint": "https://fcm.googleapis.com/test",
			"keys": map[string]string{
				"p256dh": base64.RawURLEncoding.EncodeToString(uaPriv.PublicKey().Bytes()),
				"auth":   base64.RawURLEncoding.EncodeToString(authBytes),
			},
		},
	}
	reqBody, _ := json.Marshal(reqBodyMap)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/push/notify", strings.NewReader(string(reqBody)))
	handler.ServeHTTP(rec, req)
	// First request should not be 429
	if rec.Code == http.StatusTooManyRequests {
		t.Fatalf("first request should not be rate limited")
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/push/notify", strings.NewReader(string(reqBody)))
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want 429", rec2.Code)
	}
}


