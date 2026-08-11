package pubsub

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/api/idtoken"
)

// PushAuthorizer validates the identity attached to a Pub/Sub push request.
type PushAuthorizer interface {
	Authorize(ctx context.Context, authorization, audience string) error
}

// IDTokenClaims contains the identity claims required by a push endpoint.
type IDTokenClaims struct {
	Email         string
	EmailVerified bool
}

// IDTokenValidator verifies a Google-signed OIDC token for one audience.
type IDTokenValidator interface {
	Validate(ctx context.Context, token, audience string) (IDTokenClaims, error)
}

type googleIDTokenValidator struct{}

func (googleIDTokenValidator) Validate(ctx context.Context, token, audience string) (IDTokenClaims, error) {
	payload, err := idtoken.Validate(ctx, token, audience)
	if err != nil {
		return IDTokenClaims{}, err
	}
	email, _ := payload.Claims["email"].(string)
	emailVerified, _ := payload.Claims["email_verified"].(bool)
	return IDTokenClaims{Email: email, EmailVerified: emailVerified}, nil
}

// OIDCPushAuthorizer validates the Google signature/audience and then pins the
// invocation to the dedicated Pub/Sub push service account.
type OIDCPushAuthorizer struct {
	expectedEmail string
	validator     IDTokenValidator
}

// NewOIDCPushAuthorizer constructs a production OIDC authorizer. Passing nil
// uses Google's ID-token validator.
func NewOIDCPushAuthorizer(expectedEmail string, validator IDTokenValidator) *OIDCPushAuthorizer {
	if validator == nil {
		validator = googleIDTokenValidator{}
	}
	return &OIDCPushAuthorizer{expectedEmail: strings.TrimSpace(expectedEmail), validator: validator}
}

// Authorize verifies one bearer token and exact service-account identity.
func (a *OIDCPushAuthorizer) Authorize(ctx context.Context, authorization, audience string) error {
	if a.expectedEmail == "" || strings.TrimSpace(audience) == "" {
		return errors.New("pubsub: push OIDC authorizer is not configured")
	}
	token, err := bearerToken(authorization)
	if err != nil {
		return err
	}
	claims, err := a.validator.Validate(ctx, token, audience)
	if err != nil {
		return fmt.Errorf("pubsub: validate push OIDC token: %w", err)
	}
	if !claims.EmailVerified || claims.Email != a.expectedEmail {
		return errors.New("pubsub: push identity does not match expected service account")
	}
	return nil
}

// StaticPushAuthorizer protects emulator push endpoints with a configured
// bearer token. It is not valid for managed GCP delivery.
type StaticPushAuthorizer struct {
	token string
}

// NewStaticPushAuthorizer constructs an emulator-only token authorizer.
func NewStaticPushAuthorizer(token string) *StaticPushAuthorizer {
	return &StaticPushAuthorizer{token: strings.TrimSpace(token)}
}

// Authorize compares the configured emulator token in constant time.
func (a *StaticPushAuthorizer) Authorize(_ context.Context, authorization, _ string) error {
	if a.token == "" {
		return errors.New("pubsub: static push token is not configured")
	}
	token, err := bearerToken(authorization)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(a.token)) != 1 {
		return errors.New("pubsub: invalid static push token")
	}
	return nil
}

func bearerToken(authorization string) (string, error) {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errors.New("pubsub: bearer token is required")
	}
	return parts[1], nil
}
