package mesh

import (
	"encoding/json"
	"sync"
	"time"
)

const signalingBufferCapacity = 64

// SignalingType represents WebRTC signaling envelope categories.
type SignalingType string

const (
	SignalOffer        SignalingType = "OFFER"
	SignalAnswer       SignalingType = "ANSWER"
	SignalICECandidate SignalingType = "ICE_CANDIDATE"
	SignalPeerJoined   SignalingType = "PEER_JOINED"
	SignalPeerLeft     SignalingType = "PEER_LEFT"
)

// SignalingEnvelope is the wire format for WebRTC signaling messages relayed across local LAN / hotspot networks.
type SignalingEnvelope struct {
	Type          SignalingType   `json:"type"`
	SearchPartyID string          `json:"searchPartyId"`
	SenderNodeID  string          `json:"senderNodeId"`
	TargetNodeID  string          `json:"targetNodeId,omitempty"`
	SDP           string          `json:"sdp,omitempty"`
	Candidate     any             `json:"candidate,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
	Timestamp     time.Time       `json:"timestamp"`
}

type peerSubscriber struct {
	nodeID string
	ch     chan SignalingEnvelope
}

// SignalingHub coordinates real-time WebRTC peer signaling envelopes keyed by searchPartyID in a thread-safe manner.
type SignalingHub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan SignalingEnvelope]*peerSubscriber
}

// NewSignalingHub constructs an initialized SignalingHub instance.
func NewSignalingHub() *SignalingHub {
	return &SignalingHub{
		subscribers: make(map[string]map[chan SignalingEnvelope]*peerSubscriber),
	}
}

// Subscribe adds a node's SSE channel to the given searchPartyID.
// It automatically broadcasts a PEER_JOINED event to all other active peers in that search party.
// It returns a receive-only channel with capacity 64 and an idempotent unsubscribe cleanup function.
// When the returned unsubscribe function is called, it broadcasts a PEER_LEFT event to remaining peers and closes the channel.
func (h *SignalingHub) Subscribe(searchPartyID, nodeID string) (<-chan SignalingEnvelope, func()) {
	ch := make(chan SignalingEnvelope, signalingBufferCapacity)
	sub := &peerSubscriber{
		nodeID: nodeID,
		ch:     ch,
	}

	h.mu.Lock()
	partySubs, exists := h.subscribers[searchPartyID]
	if !exists {
		partySubs = make(map[chan SignalingEnvelope]*peerSubscriber)
		h.subscribers[searchPartyID] = partySubs
	}
	partySubs[ch] = sub
	h.mu.Unlock()

	// Broadcast PEER_JOINED to active peers
	if nodeID != "" {
		h.Broadcast(SignalingEnvelope{
			Type:          SignalPeerJoined,
			SearchPartyID: searchPartyID,
			SenderNodeID:  nodeID,
			Timestamp:     time.Now().UTC(),
		})
	}

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			h.mu.Lock()
			if subs, ok := h.subscribers[searchPartyID]; ok {
				delete(subs, ch)
				if len(subs) == 0 {
					delete(h.subscribers, searchPartyID)
				}
			}
			h.mu.Unlock()

			if nodeID != "" {
				h.Broadcast(SignalingEnvelope{
					Type:          SignalPeerLeft,
					SearchPartyID: searchPartyID,
					SenderNodeID:  nodeID,
					Timestamp:     time.Now().UTC(),
				})
			}
			close(ch)
		})
	}

	return ch, unsubscribe
}

// Broadcast sends a signaling envelope to target peers in envelope.SearchPartyID.
// It delegates to Relay.
func (h *SignalingHub) Broadcast(envelope SignalingEnvelope) {
	h.Relay(envelope)
}

// Relay routes a signaling envelope to peers in envelope.SearchPartyID.
// If envelope.TargetNodeID is specified, it delivers only to subscribers matching that nodeID.
// Otherwise, it delivers to all subscribers in the search party except envelope.SenderNodeID.
// Channel sends are non-blocking with buffer capacity 64 to prevent slow consumers from head-of-line blocking peers.
func (h *SignalingHub) Relay(envelope SignalingEnvelope) {
	if envelope.SearchPartyID == "" {
		return
	}
	if envelope.Timestamp.IsZero() {
		envelope.Timestamp = time.Now().UTC()
	}

	h.mu.RLock()
	partySubs, exists := h.subscribers[envelope.SearchPartyID]
	if !exists || len(partySubs) == 0 {
		h.mu.RUnlock()
		return
	}

	targets := make([]chan SignalingEnvelope, 0, len(partySubs))
	for ch, sub := range partySubs {
		if envelope.TargetNodeID != "" {
			if sub.nodeID == envelope.TargetNodeID {
				targets = append(targets, ch)
			}
			continue
		}
		if sub.nodeID != envelope.SenderNodeID {
			targets = append(targets, ch)
		}
	}
	h.mu.RUnlock()

	for _, ch := range targets {
		select {
		case ch <- envelope:
		default:
			// Buffer full (capacity 64 exceeded); drop to prevent blocking
		}
	}
}

// PeerCount returns the current number of active subscribers for a given search party.
func (h *SignalingHub) PeerCount(searchPartyID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers[searchPartyID])
}

// PartyCount returns the current number of search parties with active subscribers.
func (h *SignalingHub) PartyCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers)
}
