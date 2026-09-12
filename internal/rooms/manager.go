package rooms

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

var (
	ErrRoomNotFound = errors.New("room not found")
	ErrUnauthorized = errors.New("invalid room secret")
	ErrRoomFull     = errors.New("room already has two participants")
	ErrRoomExpired  = errors.New("room expired")
	ErrAtCapacity   = errors.New("server is at room capacity")
)

type Peer struct {
	ID   string
	Send chan []byte

	deadOnce sync.Once
	dead     chan struct{}
}

func NewPeer(id string, sendBuffer int) *Peer {
	return &Peer{
		ID:   id,
		Send: make(chan []byte, sendBuffer),
		dead: make(chan struct{}),
	}
}

// Enqueue queues payload for delivery to the peer. It reports false when the
// payload could not be queued, which also marks the peer dead.
//
// A full send buffer means the peer's socket has stalled. Discarding the
// payload is not a safe fallback: signaling has no retransmit, so a dropped
// offer, answer, or candidate leaves the call permanently half-negotiated with
// no error surfaced to either side. Tearing the connection down instead lets
// the client observe the failure and reconnect.
func (p *Peer) Enqueue(payload []byte) bool {
	select {
	case <-p.dead:
		return false
	default:
	}
	select {
	case p.Send <- payload:
		return true
	default:
		p.Kill()
		return false
	}
}

// Kill marks the peer dead, unblocking anything waiting on Dead. Safe to call
// repeatedly and from multiple goroutines.
func (p *Peer) Kill() {
	p.deadOnce.Do(func() { close(p.dead) })
}

// Dead is closed once the peer can no longer accept payloads.
func (p *Peer) Dead() <-chan struct{} { return p.dead }

type Room struct {
	ID        string
	CreatedAt time.Time
	ExpiresAt time.Time

	secretHash      [32]byte
	peers           map[string]*Peer
	roles           map[string]string
	sessionTokens   map[string]string
	hadParticipants bool
	emptySince      *time.Time
}

