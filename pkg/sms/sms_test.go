package sms_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/sms"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestNormalizeE164(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "standard 10-digit North American",
			input: "2065550199",
			want:  "+12065550199",
		},
		{
			name:  "formatted with parentheses and dashes",
			input: "(206) 555-0199",
			want:  "+12065550199",
		},
		{
			name:  "dotted format with whitespace",
			input: "  206.555.0199  ",
			want:  "+12065550199",
		},
		{
			name:  "11 digits starting with 1",
			input: "12065550199",
			want:  "+12065550199",
		},
		{
			name:  "already standard E.164 with +1",
			input: "+12065550199",
			want:  "+12065550199",
		},
		{
			name:  "international E.164 number",
			input: "+442071838750",
			want:  "+442071838750",
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "only spaces",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "letters only",
			input:   "abcdefg",
			wantErr: true,
		},
		{
			name:    "too short",
			input:   "12345",
			wantErr: true,
		},
		{
			name:    "too long (over 15 digits)",
			input:   "1234567890123456",
			wantErr: true,
		},
		{
			name:    "misplaced plus sign",
			input:   "+12345+6789",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sms.NormalizeE164(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizeE164(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("NormalizeE164(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTokenizeNumber(t *testing.T) {
	key := []byte("secret-salt-key-for-test-12345678")

	t.Run("consistent token for same number in different formats", func(t *testing.T) {
		token1 := sms.TokenizeNumber("+12065550199", key)
		token2 := sms.TokenizeNumber("(206) 555-0199", key)
		token3 := sms.TokenizeNumber("2065550199", key)

		if token1 == "" {
			t.Fatal("token should not be empty")
		}
		if len(token1) != 64 {
			t.Fatalf("expected 64-char hex string, got len %d (%q)", len(token1), token1)
		}
		if token1 != token2 {
			t.Errorf("token1 %q != token2 %q", token1, token2)
		}
		if token1 != token3 {
			t.Errorf("token1 %q != token3 %q", token1, token3)
		}
	})

	t.Run("different phone numbers produce different tokens", func(t *testing.T) {
		tokenA := sms.TokenizeNumber("+12065550199", key)
		tokenB := sms.TokenizeNumber("+12065550200", key)

		if tokenA == tokenB {
			t.Errorf("expected different tokens for different numbers, got same %q", tokenA)
		}
	})

	t.Run("different keys produce different tokens", func(t *testing.T) {
		key2 := []byte("different-key-1234567890123456")
		token1 := sms.TokenizeNumber("+12065550199", key)
		token2 := sms.TokenizeNumber("+12065550199", key2)

		if token1 == token2 {
			t.Errorf("expected different tokens for different keys, got same %q", token1)
		}
	})

	t.Run("zero PII leaked", func(t *testing.T) {
		token := sms.TokenizeNumber("+12065550199", key)
		if strings.Contains(token, "206") || strings.Contains(token, "5550199") {
			t.Errorf("token leaks phone digits: %s", token)
		}
	})
}

func TestMockProvider(t *testing.T) {
	mock := sms.NewMockProvider()

	ctx := context.Background()
	err := mock.SendSMS(ctx, "+12065550199", "Test alert message")
	if err != nil {
		t.Fatalf("SendSMS unexpected error: %v", err)
	}

	msgs := mock.SentMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(msgs))
	}
	if msgs[0].To != "+12065550199" {
		t.Errorf("expected To %q, got %q", "+12065550199", msgs[0].To)
	}
	if msgs[0].Body != "Test alert message" {
		t.Errorf("expected Body %q, got %q", "Test alert message", msgs[0].Body)
	}

	last, ok := mock.LastMessage()
	if !ok || last.Body != "Test alert message" {
		t.Errorf("LastMessage() = (%+v, %v)", last, ok)
	}

	// Error simulation
	expectedErr := errors.New("carrier network timeout")
	mock.SetError(expectedErr)
	if err := mock.SendSMS(ctx, "+12065550199", "Fail message"); !errors.Is(err, expectedErr) {
		t.Fatalf("expected injected error %v, got %v", expectedErr, err)
	}

	// Reset
	mock.Reset()
	if len(mock.SentMessages()) != 0 {
		t.Errorf("expected 0 messages after reset, got %d", len(mock.SentMessages()))
	}
	if err := mock.SendSMS(ctx, "+12065550199", "After reset"); err != nil {
		t.Errorf("SendSMS failed after reset: %v", err)
	}
}

func TestParseCommand(t *testing.T) {
	tests := []struct {
		body        string
		wantCommand sms.CommandType
		wantArg     string
	}{
		{
			body:        "CLAIM SEC-01",
			wantCommand: sms.CommandClaim,
			wantArg:     "SEC-01",
		},
		{
			body:        "claim sec-north-4",
			wantCommand: sms.CommandClaim,
			wantArg:     "sec-north-4",
		},
		{
			body:        "SIGHTED Near Pine St dog running east",
			wantCommand: sms.CommandSighted,
			wantArg:     "Near Pine St dog running east",
		},
		{
			body:        "sighted spotted behind grocery store",
			wantCommand: sms.CommandSighted,
			wantArg:     "spotted behind grocery store",
		},
		{
			body:        "STATUS",
			wantCommand: sms.CommandStatus,
			wantArg:     "",
		},
		{
			body:        "  status  ",
			wantCommand: sms.CommandStatus,
			wantArg:     "",
		},
		{
			body:        "STOP",
			wantCommand: sms.CommandOptOut,
			wantArg:     "",
		},
		{
			body:        "stop",
			wantCommand: sms.CommandOptOut,
			wantArg:     "",
		},
		{
			body:        "UNSUBSCRIBE",
			wantCommand: sms.CommandUnsubscribe,
			wantArg:     "",
		},
		{
			body:        "unsubscribe",
			wantCommand: sms.CommandUnsubscribe,
			wantArg:     "",
		},
		{
			body:        "START",
			wantCommand: sms.CommandOptIn,
			wantArg:     "",
		},
		{
			body:        "start",
			wantCommand: sms.CommandOptIn,
			wantArg:     "",
		},
		{
			body:        "HELLO WORLD",
			wantCommand: sms.CommandUnknown,
			wantArg:     "HELLO WORLD",
		},
		{
			body:        "",
			wantCommand: sms.CommandUnknown,
			wantArg:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			cmd, arg := sms.ParseCommand(tt.body)
			if cmd != tt.wantCommand {
				t.Errorf("ParseCommand(%q) cmd = %v, want %v", tt.body, cmd, tt.wantCommand)
			}
			if arg != tt.wantArg {
				t.Errorf("ParseCommand(%q) arg = %q, want %q", tt.body, arg, tt.wantArg)
			}
		})
	}
}

