package signaling

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"

	"ipv6call/internal/rooms"
)

const (
	maxMessageSize = 512 * 1024
	sendBufferSize = 64
	pongWait       = 60 * time.Second
	pingPeriod     = 25 * time.Second
	writeWait      = 10 * time.Second
)

type Handler struct {
	rooms    *rooms.Manager
	upgrader websocket.Upgrader
}

func NewHandler(manager *rooms.Manager) *Handler {
	h := &Handler{rooms: manager}
	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     sameOrigin,
	}
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slog.Debug("websocket upgrade requested")
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	conn.SetReadLimit(maxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	_, firstRaw, err := conn.ReadMessage()
	if err != nil {
		slog.Warn("failed to read initial join message", "error", err)
		return
	}
	var first Message
	if err := json.Unmarshal(firstRaw, &first); err != nil || first.Type != "join" || first.Room == "" {
		slog.Warn("invalid initial message", "error", err)
		_ = conn.WriteJSON(Message{Type: "error", Payload: mustJSON(ErrorPayload{Code: "bad_join", Message: "first message must be a valid join"})})
		return
	}
	var join JoinPayload
	if err := json.Unmarshal(first.Payload, &join); err != nil || join.Secret == "" || join.ClientID == "" || len(join.ClientID) > 128 {
		slog.Warn("invalid join payload", "error", err)
		_ = conn.WriteJSON(Message{Type: "error", Payload: mustJSON(ErrorPayload{Code: "bad_join", Message: "invalid join payload"})})
		return
	}

	peer := rooms.NewPeer(join.ClientID, sendBufferSize)
	role, participants, err := h.rooms.Join(first.Room, join.Secret, peer)
	if err != nil {
		slog.Warn("room join rejected", "error", err)
		code, message := joinError(err)
		_ = conn.WriteJSON(Message{Type: "error", Payload: mustJSON(ErrorPayload{Code: code, Message: message})})
		return
	}

	// Room and client identifiers are logged only under DEBUG_SIGNALING; the
	// default level records no data that links a room to its participants.
	slog.Debug("peer joined room", "room", first.Room, "client_id", join.ClientID, "role", role, "participants", participants)

	done := make(chan struct{})
	defer close(done)
	go writePump(conn, peer, done)

	h.send(peer, Message{Type: "joined", Room: first.Room, Payload: mustJSON(map[string]any{
		"role": role, "participants": participants,
	})})
	if participants == 2 {
		slog.Debug("both participants connected, broadcasting peer-ready", "room", first.Room)
		if delivered := h.broadcast(first.Room, "", Message{Type: "peer-ready", Room: first.Room}); delivered < participants {
			slog.Warn("peer-ready not delivered to every participant", "delivered", delivered, "participants", participants)
		}
	}

	defer func() {
		remaining, removed := h.rooms.Leave(first.Room, peer.ID, peer)
		if removed {
			slog.Debug("peer left room", "room", first.Room, "client_id", peer.ID, "remaining", remaining)
			if remaining > 0 {
				h.broadcast(first.Room, peer.ID, Message{Type: "peer-left", Room: first.Room})
			}
		}
	}()

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}

		if !h.rooms.IsCurrent(first.Room, peer.ID, peer) {
			return
		}

		var msg Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			slog.Warn("invalid signaling json", "error", err)
			h.sendError(peer, "bad_json", "invalid signaling message")
			continue
		}

		switch msg.Type {
		case "ping":
			h.send(peer, Message{Type: "pong", Room: first.Room})
			continue
		case "hangup":
			slog.Debug("call hangup requested", "room", first.Room, "client_id", peer.ID)
			h.broadcast(first.Room, peer.ID, Message{Type: "hangup", Room: first.Room})
			h.rooms.Delete(first.Room)
			return
		}

		msg.Room = first.Room
		sanitized, err := sanitizeMessage(msg)
		if err != nil {
			slog.Warn("signaling message rejected", "type", msg.Type, "error", err)
			if msg.Type == "ice-candidate" {
				h.sendError(peer, "invalid_candidate", "invalid ICE candidate rejected")
			} else {
				h.sendError(peer, "bad_signal", "invalid signaling payload")
			}
			continue
		}
		payload, err := json.Marshal(sanitized)
		if err != nil {
			h.sendError(peer, "server_error", "could not encode signaling message")
			continue
		}
		slog.Debug("relaying signaling message", "room", first.Room, "type", msg.Type, "from", peer.ID)
		if h.rooms.Broadcast(first.Room, peer.ID, payload) == 0 {
			h.sendError(peer, "peer_unavailable", "other participant is not connected")
		}
	}
}

// send queues msg to a single peer. A queue failure kills the peer, which
// closes its socket; there is no safe way to silently skip a signaling message.
func (h *Handler) send(peer *rooms.Peer, msg Message) bool {
	payload, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	if !peer.Enqueue(payload) {
		slog.Warn("peer send buffer stalled, dropping connection", "type", msg.Type)
		return false
	}
	return true
}

func (h *Handler) sendError(peer *rooms.Peer, code, message string) {
	h.send(peer, Message{Type: "error", Payload: mustJSON(ErrorPayload{Code: code, Message: message})})
}

// broadcast fans msg out to the room and reports how many peers accepted it.
func (h *Handler) broadcast(roomID, exceptPeerID string, msg Message) int {
	payload, err := json.Marshal(msg)
	if err != nil {
		return 0
	}
	return h.rooms.Broadcast(roomID, exceptPeerID, payload)
}

func writePump(conn *websocket.Conn, peer *rooms.Peer, done <-chan struct{}) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	// Closing the socket here unblocks the read loop so a stalled or killed
	// peer does not linger until the pong deadline expires.
	defer conn.Close()
	for {
		select {
		case payload := <-peer.Send:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-peer.Dead():
			return
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

// sameOrigin requires an Origin header matching the request host. Browsers
// always send Origin on a WebSocket upgrade, so a missing header means a
// non-browser client and is rejected rather than trusted.
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return u.Host == r.Host
}

func joinError(err error) (string, string) {
	switch {
	case errors.Is(err, rooms.ErrUnauthorized):
		return "unauthorized", "invalid room secret"
	case errors.Is(err, rooms.ErrRoomFull):
		return "room_full", "call already has two participants"
	case errors.Is(err, rooms.ErrRoomExpired):
		return "room_expired", "call has expired"
	case errors.Is(err, rooms.ErrAtCapacity):
		return "at_capacity", "server is at capacity"
	default:
		return "room_not_found", "call not found"
	}
}

func mustJSON(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic("mustJSON: " + err.Error())
	}
	return data
}
