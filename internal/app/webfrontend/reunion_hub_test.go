package webfrontend

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestReunionHubConcurrency(t *testing.T) {
	t.Parallel()
	hub := NewReunionHub()

	ch1, unsub1 := hub.Subscribe("match-1")
	defer unsub1()
	ch2, unsub2 := hub.Subscribe("match-1")
	defer unsub2()
	chOther, unsubOther := hub.Subscribe("match-2")
	defer unsubOther()

	event := domain.ReunionStreamEvent{
		EventID:   "evt-1",
		Type:      domain.ReunionEventMessageCreated,
		MatchID:   "match-1",
		Timestamp: time.Now().UTC(),
	}

	hub.BroadcastLocal(event)

	select {
	case received := <-ch1:
		if received.EventID != "evt-1" {
			t.Fatalf("ch1 received wrong event: %v", received)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for ch1")
	}

	select {
	case received := <-ch2:
		if received.EventID != "evt-1" {
			t.Fatalf("ch2 received wrong event: %v", received)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for ch2")
	}

	select {
	case unexpected := <-chOther:
		t.Fatalf("chOther should not receive match-1 event, got: %v", unexpected)
	default:
		// Expected
	}
}

func TestReunionHubBufferCapacityAndDropOnFull(t *testing.T) {
	t.Parallel()
	hub := NewReunionHub()

	ch, unsub := hub.Subscribe("match-buffer-test")
	defer unsub()

	if gotCap := cap(ch); gotCap != 16 {
		t.Fatalf("expected channel buffer capacity 16, got %d", gotCap)
	}

	// Broadcast 17 events without reading from ch
	for i := 0; i < 17; i++ {
		hub.BroadcastLocal(domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt-%d", i),
			Type:      domain.ReunionEventMessageCreated,
			MatchID:   "match-buffer-test",
			Timestamp: time.Now().UTC(),
		})
	}

	// Read the first 16 events
	for i := 0; i < 16; i++ {
		select {
		case evt := <-ch:
			expectedID := fmt.Sprintf("evt-%d", i)
			if evt.EventID != expectedID {
				t.Fatalf("event %d: expected %s, got %s", i, expectedID, evt.EventID)
			}
		default:
			t.Fatalf("expected event %d in buffer, but channel was empty", i)
		}
	}

	// The 17th event should have been dropped due to full buffer
	select {
	case dropped := <-ch:
		t.Fatalf("unexpected 17th event received (should have dropped): %v", dropped)
	default:
		// Expected: channel is now empty
	}
}

func TestReunionHubUnsubscribeAndCleanup(t *testing.T) {
	t.Parallel()
	hub := NewReunionHub()

	ch1, unsub1 := hub.Subscribe("match-1")
	_, unsub2 := hub.Subscribe("match-1")
	_, unsubOther := hub.Subscribe("match-2")

	if count := hub.SubscriberCount("match-1"); count != 2 {
		t.Fatalf("expected 2 subscribers for match-1, got %d", count)
	}
	if count := hub.MatchCount(); count != 2 {
		t.Fatalf("expected 2 active matches, got %d", count)
	}

	// Unsubscribe ch1
	unsub1()
	if count := hub.SubscriberCount("match-1"); count != 1 {
		t.Fatalf("expected 1 subscriber for match-1, got %d", count)
	}

	// Idempotent unsub1 call should not panic or change count
	unsub1()
	if count := hub.SubscriberCount("match-1"); count != 1 {
		t.Fatalf("expected 1 subscriber for match-1 after redundant unsub, got %d", count)
	}

	// Direct Unsubscribe call with already unsubscribed ch1 should be safe
	hub.Unsubscribe("match-1", ch1)
	if count := hub.SubscriberCount("match-1"); count != 1 {
		t.Fatalf("expected 1 subscriber for match-1 after explicit Unsubscribe, got %d", count)
	}

	// Unsubscribe remaining match-1 subscriber
	unsub2()
	if count := hub.SubscriberCount("match-1"); count != 0 {
		t.Fatalf("expected 0 subscribers for match-1, got %d", count)
	}
	// The empty match entry should be purged
	if count := hub.MatchCount(); count != 1 {
		t.Fatalf("expected 1 active match after purging match-1, got %d", count)
	}

	// Unsubscribe match-2
	unsubOther()
	if count := hub.MatchCount(); count != 0 {
		t.Fatalf("expected 0 active matches after purging all, got %d", count)
	}

	// Broadcast to purged match should be a safe no-op
	hub.BroadcastLocal(domain.ReunionStreamEvent{
		EventID: "evt-purged",
		MatchID: "match-1",
	})
}

func TestReunionHubConcurrentStressRace(t *testing.T) {
	t.Parallel()
	hub := NewReunionHub()

	matches := []string{"match-stress-1", "match-stress-2", "match-stress-3"}
	const numWorkers = 8
	const duration = 200 * time.Millisecond

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Spawn broadcaster workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			seq := 0
			for {
				select {
				case <-stop:
					return
				default:
					matchID := matches[seq%len(matches)]
					hub.BroadcastLocal(domain.ReunionStreamEvent{
						EventID:   fmt.Sprintf("evt-w%d-%d", workerID, seq),
						Type:      domain.ReunionEventMessageCreated,
						MatchID:   matchID,
						Timestamp: time.Now().UTC(),
					})
					seq++
				}
			}
		}(w)
	}

	// Spawn subscriber/reader workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			seq := 0
			for {
				select {
				case <-stop:
					return
				default:
					matchID := matches[seq%len(matches)]
					ch, unsub := hub.Subscribe(matchID)

					// Read a few items non-blockingly
					for r := 0; r < 3; r++ {
						select {
						case <-ch:
						default:
						}
					}

					unsub()
					seq++
				}
			}
		}(w)
	}

	time.Sleep(duration)
	close(stop)
	wg.Wait()
}
