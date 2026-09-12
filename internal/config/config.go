package config

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"ipv6call/internal/push"
)

type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

type Config struct {
	ListenAddr        string
	PublicBaseURL     string
	RoomTTL           time.Duration
	EmptyRoomGrace    time.Duration
	ShutdownTimeout   time.Duration
	DevDiagnostics    bool
	MaxRooms          int
	RoomCreateRate    float64
	RoomCreateBurst   float64
	PushRate          float64
	PushBurst         float64
	TrustProxyHeaders bool
	StaticICEServers  []ICEServer
	STUNURLs          []string
	TURNURLs          []string
	TURNSharedSecret  string
	TURNCredentialTTL time.Duration
	VAPIDPublicKey    string
	VAPIDPrivateKey   string
	VAPIDSubject      string
	VAPIDKeys         *push.VAPIDKeys
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddr:        envOr("LISTEN_ADDR", "[::]:8080"),
		PublicBaseURL:     strings.TrimRight(envOr("PUBLIC_BASE_URL", "http://[::1]:8080"), "/"),
		RoomTTL:           durationOr("ROOM_TTL", 2*time.Hour),
		EmptyRoomGrace:    durationOr("EMPTY_ROOM_GRACE", 45*time.Second),
		ShutdownTimeout:   durationOr("SHUTDOWN_TIMEOUT", 10*time.Second),
		DevDiagnostics:    boolOr("DEV_DIAGNOSTICS", false),
		MaxRooms:          intOr("MAX_ROOMS", 5000),
		RoomCreateRate:    floatOr("ROOM_CREATE_RATE", 0.2),
		RoomCreateBurst:   floatOr("ROOM_CREATE_BURST", 5),
		PushRate:          floatOr("PUSH_RATE", 2.0),
		PushBurst:         floatOr("PUSH_BURST", 10),
		TrustProxyHeaders: boolOr("TRUST_PROXY_HEADERS", false),
		STUNURLs:          csvEnv("STUN_URLS"),
		TURNURLs:          csvEnv("TURN_URLS"),
		TURNSharedSecret:  strings.TrimSpace(os.Getenv("TURN_SHARED_SECRET")),
		TURNCredentialTTL: durationOr("TURN_CREDENTIAL_TTL", time.Hour),
		VAPIDPublicKey:    strings.TrimSpace(os.Getenv("VAPID_PUBLIC_KEY")),
		VAPIDPrivateKey:   strings.TrimSpace(os.Getenv("VAPID_PRIVATE_KEY")),
		VAPIDSubject:      strings.TrimSpace(os.Getenv("VAPID_SUBJECT")),
	}

	if cfg.VAPIDSubject == "" {
		if strings.HasPrefix(cfg.PublicBaseURL, "http") {
			cfg.VAPIDSubject = cfg.PublicBaseURL
		} else {
			cfg.VAPIDSubject = "mailto:admin@sslip.io"
		}
	}

	if cfg.VAPIDPrivateKey != "" {
		keys, err := push.LoadVAPIDKeys(cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey)
		if err != nil {
			return Config{}, fmt.Errorf("load VAPID keys: %w", err)
		}
		cfg.VAPIDKeys = keys
		cfg.VAPIDPublicKey = keys.PublicKeyBase64()
		cfg.VAPIDPrivateKey = keys.PrivateKeyBase64()
	} else {
		keyFile := envOr("VAPID_KEYS_FILE", "vapid_keys.json")
		var loadedFromFile bool
		if fileData, err := os.ReadFile(keyFile); err == nil {
			var saved struct {
				PublicKey  string `json:"publicKey"`
				PrivateKey string `json:"privateKey"`
			}
			if err := json.Unmarshal(fileData, &saved); err == nil && saved.PrivateKey != "" {
				if keys, err := push.LoadVAPIDKeys(saved.PublicKey, saved.PrivateKey); err == nil {
					cfg.VAPIDKeys = keys
					cfg.VAPIDPublicKey = keys.PublicKeyBase64()
					cfg.VAPIDPrivateKey = keys.PrivateKeyBase64()
					loadedFromFile = true
				}
			}
		}

		if !loadedFromFile {
			keys, err := push.GenerateVAPIDKeys()
			if err != nil {
				return Config{}, fmt.Errorf("generate VAPID keys: %w", err)
			}
			cfg.VAPIDKeys = keys
			cfg.VAPIDPublicKey = keys.PublicKeyBase64()
			cfg.VAPIDPrivateKey = keys.PrivateKeyBase64()

			savedJSON, _ := json.MarshalIndent(map[string]string{
				"publicKey":  cfg.VAPIDPublicKey,
				"privateKey": cfg.VAPIDPrivateKey,
			}, "", "  ")
			_ = os.WriteFile(keyFile, savedJSON, 0600)
		}
	}

	if raw := strings.TrimSpace(os.Getenv("ICE_SERVERS_JSON")); raw != "" {
		if err := json.Unmarshal([]byte(raw), &cfg.StaticICEServers); err != nil {
			return Config{}, fmt.Errorf("parse ICE_SERVERS_JSON: %w", err)
		}
	}

	if len(cfg.TURNURLs) > 0 && cfg.TURNSharedSecret == "" {
		return Config{}, fmt.Errorf("TURN_URLS is set but TURN_SHARED_SECRET is empty")
	}
	if cfg.RoomTTL <= 0 {
		return Config{}, fmt.Errorf("ROOM_TTL must be positive")
	}
	if cfg.EmptyRoomGrace < 0 {
		return Config{}, fmt.Errorf("EMPTY_ROOM_GRACE cannot be negative")
	}
	if cfg.MaxRooms < 0 {
		return Config{}, fmt.Errorf("MAX_ROOMS cannot be negative")
	}
	if cfg.RoomCreateRate < 0 {
		return Config{}, fmt.Errorf("ROOM_CREATE_RATE cannot be negative")
	}
	if cfg.RoomCreateRate > 0 && cfg.RoomCreateBurst < 1 {
		return Config{}, fmt.Errorf("ROOM_CREATE_BURST must be at least 1 when rate limiting is enabled")
	}
	if cfg.PushRate < 0 {
		return Config{}, fmt.Errorf("PUSH_RATE cannot be negative")
	}
	if cfg.PushRate > 0 && cfg.PushBurst < 1 {
		return Config{}, fmt.Errorf("PUSH_BURST must be at least 1 when push rate limiting is enabled")
	}

	return cfg, nil
}

