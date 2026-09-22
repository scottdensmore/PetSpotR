package mesh_test

import (
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/mesh"
)

func TestSignalingHub_LifecycleAndTargeting(t *testing.T) {
	hub := mesh.NewSignalingHub()
	partyID := "party-lifecycle-1"

	// 1. Peer 1 subscribes
	ch1, unsub1 := hub.Subscribe(partyID, "node-1")
	defer unsub1()

	if count := hub.PeerCount(partyID); count != 1 {
		t.Fatalf("expected PeerCount=1, got %d", count)
	}

	// 2. Peer 2 subscribes; Peer 1 should receive PEER_JOINED
	ch2, unsub2 := hub.Subscribe(partyID, "node-2")
	defer unsub2()

	if count := hub.PeerCount(partyID); count != 2 {
		t.Fatalf("expected PeerCount=2, got %d", count)
	}

	select {
	case env := <-ch1:
		if env.Type != mesh.SignalPeerJoined || env.SenderNodeID != "node-2" {
			t.Errorf("expected PEER_JOINED from node-2, got %+v", env)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for PEER_JOINED event on ch1")
	}

	// 3. Targeted message from node-1 to node-2
	hub.Relay(mesh.SignalingEnvelope{
		Type:          mesh.SignalOffer,
		SearchPartyID: partyID,
		SenderNodeID:  "node-1",
		TargetNodeID:  "node-2",
		SDP:           "v=0...",
	})

	select {
	case env := <-ch2:
		if env.Type != mesh.SignalOffer || env.SenderNodeID != "node-1" {
			t.Errorf("expected OFFER from node-1, got %+v", env)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for OFFER on ch2")
	}

	// Sender node-1 should not receive its own message
	select {
	case env := <-ch1:
		t.Fatalf("node-1 unexpectedly received message: %+v", env)
	default:
		// expected
	}

	// 4. Unsubscribe node-2; node-1 should receive PEER_LEFT
	unsub2()

	select {
	case env := <-ch1:
		if env.Type != mesh.SignalPeerLeft || env.SenderNodeID != "node-2" {
			t.Errorf("expected PEER_LEFT from node-2, got %+v", env)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for PEER_LEFT on ch1")
	}

	if count := hub.PeerCount(partyID); count != 1 {
		t.Errorf("expected PeerCount=1 after node-2 left, got %d", count)
	}
}

func TestSignalingHub_ConcurrentRelay(t *testing.T) {
	hub := mesh.NewSignalingHub()
	partyID := "party-concurrent"

	const numPeers = 8
	channels := make([]<-chan mesh.SignalingEnvelope, numPeers)
	unsubs := make([]func(), numPeers)

	for i := 0; i < numPeers; i++ {
		nodeID := string(rune('A' + i))
		channels[i], unsubs[i] = hub.Subscribe(partyID, nodeID)
		defer unsubs[i]()
	}

	var wg sync.WaitGroup
	const msgsPerPeer = 20

	for i := 0; i < numPeers; i++ {
		wg.Add(1)
		go func(senderIdx int) {
			defer wg.Done()
			senderID := string(rune('A' + senderIdx))
			for m := 0; m < msgsPerPeer; m++ {
				hub.Relay(mesh.SignalingEnvelope{
					Type:          mesh.SignalICECandidate,
					SearchPartyID: partyID,
					SenderNodeID:  senderID,
				})
			}
		}(i)
	}

	wg.Wait()
}

func TestSignalingHub_ConcurrentUnsubscribeAndRelay(t *testing.T) {
	hub := mesh.NewSignalingHub()
	partyID := "party-race-unsub"

	const iterations = 50
	var wg sync.WaitGroup

	for i := 0; i < iterations; i++ {
		wg.Add(2)

		nodeID := "node-ephemeral"
		ch, unsub := hub.Subscribe(partyID, nodeID)

		// Goroutine 1: Continuous relay
		go func() {
			defer wg.Done()
			for r := 0; r < 20; r++ {
				hub.Relay(mesh.SignalingEnvelope{
					Type:          mesh.SignalICECandidate,
					SearchPartyID: partyID,
					SenderNodeID:  "node-other",
					TargetNodeID:  nodeID,
				})
			}
		}()

		// Goroutine 2: Concurrent drain and unsubscribe
		go func() {
			defer wg.Done()
			// Consume any events if present
			go func() {
				for range ch {
				}
			}()
			time.Sleep(1 * time.Millisecond)
			unsub()
		}()
	}

	wg.Wait()
}
