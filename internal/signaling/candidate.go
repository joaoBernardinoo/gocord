package signaling

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

func sanitizeMessage(msg Message) (Message, error) {
	switch msg.Type {
	case "ice-candidate":
		var candidate ICECandidate
		if err := json.Unmarshal(msg.Payload, &candidate); err != nil {
			return Message{}, fmt.Errorf("decode ICE candidate: %w", err)
		}
		if !isValidCandidate(candidate.Candidate) {
			return Message{}, fmt.Errorf("invalid ICE candidate")
		}
	case "offer", "answer":
		var description SessionDescription
		if err := json.Unmarshal(msg.Payload, &description); err != nil {
			return Message{}, fmt.Errorf("decode session description: %w", err)
		}
		if description.Type != msg.Type {
			return Message{}, fmt.Errorf("SDP type does not match message type")
		}
		description.SDP = sanitizeSDP(description.SDP)
		payload, err := json.Marshal(description)
		if err != nil {
			return Message{}, fmt.Errorf("encode sanitized session description: %w", err)
		}
		msg.Payload = payload
	default:
		return Message{}, fmt.Errorf("unsupported signaling message type %q", msg.Type)
	}
	return msg, nil
}

func sanitizeSDP(sdp string) string {
	separator := "\n"
	if strings.Contains(sdp, "\r\n") {
		separator = "\r\n"
	}
	lines := strings.Split(sdp, separator)
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "a=candidate:") && !isValidCandidate(strings.TrimPrefix(trimmed, "a=")) {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, separator)
}

func isValidCandidate(candidate string) bool {
	candidate = strings.TrimSpace(candidate)
	candidate = strings.TrimPrefix(candidate, "a=")
	fields := strings.Fields(candidate)
	if len(fields) < 6 || !strings.HasPrefix(fields[0], "candidate:") {
		return false
	}

	address := strings.Trim(fields[4], "[]")
	if zone := strings.LastIndex(address, "%"); zone >= 0 {
		address = address[:zone]
	}
	ip := net.ParseIP(address)
	return ip != nil
}
