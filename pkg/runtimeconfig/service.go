package runtimeconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Environment represents the operational runtime tier of a PetSpotR service.
type Environment string

const (
	EnvironmentDevelopment   Environment = "development"
	EnvironmentLocalEmulator Environment = "local-emulator"
	EnvironmentStaging       Environment = "staging"
	EnvironmentProduction    Environment = "production"
)

// RedactedPlaceholder is the string shown in place of sensitive credential values.
const RedactedPlaceholder = "[REDACTED]"

// SecretString encapsulates sensitive strings (passwords, tokens, API keys)
// and implements fmt.Stringer and fmt.GoStringer to avoid leaking values into logs.
type SecretString string

// String returns the redacted placeholder for non-empty secrets, or empty string.
func (s SecretString) String() string {
	if s == "" {
		return ""
	}
	return RedactedPlaceholder
}

// GoString formats the SecretString safely for %#v representations.
func (s SecretString) GoString() string {
	if s == "" {
		return `""`
	}
	return fmt.Sprintf("%q", RedactedPlaceholder)
}

// MarshalJSON ensures SecretString is not exposed when serializing to JSON.
func (s SecretString) MarshalJSON() ([]byte, error) {
	if s == "" {
		return json.Marshal("")
	}
	return json.Marshal(RedactedPlaceholder)
}

// Expose returns the underlying sensitive plaintext. Use strictly when dispatching
// authenticated requests to external providers or crypto engines.
func (s SecretString) Expose() string {
	return string(s)
}

// Redact returns RedactedPlaceholder for non-empty strings, or empty string.
func Redact(val string) string {
	if strings.TrimSpace(val) == "" {
		return ""
	}
	return RedactedPlaceholder
}

// ParseEnvironment parses a string into an Environment enum, defaulting to EnvironmentDevelopment.
func ParseEnvironment(raw string) (Environment, error) {
	val := strings.ToLower(strings.TrimSpace(raw))
	if val == "" {
		return EnvironmentDevelopment, nil
	}
	switch val {
	case string(EnvironmentDevelopment), "dev":
		return EnvironmentDevelopment, nil
	case string(EnvironmentLocalEmulator), "local", "emulator":
		return EnvironmentLocalEmulator, nil
	case string(EnvironmentStaging), "stage":
		return EnvironmentStaging, nil
	case string(EnvironmentProduction), "prod":
		return EnvironmentProduction, nil
	default:
		return "", fmt.Errorf("unknown environment %q: must be one of development, local-emulator, staging, production", raw)
	}
}

// ParsePort parses a port string and validates that it falls within 1..65535, defaulting to 8080 when empty.
func ParsePort(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 8080, nil
	}
	p, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid PORT %q: must be a numeric integer", raw)
	}
	if p < 1 || p > 65535 {
		return 0, fmt.Errorf("invalid PORT %d: must be between 1 and 65535", p)
	}
	return p, nil
}

// isPlaceholder detects whether a value is empty or contains an unconfigured placeholder token.
func isPlaceholder(val string) bool {
	clean := strings.ToLower(strings.TrimSpace(val))
	if clean == "" {
		return true
	}
	placeholders := []string{
		"placeholder", "changeme", "change-me", "todo", "xxx",
		"your-project-id", "your-bucket", "your-topic", "your-subscription",
		"your-api-key", "your-secret", "your-token", "set-me", "fixme",
		"default", "undefined", "none", "null", "tbd",
	}
	for _, p := range placeholders {
		if clean == p {
			return true
		}
	}
	if strings.HasPrefix(clean, "your-") {
		return true
	}
	if strings.HasPrefix(clean, "<") && strings.HasSuffix(clean, ">") {
		return true
	}
	return false
}

// ServiceConfig holds shared operational configuration across all PetSpotR microservices.
type ServiceConfig struct {
	ServiceName           string
	Port                  int
	Environment           Environment
	ProjectID             string
	StorageBucket         string
	FirestoreDatabase     string
	FirestoreEmulatorHost string
	PubSubEmulatorHost    string
	StorageEmulatorHost   string
}

