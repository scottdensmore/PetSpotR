package notification

import (
	"log"
	"os"
	"strings"
)

// ProviderConfig holds configuration for all notification providers.
type ProviderConfig struct {
	SendGridAPIKey     string
	SendGridFromEmail  string
	TwilioAccountSID   string
	TwilioAuthToken    string
	TwilioFromNumber   string
	DefaultCountryCode string
	SMSMaxLength       int
}

// LoadProviderConfigFromEnv loads notification provider settings from environment variables.
func LoadProviderConfigFromEnv() ProviderConfig {
	return LoadProviderConfig(os.Getenv)
}

// LoadProviderConfig loads notification provider settings using a lookup function.
func LoadProviderConfig(lookup func(string) string) ProviderConfig {
	return ProviderConfig{
		SendGridAPIKey:     strings.TrimSpace(lookup("SENDGRID_API_KEY")),
		SendGridFromEmail:  strings.TrimSpace(lookup("SENDGRID_FROM_EMAIL")),
		TwilioAccountSID:   strings.TrimSpace(lookup("TWILIO_ACCOUNT_SID")),
		TwilioAuthToken:    strings.TrimSpace(lookup("TWILIO_AUTH_TOKEN")),
		TwilioFromNumber:   strings.TrimSpace(lookup("TWILIO_FROM_NUMBER")),
		DefaultCountryCode: strings.TrimSpace(lookup("SMS_DEFAULT_COUNTRY_CODE")),
	}
}

// NewProvidersFromConfig constructs providers based on configuration.
// If SendGrid or Twilio keys are unset, it falls back to a logging provider for local dev mode.
func NewProvidersFromConfig(cfg ProviderConfig) (EmailProvider, SMSProvider, PushProvider) {
	var emailProvider EmailProvider
	if cfg.SendGridAPIKey != "" {
		opts := []SendGridOption{}
		if cfg.SendGridFromEmail != "" {
			opts = append(opts, WithSendGridFromEmail(cfg.SendGridFromEmail))
		}
		p, err := NewSendGridEmailProvider(cfg.SendGridAPIKey, opts...)
		if err != nil {
			log.Printf("[Notification] Warning: Failed to init SendGrid provider: %v; falling back to logging provider", err)
			emailProvider = NewLoggingEmailProvider()
		} else {
			log.Printf("[Notification] Initialized SendGrid email provider")
			emailProvider = p
		}
	} else {
		log.Printf("[Notification] SENDGRID_API_KEY unset; using local development logging email provider")
		emailProvider = NewLoggingEmailProvider()
	}

	var smsProvider SMSProvider
	if cfg.TwilioAccountSID != "" && cfg.TwilioAuthToken != "" {
		opts := []TwilioOption{}
		if cfg.TwilioFromNumber != "" {
			opts = append(opts, WithTwilioFromNumber(cfg.TwilioFromNumber))
		}
		p, err := NewTwilioSMSProvider(cfg.TwilioAccountSID, cfg.TwilioAuthToken, opts...)
		if err != nil {
			log.Printf("[Notification] Warning: Failed to init Twilio provider: %v; falling back to logging provider", err)
			smsProvider = NewLoggingSMSProvider()
		} else {
			log.Printf("[Notification] Initialized Twilio SMS provider")
			smsProvider = p
		}
	} else {
		log.Printf("[Notification] TWILIO credentials unset; using local development logging SMS provider")
		smsProvider = NewLoggingSMSProvider()
	}

	pushProvider := NewLoggingPushProvider()
	return emailProvider, smsProvider, pushProvider
}
