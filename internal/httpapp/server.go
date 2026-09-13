package httpapp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ipv6call/internal/config"
	"ipv6call/internal/push"
	"ipv6call/internal/rooms"
	"ipv6call/internal/signaling"
	webassets "ipv6call/web"
)

type App struct {
	cfg         config.Config
	rooms       *rooms.Manager
	signaling   *signaling.Handler
	assets      http.Handler
	index       []byte
	styles      []byte
	appJS       []byte
	sw          []byte
	baseURL     *url.URL
	createLimit *rateLimiter
	pushLimit   *rateLimiter
	pushSender  *push.Sender
}

func New(cfg config.Config, manager *rooms.Manager, signalingHandler *signaling.Handler) (*App, error) {
	index := []byte(webassets.IndexHTML)
	if cfg.PublicBaseURL != "" && !strings.Contains(cfg.PublicBaseURL, "[::1]") {
		index = bytes.ReplaceAll(index, []byte(`content="/assets/favicon.png"`), []byte(fmt.Sprintf(`content="%s/assets/favicon.png"`, cfg.PublicBaseURL)))
	}
	sw := []byte(webassets.ServiceWorkerJS)
	styles := []byte(webassets.StylesCSS)
	appJS := []byte(webassets.AppJS)
	baseURL, err := url.Parse(cfg.PublicBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse PUBLIC_BASE_URL: %w", err)
	}
	assetsFS, err := fs.Sub(webassets.Files, "assets")
	if err != nil {
		return nil, fmt.Errorf("sub embedded assets: %w", err)
	}
	return &App{
		cfg:         cfg,
		rooms:       manager,
		signaling:   signalingHandler,
		assets:      cacheControlAssets(http.FileServer(http.FS(assetsFS))),
		index:       index,
		styles:      styles,
		appJS:       appJS,
		sw:          sw,
		baseURL:     baseURL,
		createLimit: newRateLimiter(cfg.RoomCreateRate, cfg.RoomCreateBurst),
		pushLimit:   newRateLimiter(cfg.PushRate, cfg.PushBurst),
		pushSender:  push.NewSender(cfg.VAPIDKeys, cfg.VAPIDSubject, nil),
	}, nil
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/rooms", a.createRoom)
	mux.HandleFunc("GET /api/rooms/{room}", a.roomStatus)
	mux.HandleFunc("GET /api/config", a.clientConfig)
	mux.HandleFunc("POST /api/push/notify", a.pushNotify)
	mux.HandleFunc("GET /assets/styles.css", a.stylesCSS)
	mux.HandleFunc("GET /assets/app.js", a.appScript)
	mux.HandleFunc("GET /sw.js", a.serviceWorker)
	mux.HandleFunc("GET /assets/sw.js", a.serviceWorker)
	mux.Handle("GET /ws/", a.signaling)
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", a.assets))
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/assets/favicon.ico", http.StatusMovedPermanently)
	})
	mux.HandleFunc("GET /join/{room}", a.indexPage)
	mux.HandleFunc("GET /friends", a.indexPage)
	mux.HandleFunc("GET /contacts", a.indexPage)
	mux.HandleFunc("GET /", a.indexPage)
	return securityHeaders(mux)
}

func (a *App) createRoom(w http.ResponseWriter, r *http.Request) {
	// Room creation is unauthenticated, so it is the one endpoint that can grow
	// server state without bound. Limit per client, and let the manager's cap
	// backstop a distributed attempt.
	if !a.createLimit.allow(clientKey(r, a.cfg.TrustProxyHeaders)) {
		w.Header().Set("Retry-After", "60")
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many rooms created, slow down"})
		return
	}

	room, secret, err := a.rooms.Create()
	if errors.Is(err, rooms.ErrAtCapacity) {
		w.Header().Set("Retry-After", "120")
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "server is at capacity, try again shortly"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create room"})
		return
	}
	inviteURL := a.baseURL.JoinPath("join", room.ID)
	inviteURL.Fragment = secret
	writeJSON(w, http.StatusCreated, map[string]any{
		"room":      room.ID,
		"secret":    secret,
		"url":       inviteURL.String(),
		"expiresAt": room.ExpiresAt,
	})
}

func (a *App) roomStatus(w http.ResponseWriter, r *http.Request) {
	status := a.rooms.Status(r.PathValue("room"))
	if !status.Exists {
		writeJSON(w, http.StatusNotFound, map[string]any{"exists": false})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (a *App) clientConfig(w http.ResponseWriter, r *http.Request) {
	iceServers, err := a.cfg.ClientICEServers(time.Now())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not generate ICE configuration"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"iceServers":     iceServers,
		"ipv6Only":       false,
		"devDiagnostics": a.cfg.DevDiagnostics,
		"vapidPublicKey": a.pushSender.VAPIDPublicKey(),
	})
}

func (a *App) pushNotify(w http.ResponseWriter, r *http.Request) {
	if !a.pushLimit.allow(clientKey(r, a.cfg.TrustProxyHeaders)) {
		w.Header().Set("Retry-After", "60")
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many push notifications, slow down"})
		return
	}

	var req struct {
		Subscription push.Subscription `json:"subscription"`
		Payload      json.RawMessage   `json:"payload"`
		TTL          int               `json:"ttl,omitempty"`
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if strings.TrimSpace(req.Subscription.Endpoint) == "" ||
		strings.TrimSpace(req.Subscription.Keys.P256DH) == "" ||
		strings.TrimSpace(req.Subscription.Keys.Auth) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "subscription missing endpoint or keys"})
		return
	}

	err := a.pushSender.Send(r.Context(), req.Subscription, []byte(req.Payload), req.TTL)
	if errors.Is(err, push.ErrSubscriptionExpired) {
		writeJSON(w, http.StatusGone, map[string]string{"error": "subscription has expired"})
		return
	}
	if errors.Is(err, push.ErrInvalidSubscription) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid subscription parameters"})
		return
	}
	if errors.Is(err, push.ErrVAPIDKeyMismatch) {
		writeJSON(w, http.StatusGone, map[string]string{"error": "contact's Call Card was created with a different server key. Ask them to click 'Refresh' and send an updated Call Card."})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) stylesCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(a.styles)
}

func (a *App) appScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(a.appJS)
}

func (a *App) serviceWorker(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Service-Worker-Allowed", "/")
	_, _ = w.Write(a.sw)
}

func (a *App) indexPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/friends" && r.URL.Path != "/contacts" && !strings.HasPrefix(r.URL.Path, "/join/") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(a.index)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No inline scripts or styles exist in the frontend, so neither
		// 'unsafe-inline' nor a script hash is needed.
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; media-src 'self' blob:; connect-src 'self' ws: wss:; worker-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("Permissions-Policy", "camera=(self), microphone=(self), fullscreen=(self)")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func cacheControlAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		next.ServeHTTP(w, r)
	})
}


