package sms

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

const (
	DefaultBroadcastRadiusMiles = 2.0
	MaxSMSLength                = 160
)

// Subscriber represents a community subscriber for hyper-local SMS broadcasts.
type Subscriber struct {
	ID          string               `json:"id"`
	Phone       string               `json:"phone"`
	Coordinates domain.LocationPoint `json:"coordinates"`
	RadiusMiles float64              `json:"radiusMiles"`
}

// BroadcastDeliveryRecord is persisted in store.SMSDeliveriesCollection for audit and compliance.
type BroadcastDeliveryRecord struct {
	DeliveryID     string    `json:"deliveryId"`
	PetID          string    `json:"petId"`
	TokenizedPhone string    `json:"tokenizedPhone"`
	SentAt         time.Time `json:"sentAt"`
	DistanceMiles  float64   `json:"distanceMiles"`
	Status         string    `json:"status"` // "delivered", "skipped_optout", "skipped_radius", "failed"
	ErrorMessage   string    `json:"errorMessage,omitempty"`
}

// BroadcastResult summarizes delivery outcome for a subscriber.
type BroadcastResult struct {
	SubscriberID   string  `json:"subscriberId"`
	Phone          string  `json:"phone,omitempty"`
	TokenizedPhone string  `json:"tokenizedPhone"`
	DistanceMiles  float64 `json:"distanceMiles"`
	Delivered      bool    `json:"delivered"`
	Skipped        bool    `json:"skipped"`
	Reason         string  `json:"reason,omitempty"`
}

// BroadcastOption configures BroadcastWorker.
type BroadcastOption func(*BroadcastWorker)

// WithRadiusMiles sets custom max radius in miles.
func WithRadiusMiles(radius float64) BroadcastOption {
	return func(w *BroadcastWorker) {
		if radius > 0 {
			w.radiusMiles = radius
		}
	}
}

// BroadcastWorker executes hyper-local radius alert dispatches.
type BroadcastWorker struct {
	provider    Provider
	optOutMgr   *OptOutManager
	stateStore  store.StateStore
	secretKey   []byte
	radiusMiles float64
}

// NewBroadcastWorker constructs a BroadcastWorker instance.
func NewBroadcastWorker(
	provider Provider,
	optOutMgr *OptOutManager,
	stateStore store.StateStore,
	secretKey []byte,
	opts ...BroadcastOption,
) *BroadcastWorker {
	if len(secretKey) == 0 {
		secretKey = []byte("petspotr-default-sms-salt-2026")
	}
	if optOutMgr == nil && stateStore != nil {
		optOutMgr = NewOptOutManager(stateStore, secretKey)
	}

	w := &BroadcastWorker{
		provider:    provider,
		optOutMgr:   optOutMgr,
		stateStore:  stateStore,
		secretKey:   secretKey,
		radiusMiles: DefaultBroadcastRadiusMiles,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(w)
		}
	}

	return w
}

// FormatAlertMessage formats a concise hyper-local alert adhering to 160-char SMS limit.
func (w *BroadcastWorker) FormatAlertMessage(petName, location string, distMiles float64) string {
	name := strings.TrimSpace(petName)
	if name == "" {
		name = "Pet"
	}
	loc := strings.TrimSpace(location)
	if loc == "" {
		loc = "nearby"
	}

	msg := fmt.Sprintf("🚨 PETSPOTR: Lost (%s) near %s (~%.1fmi). Reply CLAIM <sec> to search, SIGHTED <info> to report. Reply STOP to cancel.",
		name, loc, distMiles)

	if len(msg) <= MaxSMSLength {
		return msg
	}

	excess := len(msg) - MaxSMSLength
	if excess < len(loc)-3 && len(loc) > 6 {
		loc = loc[:len(loc)-excess-3] + "..."
		msg = fmt.Sprintf("🚨 PETSPOTR: Lost (%s) near %s (~%.1fmi). Reply CLAIM <sec> to search, SIGHTED <info> to report. Reply STOP to cancel.",
			name, loc, distMiles)
	}

	if len(msg) > MaxSMSLength {
		msg = msg[:MaxSMSLength-3] + "..."
	}

	return msg
}

// BroadcastLostPet evaluates subscribers within radius and dispatches alerts.
func (w *BroadcastWorker) BroadcastLostPet(
	ctx context.Context,
	pet domain.LostPetRecord,
	subscribers []Subscriber,
) ([]BroadcastResult, error) {
	if pet.Coordinates == nil {
		return nil, errors.New("pet coordinates are required for radius broadcast")
	}

	results := make([]BroadcastResult, 0, len(subscribers))
	petCoords := *pet.Coordinates

	petName := pet.PetName
	if petName == "" {
		petName = pet.PetID
	}

	for _, sub := range subscribers {
		distMiles := domain.HaversineDistanceMiles(petCoords, sub.Coordinates)
		token := TokenizeNumber(sub.Phone, w.secretKey)

		res := BroadcastResult{
			SubscriberID:   sub.ID,
			Phone:          sub.Phone,
			TokenizedPhone: token,
			DistanceMiles:  distMiles,
		}

		effRadius := w.radiusMiles
		if sub.RadiusMiles > 0 && sub.RadiusMiles < effRadius {
			effRadius = sub.RadiusMiles
		}

		if distMiles > effRadius {
			res.Skipped = true
			res.Reason = fmt.Sprintf("outside radius (distance %.2f mi > threshold %.2f mi)", distMiles, effRadius)
			results = append(results, res)
			continue
		}

		if w.optOutMgr != nil {
			optedOut, err := w.optOutMgr.IsOptedOut(ctx, sub.Phone)
			if err == nil && optedOut {
				res.Skipped = true
				res.Reason = "subscriber opted out"
				results = append(results, res)
				w.recordDelivery(ctx, pet.PetID, token, distMiles, "skipped_optout", "")
				continue
			}
		}

		body := w.FormatAlertMessage(petName, pet.Location, distMiles)
		sendErr := w.provider.SendSMS(ctx, sub.Phone, body)
		if sendErr != nil {
			res.Skipped = false
			res.Delivered = false
			res.Reason = sendErr.Error()
			results = append(results, res)
			w.recordDelivery(ctx, pet.PetID, token, distMiles, "failed", sendErr.Error())
			continue
		}

		res.Delivered = true
		results = append(results, res)
		w.recordDelivery(ctx, pet.PetID, token, distMiles, "delivered", "")
	}

	return results, nil
}

func (w *BroadcastWorker) recordDelivery(
	ctx context.Context,
	petID, token string,
	distance float64,
	status, errMsg string,
) {
	if w.stateStore == nil {
		return
	}

	deliveryID := generateDeliveryID()
	record := BroadcastDeliveryRecord{
		DeliveryID:     deliveryID,
		PetID:          petID,
		TokenizedPhone: token,
		SentAt:         time.Now().UTC(),
		DistanceMiles:  distance,
		Status:         status,
		ErrorMessage:   errMsg,
	}

	data, err := json.Marshal(record)
	if err == nil {
		_ = w.stateStore.SaveState(ctx, store.SMSDeliveriesCollection, deliveryID, data)
	}
}

func generateDeliveryID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("sms_del_%s_%d", hex.EncodeToString(b[:8]), time.Now().UnixNano())
}