type Status struct {
	Exists       bool      `json:"exists"`
	Participants int       `json:"participants"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
}

type Manager struct {
	mu         sync.RWMutex
	rooms      map[string]*Room
	ttl        time.Duration
	emptyGrace time.Duration
	maxRooms   int
	now        func() time.Time
}

// NewManager builds a room manager. maxRooms bounds how many rooms may exist at
// once; zero or negative means unbounded.
func NewManager(ttl, emptyGrace time.Duration, maxRooms int) *Manager {
	return &Manager{
		rooms:      make(map[string]*Room),
		ttl:        ttl,
		emptyGrace: emptyGrace,
		maxRooms:   maxRooms,
		now:        time.Now,
	}
}

func (m *Manager) Create() (*Room, string, error) {
	roomID, err := randomToken(18)
	if err != nil {
		return nil, "", err
	}
	secret, err := randomToken(32)
	if err != nil {
		return nil, "", err
	}

	now := m.now()
	room := &Room{
		ID:            roomID,
		CreatedAt:     now,
		ExpiresAt:     now.Add(m.ttl),
		secretHash:    sha256.Sum256([]byte(secret)),
		peers:         make(map[string]*Peer, 2),
		roles:         make(map[string]string, 2),
		sessionTokens: make(map[string]string, 2),
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.maxRooms > 0 && len(m.rooms) >= m.maxRooms {
		// Reclaim what is already collectable before rejecting: the periodic
		// sweep may be up to its interval away.
		m.evictLocked(now)
		if len(m.rooms) >= m.maxRooms {
			return nil, "", ErrAtCapacity
		}
	}
	m.rooms[room.ID] = room
	return room, secret, nil
}

// Count reports how many rooms currently exist.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.rooms)
}

// Join admits peer into the room, returning its assigned role and a session
// token bound to peer.ID. Once a client ID has taken a role, reclaiming that
// same role (e.g. a signaling reconnect) requires presenting the exact token
// handed back from the first successful join, so knowing/guessing another
// participant's client ID alone is not enough to hijack their slot mid-call.
func (m *Manager) Join(roomID, secret string, peer *Peer, sessionToken string) (role string, participants int, token string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[roomID]
	if !ok {
		return "", 0, "", ErrRoomNotFound
	}
	now := m.now()
	if !now.Before(room.ExpiresAt) {
		delete(m.rooms, roomID)
		return "", 0, "", ErrRoomExpired
	}

	hash := sha256.Sum256([]byte(secret))
	if subtle.ConstantTimeCompare(hash[:], room.secretHash[:]) != 1 {
		return "", 0, "", ErrUnauthorized
	}

	role, known := room.roles[peer.ID]
	if !known {
		if len(room.roles) >= 2 {
			return "", len(room.peers), "", ErrRoomFull
		}
		if len(room.roles) == 0 {
			role = "caller"
		} else {
			role = "callee"
		}
		newToken, err := randomToken(18)
		if err != nil {
			return "", 0, "", err
		}
		room.roles[peer.ID] = role
		room.sessionTokens[peer.ID] = newToken
	} else {
		expected := room.sessionTokens[peer.ID]
		if expected == "" || subtle.ConstantTimeCompare([]byte(sessionToken), []byte(expected)) != 1 {
			return "", len(room.peers), "", ErrUnauthorized
		}
	}

	room.peers[peer.ID] = peer
	room.hadParticipants = true
	room.emptySince = nil
	return role, len(room.peers), room.sessionTokens[peer.ID], nil
}

// Leave removes a peer only if current is still the active connection for that client ID.
// This prevents a replaced/reconnected WebSocket from being removed by the old socket's defer.
func (m *Manager) Leave(roomID, peerID string, current *Peer) (remaining int, removed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[roomID]
	if !ok {
		return 0, false
	}
	peer, ok := room.peers[peerID]
	if !ok || peer != current {
		return len(room.peers), false
	}

	delete(room.peers, peerID)
	if len(room.peers) == 0 && room.hadParticipants {
		now := m.now()
		room.emptySince = &now
	}
	return len(room.peers), true
}

func (m *Manager) IsCurrent(roomID, peerID string, current *Peer) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	room, ok := m.rooms[roomID]
	if !ok {
		return false
	}
	peer, ok := room.peers[peerID]
	return ok && peer == current
}

// Broadcast queues payload to every peer in the room except exceptPeerID and
// returns how many peers accepted it. Peers whose buffer has stalled are marked
// dead by Enqueue and are not counted.
func (m *Manager) Broadcast(roomID, exceptPeerID string, payload []byte) int {
	m.mu.RLock()
	room, ok := m.rooms[roomID]
	if !ok {
		m.mu.RUnlock()
		return 0
	}
	peers := make([]*Peer, 0, len(room.peers))
	for id, peer := range room.peers {
		if id != exceptPeerID {
			peers = append(peers, peer)
		}
	}
	m.mu.RUnlock()

	sent := 0
	for _, peer := range peers {
		if peer.Enqueue(payload) {
			sent++
		}
	}
	return sent
}

func (m *Manager) Delete(roomID string) {
	m.mu.Lock()
	delete(m.rooms, roomID)
	m.mu.Unlock()
}

func (m *Manager) Status(roomID string) Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	room, ok := m.rooms[roomID]
	if !ok || !m.now().Before(room.ExpiresAt) {
		return Status{}
	}
	return Status{Exists: true, Participants: len(room.peers), ExpiresAt: room.ExpiresAt}
}

func (m *Manager) Cleanup() int {
	m.mu.Lock()
	removed := m.evictLocked(m.now())
	m.mu.Unlock()
	return removed
}

// evictLocked drops expired and long-empty rooms. Callers must hold mu.
func (m *Manager) evictLocked(now time.Time) int {
	removed := 0
	for id, room := range m.rooms {
		expired := !now.Before(room.ExpiresAt)
		emptyTooLong := room.emptySince != nil && now.Sub(*room.emptySince) >= m.emptyGrace
		if expired || emptyTooLong {
			delete(m.rooms, id)
			removed++
		}
	}
	return removed
}

func randomToken(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