func (c Config) ClientICEServers(now time.Time) ([]ICEServer, error) {
	servers := make([]ICEServer, 0, len(c.StaticICEServers)+2)
	servers = append(servers, c.StaticICEServers...)

	if len(c.STUNURLs) > 0 {
		servers = append(servers, ICEServer{URLs: append([]string(nil), c.STUNURLs...)})
	}

	if len(c.TURNURLs) > 0 {
		username, credential, err := turnCredentials(c.TURNSharedSecret, now.Add(c.TURNCredentialTTL))
		if err != nil {
			return nil, err
		}
		servers = append(servers, ICEServer{
			URLs:       append([]string(nil), c.TURNURLs...),
			Username:   username,
			Credential: credential,
		})
	}

	return servers, nil
}

func turnCredentials(secret string, expiry time.Time) (string, string, error) {
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", "", fmt.Errorf("generate TURN credential nonce: %w", err)
	}

	username := fmt.Sprintf("%d:%s", expiry.Unix(), base64.RawURLEncoding.EncodeToString(nonce[:]))
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write([]byte(username))
	credential := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return username, credential, nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationOr(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		slog.Warn("invalid duration env var, using default", "key", key, "value", value, "default", fallback)
		return fallback
	}
	return parsed
}

func boolOr(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		slog.Warn("invalid bool env var, using default", "key", key, "value", value, "default", fallback)
		return fallback
	}
	return parsed
}

func intOr(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		slog.Warn("invalid integer env var, using default", "key", key, "value", value, "default", fallback)
		return fallback
	}
	return parsed
}

func floatOr(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		slog.Warn("invalid float env var, using default", "key", key, "value", value, "default", fallback)
		return fallback
	}
	return parsed
}

func csvEnv(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}