// Validate verifies that ServiceConfig complies with environment rules.
func (c ServiceConfig) Validate() error {
	var errs []error

	if c.Port < 1 || c.Port > 65535 {
		errs = append(errs, fmt.Errorf("port %d out of valid range (1..65535)", c.Port))
	}

	switch c.Environment {
	case EnvironmentDevelopment, EnvironmentLocalEmulator, EnvironmentStaging, EnvironmentProduction:
	default:
		errs = append(errs, fmt.Errorf("invalid environment %q", c.Environment))
	}

	isProdLike := c.Environment == EnvironmentProduction || c.Environment == EnvironmentStaging

	if isProdLike {
		if c.ProjectID == "" || isPlaceholder(c.ProjectID) {
			errs = append(errs, fmt.Errorf("ProjectID is required and cannot be empty or placeholder in %s", c.Environment))
		}
		// StorageBucket is strictly required for production unless the service explicitly does not require storage.
		if c.ServiceName != "notification-service" {
			if c.StorageBucket == "" || isPlaceholder(c.StorageBucket) {
				errs = append(errs, fmt.Errorf("StorageBucket is required and cannot be empty or placeholder in %s", c.Environment))
			}
		}
		if c.FirestoreEmulatorHost != "" {
			errs = append(errs, fmt.Errorf("FIRESTORE_EMULATOR_HOST must not be set in %s", c.Environment))
		}
		if c.PubSubEmulatorHost != "" {
			errs = append(errs, fmt.Errorf("PUBSUB_EMULATOR_HOST must not be set in %s", c.Environment))
		}
		if c.StorageEmulatorHost != "" {
			errs = append(errs, fmt.Errorf("STORAGE_EMULATOR_HOST must not be set in %s", c.Environment))
		}
	}

	return errors.Join(errs...)
}

// String returns a log-safe string representation of ServiceConfig.
func (c ServiceConfig) String() string {
	return fmt.Sprintf(
		"ServiceConfig{Name: %s, Port: %d, Env: %s, ProjectID: %s, StorageBucket: %s, FirestoreDB: %s}",
		c.ServiceName, c.Port, c.Environment, c.ProjectID, c.StorageBucket, c.FirestoreDatabase,
	)
}

// RedactedString returns a log-safe string representation of ServiceConfig.
func (c ServiceConfig) RedactedString() string {
	return c.String()
}

// LoadServiceConfig loads common ServiceConfig using the provided lookup function.
func LoadServiceConfig(serviceName string, lookup func(string) string) (ServiceConfig, error) {
	rawEnv := lookup("PETSPOTR_ENV")
	if rawEnv == "" {
		rawEnv = lookup("ENVIRONMENT")
	}
	env, err := ParseEnvironment(rawEnv)
	if err != nil {
		return ServiceConfig{}, err
	}

	port, err := ParsePort(lookup("PORT"))
	if err != nil {
		return ServiceConfig{}, err
	}

	projectID := strings.TrimSpace(lookup("GOOGLE_CLOUD_PROJECT"))
	if projectID == "" && (env == EnvironmentDevelopment || env == EnvironmentLocalEmulator) {
		projectID = "petspotr-local"
	}

	storageBucket := strings.TrimSpace(lookup("STORAGE_BUCKET_NAME"))
	if storageBucket == "" {
		storageBucket = strings.TrimSpace(lookup("PET_IMAGES_BUCKET"))
	}
	if storageBucket == "" {
		storageBucket = strings.TrimSpace(lookup("PETSPOTR_IMAGE_BUCKET"))
	}
	if storageBucket == "" && (env == EnvironmentDevelopment || env == EnvironmentLocalEmulator) && serviceName != "notification-service" {
		storageBucket = "petspotr-local-petimages"
	}

	firestoreDB := strings.TrimSpace(lookup("FIRESTORE_DATABASE_ID"))
	if firestoreDB == "" {
		firestoreDB = "(default)"
	}

	cfg := ServiceConfig{
		ServiceName:           serviceName,
		Port:                  port,
		Environment:           env,
		ProjectID:             projectID,
		StorageBucket:         storageBucket,
		FirestoreDatabase:     firestoreDB,
		FirestoreEmulatorHost: strings.TrimSpace(lookup("FIRESTORE_EMULATOR_HOST")),
		PubSubEmulatorHost:    strings.TrimSpace(lookup("PUBSUB_EMULATOR_HOST")),
		StorageEmulatorHost:   strings.TrimSpace(lookup("STORAGE_EMULATOR_HOST")),
	}

	if err := cfg.Validate(); err != nil {
		return ServiceConfig{}, err
	}
	return cfg, nil
}

// LoadServiceConfigFromEnv loads ServiceConfig from the operating system environment.
func LoadServiceConfigFromEnv(serviceName string) (ServiceConfig, error) {
	return LoadServiceConfig(serviceName, os.Getenv)
}

// RateLimitConfig defines rate-limiting thresholds.
type RateLimitConfig struct {
	Enabled           bool
	RequestsPerMinute int
	Burst             int
}

// WebFrontendConfig contains the configuration required by cmd/web-frontend.
type WebFrontendConfig struct {
	Service             ServiceConfig
	SessionSecret       SecretString
	SessionCookieName   string
	IdentityMode        IdentityMode
	IdentityAPIKey      SecretString
	IdentityAuthDomain  string
	IdentityProjectID   string
	IdentityEmulatorURL string
	RateLimit           RateLimitConfig
}

