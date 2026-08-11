package pubsub

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

const maxWrappedPushBytes = 14 << 20

type pushDelivery struct {
	Subscription string      `json:"subscription"`
	Message      pushMessage `json:"message"`
}

type pushMessage struct {
	Data      string `json:"data"`
	MessageID string `json:"messageId"`
}

// NewPushHandler constructs a wrapped Pub/Sub push endpoint. Only successful
// processing returns a 2xx acknowledgement; every failure requests redelivery.
func NewPushHandler(authorizer PushAuthorizer, expectedSubscription string, handler Handler) http.Handler {
	expectedSubscription = strings.TrimSpace(expectedSubscription)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if authorizer == nil || handler == nil || expectedSubscription == "" {
			http.Error(w, "push endpoint is not configured", http.StatusServiceUnavailable)
			return
		}
		authorization := r.Header.Get("Authorization")
		if authorization == "" {
			if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
				authorization = "Bearer " + token
			}
		}
		if err := authorizer.Authorize(r.Context(), authorization, requestAudience(r)); err != nil {
			http.Error(w, "unauthorized push identity", http.StatusUnauthorized)
			return
		}

		body := http.MaxBytesReader(w, r.Body, maxWrappedPushBytes)
		defer func() { _ = body.Close() }()
		var delivery pushDelivery
		decoder := json.NewDecoder(body)
		if err := decoder.Decode(&delivery); err != nil {
			http.Error(w, "invalid Pub/Sub push body", http.StatusBadRequest)
			return
		}
		var trailing any
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			http.Error(w, "invalid trailing Pub/Sub push data", http.StatusBadRequest)
			return
		}
		if delivery.Subscription != expectedSubscription || strings.TrimSpace(delivery.Message.MessageID) == "" {
			http.Error(w, "unexpected Pub/Sub delivery metadata", http.StatusBadRequest)
			return
		}
		payload, err := base64.StdEncoding.DecodeString(delivery.Message.Data)
		if err != nil || len(payload) == 0 {
			http.Error(w, "invalid Pub/Sub message data", http.StatusBadRequest)
			return
		}
		if err := handler(r.Context(), payload); err != nil {
			http.Error(w, "message processing failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func requestAudience(r *http.Request) string {
	scheme := "https"
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		scheme = forwarded
	} else if r.URL.Scheme != "" {
		scheme = r.URL.Scheme
	} else if r.TLS == nil && (strings.HasPrefix(r.Host, "localhost") || strings.HasPrefix(r.Host, "127.0.0.1")) {
		scheme = "http"
	}
	return scheme + "://" + r.Host
}
