package petmatcher

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

const (
	matcherTestFoundSubscription = "projects/local/subscriptions/found-pet-matcher"
	matcherTestLostSubscription  = "projects/local/subscriptions/lost-pet-matcher-analysis"
)

func TestMatcherHTTPHandlerBindsLostAnalysisToItsOwnSubscription(t *testing.T) {
	stateStore := store.NewMemoryStore()
	worker := NewWorker(stateStore, pubsub.NewMemoryPubSub(), nil)
	handler := NewHTTPHandler(
		worker,
		pubsub.NewStaticPushAuthorizer("local-secret"),
		matcherTestFoundSubscription,
		matcherTestLostSubscription,
	)
	eventData, eventID := encodeLostAnalysisEvent(t, domain.LostPetReportedV4{
		PetID: "lost-push-no-image", ReportedAt: time.Date(2026, time.August, 17, 21, 0, 0, 0, time.UTC),
		Location: "Seattle, WA", GeocodingStatus: domain.GeocodingPending,
		Status: domain.LostPetStatusLost,
	})

	wrongBody := matcherPushBody(t, matcherTestFoundSubscription, "message-wrong-route", eventData)
	wrongRequest := httptest.NewRequest(http.MethodPost, "/pubsub/lost-pet", bytes.NewReader(wrongBody))
	wrongRequest.Header.Set("Authorization", "Bearer local-secret")
	wrongRecorder := httptest.NewRecorder()
	handler.ServeHTTP(wrongRecorder, wrongRequest)
	if wrongRecorder.Code != http.StatusBadRequest {
		t.Fatalf("wrong subscription status = %d, want 400", wrongRecorder.Code)
	}

	body := matcherPushBody(t, matcherTestLostSubscription, "message-lost", eventData)
	request := httptest.NewRequest(http.MethodPost, "/pubsub/lost-pet", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer local-secret")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("lost analysis status = %d, want 204; body=%q", recorder.Code, recorder.Body.String())
	}
	assertCompletedLostAnalysisOperation(t, stateStore, eventID, "lost-push-no-image")
}

func TestMatcherHTTPHandlerPreservesFoundRouteAndHealth(t *testing.T) {
	handler := NewHTTPHandler(
		NewWorker(store.NewMemoryStore(), pubsub.NewMemoryPubSub(), nil),
		pubsub.NewStaticPushAuthorizer("local-secret"),
		matcherTestFoundSubscription,
		matcherTestLostSubscription,
	)
	foundData := verifiedFoundEventData(t, domain.FoundPetReportedV2{
		PetID: "found-push", ImageURL: "https://images.invalid/found.jpg",
		FoundAt:  time.Date(2026, time.August, 17, 21, 30, 0, 0, time.UTC),
		Location: "Seattle, WA",
	})
	body := matcherPushBody(t, matcherTestFoundSubscription, "message-found", foundData)
	request := httptest.NewRequest(http.MethodPost, "/pubsub/found-pet", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer local-secret")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("found route status = %d, want 204; body=%q", recorder.Code, recorder.Body.String())
	}

	healthRequest := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthRecorder := httptest.NewRecorder()
	handler.ServeHTTP(healthRecorder, healthRequest)
	if healthRecorder.Code != http.StatusOK || healthRecorder.Body.String() != "OK" {
		t.Fatalf("health response = %d %q, want 200 OK", healthRecorder.Code, healthRecorder.Body.String())
	}
}

func matcherPushBody(t *testing.T, subscription, messageID string, payload []byte) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"subscription": subscription,
		"message": map[string]string{
			"messageId": messageID,
			"data":      base64.StdEncoding.EncodeToString(payload),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}