// Validate validates WebFrontendConfig.
func (c WebFrontendConfig) Validate() error {
	var errs []error
	if err := c.Service.Validate(); err != nil {
		errs = append(errs, err)
	}

	isProdLike := c.Service.Environment == EnvironmentProduction || c.Service.Environment == EnvironmentStaging

	if isProdLike {
		if c.SessionSecret == "" || isPlaceholder(c.SessionSecret.Expose()) {
			errs = append(errs, fmt.Errorf("SessionSecret is required and cannot be empty or placeholder in %s", c.Service.Environment))
		}
		if c.IdentityMode == IdentityModeGCP {
			if c.IdentityAPIKey == "" || isPlaceholder(c.IdentityAPIKey.Expose()) {
				errs = append(errs, fmt.Errorf("IdentityAPIKey is required for GCP identity mode in %s", c.Service.Environment))
			}
			if c.IdentityAuthDomain == "" || isPlaceholder(c.IdentityAuthDomain) {
				errs = append(errs, fmt.Errorf("IdentityAuthDomain is required for GCP identity mode in %s", c.Service.Environment))
			}
			if c.IdentityProjectID == "" || isPlaceholder(c.IdentityProjectID) {
				errs = append(errs, fmt.Errorf("IdentityProjectID is required for GCP identity mode in %s", c.Service.Environment))
			}
			if c.IdentityEmulatorURL != "" {
				errs = append(errs, fmt.Errorf("IdentityEmulatorURL must not be set in %s", c.Service.Environment))
			}
		}
	}

	return errors.Join(errs...)
}

// String returns a log-safe representation of WebFrontendConfig with credentials redacted.
func (c WebFrontendConfig) String() string {
	return fmt.Sprintf(
		"WebFrontendConfig{Service: %s, SessionCookieName: %s, SessionSecret: %s, IdentityMode: %s, IdentityAPIKey: %s, IdentityAuthDomain: %s, IdentityProjectID: %s, RateLimitEnabled: %t}",
		c.Service, c.SessionCookieName, c.SessionSecret, c.IdentityMode, c.IdentityAPIKey, c.IdentityAuthDomain, c.IdentityProjectID, c.RateLimit.Enabled,
	)
}

// RedactedString returns a log-safe representation of WebFrontendConfig.
func (c WebFrontendConfig) RedactedString() string {
	return c.String()
}

// LoadWebFrontendConfig loads WebFrontendConfig using a lookup function.
func LoadWebFrontendConfig(lookup func(string) string) (WebFrontendConfig, error) {
	serviceCfg, err := LoadServiceConfig("web-frontend", lookup)
	if err != nil {
		return WebFrontendConfig{}, err
	}

	sessionSecret := strings.TrimSpace(lookup("SESSION_SECRET"))
	if sessionSecret == "" {
		sessionSecret = strings.TrimSpace(lookup("PETSPOTR_SESSION_SECRET"))
	}
	if sessionSecret == "" && (serviceCfg.Environment == EnvironmentDevelopment || serviceCfg.Environment == EnvironmentLocalEmulator) {
		sessionSecret = "dev-secret-local-webfrontend-session-key-32chars"
	}

	sessionCookieName := strings.TrimSpace(lookup("SESSION_COOKIE_NAME"))
	if sessionCookieName == "" {
		sessionCookieName = strings.TrimSpace(lookup("PETSPOTR_SESSION_COOKIE_NAME"))
	}
	if sessionCookieName == "" {
		sessionCookieName = "petspotr_session"
	}

	rawIdentityMode := strings.TrimSpace(lookup("PETSPOTR_IDENTITY_MODE"))
	if rawIdentityMode == "" {
		rawIdentityMode = string(IdentityModeDisabled)
	}

	rateLimitEnabled := strings.ToLower(strings.TrimSpace(lookup("RATE_LIMIT_ENABLED"))) == "true" || lookup("RATE_LIMIT_ENABLED") == "1"
	rpm := 60
	if rawRpm := strings.TrimSpace(lookup("RATE_LIMIT_REQUESTS_PER_MINUTE")); rawRpm != "" {
		if parsed, err := strconv.Atoi(rawRpm); err == nil && parsed > 0 {
			rpm = parsed
		}
	}
	burst := 20
	if rawBurst := strings.TrimSpace(lookup("RATE_LIMIT_BURST")); rawBurst != "" {
		if parsed, err := strconv.Atoi(rawBurst); err == nil && parsed > 0 {
			burst = parsed
		}
	}

	cfg := WebFrontendConfig{
		Service:             serviceCfg,
		SessionSecret:       SecretString(sessionSecret),
		SessionCookieName:   sessionCookieName,
		IdentityMode:        IdentityMode(rawIdentityMode),
		IdentityAPIKey:      SecretString(strings.TrimSpace(lookup("PETSPOTR_IDENTITY_WEB_API_KEY"))),
		IdentityAuthDomain:  strings.TrimSpace(lookup("PETSPOTR_IDENTITY_WEB_AUTH_DOMAIN")),
		IdentityProjectID:   strings.TrimSpace(lookup("PETSPOTR_IDENTITY_WEB_PROJECT_ID")),
		IdentityEmulatorURL: strings.TrimSpace(lookup("PETSPOTR_IDENTITY_WEB_EMULATOR_URL")),
		RateLimit: RateLimitConfig{
			Enabled:           rateLimitEnabled,
			RequestsPerMinute: rpm,
			Burst:             burst,
		},
	}

	if err := cfg.Validate(); err != nil {
		return WebFrontendConfig{}, err
	}
	return cfg, nil
}

