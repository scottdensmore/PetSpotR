package runtimeconfig

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestEnvironmentParsing(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Environment
		wantErr bool
	}{
		{name: "empty defaults to development", input: "", want: EnvironmentDevelopment},
		{name: "whitespace defaults to development", input: "   ", want: EnvironmentDevelopment},
		{name: "development exact", input: "development", want: EnvironmentDevelopment},
		{name: "dev alias", input: "dev", want: EnvironmentDevelopment},
		{name: "local-emulator exact", input: "local-emulator", want: EnvironmentLocalEmulator},
		{name: "local alias", input: "local", want: EnvironmentLocalEmulator},
		{name: "emulator alias", input: "emulator", want: EnvironmentLocalEmulator},
		{name: "staging exact", input: "staging", want: EnvironmentStaging},
		{name: "stage alias", input: "stage", want: EnvironmentStaging},
		{name: "production exact", input: "production", want: EnvironmentProduction},
		{name: "prod alias", input: "prod", want: EnvironmentProduction},
		{name: "case-insensitive and trimmed", input: "  PRODUCTION  ", want: EnvironmentProduction},
		{name: "invalid environment", input: "unknown_tier", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseEnvironment(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseEnvironment(%q) expected error, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseEnvironment(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ParseEnvironment(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestEnvironmentLookupPriority(t *testing.T) {
	// PETSPOTR_ENV takes precedence over ENVIRONMENT
	envLookup := func(k string) string {
		switch k {
		case "PETSPOTR_ENV":
			return "production"
		case "ENVIRONMENT":
			return "development"
		default:
			return ""
		}
	}
	cfg, err := LoadServiceConfig("test-service", func(k string) string {
		if k == "STORAGE_BUCKET_NAME" {
			return "my-bucket"
		}
		if k == "GOOGLE_CLOUD_PROJECT" {
			return "my-proj"
		}
		return envLookup(k)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Environment != EnvironmentProduction {
		t.Errorf("expected PETSPOTR_ENV precedence EnvironmentProduction, got %q", cfg.Environment)
	}

	// Falls back to ENVIRONMENT if PETSPOTR_ENV unset
	envFallback := func(k string) string {
		if k == "ENVIRONMENT" {
			return "staging"
		}
		if k == "STORAGE_BUCKET_NAME" {
			return "my-bucket"
		}
		if k == "GOOGLE_CLOUD_PROJECT" {
			return "my-proj"
		}
		return ""
	}
	cfg2, err := LoadServiceConfig("test-service", envFallback)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg2.Environment != EnvironmentStaging {
		t.Errorf("expected ENVIRONMENT fallback EnvironmentStaging, got %q", cfg2.Environment)
	}
}

func TestPortParsing(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "empty defaults to 8080", input: "", want: 8080},
		{name: "whitespace defaults to 8080", input: "   ", want: 8080},
		{name: "valid standard port", input: "8080", want: 8080},
		{name: "valid custom port", input: "8082", want: 8082},
		{name: "valid port 1", input: "1", want: 1},
		{name: "valid max port 65535", input: "65535", want: 65535},
		{name: "zero port invalid", input: "0", wantErr: true},
		{name: "negative port invalid", input: "-1", wantErr: true},
		{name: "large negative port invalid", input: "-8080", wantErr: true},
		{name: "port exceeding 65535", input: "65536", wantErr: true},
		{name: "port 70000", input: "70000", wantErr: true},
		{name: "non-numeric characters", input: "8080a", wantErr: true},
		{name: "non-numeric letters", input: "http", wantErr: true},
		{name: "decimal port", input: "8080.5", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParsePort(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParsePort(%q) expected error, got nil (%d)", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePort(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ParsePort(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestProductionValidationFailures(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ServiceConfig
		wantErr string
	}{
		{
			name: "missing ProjectID in production",
			cfg: ServiceConfig{
				ServiceName:   "lostpet-service",
				Port:          8080,
				Environment:   EnvironmentProduction,
				StorageBucket: "petspotr-prod-images",
			},
			wantErr: "ProjectID is required",
		},
		{
			name: "placeholder ProjectID in production",
			cfg: ServiceConfig{
				ServiceName:   "lostpet-service",
				Port:          8080,
				Environment:   EnvironmentProduction,
				ProjectID:     "your-project-id",
				StorageBucket: "petspotr-prod-images",
			},
			wantErr: "placeholder in production",
		},
		{
			name: "changeme ProjectID in staging",
			cfg: ServiceConfig{
				ServiceName:   "lostpet-service",
				Port:          8080,
				Environment:   EnvironmentStaging,
				ProjectID:     "changeme",
				StorageBucket: "petspotr-staging-images",
			},
			wantErr: "placeholder in staging",
		},
		{
			name: "missing StorageBucket in production",
			cfg: ServiceConfig{
				ServiceName: "lostpet-service",
				Port:        8080,
				Environment: EnvironmentProduction,
				ProjectID:   "petspotr-prod",
			},
			wantErr: "StorageBucket is required",
		},
		{
			name: "placeholder StorageBucket in production",
			cfg: ServiceConfig{
				ServiceName:   "lostpet-service",
				Port:          8080,
				Environment:   EnvironmentProduction,
				ProjectID:     "petspotr-prod",
				StorageBucket: "your-bucket",
			},
			wantErr: "placeholder in production",
		},
		{
			name: "invalid port in production",
			cfg: ServiceConfig{
				ServiceName:   "lostpet-service",
				Port:          0,
				Environment:   EnvironmentProduction,
				ProjectID:     "petspotr-prod",
				StorageBucket: "petspotr-prod-images",
			},
			wantErr: "port 0 out of valid range",
		},
		{
			name: "firestore emulator configured in production",
			cfg: ServiceConfig{
				ServiceName:           "lostpet-service",
				Port:                  8080,
				Environment:           EnvironmentProduction,
				ProjectID:             "petspotr-prod",
				StorageBucket:         "petspotr-prod-images",
				FirestoreEmulatorHost: "localhost:8085",
			},
			wantErr: "FIRESTORE_EMULATOR_HOST must not be set in production",
		},
		{
			name: "pubsub emulator configured in staging",
			cfg: ServiceConfig{
				ServiceName:        "lostpet-service",
				Port:               8080,
				Environment:        EnvironmentStaging,
				ProjectID:          "petspotr-stage",
				StorageBucket:      "petspotr-stage-images",
				PubSubEmulatorHost: "localhost:8086",
			},
			wantErr: "PUBSUB_EMULATOR_HOST must not be set in staging",
		},
		{
			name: "storage emulator configured in production",
			cfg: ServiceConfig{
				ServiceName:         "lostpet-service",
				Port:                8080,
				Environment:         EnvironmentProduction,
				ProjectID:           "petspotr-prod",
				StorageBucket:       "petspotr-prod-images",
				StorageEmulatorHost: "http://localhost:9023",
			},
			wantErr: "STORAGE_EMULATOR_HOST must not be set in production",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if err == nil {
				t.Fatalf("expected validation failure, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain expected substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestProductionValidationPasses(t *testing.T) {
	cfg := ServiceConfig{
		ServiceName:       "lostpet-service",
		Port:              8080,
		Environment:       EnvironmentProduction,
		ProjectID:         "petspotr-prod-12345",
		StorageBucket:     "petspotr-prod-petimages",
		FirestoreDatabase: "(default)",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected production validation to pass, got: %v", err)
	}

	stagingCfg := ServiceConfig{
		ServiceName:       "web-frontend",
		Port:              8082,
		Environment:       EnvironmentStaging,
		ProjectID:         "petspotr-staging-12345",
		StorageBucket:     "petspotr-staging-petimages",
		FirestoreDatabase: "(default)",
	}
	if err := stagingCfg.Validate(); err != nil {
		t.Fatalf("expected staging validation to pass, got: %v", err)
	}
}

func TestLocalEmulatorValidationWithEmulatorVariables(t *testing.T) {
	lookup := func(k string) string {
		switch k {
		case "PETSPOTR_ENV":
			return "local-emulator"
		case "PORT":
			return "8080"
		case "FIRESTORE_EMULATOR_HOST":
			return "localhost:8085"
		case "PUBSUB_EMULATOR_HOST":
			return "localhost:8086"
		case "STORAGE_EMULATOR_HOST":
			return "http://localhost:9023"
		default:
			return ""
		}
	}

	cfg, err := LoadServiceConfig("lostpet-service", lookup)
	if err != nil {
		t.Fatalf("LoadServiceConfig failed in local-emulator mode: %v", err)
	}

	if cfg.Environment != EnvironmentLocalEmulator {
		t.Errorf("expected EnvironmentLocalEmulator, got %s", cfg.Environment)
	}
	if cfg.ProjectID != "petspotr-local" {
		t.Errorf("expected default projectID petspotr-local, got %s", cfg.ProjectID)
	}
	if cfg.StorageBucket != "petspotr-local-petimages" {
		t.Errorf("expected default bucket petspotr-local-petimages, got %s", cfg.StorageBucket)
	}
	if cfg.FirestoreEmulatorHost != "localhost:8085" {
		t.Errorf("expected firestore emulator host localhost:8085, got %s", cfg.FirestoreEmulatorHost)
	}
	if cfg.PubSubEmulatorHost != "localhost:8086" {
		t.Errorf("expected pubsub emulator host localhost:8086, got %s", cfg.PubSubEmulatorHost)
	}
	if cfg.StorageEmulatorHost != "http://localhost:9023" {
		t.Errorf("expected storage emulator host http://localhost:9023, got %s", cfg.StorageEmulatorHost)
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected emulator config to validate successfully, got: %v", err)
	}
}

func TestSensitiveStringRedaction(t *testing.T) {
	secretVal := "super-sensitive-api-token-xyz123"
	secret := SecretString(secretVal)

	// String() redaction
	if got := secret.String(); got != RedactedPlaceholder {
		t.Errorf("secret.String() = %q, want %q", got, RedactedPlaceholder)
	}

	// GoString() redaction
	if got := fmt.Sprintf("%#v", secret); strings.Contains(got, secretVal) {
		t.Errorf("secret GoString representation leaked secret: %s", got)
	}

	// String formatting %s and %v
	if got := fmt.Sprintf("value=%s", secret); got != "value="+RedactedPlaceholder {
		t.Errorf("fmt.Sprintf(%%s, secret) = %q, want %q", got, RedactedPlaceholder)
	}
	if got := fmt.Sprintf("value=%v", secret); got != "value="+RedactedPlaceholder {
		t.Errorf("fmt.Sprintf(%%v, secret) = %q, want %q", got, RedactedPlaceholder)
	}

	// JSON serialization redaction
	data, err := json.Marshal(secret)
	if err != nil {
		t.Fatalf("json.Marshal(secret) failed: %v", err)
	}
	if strings.Contains(string(data), secretVal) {
		t.Errorf("json.Marshal leaked secret: %s", string(data))
	}
	if string(data) != fmt.Sprintf("%q", RedactedPlaceholder) {
		t.Errorf("json.Marshal output = %s, want %q", string(data), RedactedPlaceholder)
	}

	// Expose retrieves original plaintext
	if got := secret.Expose(); got != secretVal {
		t.Errorf("secret.Expose() = %q, want %q", got, secretVal)
	}

	// Empty secret redaction
	emptySecret := SecretString("")
	if got := emptySecret.String(); got != "" {
		t.Errorf("emptySecret.String() = %q, want empty string", got)
	}
	if got := emptySecret.Expose(); got != "" {
		t.Errorf("emptySecret.Expose() = %q, want empty string", got)
	}

	// Redact helper function
	if got := Redact("secret-key"); got != RedactedPlaceholder {
		t.Errorf("Redact(\"secret-key\") = %q, want %q", got, RedactedPlaceholder)
	}
	if got := Redact(""); got != "" {
		t.Errorf("Redact(\"\") = %q, want empty string", got)
	}
}

func TestWebFrontendConfig(t *testing.T) {
	t.Run("development defaults", func(t *testing.T) {
		cfg, err := LoadWebFrontendConfig(func(k string) string { return "" })
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Service.ServiceName != "web-frontend" {
			t.Errorf("expected service name web-frontend, got %s", cfg.Service.ServiceName)
		}
		if cfg.SessionSecret.Expose() == "" {
			t.Errorf("expected dev session secret default")
		}
		if cfg.SessionCookieName != "petspotr_session" {
			t.Errorf("expected petspotr_session, got %s", cfg.SessionCookieName)
		}
		if cfg.RateLimit.RequestsPerMinute != 60 || cfg.RateLimit.Burst != 20 {
			t.Errorf("unexpected rate limit defaults: %+v", cfg.RateLimit)
		}
	})

	t.Run("production requires session secret", func(t *testing.T) {
		_, err := LoadWebFrontendConfig(func(k string) string {
			switch k {
			case "PETSPOTR_ENV":
				return "production"
			case "GOOGLE_CLOUD_PROJECT":
				return "petspotr-prod"
			case "STORAGE_BUCKET_NAME":
				return "petspotr-prod-images"
			default:
				return ""
			}
		})
		if err == nil || !strings.Contains(err.Error(), "SessionSecret is required") {
			t.Fatalf("expected SessionSecret required error, got: %v", err)
		}
	})

	t.Run("production rejects placeholder session secret", func(t *testing.T) {
		_, err := LoadWebFrontendConfig(func(k string) string {
			switch k {
			case "PETSPOTR_ENV":
				return "production"
			case "GOOGLE_CLOUD_PROJECT":
				return "petspotr-prod"
			case "STORAGE_BUCKET_NAME":
				return "petspotr-prod-images"
			case "SESSION_SECRET":
				return "your-secret"
			default:
				return ""
			}
		})
		if err == nil || !strings.Contains(err.Error(), "placeholder in production") {
			t.Fatalf("expected placeholder session secret error, got: %v", err)
		}
	})

	t.Run("production log string redacts secrets", func(t *testing.T) {
		secretKey := "super-top-secret-session-key-32chars"
		cfg, err := LoadWebFrontendConfig(func(k string) string {
			switch k {
			case "PETSPOTR_ENV":
				return "production"
			case "GOOGLE_CLOUD_PROJECT":
				return "petspotr-prod"
			case "STORAGE_BUCKET_NAME":
				return "petspotr-prod-images"
			case "SESSION_SECRET":
				return secretKey
			default:
				return ""
			}
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		logStr := cfg.String()
		if strings.Contains(logStr, secretKey) {
			t.Errorf("log string leaked session secret: %s", logStr)
		}
		if !strings.Contains(logStr, RedactedPlaceholder) {
			t.Errorf("log string missing redacted placeholder: %s", logStr)
		}
	})
}

func TestLostPetConfig(t *testing.T) {
	t.Run("development defaults", func(t *testing.T) {
		cfg, err := LoadLostPetConfig(func(k string) string { return "" })
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.StoreCollection != "lost_pets" {
			t.Errorf("StoreCollection = %s, want lost_pets", cfg.StoreCollection)
		}
		if cfg.OutboxCollection != "lost_pets_outbox" {
			t.Errorf("OutboxCollection = %s, want lost_pets_outbox", cfg.OutboxCollection)
		}
		if cfg.PubSubTopic != "lost-pets" {
			t.Errorf("PubSubTopic = %s, want lost-pets", cfg.PubSubTopic)
		}
	})

	t.Run("production requires non-placeholder topic", func(t *testing.T) {
		_, err := LoadLostPetConfig(func(k string) string {
			switch k {
			case "PETSPOTR_ENV":
				return "production"
			case "GOOGLE_CLOUD_PROJECT":
				return "petspotr-prod"
			case "STORAGE_BUCKET_NAME":
				return "petspotr-prod-images"
			case "LOST_PET_TOPIC":
				return "your-topic"
			default:
				return ""
			}
		})
		if err == nil || !strings.Contains(err.Error(), "PubSubTopic is required and cannot be empty or placeholder") {
			t.Fatalf("expected placeholder topic error, got: %v", err)
		}
	})
}

func TestFoundPetConfig(t *testing.T) {
	t.Run("development defaults", func(t *testing.T) {
		cfg, err := LoadFoundPetConfig(func(k string) string { return "" })
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.StoreCollection != "found_pets" {
			t.Errorf("StoreCollection = %s, want found_pets", cfg.StoreCollection)
		}
		if cfg.OutboxCollection != "found_pets_outbox" {
			t.Errorf("OutboxCollection = %s, want found_pets_outbox", cfg.OutboxCollection)
		}
		if cfg.PubSubTopic != "found-pets" {
			t.Errorf("PubSubTopic = %s, want found-pets", cfg.PubSubTopic)
		}
	})

	t.Run("production requires non-placeholder collection", func(t *testing.T) {
		_, err := LoadFoundPetConfig(func(k string) string {
			switch k {
			case "PETSPOTR_ENV":
				return "production"
			case "GOOGLE_CLOUD_PROJECT":
				return "petspotr-prod"
			case "STORAGE_BUCKET_NAME":
				return "petspotr-prod-images"
			case "FOUND_PET_COLLECTION":
				return "changeme"
			default:
				return ""
			}
		})
		if err == nil || !strings.Contains(err.Error(), "StoreCollection is required and cannot be empty or placeholder") {
			t.Fatalf("expected placeholder collection error, got: %v", err)
		}
	})
}

func TestPetMatcherConfig(t *testing.T) {
	t.Run("development defaults", func(t *testing.T) {
		cfg, err := LoadPetMatcherConfig(func(k string) string { return "" })
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.OllamaHost != "http://localhost:11434" {
			t.Errorf("OllamaHost = %s, want http://localhost:11434", cfg.OllamaHost)
		}
		if cfg.OllamaModel != "gemma4" {
			t.Errorf("OllamaModel = %s, want gemma4", cfg.OllamaModel)
		}
		if cfg.MatchTopic != "pet-matches" {
			t.Errorf("MatchTopic = %s, want pet-matches", cfg.MatchTopic)
		}
		if cfg.FoundSubscription == "" || cfg.LostSubscription == "" {
			t.Errorf("expected default subscriptions, got found=%s, lost=%s", cfg.FoundSubscription, cfg.LostSubscription)
		}
	})

	t.Run("subscriptions must be distinct", func(t *testing.T) {
		_, err := LoadPetMatcherConfig(func(k string) string {
			switch k {
			case "PUBSUB_PUSH_SUBSCRIPTION":
				return "projects/petspotr/subscriptions/same-sub"
			case "PUBSUB_LOST_SUBSCRIPTION":
				return "projects/petspotr/subscriptions/same-sub"
			default:
				return ""
			}
		})
		if err == nil || !strings.Contains(err.Error(), "must be distinct") {
			t.Fatalf("expected distinct subscriptions error, got: %v", err)
		}
	})

	t.Run("production rejects DevPushToken and requires PushServiceAccount", func(t *testing.T) {
		_, err := LoadPetMatcherConfig(func(k string) string {
			switch k {
			case "PETSPOTR_ENV":
				return "production"
			case "GOOGLE_CLOUD_PROJECT":
				return "petspotr-prod"
			case "STORAGE_BUCKET_NAME":
				return "petspotr-prod-images"
			case "PUBSUB_PUSH_SUBSCRIPTION":
				return "projects/petspotr-prod/subscriptions/found-sub"
			case "PUBSUB_LOST_SUBSCRIPTION":
				return "projects/petspotr-prod/subscriptions/lost-sub"
			case "PUBSUB_PUSH_DEV_TOKEN":
				return "some-dev-token"
			default:
				return ""
			}
		})
		if err == nil || !strings.Contains(err.Error(), "PUBSUB_PUSH_DEV_TOKEN must not be set in production") {
			t.Fatalf("expected dev token rejection, got: %v", err)
		}
	})

	t.Run("production passes with valid config", func(t *testing.T) {
		cfg, err := LoadPetMatcherConfig(func(k string) string {
			switch k {
			case "PETSPOTR_ENV":
				return "production"
			case "GOOGLE_CLOUD_PROJECT":
				return "petspotr-prod"
			case "STORAGE_BUCKET_NAME":
				return "petspotr-prod-images"
			case "OLLAMA_HOST":
				return "http://ollama-prod:11434"
			case "OLLAMA_MODEL":
				return "gemma4:e2b"
			case "MATCH_TOPIC":
				return "pet-matches-prod"
			case "PUBSUB_PUSH_SUBSCRIPTION":
				return "projects/petspotr-prod/subscriptions/found-sub"
			case "PUBSUB_LOST_SUBSCRIPTION":
				return "projects/petspotr-prod/subscriptions/lost-sub"
			case "PUBSUB_PUSH_SERVICE_ACCOUNT":
				return "matcher-invoker@petspotr-prod.iam.gserviceaccount.com"
			default:
				return ""
			}
		})
		if err != nil {
			t.Fatalf("expected valid production config to pass, got: %v", err)
		}
		if cfg.Service.Environment != EnvironmentProduction {
			t.Errorf("expected production, got %s", cfg.Service.Environment)
		}
	})
}

func TestNotificationConfig(t *testing.T) {
	t.Run("development defaults with mock provider", func(t *testing.T) {
		cfg, err := LoadNotificationConfig(func(k string) string { return "" })
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Email.SendGridFromEmail != "alerts@petspotr.io" {
			t.Errorf("SendGridFromEmail = %s, want alerts@petspotr.io", cfg.Email.SendGridFromEmail)
		}
		if cfg.SMS.DefaultCountryCode != "+1" {
			t.Errorf("DefaultCountryCode = %s, want +1", cfg.SMS.DefaultCountryCode)
		}
		if cfg.Push.VAPIDSubject != "mailto:alerts@petspotr.io" {
			t.Errorf("VAPIDSubject = %s, want mailto:alerts@petspotr.io", cfg.Push.VAPIDSubject)
		}
	})

	t.Run("production requires provider settings", func(t *testing.T) {
		_, err := LoadNotificationConfig(func(k string) string {
			switch k {
			case "PETSPOTR_ENV":
				return "production"
			case "GOOGLE_CLOUD_PROJECT":
				return "petspotr-prod"
			case "PUBSUB_PUSH_SUBSCRIPTION":
				return "projects/petspotr-prod/subscriptions/match-sub"
			case "PUBSUB_LOST_SUBSCRIPTION":
				return "projects/petspotr-prod/subscriptions/lost-sub"
			case "PUBSUB_PUSH_SERVICE_ACCOUNT":
				return "notif-invoker@petspotr-prod.iam.gserviceaccount.com"
			default:
				return ""
			}
		})
		if err == nil || !strings.Contains(err.Error(), "at least one notification provider") {
			t.Fatalf("expected provider configuration error, got: %v", err)
		}
	})

	t.Run("production passes with valid SendGrid credentials", func(t *testing.T) {
		cfg, err := LoadNotificationConfig(func(k string) string {
			switch k {
			case "PETSPOTR_ENV":
				return "production"
			case "GOOGLE_CLOUD_PROJECT":
				return "petspotr-prod"
			case "SENDGRID_API_KEY":
				return "SG.valid-sendgrid-prod-api-key-9999"
			case "PUBSUB_PUSH_SUBSCRIPTION":
				return "projects/petspotr-prod/subscriptions/match-sub"
			case "PUBSUB_LOST_SUBSCRIPTION":
				return "projects/petspotr-prod/subscriptions/lost-sub"
			case "PUBSUB_PUSH_SERVICE_ACCOUNT":
				return "notif-invoker@petspotr-prod.iam.gserviceaccount.com"
			default:
				return ""
			}
		})
		if err != nil {
			t.Fatalf("expected valid production notification config to pass, got: %v", err)
		}
		// Check that log string redacts the SendGrid API key
		logStr := cfg.String()
		if strings.Contains(logStr, "SG.valid-sendgrid-prod-api-key-9999") {
			t.Errorf("NotificationConfig.String() leaked SendGrid key: %s", logStr)
		}
		if !strings.Contains(logStr, RedactedPlaceholder) {
			t.Errorf("NotificationConfig.String() missing [REDACTED]: %s", logStr)
		}
	})

	t.Run("production rejects placeholder provider credentials", func(t *testing.T) {
		_, err := LoadNotificationConfig(func(k string) string {
			switch k {
			case "PETSPOTR_ENV":
				return "production"
			case "GOOGLE_CLOUD_PROJECT":
				return "petspotr-prod"
			case "SENDGRID_API_KEY":
				return "your-api-key"
			case "PUBSUB_PUSH_SUBSCRIPTION":
				return "projects/petspotr-prod/subscriptions/match-sub"
			case "PUBSUB_LOST_SUBSCRIPTION":
				return "projects/petspotr-prod/subscriptions/lost-sub"
			case "PUBSUB_PUSH_SERVICE_ACCOUNT":
				return "notif-invoker@petspotr-prod.iam.gserviceaccount.com"
			default:
				return ""
			}
		})
		if err == nil || !strings.Contains(err.Error(), "placeholder") {
			t.Fatalf("expected placeholder rejection, got: %v", err)
		}
	})
}
