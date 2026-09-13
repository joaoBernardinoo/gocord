package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	ErrSubscriptionExpired = errors.New("push subscription has expired or is invalid (410 Gone)")
	ErrInvalidSubscription = errors.New("invalid push subscription endpoint or keys")
	ErrPushServiceFailed   = errors.New("push service delivery failed")
	ErrVAPIDKeyMismatch    = errors.New("push service rejected VAPID key (public key mismatch)")
)

type SubscriptionKeys struct {
	P256DH string `json:"p256dh"`
	Auth   string `json:"auth"`
}

type Subscription struct {
	Endpoint       string           `json:"endpoint"`
	ExpirationTime *int64           `json:"expirationTime,omitempty"`
	Keys           SubscriptionKeys `json:"keys"`
}

type Sender struct {
	vapidKeys     *VAPIDKeys
	subject       string
	client        *http.Client
	allowEndpoint func(*url.URL) bool
}

func NewSender(keys *VAPIDKeys, subject string, client *http.Client) *Sender {
	if client == nil {
		client = &http.Client{
			Timeout: 10 * time.Second,
		}
	}
	if subject == "" {
		subject = "mailto:admin@localhost"
	}
	return &Sender{
		vapidKeys:     keys,
		subject:       subject,
		client:        client,
		allowEndpoint: isKnownPushHost,
	}
}

var knownPushHosts = map[string]bool{
	"fcm.googleapis.com":                true,
	"updates.push.services.mozilla.com": true,
	"web.push.apple.com":                true,
}

func isKnownPushHost(u *url.URL) bool {
	if u.Scheme != "https" {
		return false
	}
	host := u.Hostname()
	if knownPushHosts[host] {
		return true
	}
	return strings.HasSuffix(host, ".notify.windows.com")
}

type autopushError struct {
	Errno   int    `json:"errno"`
	Message string `json:"message"`
}

const autopushErrnoVAPIDKeyMismatch = 109

func isVAPIDKeyMismatch(status int, body []byte) bool {
	if status != http.StatusUnauthorized && status != http.StatusForbidden {
		return false
	}
	var perr autopushError
	if err := json.Unmarshal(body, &perr); err == nil && perr.Errno == autopushErrnoVAPIDKeyMismatch {
		return true
	}
	text := string(body)
	return strings.Contains(text, "VapidPkHashMismatch") || strings.Contains(text, "VAPID public key mismatch")
}

func (s *Sender) SetAllowEndpoint(fn func(*url.URL) bool) {
	s.allowEndpoint = fn
}

func (s *Sender) VAPIDPublicKey() string {
	if s.vapidKeys == nil {
		return ""
	}
	return s.vapidKeys.PublicKeyBase64()
}

// Send sends an encrypted Web Push notification to the given subscription.
func (s *Sender) Send(ctx context.Context, sub Subscription, payload []byte, ttlSeconds int) error {
	if strings.TrimSpace(sub.Endpoint) == "" {
		return fmt.Errorf("%w: missing endpoint", ErrInvalidSubscription)
	}

	endpointURL, err := url.Parse(sub.Endpoint)
	if err != nil || !s.allowEndpoint(endpointURL) {
		return fmt.Errorf("%w: endpoint is not a recognized push service", ErrInvalidSubscription)
	}

	uaPubBytes, err := DecodeBase64Flexible(sub.Keys.P256DH)
	if err != nil || len(uaPubBytes) != 65 {
		return fmt.Errorf("%w: invalid p256dh key", ErrInvalidSubscription)
	}

	authBytes, err := DecodeBase64Flexible(sub.Keys.Auth)
	if err != nil || len(authBytes) != 16 {
		return fmt.Errorf("%w: invalid auth secret", ErrInvalidSubscription)
	}

	var bodyReader io.Reader
	var encrypted []byte
	if len(payload) > 0 {
		encrypted, err = EncryptPayload(uaPubBytes, authBytes, payload)
		if err != nil {
			return fmt.Errorf("encrypt push payload: %w", err)
		}
		bodyReader = bytes.NewReader(encrypted)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bodyReader)
	if err != nil {
		return fmt.Errorf("create push request: %w", err)
	}

	if ttlSeconds <= 0 {
		ttlSeconds = 60
	}
	req.Header.Set("TTL", strconv.Itoa(ttlSeconds))
	req.Header.Set("Urgency", "high")

	if len(encrypted) > 0 {
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("Content-Encoding", "aes128gcm")
	}

	if s.vapidKeys != nil {
		audience := fmt.Sprintf("%s://%s", endpointURL.Scheme, endpointURL.Host)
		token, err := s.vapidKeys.SignJWT(audience, s.subject, time.Now().Add(12*time.Hour))
		if err != nil {
			return fmt.Errorf("sign vapid jwt: %w", err)
		}
		req.Header.Set("Authorization", fmt.Sprintf("vapid t=%s, k=%s", token, s.vapidKeys.PublicKeyBase64()))
		req.Header.Set("Crypto-Key", fmt.Sprintf("p256ecdsa=%s", s.vapidKeys.PublicKeyBase64()))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: request error", ErrPushServiceFailed)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent:
		return nil
	case http.StatusNotFound, http.StatusGone:
		return ErrSubscriptionExpired
	default:
		msg := strings.TrimSpace(string(respBody))
		if isVAPIDKeyMismatch(resp.StatusCode, respBody) {
			if msg != "" {
				return fmt.Errorf("%w: status %d (%s)", ErrVAPIDKeyMismatch, resp.StatusCode, msg)
			}
			return fmt.Errorf("%w: status %d", ErrVAPIDKeyMismatch, resp.StatusCode)
		}
		if msg != "" {
			return fmt.Errorf("%w: status %d (%s)", ErrPushServiceFailed, resp.StatusCode, msg)
		}
		return fmt.Errorf("%w: status %d", ErrPushServiceFailed, resp.StatusCode)
	}
}
