package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"firebase.google.com/go/v4/auth"
)

type stubIdentityPlatformClient struct {
	idToken         *auth.Token
	idTokenErr      error
	sessionCookie   string
	sessionErr      error
	verifiedSession *auth.Token
	verifyErr       error
	decodedSession  *auth.Token
	decodeErr       error
	createdToken    string
	createdTTL      time.Duration
	decodedCookie   string
	verifiedCookie  string
}

func (s *stubIdentityPlatformClient) VerifyIDToken(context.Context, string) (*auth.Token, error) {
	return s.idToken, s.idTokenErr
}

func (s *stubIdentityPlatformClient) SessionCookie(_ context.Context, idToken string, ttl time.Duration) (string, error) {
	s.createdToken = idToken
	s.createdTTL = ttl
	return s.sessionCookie, s.sessionErr
}

func (s *stubIdentityPlatformClient) VerifySessionCookieAndCheckRevoked(_ context.Context, cookie string) (*auth.Token, error) {
	s.verifiedCookie = cookie
	return s.verifiedSession, s.verifyErr
}

func (s *stubIdentityPlatformClient) VerifySessionCookie(_ context.Context, cookie string) (*auth.Token, error) {
	s.decodedCookie = cookie
	if s.decodedSession != nil || s.decodeErr != nil {
		return s.decodedSession, s.decodeErr
	}
	return s.verifiedSession, nil
}

func TestIdentityPlatformSessionsCreatesSessionForRecentVerifiedIdentity(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 17, 18, 0, 0, 0, time.UTC)
	client := &stubIdentityPlatformClient{
		idToken: &auth.Token{
			AuthTime: now.Add(-2 * time.Minute).Unix(),
			Issuer:   "https://securetoken.google.com/petspotr-test",
			Audience: "petspotr-test",
			UID:      "user-101",
			Firebase: auth.FirebaseInfo{SignInProvider: "google.com"},
			Claims: map[string]interface{}{
				"email":          "Owner@Example.com",
				"email_verified": true,
			},
		},
		sessionCookie: "signed-session",
	}
	manager := newIdentityPlatformSessions(client, func() time.Time { return now })

	session, err := manager.CreateSession(context.Background(), "id-token", 5*24*time.Hour)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if client.createdToken != "id-token" || client.createdTTL != 5*24*time.Hour {
		t.Fatalf("SessionCookie() token/ttl = %q/%s", client.createdToken, client.createdTTL)
	}
	if session.Cookie != "signed-session" {
		t.Fatalf("CreateSession().Cookie = %q, want signed-session", session.Cookie)
	}
	want := Principal{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "user-101",
		Email: "owner@example.com", EmailVerified: true, SignInProvider: "google.com",
	}
	if session.Principal != want {
		t.Fatalf("CreateSession().Principal = %#v, want %#v", session.Principal, want)
	}
}

func TestIdentityPlatformSessionsRejectsUnsafeSessionCreation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 17, 18, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		token   *auth.Token
		idToken string
		ttl     time.Duration
		wantErr error
	}{
		{
			name:    "missing token",
			idToken: " ",
			ttl:     5 * 24 * time.Hour,
			wantErr: ErrUnauthenticated,
		},
		{
			name: "stale authentication",
			token: &auth.Token{
				AuthTime: now.Add(-6 * time.Minute).Unix(), Issuer: "issuer", Audience: "project", UID: "user",
				Claims: map[string]interface{}{"email": "owner@example.com", "email_verified": true},
			},
			idToken: "token",
			ttl:     5 * 24 * time.Hour,
			wantErr: ErrRecentSignInRequired,
		},
		{
			name: "unverified email",
			token: &auth.Token{
				AuthTime: now.Unix(), Issuer: "issuer", Audience: "project", UID: "user",
				Claims: map[string]interface{}{"email": "owner@example.com", "email_verified": false},
			},
			idToken: "token",
			ttl:     5 * 24 * time.Hour,
			wantErr: ErrEmailNotVerified,
		},
		{
			name: "session lifetime below provider minimum",
			token: &auth.Token{
				AuthTime: now.Unix(), Issuer: "issuer", UID: "user",
				Claims: map[string]interface{}{"email": "owner@example.com", "email_verified": true},
			},
			idToken: "token",
			ttl:     time.Minute,
			wantErr: ErrInvalidSessionLifetime,
		},
		{
			name: "session lifetime above provider maximum",
			token: &auth.Token{
				AuthTime: now.Unix(), Issuer: "issuer", UID: "user",
				Claims: map[string]interface{}{"email": "owner@example.com", "email_verified": true},
			},
			idToken: "token",
			ttl:     15 * 24 * time.Hour,
			wantErr: ErrInvalidSessionLifetime,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &stubIdentityPlatformClient{idToken: tt.token, sessionCookie: "must-not-be-created"}
			manager := newIdentityPlatformSessions(client, func() time.Time { return now })
			_, err := manager.CreateSession(context.Background(), tt.idToken, tt.ttl)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CreateSession() error = %v, want %v", err, tt.wantErr)
			}
			if client.createdToken != "" {
				t.Fatalf("unsafe input reached SessionCookie(): %q", client.createdToken)
			}
		})
	}
}