// LoadWebFrontendConfigFromEnv loads WebFrontendConfig from environment variables.
func LoadWebFrontendConfigFromEnv() (WebFrontendConfig, error) {
	return LoadWebFrontendConfig(os.Getenv)
}

// LostPetConfig contains configuration for cmd/lostpet-service.
type LostPetConfig struct {
	Service          ServiceConfig
	StoreCollection  string
	OutboxCollection string
	PubSubTopic      string
}

// Validate validates LostPetConfig.
func (c LostPetConfig) Validate() error {
	var errs []error
	if err := c.Service.Validate(); err != nil {
		errs = append(errs, err)
	}

	if c.Service.Environment == EnvironmentProduction || c.Service.Environment == EnvironmentStaging {
		if c.StoreCollection == "" || isPlaceholder(c.StoreCollection) {
			errs = append(errs, fmt.Errorf("StoreCollection is required and cannot be empty or placeholder in %s", c.Service.Environment))
		}
		if c.OutboxCollection == "" || isPlaceholder(c.OutboxCollection) {
			errs = append(errs, fmt.Errorf("OutboxCollection is required and cannot be empty or placeholder in %s", c.Service.Environment))
		}
		if c.PubSubTopic == "" || isPlaceholder(c.PubSubTopic) {
			errs = append(errs, fmt.Errorf("PubSubTopic is required and cannot be empty or placeholder in %s", c.Service.Environment))
		}
	}
	return errors.Join(errs...)
}

// String returns a log-safe representation of LostPetConfig.
func (c LostPetConfig) String() string {
	return fmt.Sprintf(
		"LostPetConfig{Service: %s, StoreCollection: %s, OutboxCollection: %s, PubSubTopic: %s}",
		c.Service, c.StoreCollection, c.OutboxCollection, c.PubSubTopic,
	)
}

// RedactedString returns a log-safe representation of LostPetConfig.
func (c LostPetConfig) RedactedString() string {
	return c.String()
}

// LoadLostPetConfig loads LostPetConfig using a lookup function.
func LoadLostPetConfig(lookup func(string) string) (LostPetConfig, error) {
	serviceCfg, err := LoadServiceConfig("lostpet-service", lookup)
	if err != nil {
		return LostPetConfig{}, err
	}

	storeCol := strings.TrimSpace(lookup("LOST_PET_COLLECTION"))
	if storeCol == "" {
		storeCol = strings.TrimSpace(lookup("FIRESTORE_LOST_COLLECTION"))
	}
	if storeCol == "" {
		storeCol = "lost_pets"
	}

	outboxCol := strings.TrimSpace(lookup("LOST_PET_OUTBOX_COLLECTION"))
	if outboxCol == "" {
		outboxCol = strings.TrimSpace(lookup("FIRESTORE_LOST_OUTBOX_COLLECTION"))
	}
	if outboxCol == "" {
		outboxCol = "lost_pets_outbox"
	}

	topic := strings.TrimSpace(lookup("LOST_PET_TOPIC"))
	if topic == "" {
		topic = strings.TrimSpace(lookup("PUBSUB_LOST_TOPIC"))
	}
	if topic == "" {
		topic = "lost-pets"
	}

	cfg := LostPetConfig{
		Service:          serviceCfg,
		StoreCollection:  storeCol,
		OutboxCollection: outboxCol,
		PubSubTopic:      topic,
	}

	if err := cfg.Validate(); err != nil {
		return LostPetConfig{}, err
	}
	return cfg, nil
}

