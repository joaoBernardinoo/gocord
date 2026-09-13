package signaling

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSameOriginRequiresMatchingSchemeAndHost(t *testing.T) {
	h := &Handler{trustProxyHeaders: false}

	newReq := func(origin, host string, isTLS bool, forwardedProto string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "http://"+host+"/ws/room1", nil)
		r.Host = host
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if forwardedProto != "" {
			r.Header.Set("X-Forwarded-Proto", forwardedProto)
		}
		if isTLS {
			r.TLS = &tls.ConnectionState{}
		}
		return r
	}

	if h.sameOrigin(newReq("", "call.example.com", false, "")) {
		t.Fatal("missing Origin header must be rejected")
	}
	if !h.sameOrigin(newReq("http://call.example.com", "call.example.com", false, "")) {
		t.Fatal("matching http origin over a plain request should be accepted")
	}
	if h.sameOrigin(newReq("https://call.example.com", "call.example.com", false, "")) {
		t.Fatal("https origin over a non-TLS request must be rejected")
	}
	if !h.sameOrigin(newReq("https://call.example.com", "call.example.com", true, "")) {
		t.Fatal("https origin over a TLS request should be accepted")
	}
	if h.sameOrigin(newReq("http://evil.example.com", "call.example.com", false, "")) {
		t.Fatal("mismatched host must be rejected")
	}

	untrusted := &Handler{trustProxyHeaders: false}
	if untrusted.sameOrigin(newReq("https://call.example.com", "call.example.com", false, "https")) {
		t.Fatal("X-Forwarded-Proto must be ignored when proxy headers are not trusted")
	}
	trusted := &Handler{trustProxyHeaders: true}
	if !trusted.sameOrigin(newReq("https://call.example.com", "call.example.com", false, "https")) {
		t.Fatal("X-Forwarded-Proto should be honored when proxy headers are trusted")
	}
}