func TestOptOutManager(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	secret := []byte("opt-out-secret-key-test")
	mgr := sms.NewOptOutManager(st, secret)

	phone := "+12065550199"

	// 1. Initially not opted out
	optedOut, err := mgr.IsOptedOut(ctx, phone)
	if err != nil {
		t.Fatalf("IsOptedOut error: %v", err)
	}
	if optedOut {
		t.Error("new phone should not be opted out")
	}

	// 2. STOP keyword marks opted out
	handled, reply, err := mgr.HandleKeyword(ctx, phone, "STOP")
	if err != nil {
		t.Fatalf("HandleKeyword error: %v", err)
	}
	if !handled {
		t.Error("HandleKeyword should return handled = true for STOP")
	}
	if !strings.Contains(strings.ToLower(reply), "unsubscribed") {
		t.Errorf("expected reply to contain unsubscribed, got: %s", reply)
	}

	// 3. Verify opted out is now true
	optedOut, err = mgr.IsOptedOut(ctx, phone)
	if err != nil {
		t.Fatalf("IsOptedOut error: %v", err)
	}
	if !optedOut {
		t.Error("expected phone to be opted out after STOP")
	}

	// 4. Verify data in store uses tokenized key (zero PII)
	token := sms.TokenizeNumber(phone, secret)
	raw, err := st.GetState(ctx, store.SMSOptOutCollection, token)
	if err != nil {
		t.Fatalf("failed to retrieve opt-out record by token: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("opt-out record should not be empty")
	}
	if strings.Contains(string(raw), "2065550199") {
		t.Errorf("opt-out record in store contains plaintext phone: %s", string(raw))
	}

	// 5. START keyword resubscribes
	handled, reply, err = mgr.HandleKeyword(ctx, phone, "START")
	if err != nil {
		t.Fatalf("HandleKeyword START error: %v", err)
	}
	if !handled {
		t.Error("HandleKeyword should return handled = true for START")
	}
	if !strings.Contains(strings.ToLower(reply), "resubscribed") && !strings.Contains(strings.ToLower(reply), "subscribed") {
		t.Errorf("expected reply to mention subscribed, got: %s", reply)
	}

	// 6. Verify opted out is now false
	optedOut, err = mgr.IsOptedOut(ctx, phone)
	if err != nil {
		t.Fatalf("IsOptedOut error: %v", err)
	}
	if optedOut {
		t.Error("expected phone to NOT be opted out after START")
	}

	// 7. UNSUBSCRIBE keyword also opts out
	handled, _, err = mgr.HandleKeyword(ctx, phone, "UNSUBSCRIBE")
	if err != nil || !handled {
		t.Fatalf("UNSUBSCRIBE failed: handled=%v err=%v", handled, err)
	}
	optedOut, _ = mgr.IsOptedOut(ctx, phone)
	if !optedOut {
		t.Error("expected opted out after UNSUBSCRIBE")
	}

	// 8. Non-opt keyword returns handled = false
	handled, _, err = mgr.HandleKeyword(ctx, phone, "CLAIM SEC-1")
	if err != nil {
		t.Fatalf("unexpected error on non-opt keyword: %v", err)
	}
	if handled {
		t.Error("non-opt keyword should return handled = false")
	}
}

func TestDefaultSMSSalt(t *testing.T) {
	orig := os.Getenv("PETSPOTR_SMS_SALT")
	defer func() {
		if orig != "" {
			_ = os.Setenv("PETSPOTR_SMS_SALT", orig)
		} else {
			_ = os.Unsetenv("PETSPOTR_SMS_SALT")
		}
	}()

	_ = os.Unsetenv("PETSPOTR_SMS_SALT")
	defaultSalt := sms.DefaultSMSSalt()
	if string(defaultSalt) != "petspotr-default-sms-salt-2026" {
		t.Errorf("DefaultSMSSalt = %q, want petspotr-default-sms-salt-2026", string(defaultSalt))
	}

	_ = os.Setenv("PETSPOTR_SMS_SALT", "custom-production-salt-secret")
	customSalt := sms.DefaultSMSSalt()
	if string(customSalt) != "custom-production-salt-secret" {
		t.Errorf("DefaultSMSSalt = %q, want custom-production-salt-secret", string(customSalt))
	}

	// Verify TokenizeNumber uses the custom salt when secretKey is nil
	tokenWithEnv := sms.TokenizeNumber("+12065550199", nil)
	expectedToken := sms.TokenizeNumber("+12065550199", []byte("custom-production-salt-secret"))
	if tokenWithEnv != expectedToken {
		t.Errorf("TokenizeNumber with nil key = %q, want %q", tokenWithEnv, expectedToken)
	}
}