// LoadLostPetConfigFromEnv loads LostPetConfig from environment variables.
func LoadLostPetConfigFromEnv() (LostPetConfig, error) {
	return LoadLostPetConfig(os.Getenv)
}

// FoundPetConfig contains configuration for cmd/foundpet-service.
type FoundPetConfig struct {
	Service          ServiceConfig
	StoreCollection  string
	OutboxCollection string
	PubSubTopic      string
}

// Validate validates FoundPetConfig.
func (c FoundPetConfig) Validate() error {
	var errs []error
	if err := c.Service.Validate(); err != nil {
		errs = append(errs, err)
	}

	if c.Service.Environment == EnvironmentProduction || c.Service.Environment == EnvironmentStaging {
		if c.StoreCollection == "" || isPlaceholder(c.StoreCollection) {
			errs = append(errs, fmt.Errorf("StoreCollection is required and cannot be empty or placeholder in %s", c.Service.Environment))
		}
		if c.OutboxCollection == "" || isPlaceholder(c.OutboxCollection) {
			errs = append(errs, fmt.Errorf("OutboxCollection is required and cannot be empty or placeholder in %s", c.Service.Environment))
		}
		if c.PubSubTopic == "" || isPlaceholder(c.PubSubTopic) {
			errs = append(errs, fmt.Errorf("PubSubTopic is required and cannot be empty or placeholder in %s", c.Service.Environment))
		}
	}
	return errors.Join(errs...)
}

// String returns a log-safe representation of FoundPetConfig.
func (c FoundPetConfig) String() string {
	return fmt.Sprintf(
		"FoundPetConfig{Service: %s, StoreCollection: %s, OutboxCollection: %s, PubSubTopic: %s}",
		c.Service, c.StoreCollection, c.OutboxCollection, c.PubSubTopic,
	)
}

// RedactedString returns a log-safe representation of FoundPetConfig.
func (c FoundPetConfig) RedactedString() string {
	return c.String()
}

// LoadFoundPetConfig loads FoundPetConfig using a lookup function.
func LoadFoundPetConfig(lookup func(string) string) (FoundPetConfig, error) {
	serviceCfg, err := LoadServiceConfig("foundpet-service", lookup)
	if err != nil {
		return FoundPetConfig{}, err
	}

	storeCol := strings.TrimSpace(lookup("FOUND_PET_COLLECTION"))
	if storeCol == "" {
		storeCol = strings.TrimSpace(lookup("FIRESTORE_FOUND_COLLECTION"))
	}
	if storeCol == "" {
		storeCol = "found_pets"
	}

	outboxCol := strings.TrimSpace(lookup("FOUND_PET_OUTBOX_COLLECTION"))
	if outboxCol == "" {
		outboxCol = strings.TrimSpace(lookup("FIRESTORE_FOUND_OUTBOX_COLLECTION"))
	}
	if outboxCol == "" {
		outboxCol = "found_pets_outbox"
	}

	topic := strings.TrimSpace(lookup("FOUND_PET_TOPIC"))
	if topic == "" {
		topic = strings.TrimSpace(lookup("PUBSUB_FOUND_TOPIC"))
	}
	if topic == "" {
		topic = "found-pets"
	}

	cfg := FoundPetConfig{
		Service:          serviceCfg,
		StoreCollection:  storeCol,
		OutboxCollection: outboxCol,
		PubSubTopic:      topic,
	}

	if err := cfg.Validate(); err != nil {
		return FoundPetConfig{}, err
	}
	return cfg, nil
}

// LoadFoundPetConfigFromEnv loads FoundPetConfig from environment variables.
func LoadFoundPetConfigFromEnv() (FoundPetConfig, error) {
	return LoadFoundPetConfig(os.Getenv)
}

// PetMatcherConfig contains configuration for cmd/pet-matcher.
type PetMatcherConfig struct {
	Service            ServiceConfig
	OllamaHost         string
	OllamaModel        string
	MatchTopic         string
	FoundSubscription  string
	LostSubscription   string
	PushServiceAccount string
	DevPushToken       SecretString
}

