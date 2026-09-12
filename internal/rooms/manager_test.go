package rooms

import (
	"errors"
	"testing"
	"time"
)

func TestRoomCapacityAndReconnect(t *testing.T) {
	manager := NewManager(time.Hour, 30*time.Second, 0)
	room, secret, err := manager.Create()
	if err != nil {
		t.Fatal(err)
	}

	first := NewPeer("a", 2)
	role, participants, token, err := manager.Join(room.ID, secret, first, "")
	if err != nil || role != "caller" || participants != 1 {
		t.Fatalf("first join: role=%q participants=%d err=%v", role, participants, err)
	}

	second := NewPeer("b", 2)
	role, participants, _, err = manager.Join(room.ID, secret, second, "")
	if err != nil || role != "callee" || participants != 2 {
		t.Fatalf("second join: role=%q participants=%d err=%v", role, participants, err)
	}

	third := NewPeer("c", 2)
	if _, _, _, err := manager.Join(room.ID, secret, third, ""); !errors.Is(err, ErrRoomFull) {
		t.Fatalf("expected ErrRoomFull, got %v", err)
	}

	if _, _, _, err := manager.Join(room.ID, secret, NewPeer("a", 2), "wrong-token"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized reclaiming a role with the wrong session token, got %v", err)
	}

	reconnected := NewPeer("a", 2)
	role, participants, _, err = manager.Join(room.ID, secret, reconnected, token)
	if err != nil || role != "caller" || participants != 2 {
		t.Fatalf("reconnect: role=%q participants=%d err=%v", role, participants, err)
	}
	if manager.IsCurrent(room.ID, "a", first) {
		t.Fatal("old peer should no longer be current")
	}
	if !manager.IsCurrent(room.ID, "a", reconnected) {
		t.Fatal("reconnected peer should be current")
	}
}

func TestRoomSecretAndCleanup(t *testing.T) {
	manager := NewManager(time.Hour, 5*time.Second, 0)
	now := time.Unix(1_700_000_000, 0)
	manager.now = func() time.Time { return now }

	room, secret, err := manager.Create()
	if err != nil {
		t.Fatal(err)
	}
	peer := NewPeer("a", 1)
	if _, _, _, err := manager.Join(room.ID, "wrong", peer, ""); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
	if _, _, _, err := manager.Join(room.ID, secret, peer, ""); err != nil {
		t.Fatal(err)
	}
	if _, removed := manager.Leave(room.ID, peer.ID, peer); !removed {
		t.Fatal("expected peer to be removed")
	}

	now = now.Add(6 * time.Second)
	if removed := manager.Cleanup(); removed != 1 {
		t.Fatalf("expected one cleaned room, got %d", removed)
	}
	if manager.Status(room.ID).Exists {
		t.Fatal("room should have been cleaned up")
	}
}

func TestEnqueueKillsPeerWhenBufferFull(t *testing.T) {
	peer := NewPeer("a", 1)
	if !peer.Enqueue([]byte("first")) {
		t.Fatal("first enqueue should succeed")
	}

	// The buffer is now full. A signaling message has no retransmit, so the
	// peer must be killed rather than the payload discarded.
	if peer.Enqueue([]byte("second")) {
		t.Fatal("enqueue past capacity should fail")
	}
	select {
	case <-peer.Dead():
	default:
		t.Fatal("peer should be marked dead after a failed enqueue")
	}

	// Draining does not revive a dead peer.
	<-peer.Send
	if peer.Enqueue([]byte("third")) {
		t.Fatal("a dead peer must not accept further payloads")
	}
}

func TestBroadcastReportsOnlyPeersThatAccepted(t *testing.T) {
	manager := NewManager(time.Hour, time.Minute, 0)
	room, secret, err := manager.Create()
	if err != nil {
		t.Fatal(err)
	}

	healthy := NewPeer("a", 4)
	stalled := NewPeer("b", 1)
	for _, peer := range []*Peer{healthy, stalled} {
		if _, _, _, err := manager.Join(room.ID, secret, peer, ""); err != nil {
			t.Fatal(err)
		}
	}
	stalled.Send <- []byte("backlog") // fill the stalled peer's only slot

	if sent := manager.Broadcast(room.ID, "", []byte("offer")); sent != 1 {
		t.Fatalf("expected exactly one delivery, got %d", sent)
	}
	select {
	case <-stalled.Dead():
	default:
		t.Fatal("stalled peer should have been killed by the broadcast")
	}
}

