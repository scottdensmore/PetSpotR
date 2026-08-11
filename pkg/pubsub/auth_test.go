package pubsub_test

import (
	"context"
	"errors"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/pubsub"
)

type stubIDTokenValidator struct {
	claims pubsub.IDTokenClaims
	err    error
	token  string
	aud    string
}

func (v *stubIDTokenValidator) Validate(_ context.Context, token, audience string) (pubsub.IDTokenClaims, error) {
	v.token = token
	v.aud = audience
	return v.claims, v.err
}

func TestOIDCPushAuthorizerValidatesExactInvocationIdentity(t *testing.T) {
	validator := &stubIDTokenValidator{claims: pubsub.IDTokenClaims{
		Email:         "pubsub-pet-matcher-invoker@petspotr-test.iam.gserviceaccount.com",
		EmailVerified: true,
	}}
	authorizer := pubsub.NewOIDCPushAuthorizer(
		"pubsub-pet-matcher-invoker@petspotr-test.iam.gserviceaccount.com",
		validator,
	)

	if err := authorizer.Authorize(context.Background(), "Bearer signed-token", "https://pet-matcher.example.test"); err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	if validator.token != "signed-token" || validator.aud != "https://pet-matcher.example.test" {
		t.Fatalf("validator token = %q, audience = %q", validator.token, validator.aud)
	}

	validator.claims.Email = "other@petspotr-test.iam.gserviceaccount.com"
	if err := authorizer.Authorize(context.Background(), "Bearer signed-token", "https://pet-matcher.example.test"); err == nil {
		t.Fatal("Authorize() accepted unexpected service-account email")
	}
	validator.claims.Email = "pubsub-pet-matcher-invoker@petspotr-test.iam.gserviceaccount.com"
	validator.claims.EmailVerified = false
	if err := authorizer.Authorize(context.Background(), "Bearer signed-token", "https://pet-matcher.example.test"); err == nil {
		t.Fatal("Authorize() accepted unverified email")
	}
	validator.err = errors.New("invalid signature")
	if err := authorizer.Authorize(context.Background(), "Bearer signed-token", "https://pet-matcher.example.test"); err == nil {
		t.Fatal("Authorize() accepted invalid signature")
	}
}

func TestStaticPushAuthorizerUsesConstantConfiguredToken(t *testing.T) {
	authorizer := pubsub.NewStaticPushAuthorizer("local-secret")
	if err := authorizer.Authorize(context.Background(), "Bearer local-secret", "http://localhost:8083"); err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	if err := authorizer.Authorize(context.Background(), "Bearer wrong", "http://localhost:8083"); err == nil {
		t.Fatal("Authorize() accepted wrong token")
	}
}
