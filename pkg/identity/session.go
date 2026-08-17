// Package identity defines PetSpotR's provider-neutral human identity boundary.
package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"firebase.google.com/go/v4/auth"
)

const (
	minimumSessionLifetime   = 5 * time.Minute
	maximumSessionLifetime   = 14 * 24 * time.Hour
	maximumAuthenticationAge = 5 * time.Minute
)

var (
	// ErrUnauthenticated means a token or session does not establish an identity.
	ErrUnauthenticated = errors.New("identity: unauthenticated")
	// ErrEmailNotVerified means the identity has no verified contact address.
	ErrEmailNotVerified = errors.New("identity: verified email is required")
	// ErrRecentSignInRequired means the ID token is too old to create a session.
	ErrRecentSignInRequired = errors.New("identity: recent sign-in is required")
	// ErrInvalidSessionLifetime means a requested lifetime violates provider bounds.
	ErrInvalidSessionLifetime = errors.New("identity: invalid session lifetime")
)

// Principal is the normalized identity trusted by PetSpotR authorization code.
// Issuer is the canonical Identity Platform project authority rather than the
// token-type-specific raw issuer. Issuer and Subject form the stable ownership
// key; email is display/contact data. Tenant identities are rejected until the
// runtime can perform tenant-scoped revocation checks.
type Principal struct {
	Issuer         string `json:"issuer"`
	Subject        string `json:"subject"`
	Email          string `json:"email"`
	EmailVerified  bool   `json:"emailVerified"`
	SignInProvider string `json:"signInProvider,omitempty"`
}

// Session is a provider-issued cookie plus its verified principal.
type Session struct {
	Cookie    string
	Principal Principal
}

// SessionManager creates and verifies human-user sessions without exposing a
// provider-specific token type to HTTP or domain packages.
type SessionManager interface {
	CreateSession(context.Context, string, time.Duration) (Session, error)
	VerifySession(context.Context, string) (Principal, error)
}

type identityPlatformClient interface {
	VerifyIDToken(context.Context, string) (*auth.Token, error)
	SessionCookie(context.Context, string, time.Duration) (string, error)
	VerifySessionCookie(context.Context, string) (*auth.Token, error)
	VerifySessionCookieAndCheckRevoked(context.Context, string) (*auth.Token, error)
}

// IdentityPlatformSessions implements SessionManager with Google Identity
// Platform through the Firebase Admin SDK.
type IdentityPlatformSessions struct {
	client identityPlatformClient
	now    func() time.Time
}

// NewIdentityPlatformSessions constructs the managed Identity Platform adapter.
func NewIdentityPlatformSessions(client *auth.Client) (*IdentityPlatformSessions, error) {
	if client == nil {
		return nil, errors.New("identity: Identity Platform client is required")
	}
	return newIdentityPlatformSessions(client, time.Now), nil
}

func newIdentityPlatformSessions(client identityPlatformClient, now func() time.Time) *IdentityPlatformSessions {
	return &IdentityPlatformSessions{client: client, now: now}
}

// CreateSession exchanges a recently issued ID token for a bounded session.
func (s *IdentityPlatformSessions) CreateSession(
	ctx context.Context,
	idToken string,
	ttl time.Duration,
) (Session, error) {
	idToken = strings.TrimSpace(idToken)
	if idToken == "" {
		return Session{}, ErrUnauthenticated
	}
	if ttl < minimumSessionLifetime || ttl > maximumSessionLifetime {
		return Session{}, ErrInvalidSessionLifetime
	}

	token, err := s.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return Session{}, errors.Join(ErrUnauthenticated, fmt.Errorf("verify Identity Platform ID token: %w", err))
	}
	principal, err := principalFromToken(token)
	if err != nil {
		return Session{}, err
	}
	authenticatedAt := time.Unix(token.AuthTime, 0)
	age := s.now().Sub(authenticatedAt)
	if token.AuthTime <= 0 || age < -time.Minute || age > maximumAuthenticationAge {
		return Session{}, ErrRecentSignInRequired
	}

	cookie, err := s.client.SessionCookie(ctx, idToken, ttl)
	if err != nil {
		return Session{}, fmt.Errorf("create Identity Platform session cookie: %w", err)
	}
	return Session{Cookie: cookie, Principal: principal}, nil
}

// VerifySession validates the signature, expiry, and server-side revocation
// state of an Identity Platform session cookie.
func (s *IdentityPlatformSessions) VerifySession(ctx context.Context, sessionCookie string) (Principal, error) {
	sessionCookie = strings.TrimSpace(sessionCookie)
	if sessionCookie == "" {
		return Principal{}, ErrUnauthenticated
	}
	decoded, err := s.client.VerifySessionCookie(ctx, sessionCookie)
	if err != nil {
		return Principal{}, errors.Join(ErrUnauthenticated, fmt.Errorf("verify Identity Platform session cookie: %w", err))
	}
	if _, err := principalFromToken(decoded); err != nil {
		return Principal{}, err
	}
	token, err := s.client.VerifySessionCookieAndCheckRevoked(ctx, sessionCookie)
	if err != nil {
		return Principal{}, errors.Join(ErrUnauthenticated, fmt.Errorf("verify Identity Platform session cookie: %w", err))
	}
	return principalFromToken(token)
}

func principalFromToken(token *auth.Token) (Principal, error) {
	if token == nil || strings.TrimSpace(token.Issuer) == "" ||
		strings.TrimSpace(token.Audience) == "" || token.UID == "" {
		return Principal{}, ErrUnauthenticated
	}
	if strings.TrimSpace(token.Firebase.Tenant) != "" {
		return Principal{}, errors.Join(ErrUnauthenticated, errors.New("identity: tenant sessions are not supported"))
	}
	email, emailOK := token.Claims["email"].(string)
	verified, verifiedOK := token.Claims["email_verified"].(bool)
	email = strings.ToLower(strings.TrimSpace(email))
	if !emailOK || email == "" || !verifiedOK || !verified {
		return Principal{}, ErrEmailNotVerified
	}
	return Principal{
		Issuer:         "https://securetoken.google.com/" + strings.TrimSpace(token.Audience),
		Subject:        token.UID,
		Email:          email,
		EmailVerified:  true,
		SignInProvider: strings.TrimSpace(token.Firebase.SignInProvider),
	}, nil
}