func TestCreateRejectsBeyondRoomCap(t *testing.T) {
	manager := NewManager(time.Hour, time.Minute, 2)
	for i := 0; i < 2; i++ {
		if _, _, err := manager.Create(); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	if _, _, err := manager.Create(); !errors.Is(err, ErrAtCapacity) {
		t.Fatalf("expected ErrAtCapacity, got %v", err)
	}
	if manager.Count() != 2 {
		t.Fatalf("expected 2 rooms, got %d", manager.Count())
	}
}

func TestCreateReclaimsExpiredRoomsAtCap(t *testing.T) {
	manager := NewManager(time.Hour, time.Minute, 1)
	now := time.Unix(1_700_000_000, 0)
	manager.now = func() time.Time { return now }

	if _, _, err := manager.Create(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.Create(); !errors.Is(err, ErrAtCapacity) {
		t.Fatalf("expected ErrAtCapacity while the first room is live, got %v", err)
	}

	// Once the first room has expired, the cap should not block a new one even
	// before the periodic sweep runs.
	now = now.Add(2 * time.Hour)
	if _, _, err := manager.Create(); err != nil {
		t.Fatalf("expected expired room to be reclaimed, got %v", err)
	}
	if manager.Count() != 1 {
		t.Fatalf("expected 1 room, got %d", manager.Count())
	}
}

// A peer that drops and comes back within the grace window must find its room
// intact, and the stale socket's deferred Leave must not evict the new one.
func TestRoomSurvivesPeerReconnectWithinGrace(t *testing.T) {
	manager := NewManager(time.Hour, 45*time.Second, 0)
	now := time.Unix(1_700_000_000, 0)
	manager.now = func() time.Time { return now }

	room, secret, err := manager.Create()
	if err != nil {
		t.Fatal(err)
	}

	original := NewPeer("a", 4)
	_, _, origToken, err := manager.Join(room.ID, secret, original, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, removed := manager.Leave(room.ID, "a", original); !removed {
		t.Fatal("expected the dropped peer to be removed")
	}

	// Still inside the grace window.
	now = now.Add(20 * time.Second)
	if removed := manager.Cleanup(); removed != 0 {
		t.Fatalf("room should survive inside the grace window, cleaned %d", removed)
	}

	reconnected := NewPeer("a", 4)
	role, participants, _, err := manager.Join(room.ID, secret, reconnected, origToken)
	if err != nil {
		t.Fatalf("reconnect within grace: %v", err)
	}
	if role != "caller" || participants != 1 {
		t.Fatalf("reconnect: role=%q participants=%d, want caller/1", role, participants)
	}

	// The stale socket's deferred Leave must be a no-op now.
	if _, removed := manager.Leave(room.ID, "a", original); removed {
		t.Fatal("stale socket must not evict the reconnected peer")
	}
	if !manager.IsCurrent(room.ID, "a", reconnected) {
		t.Fatal("reconnected peer should still be current")
	}

	// Rejoining must have cleared emptySince, so the grace deadline no longer applies.
	now = now.Add(2 * time.Minute)
	if removed := manager.Cleanup(); removed != 0 {
		t.Fatalf("occupied room should not be cleaned, cleaned %d", removed)
	}
}

func TestRoomExpiresAfterGraceWithNoReconnect(t *testing.T) {
	manager := NewManager(time.Hour, 45*time.Second, 0)
	now := time.Unix(1_700_000_000, 0)
	manager.now = func() time.Time { return now }

	room, secret, err := manager.Create()
	if err != nil {
		t.Fatal(err)
	}
	peer := NewPeer("a", 4)
	if _, _, _, err := manager.Join(room.ID, secret, peer, ""); err != nil {
		t.Fatal(err)
	}
	manager.Leave(room.ID, "a", peer)

	now = now.Add(46 * time.Second)
	if removed := manager.Cleanup(); removed != 1 {
		t.Fatalf("expected the abandoned room to be cleaned, cleaned %d", removed)
	}
	if _, _, _, err := manager.Join(room.ID, secret, NewPeer("a", 4), ""); !errors.Is(err, ErrRoomNotFound) {
		t.Fatalf("expected ErrRoomNotFound after grace, got %v", err)
	}
}
