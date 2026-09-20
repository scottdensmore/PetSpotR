package sms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/store"
)

// Standard opt-out and opt-in reply messages.
const (
	ReplyUnsubscribed = "You have unsubscribed from PetSpotR SMS alerts and will no longer receive messages. Reply START to resubscribe."
	ReplyResubscribed = "You have resubscribed to PetSpotR SMS alerts. Reply STOP to cancel at any time."
)

// OptOutRecord tracks opt-out status keyed by tokenized phone number.
type OptOutRecord struct {
	TokenizedPhone string    `json:"tokenizedPhone"`
	OptedOut       bool      `json:"optedOut"`
	Reason         string    `json:"reason,omitempty"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// OptOutManager manages subscriber opt-out states adhering to TCPA/CTIA compliance.
type OptOutManager struct {
	stateStore store.StateStore
	secretKey  []byte
}

// NewOptOutManager constructs an OptOutManager.
func NewOptOutManager(stateStore store.StateStore, secretKey []byte) *OptOutManager {
	if len(secretKey) == 0 {
		secretKey = DefaultSMSSalt()
	}
	return &OptOutManager{
		stateStore: stateStore,
		secretKey:  secretKey,
	}
}

// IsOptedOut checks whether the given phone number is currently opted out.
func (m *OptOutManager) IsOptedOut(ctx context.Context, phone string) (bool, error) {
	if m.stateStore == nil {
		return false, errors.New("state store is nil")
	}
	token := TokenizeNumber(phone, m.secretKey)
	raw, err := m.stateStore.GetState(ctx, store.SMSOptOutCollection, token)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query opt-out state: %w", err)
	}

	var rec OptOutRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return false, fmt.Errorf("decode opt-out record: %w", err)
	}

	return rec.OptedOut, nil
}

// OptOut flags a phone number as opted out.
func (m *OptOutManager) OptOut(ctx context.Context, phone string, reason string) error {
	if m.stateStore == nil {
		return errors.New("state store is nil")
	}
	token := TokenizeNumber(phone, m.secretKey)
	rec := OptOutRecord{
		TokenizedPhone: token,
		OptedOut:       true,
		Reason:         reason,
		UpdatedAt:      time.Now().UTC(),
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("encode opt-out record: %w", err)
	}
	return m.stateStore.SaveState(ctx, store.SMSOptOutCollection, token, data)
}

// OptIn clears the opt-out flag for a phone number.
func (m *OptOutManager) OptIn(ctx context.Context, phone string) error {
	if m.stateStore == nil {
		return errors.New("state store is nil")
	}
	token := TokenizeNumber(phone, m.secretKey)
	rec := OptOutRecord{
		TokenizedPhone: token,
		OptedOut:       false,
		Reason:         "resubscribed",
		UpdatedAt:      time.Now().UTC(),
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("encode opt-in record: %w", err)
	}
	return m.stateStore.SaveState(ctx, store.SMSOptOutCollection, token, data)
}

// HandleKeyword inspects keyword (e.g. STOP, START, UNSUBSCRIBE) and handles opt-out state changes.
func (m *OptOutManager) HandleKeyword(ctx context.Context, phone string, keyword string) (bool, string, error) {
	cmd, _ := ParseCommand(keyword)
	switch cmd {
	case CommandOptOut, CommandUnsubscribe:
		if err := m.OptOut(ctx, phone, strings.ToUpper(strings.TrimSpace(keyword))); err != nil {
			return true, "", err
		}
		return true, ReplyUnsubscribed, nil
	case CommandOptIn:
		if err := m.OptIn(ctx, phone); err != nil {
			return true, "", err
		}
		return true, ReplyResubscribed, nil
	default:
		return false, "", nil
	}
}
