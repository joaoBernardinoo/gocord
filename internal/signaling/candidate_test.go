package signaling

import (
	"strings"
	"testing"
)

func TestIsValidCandidate(t *testing.T) {
	valid := []string{
		"candidate:1 1 udp 2122260223 2001:db8::10 50000 typ host",
		"candidate:2 1 udp 1686052607 fd00::1234 3478 typ relay",
		"candidate:1 1 udp 2122260223 192.0.2.10 50000 typ host",
	}
	for _, candidate := range valid {
		if !isValidCandidate(candidate) {
			t.Fatalf("expected valid candidate: %s", candidate)
		}
	}

	invalid := []string{
		"candidate:1 1 udp 2122260223 notanip 50000 typ host",
		"not-a-candidate",
	}
	for _, candidate := range invalid {
		if isValidCandidate(candidate) {
			t.Fatalf("expected rejected candidate: %s", candidate)
		}
	}
}

func TestSanitizeSDPKeepsValidCandidates(t *testing.T) {
	sdp := strings.Join([]string{
		"v=0",
		"c=IN IP4 0.0.0.0",
		"a=candidate:1 1 udp 1 192.0.2.10 50000 typ host",
		"a=candidate:2 1 udp 1 2001:db8::10 50001 typ host",
		"a=end-of-candidates",
		"",
	}, "\r\n")

	got := sanitizeSDP(sdp)
	if !strings.Contains(got, "192.0.2.10") {
		t.Fatalf("IPv4 candidate was unexpectedly removed: %q", got)
	}
	if !strings.Contains(got, "2001:db8::10") {
		t.Fatalf("IPv6 candidate was removed: %q", got)
	}
}