// Validate validates PetMatcherConfig.
func (c PetMatcherConfig) Validate() error {
	var errs []error
	if err := c.Service.Validate(); err != nil {
		errs = append(errs, err)
	}

	if c.FoundSubscription == "" {
		errs = append(errs, fmt.Errorf("FoundSubscription is required"))
	}
	if c.LostSubscription == "" {
		errs = append(errs, fmt.Errorf("LostSubscription is required"))
	}
	if c.FoundSubscription != "" && c.LostSubscription != "" && c.FoundSubscription == c.LostSubscription {
		errs = append(errs, fmt.Errorf("FoundSubscription and LostSubscription must be distinct"))
	}

	isProdLike := c.Service.Environment == EnvironmentProduction || c.Service.Environment == EnvironmentStaging
	if isProdLike {
		if c.OllamaHost == "" || isPlaceholder(c.OllamaHost) {
			errs = append(errs, fmt.Errorf("OllamaHost is required and cannot be empty or placeholder in %s", c.Service.Environment))
		}
		if c.OllamaModel == "" || isPlaceholder(c.OllamaModel) {
			errs = append(errs, fmt.Errorf("OllamaModel is required and cannot be empty or placeholder in %s", c.Service.Environment))
		}
		if c.MatchTopic == "" || isPlaceholder(c.MatchTopic) {
			errs = append(errs, fmt.Errorf("MatchTopic is required and cannot be empty or placeholder in %s", c.Service.Environment))
		}
		if isPlaceholder(c.FoundSubscription) {
			errs = append(errs, fmt.Errorf("FoundSubscription cannot be a placeholder in %s", c.Service.Environment))
		}
		if isPlaceholder(c.LostSubscription) {
			errs = append(errs, fmt.Errorf("LostSubscription cannot be a placeholder in %s", c.Service.Environment))
		}
		if c.PushServiceAccount == "" || isPlaceholder(c.PushServiceAccount) {
			errs = append(errs, fmt.Errorf("PushServiceAccount is required in %s", c.Service.Environment))
		}
		if c.DevPushToken != "" {
			errs = append(errs, fmt.Errorf("PUBSUB_PUSH_DEV_TOKEN must not be set in %s", c.Service.Environment))
		}
	}

	return errors.Join(errs...)
}

// String returns a log-safe representation of PetMatcherConfig.
func (c PetMatcherConfig) String() string {
	return fmt.Sprintf(
		"PetMatcherConfig{Service: %s, OllamaHost: %s, OllamaModel: %s, MatchTopic: %s, FoundSubscription: %s, LostSubscription: %s, PushServiceAccount: %s, DevPushToken: %s}",
		c.Service, c.OllamaHost, c.OllamaModel, c.MatchTopic, c.FoundSubscription, c.LostSubscription, c.PushServiceAccount, c.DevPushToken,
	)
}

// RedactedString returns a log-safe representation of PetMatcherConfig.
func (c PetMatcherConfig) RedactedString() string {
	return c.String()
}

// LoadPetMatcherConfig loads PetMatcherConfig using a lookup function.
func LoadPetMatcherConfig(lookup func(string) string) (PetMatcherConfig, error) {
	serviceCfg, err := LoadServiceConfig("pet-matcher", lookup)
	if err != nil {
		return PetMatcherConfig{}, err
	}

	ollamaHost := strings.TrimSpace(lookup("OLLAMA_HOST"))
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434"
	}

	ollamaModel := strings.TrimSpace(lookup("OLLAMA_MODEL"))
	if ollamaModel == "" {
		ollamaModel = "gemma4"
	}

	matchTopic := strings.TrimSpace(lookup("MATCH_TOPIC"))
	if matchTopic == "" {
		matchTopic = strings.TrimSpace(lookup("PUBSUB_MATCH_TOPIC"))
	}
	if matchTopic == "" {
		matchTopic = "pet-matches"
	}

	foundSub := strings.TrimSpace(lookup("PUBSUB_PUSH_SUBSCRIPTION"))
	if foundSub == "" {
		foundSub = strings.TrimSpace(lookup("PUBSUB_FOUND_SUBSCRIPTION"))
	}
	if foundSub == "" && (serviceCfg.Environment == EnvironmentDevelopment || serviceCfg.Environment == EnvironmentLocalEmulator) {
		foundSub = "projects/petspotr-local/subscriptions/found-pet-matcher"
	}

	lostSub := strings.TrimSpace(lookup("PUBSUB_LOST_SUBSCRIPTION"))
	if lostSub == "" && (serviceCfg.Environment == EnvironmentDevelopment || serviceCfg.Environment == EnvironmentLocalEmulator) {
		lostSub = "projects/petspotr-local/subscriptions/lost-pet-matcher-analysis"
	}

	devToken := strings.TrimSpace(lookup("PUBSUB_PUSH_DEV_TOKEN"))
	if devToken == "" && (serviceCfg.Environment == EnvironmentDevelopment || serviceCfg.Environment == EnvironmentLocalEmulator) {
		devToken = "petspotr-local-push"
	}

	cfg := PetMatcherConfig{
		Service:            serviceCfg,
		OllamaHost:         ollamaHost,
		OllamaModel:        ollamaModel,
		MatchTopic:         matchTopic,
		FoundSubscription:  foundSub,
		LostSubscription:   lostSub,
		PushServiceAccount: strings.TrimSpace(lookup("PUBSUB_PUSH_SERVICE_ACCOUNT")),
		DevPushToken:       SecretString(devToken),
	}

	if err := cfg.Validate(); err != nil {
		return PetMatcherConfig{}, err
	}
	return cfg, nil
}

