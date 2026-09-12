package signaling

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"ipv6call/internal/rooms"
)

// dialPeer opens a signaling connection and completes the join handshake,
// returning the connection and the "joined" message payload.
func dialPeer(t *testing.T, wsURL, origin, room, secret, clientID string) (*websocket.Conn, map[string]any) {
	t.Helper()
	header := http.Header{}
	header.Set("Origin", origin)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial %s: %v", clientID, err)
	}

	if err := conn.WriteJSON(Message{Type: "join", Room: room, Payload: mustJSON(JoinPayload{
		Secret:   secret,
		ClientID: clientID,
	})}); err != nil {
		t.Fatalf("%s: write join: %v", clientID, err)
	}

	msg := readMessage(t, conn, "joined")
	var payload map[string]any
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("%s: decode joined payload: %v", clientID, err)
	}
	return conn, payload
}

// readMessage reads the next message and fails the test unless it has
// wantType, skipping any "pong"/"peer-ready" noise isn't attempted here: the
// flow below is sequenced so each read has exactly one expected message.
func readMessage(t *testing.T, conn *websocket.Conn, wantType string) Message {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var msg Message
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read message (want %q): %v", wantType, err)
	}
	if msg.Type != wantType {
		t.Fatalf("message type = %q, want %q (payload=%s)", msg.Type, wantType, string(msg.Payload))
	}
	return msg
}

func TestSignalingEndToEndJoinOfferAnswerHangup(t *testing.T) {
	manager := rooms.NewManager(time.Hour, 45*time.Second, 0)
	handler := NewHandler(manager, false)
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/room"
	origin := "http://" + strings.TrimPrefix(server.URL, "http://")

	room, secret, err := manager.Create()
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	callerConn, callerJoined := dialPeer(t, wsURL, origin, room.ID, secret, "caller")
	defer callerConn.Close()
	if callerJoined["role"] != "caller" {
		t.Fatalf("caller role = %v, want caller", callerJoined["role"])
	}

	calleeConn, calleeJoined := dialPeer(t, wsURL, origin, room.ID, secret, "callee")
	defer calleeConn.Close()
	if calleeJoined["role"] != "callee" {
		t.Fatalf("callee role = %v, want callee", calleeJoined["role"])
	}

	// Both participants are now present: "peer-ready" is broadcast to both,
	// and the caller is expected to kick off the offer.
	readMessage(t, callerConn, "peer-ready")
	readMessage(t, calleeConn, "peer-ready")

	offerSDP := "v=0\r\no=- 0 0 IN IP4 127.0.0.1\r\ns=-\r\n"
	if err := callerConn.WriteJSON(Message{Type: "offer", Room: room.ID, Payload: mustJSON(SessionDescription{
		Type: "offer", SDP: offerSDP,
	})}); err != nil {
		t.Fatalf("caller: write offer: %v", err)
	}

	offerMsg := readMessage(t, calleeConn, "offer")
	var relayedOffer SessionDescription
	if err := json.Unmarshal(offerMsg.Payload, &relayedOffer); err != nil {
		t.Fatalf("decode relayed offer: %v", err)
	}
	if relayedOffer.Type != "offer" || relayedOffer.SDP != offerSDP {
		t.Fatalf("relayed offer = %+v, want type=offer sdp=%q", relayedOffer, offerSDP)
	}

	answerSDP := "v=0\r\no=- 1 0 IN IP4 127.0.0.1\r\ns=-\r\n"
	if err := calleeConn.WriteJSON(Message{Type: "answer", Room: room.ID, Payload: mustJSON(SessionDescription{
		Type: "answer", SDP: answerSDP,
	})}); err != nil {
		t.Fatalf("callee: write answer: %v", err)
	}

	answerMsg := readMessage(t, callerConn, "answer")
	var relayedAnswer SessionDescription
	if err := json.Unmarshal(answerMsg.Payload, &relayedAnswer); err != nil {
		t.Fatalf("decode relayed answer: %v", err)
	}
	if relayedAnswer.Type != "answer" || relayedAnswer.SDP != answerSDP {
		t.Fatalf("relayed answer = %+v, want type=answer sdp=%q", relayedAnswer, answerSDP)
	}

	// Hangup must reach the other participant even though the room is torn
	// down shortly after.
	if err := callerConn.WriteJSON(Message{Type: "hangup", Room: room.ID}); err != nil {
		t.Fatalf("caller: write hangup: %v", err)
	}
	readMessage(t, calleeConn, "hangup")

	if !waitFor(5*time.Second, func() bool {
		return !manager.Status(room.ID).Exists
	}) {
		t.Fatal("room was not cleaned up after hangup")
	}
}

func waitFor(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond()
}
