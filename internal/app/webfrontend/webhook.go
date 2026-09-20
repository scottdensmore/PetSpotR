package webfrontend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/store"
	"github.com/scottdensmore/petspotr/pkg/webhook"
)

type createWebhookRequest struct {
	ID           string            `json:"id,omitempty"`
	PartnerID    string            `json:"partnerId,omitempty"`
	TargetURL    string            `json:"targetUrl"`
	Secret       string            `json:"secret,omitempty"`
	FilterEvents []string          `json:"filterEvents,omitempty"`
	GeoFence     *webhook.GeoFence `json:"geoFence,omitempty"`
}

func generateWebhookSecret() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func generateWebhookID() string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("wh-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("wh-%s", hex.EncodeToString(random[:]))
}

func (s *Server) handleApiWebhooks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleApiCreateWebhook(w, r)
	case http.MethodGet:
		s.handleApiListWebhooks(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleApiCreateWebhook(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req createWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	targetURL := strings.TrimSpace(req.TargetURL)
	if targetURL == "" {
		http.Error(w, "targetUrl is required", http.StatusBadRequest)
		return
	}

	if err := webhook.ValidateURLWithOptions(targetURL, s.allowLocalhostWebhooks); err != nil {
		http.Error(w, fmt.Sprintf("invalid targetUrl: %v", err), http.StatusBadRequest)
		return
	}

	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = generateWebhookID()
	}

	secret := strings.TrimSpace(req.Secret)
	if secret == "" {
		sec, err := generateWebhookSecret()
		if err != nil {
			http.Error(w, "failed to generate webhook secret", http.StatusInternalServerError)
			return
		}
		secret = sec
	}

	partnerID := strings.TrimSpace(req.PartnerID)
	if partnerID == "" {
		partnerID = "default"
	}

	filterEvents := req.FilterEvents
	if filterEvents == nil {
		filterEvents = []string{}
	}

	sub := webhook.WebhookSubscription{
		ID:           id,
		PartnerID:    partnerID,
		TargetURL:    targetURL,
		Secret:       secret,
		FilterEvents: filterEvents,
		GeoFence:     req.GeoFence,
		Active:       true,
		CreatedAt:    time.Now().UTC(),
	}

	data, err := json.Marshal(sub)
	if err != nil {
		http.Error(w, "failed to marshal webhook subscription", http.StatusInternalServerError)
		return
	}

	if err := s.stateStore.SaveState(r.Context(), store.WebhooksCollection, sub.ID, data); err != nil {
		http.Error(w, "failed to save webhook subscription", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(sub)
}

func (s *Server) handleApiListWebhooks(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	rawItems, err := s.stateStore.ListState(r.Context(), store.WebhooksCollection)
	if err != nil && !errors.Is(err, store.ErrStoreNotFound) && !errors.Is(err, store.ErrNotFound) {
		http.Error(w, "failed to list webhooks", http.StatusInternalServerError)
		return
	}

	subs := make([]webhook.WebhookSubscription, 0)
	for _, b := range rawItems {
		var sub webhook.WebhookSubscription
		if err := json.Unmarshal(b, &sub); err != nil {
			continue
		}
		if sub.Active {
			sub.Secret = "••••••••"
			subs = append(subs, sub)
		}
	}

	sort.Slice(subs, func(i, j int) bool {
		return subs[i].CreatedAt.Before(subs[j].CreatedAt)
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(subs)
}

func (s *Server) handleApiWebhookByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodDelete:
		s.handleApiDeleteWebhook(w, r)
	default:
		w.Header().Set("Allow", http.MethodDelete)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleApiDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.NotFound(w, r)
		return
	}

	_, err := s.stateStore.GetState(r.Context(), store.WebhooksCollection, id)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "failed to get webhook", http.StatusInternalServerError)
		return
	}

	if err := s.stateStore.DeleteState(r.Context(), store.WebhooksCollection, id); err != nil {
		http.Error(w, "failed to delete webhook", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleApiWebhookTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.NotFound(w, r)
		return
	}

	subBytes, err := s.stateStore.GetState(r.Context(), store.WebhooksCollection, id)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "failed to get webhook", http.StatusInternalServerError)
		return
	}

	var sub webhook.WebhookSubscription
	if err := json.Unmarshal(subBytes, &sub); err != nil {
		http.Error(w, "corrupt webhook subscription record", http.StatusInternalServerError)
		return
	}

	if !sub.Active {
		http.Error(w, "webhook subscription is inactive", http.StatusBadRequest)
		return
	}

	dispatcher := s.webhookDispatcher
	if dispatcher == nil {
		opts := []webhook.DispatcherOption{}
		if s.allowLocalhostWebhooks {
			opts = append(opts, webhook.WithAllowLocalhost(true))
		}
		dispatcher = webhook.NewDispatcher(s.stateStore, opts...)
	}

	pingPayload := map[string]any{
		"event":          "ping",
		"subscriptionId": sub.ID,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"message":        "PetSpotR webhook test ping",
	}
	payloadBytes, err := json.Marshal(pingPayload)
	if err != nil {
		http.Error(w, "failed to marshal ping payload", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	record, deliverErr := dispatcher.Deliver(ctx, &sub, "ping", payloadBytes, nil)
	if record == nil && deliverErr != nil {
		http.Error(w, fmt.Sprintf("failed to deliver test ping: %v", deliverErr), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(record)
}