// LoadPetMatcherConfigFromEnv loads PetMatcherConfig from environment variables.
func LoadPetMatcherConfigFromEnv() (PetMatcherConfig, error) {
	return LoadPetMatcherConfig(os.Getenv)
}

// EmailProviderConfig holds credentials and settings for SendGrid email delivery.
type EmailProviderConfig struct {
	SendGridAPIKey    SecretString
	SendGridFromEmail string
}

// SMSProviderConfig holds credentials and settings for Twilio SMS delivery.
type SMSProviderConfig struct {
	TwilioAccountSID   SecretString
	TwilioAuthToken    SecretString
	TwilioFromNumber   string
	DefaultCountryCode string
}

// WebPushProviderConfig holds VAPID credentials for Web Push delivery.
type WebPushProviderConfig struct {
	VAPIDPublicKey  string
	VAPIDPrivateKey SecretString
	VAPIDSubject    string
}

// NotificationConfig contains configuration for cmd/notification-service.
type NotificationConfig struct {
	Service                ServiceConfig
	Email                  EmailProviderConfig
	SMS                    SMSProviderConfig
	Push                   WebPushProviderConfig
	MatchFoundSubscription string
	LostPetSubscription    string
	PushServiceAccount     string
	DevPushToken           SecretString
}

// Validate validates NotificationConfig.
func (c NotificationConfig) Validate() error {
	var errs []error
	if err := c.Service.Validate(); err != nil {
		errs = append(errs, err)
	}

	if c.MatchFoundSubscription == "" {
		errs = append(errs, fmt.Errorf("MatchFoundSubscription is required"))
	}
	if c.LostPetSubscription == "" {
		errs = append(errs, fmt.Errorf("LostPetSubscription is required"))
	}
	if c.MatchFoundSubscription != "" && c.LostPetSubscription != "" && c.MatchFoundSubscription == c.LostPetSubscription {
		errs = append(errs, fmt.Errorf("MatchFoundSubscription and LostPetSubscription must be distinct"))
	}

	isProdLike := c.Service.Environment == EnvironmentProduction || c.Service.Environment == EnvironmentStaging
	if isProdLike {
		if isPlaceholder(c.MatchFoundSubscription) {
			errs = append(errs, fmt.Errorf("MatchFoundSubscription cannot be a placeholder in %s", c.Service.Environment))
		}
		if isPlaceholder(c.LostPetSubscription) {
			errs = append(errs, fmt.Errorf("LostPetSubscription cannot be a placeholder in %s", c.Service.Environment))
		}
		if c.PushServiceAccount == "" || isPlaceholder(c.PushServiceAccount) {
			errs = append(errs, fmt.Errorf("PushServiceAccount is required in %s", c.Service.Environment))
		}
		if c.DevPushToken != "" {
			errs = append(errs, fmt.Errorf("PUBSUB_PUSH_DEV_TOKEN must not be set in %s", c.Service.Environment))
		}

		// At least one provider must be configured without placeholder credentials in production/staging.
		emailConfigured := c.Email.SendGridAPIKey != "" && !isPlaceholder(c.Email.SendGridAPIKey.Expose())
		smsConfigured := c.SMS.TwilioAuthToken != "" && !isPlaceholder(c.SMS.TwilioAuthToken.Expose())
		pushConfigured := c.Push.VAPIDPrivateKey != "" && !isPlaceholder(c.Push.VAPIDPrivateKey.Expose())

		if !emailConfigured && !smsConfigured && !pushConfigured {
			errs = append(errs, fmt.Errorf("at least one notification provider (SendGrid, Twilio, or VAPID) must be configured with non-placeholder credentials in %s", c.Service.Environment))
		}

		if c.Email.SendGridAPIKey != "" && isPlaceholder(c.Email.SendGridAPIKey.Expose()) {
			errs = append(errs, fmt.Errorf("SendGridAPIKey cannot be a placeholder in %s", c.Service.Environment))
		}
		if c.SMS.TwilioAuthToken != "" && isPlaceholder(c.SMS.TwilioAuthToken.Expose()) {
			errs = append(errs, fmt.Errorf("TwilioAuthToken cannot be a placeholder in %s", c.Service.Environment))
		}
		if c.Push.VAPIDPrivateKey != "" && isPlaceholder(c.Push.VAPIDPrivateKey.Expose()) {
			errs = append(errs, fmt.Errorf("VAPIDPrivateKey cannot be a placeholder in %s", c.Service.Environment))
		}
	}

	return errors.Join(errs...)
}