func TestIdentityPlatformSessionsVerifiesRevocationAndPrincipal(t *testing.T) {
	t.Parallel()

	client := &stubIdentityPlatformClient{verifiedSession: &auth.Token{
		Issuer:   "https://session.firebase.google.com/petspotr-test",
		Audience: "petspotr-test",
		UID:      "user-202",
		Firebase: auth.FirebaseInfo{
			SignInProvider: "password",
		},
		Claims: map[string]interface{}{
			"email":          "Finder@Example.com",
			"email_verified": true,
		},
	}}
	manager := newIdentityPlatformSessions(client, time.Now)

	principal, err := manager.VerifySession(context.Background(), "signed-session")
	if err != nil {
		t.Fatalf("VerifySession() error = %v", err)
	}
	if client.decodedCookie != "signed-session" || client.verifiedCookie != "signed-session" {
		t.Fatalf("decoded/verified cookie = %q/%q, want signed-session", client.decodedCookie, client.verifiedCookie)
	}
	want := Principal{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "user-202",
		Email: "finder@example.com", EmailVerified: true, SignInProvider: "password",
	}
	if principal != want {
		t.Fatalf("VerifySession() = %#v, want %#v", principal, want)
	}
}

func TestIdentityPlatformSessionsKeepsOwnershipIdentityAcrossTokenTypes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 17, 18, 0, 0, 0, time.UTC)
	claims := map[string]interface{}{"email": "owner@example.com", "email_verified": true}
	client := &stubIdentityPlatformClient{
		idToken: &auth.Token{
			AuthTime: now.Unix(),
			Issuer:   "https://securetoken.google.com/petspotr-test",
			Audience: "petspotr-test",
			UID:      "user-303",
			Claims:   claims,
		},
		sessionCookie: "signed-session",
		verifiedSession: &auth.Token{
			Issuer:   "https://session.firebase.google.com/petspotr-test",
			Audience: "petspotr-test",
			UID:      "user-303",
			Claims:   claims,
		},
	}
	manager := newIdentityPlatformSessions(client, func() time.Time { return now })

	created, err := manager.CreateSession(context.Background(), "id-token", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	verified, err := manager.VerifySession(context.Background(), created.Cookie)
	if err != nil {
		t.Fatalf("VerifySession() error = %v", err)
	}
	if created.Principal != verified {
		t.Fatalf("principal changed across token types: created %#v, verified %#v", created.Principal, verified)
	}
	if verified.Issuer != "https://securetoken.google.com/petspotr-test" {
		t.Fatalf("canonical issuer = %q", verified.Issuer)
	}
}

func TestIdentityPlatformSessionsRejectsSameUIDTenantsBeforeProjectRevocation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 17, 18, 0, 0, 0, time.UTC)
	for _, tenant := range []string{"rescue-east", "rescue-west"} {
		tenant := tenant
		t.Run(tenant, func(t *testing.T) {
			t.Parallel()
			token := &auth.Token{
				AuthTime: now.Unix(), Issuer: "https://securetoken.google.com/petspotr-test",
				Audience: "petspotr-test", UID: "shared-uid",
				Firebase: auth.FirebaseInfo{Tenant: tenant},
				Claims:   map[string]interface{}{"email": "owner@example.com", "email_verified": true},
			}
			client := &stubIdentityPlatformClient{
				idToken: token, sessionCookie: "must-not-be-created", decodedSession: token,
				verifiedSession: token,
			}
			manager := newIdentityPlatformSessions(client, func() time.Time { return now })

			if _, err := manager.CreateSession(context.Background(), "id-token", 24*time.Hour); !errors.Is(err, ErrUnauthenticated) {
				t.Fatalf("CreateSession() error = %v, want unauthenticated", err)
			}
			if client.createdToken != "" {
				t.Fatalf("tenant identity reached SessionCookie(): %q", client.createdToken)
			}
			if _, err := manager.VerifySession(context.Background(), "session-cookie"); !errors.Is(err, ErrUnauthenticated) {
				t.Fatalf("VerifySession() error = %v, want unauthenticated", err)
			}
			if client.verifiedCookie != "" {
				t.Fatalf("tenant identity reached project-scoped revocation: %q", client.verifiedCookie)
			}
		})
	}
}

func TestIdentityPlatformSessionsPreservesOpaqueSubjects(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 17, 18, 0, 0, 0, time.UTC)
	principals := make([]Principal, 0, 2)
	for _, uid := range []string{"owner", " owner "} {
		client := &stubIdentityPlatformClient{
			idToken: &auth.Token{
				AuthTime: now.Unix(), Issuer: "issuer", Audience: "petspotr-test", UID: uid,
				Claims: map[string]interface{}{"email": "owner@example.com", "email_verified": true},
			},
			sessionCookie: "signed-session",
		}
		manager := newIdentityPlatformSessions(client, func() time.Time { return now })
		session, err := manager.CreateSession(context.Background(), "id-token", 24*time.Hour)
		if err != nil {
			t.Fatalf("CreateSession(%q) error = %v", uid, err)
		}
		principals = append(principals, session.Principal)
	}
	if principals[0].Subject == principals[1].Subject {
		t.Fatalf("opaque subjects collapsed: %#v", principals)
	}
	if principals[1].Subject != " owner " {
		t.Fatalf("subject was normalized to %q", principals[1].Subject)
	}
}

func TestIdentityPlatformSessionsNormalizesProviderFailures(t *testing.T) {
	t.Parallel()

	providerErr := errors.New("provider rejected token")
	client := &stubIdentityPlatformClient{idTokenErr: providerErr, decodeErr: providerErr, verifyErr: providerErr}
	manager := newIdentityPlatformSessions(client, time.Now)

	if _, err := manager.CreateSession(context.Background(), "bad-id-token", 24*time.Hour); !errors.Is(err, ErrUnauthenticated) || !errors.Is(err, providerErr) {
		t.Fatalf("CreateSession() error = %v, want wrapped provider and unauthenticated errors", err)
	}
	if _, err := manager.VerifySession(context.Background(), "bad-session"); !errors.Is(err, ErrUnauthenticated) || !errors.Is(err, providerErr) {
		t.Fatalf("VerifySession() error = %v, want wrapped provider and unauthenticated errors", err)
	}
}
