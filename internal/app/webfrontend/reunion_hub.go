package webfrontend

import (
	"sync"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

const reunionSubscriberBufferCapacity = 16

// ReunionHub distributes local real-time stream events (messages, presence, reunion resolution)
// to active subscriber channels per match in a thread-safe manner.
type ReunionHub struct {
	mu          sync.RWMutex
	subscribers map[string]map[<-chan domain.ReunionStreamEvent]chan domain.ReunionStreamEvent
}

// NewReunionHub creates an initialized ReunionHub instance.
func NewReunionHub() *ReunionHub {
	return &ReunionHub{
		subscribers: make(map[string]map[<-chan domain.ReunionStreamEvent]chan domain.ReunionStreamEvent),
	}
}

// Subscribe registers a new subscriber channel for the given matchID.
// It returns a receive-only channel with capacity 16 and an idempotent unsubscribe cleanup function.
func (h *ReunionHub) Subscribe(matchID string) (<-chan domain.ReunionStreamEvent, func()) {
	ch := make(chan domain.ReunionStreamEvent, reunionSubscriberBufferCapacity)

	h.mu.Lock()
	matchSubs, exists := h.subscribers[matchID]
	if !exists {
		matchSubs = make(map[<-chan domain.ReunionStreamEvent]chan domain.ReunionStreamEvent)
		h.subscribers[matchID] = matchSubs
	}
	matchSubs[ch] = ch
	h.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			h.Unsubscribe(matchID, ch)
		})
	}

	return ch, unsubscribe
}

// Unsubscribe removes a specific channel subscription for matchID.
// If no subscribers remain for the match, the match entry is purged from the map.
func (h *ReunionHub) Unsubscribe(matchID string, ch <-chan domain.ReunionStreamEvent) {
	if ch == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	matchSubs, exists := h.subscribers[matchID]
	if !exists {
		return
	}

	delete(matchSubs, ch)
	if len(matchSubs) == 0 {
		delete(h.subscribers, matchID)
	}
}

// BroadcastLocal sends an event to all active subscriber channels for event.MatchID.
// Sends are non-blocking; if a subscriber's buffer is full, the event is dropped for that subscriber
// to prevent slow consumers from head-of-line blocking other subscribers or publishers.
func (h *ReunionHub) BroadcastLocal(event domain.ReunionStreamEvent) {
	h.mu.RLock()
	matchSubs, exists := h.subscribers[event.MatchID]
	if !exists || len(matchSubs) == 0 {
		h.mu.RUnlock()
		return
	}

	// Copy channel references under read lock so channels can be sent to without holding the mutex
	targets := make([]chan domain.ReunionStreamEvent, 0, len(matchSubs))
	for _, ch := range matchSubs {
		targets = append(targets, ch)
	}
	h.mu.RUnlock()

	for _, ch := range targets {
		select {
		case ch <- event:
		default:
			// Buffer full (capacity 16 exceeded); drop to prevent blocking
		}
	}
}

// SubscriberCount returns the current number of active subscribers for a match.
func (h *ReunionHub) SubscriberCount(matchID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers[matchID])
}

// MatchCount returns the current number of matches with active subscriptions.
func (h *ReunionHub) MatchCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers)
}