// String returns a log-safe representation of NotificationConfig with credentials redacted.
func (c NotificationConfig) String() string {
	return fmt.Sprintf(
		"NotificationConfig{Service: %s, EmailFrom: %s, EmailKey: %s, TwilioSID: %s, TwilioToken: %s, TwilioFrom: %s, VAPIDKey: %s, MatchSub: %s, LostSub: %s, PushSA: %s, DevPushToken: %s}",
		c.Service, c.Email.SendGridFromEmail, c.Email.SendGridAPIKey, c.SMS.TwilioAccountSID, c.SMS.TwilioAuthToken, c.SMS.TwilioFromNumber, c.Push.VAPIDPrivateKey, c.MatchFoundSubscription, c.LostPetSubscription, c.PushServiceAccount, c.DevPushToken,
	)
}

// RedactedString returns a log-safe representation of NotificationConfig.
func (c NotificationConfig) RedactedString() string {
	return c.String()
}

// LoadNotificationConfig loads NotificationConfig using a lookup function.
func LoadNotificationConfig(lookup func(string) string) (NotificationConfig, error) {
	serviceCfg, err := LoadServiceConfig("notification-service", lookup)
	if err != nil {
		return NotificationConfig{}, err
	}

	fromEmail := strings.TrimSpace(lookup("SENDGRID_FROM_EMAIL"))
	if fromEmail == "" {
		fromEmail = "alerts@petspotr.io"
	}

	smsCountryCode := strings.TrimSpace(lookup("SMS_DEFAULT_COUNTRY_CODE"))
	if smsCountryCode == "" {
		smsCountryCode = "+1"
	}

	vapidSubject := strings.TrimSpace(lookup("VAPID_SUBJECT"))
	if vapidSubject == "" {
		vapidSubject = "mailto:alerts@petspotr.io"
	}

	matchSub := strings.TrimSpace(lookup("PUBSUB_PUSH_SUBSCRIPTION"))
	if matchSub == "" && (serviceCfg.Environment == EnvironmentDevelopment || serviceCfg.Environment == EnvironmentLocalEmulator) {
		matchSub = "projects/petspotr-local/subscriptions/match-found-notification-backlog"
	}

	lostSub := strings.TrimSpace(lookup("PUBSUB_LOST_SUBSCRIPTION"))
	if lostSub == "" && (serviceCfg.Environment == EnvironmentDevelopment || serviceCfg.Environment == EnvironmentLocalEmulator) {
		lostSub = "projects/petspotr-local/subscriptions/lost-pet-notification"
	}

	devToken := strings.TrimSpace(lookup("PUBSUB_PUSH_DEV_TOKEN"))
	if devToken == "" && (serviceCfg.Environment == EnvironmentDevelopment || serviceCfg.Environment == EnvironmentLocalEmulator) {
		devToken = "petspotr-local-push"
	}

	cfg := NotificationConfig{
		Service: serviceCfg,
		Email: EmailProviderConfig{
			SendGridAPIKey:    SecretString(strings.TrimSpace(lookup("SENDGRID_API_KEY"))),
			SendGridFromEmail: fromEmail,
		},
		SMS: SMSProviderConfig{
			TwilioAccountSID:   SecretString(strings.TrimSpace(lookup("TWILIO_ACCOUNT_SID"))),
			TwilioAuthToken:    SecretString(strings.TrimSpace(lookup("TWILIO_AUTH_TOKEN"))),
			TwilioFromNumber:   strings.TrimSpace(lookup("TWILIO_FROM_NUMBER")),
			DefaultCountryCode: smsCountryCode,
		},
		Push: WebPushProviderConfig{
			VAPIDPublicKey:  strings.TrimSpace(lookup("VAPID_PUBLIC_KEY")),
			VAPIDPrivateKey: SecretString(strings.TrimSpace(lookup("VAPID_PRIVATE_KEY"))),
			VAPIDSubject:    vapidSubject,
		},
		MatchFoundSubscription: matchSub,
		LostPetSubscription:    lostSub,
		PushServiceAccount:     strings.TrimSpace(lookup("PUBSUB_PUSH_SERVICE_ACCOUNT")),
		DevPushToken:           SecretString(devToken),
	}

	if err := cfg.Validate(); err != nil {
		return NotificationConfig{}, err
	}
	return cfg, nil
}

// LoadNotificationConfigFromEnv loads NotificationConfig from environment variables.
func LoadNotificationConfigFromEnv() (NotificationConfig, error) {
	return LoadNotificationConfig(os.Getenv)
}
