package e2e_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/identity"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestExpiredIdentityPlatformSessionRejectsProtectedReportWithAuthEmulator(t *testing.T) {
	host := strings.TrimSpace(os.Getenv("FIREBASE_AUTH_EMULATOR_HOST"))
	if host == "" {
		t.Skip("FIREBASE_AUTH_EMULATOR_HOST is not set")
	}
	const projectID = "demo-petspotr-auth"
	requireLoopbackAuthEmulator(t, host, projectID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	email := fmt.Sprintf("expired-owner-%d@example.com", time.Now().UnixNano())
	password := "emulator-only-password-123"

	setupApp, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		t.Fatalf("initialize emulator setup app: %v", err)
	}
	setupAuth, err := setupApp.Auth(ctx)
	if err != nil {
		t.Fatalf("initialize emulator setup auth: %v", err)
	}
	created, err := setupAuth.CreateUser(ctx, (&auth.UserToCreate{}).
		Email(email).
		EmailVerified(true).
		Password(password))
	if err != nil {
		t.Fatalf("create verified emulator user: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = setupAuth.DeleteUser(cleanupCtx, created.UID)
	})
	idToken := signInToAuthEmulator(t, ctx, host, email, password)

	identityRuntime, err := runtimeconfig.NewIdentityRuntime(ctx, runtimeconfig.IdentityConfig{
		Mode: runtimeconfig.IdentityModeLocalEmulator, ProjectID: projectID, AuthEmulatorHost: host,
	})
	if err != nil {
		t.Fatalf("NewIdentityRuntime() error = %v", err)
	}
	state := store.NewMemoryStore()
	srv := httptestServer(t, state, identityRuntime.Sessions, identityRuntime.SecureCookies)
	defer srv.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}

	csrfResponse := productRequest(t, client, http.MethodGet, srv.URL+"/api/v1/session/csrf", "", "")
	if csrfResponse.StatusCode != http.StatusOK {
		t.Fatalf("CSRF status = %d, want %d", csrfResponse.StatusCode, http.StatusOK)
	}
	var csrfBody map[string]string
	decodeResponseJSON(t, csrfResponse, &csrfBody)
	csrfToken := csrfBody["csrfToken"]
	if csrfToken == "" {
		t.Fatal("CSRF response token is empty")
	}

	loginBody, err := json.Marshal(map[string]string{"idToken": idToken})
	if err != nil {
		t.Fatalf("marshal session request: %v", err)
	}
	login := productRequest(t, client, http.MethodPost, srv.URL+"/api/v1/session", string(loginBody), csrfToken)
	if login.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(login.Body)
		_ = login.Body.Close()
		t.Fatalf("session login status = %d, want %d; body = %s", login.StatusCode, http.StatusCreated, body)
	}
	var principal identity.Principal
	decodeResponseJSON(t, login, &principal)
	if principal.Subject != created.UID || principal.Email != email || !principal.EmailVerified {
		t.Fatalf("login principal = %#v, want UID %q and verified email %q", principal, created.UID, email)
	}

	current := productRequest(t, client, http.MethodGet, srv.URL+"/api/v1/session", "", "")
	if current.StatusCode != http.StatusOK {
		t.Fatalf("current session status = %d, want %d", current.StatusCode, http.StatusOK)
	}
	var verified identity.Principal
	decodeResponseJSON(t, current, &verified)
	if verified != principal {
		t.Fatalf("verified principal = %#v, want %#v", verified, principal)
	}

	baseURL, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse product server URL: %v", err)
	}
	sessionCookie := requireCookie(t, jar.Cookies(baseURL), "petspotr_session")
	expiredCookie := expireUnsignedEmulatorSessionCookie(t, sessionCookie.Value, projectID)
	jar.SetCookies(baseURL, []*http.Cookie{{Name: sessionCookie.Name, Value: expiredCookie, Path: "/"}})

	reportID := fmt.Sprintf("lost-expired-%d", time.Now().UnixNano())
	reportBody, err := json.Marshal(map[string]string{
		"petId": reportID, "petName": "Buddy", "reporterEmail": "spoofed@example.com", "location": "Seattle, WA",
	})
	if err != nil {
		t.Fatalf("marshal expired-session report: %v", err)
	}
	report := productRequest(
		t, client, http.MethodPost, srv.URL+"/api/v1/lost-pets", string(reportBody), csrfToken,
	)
	if report.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(report.Body)
		_ = report.Body.Close()
		t.Fatalf("expired-session report status = %d, want %d; body = %s", report.StatusCode, http.StatusUnauthorized, body)
	}
	responseBody, err := io.ReadAll(report.Body)
	if err != nil {
		t.Fatalf("read expired-session response: %v", err)
	}
	_ = report.Body.Close()
	if !bytes.Equal(responseBody, []byte("Authentication required\n")) {
		t.Fatalf("expired-session response leaked provider detail: %q", responseBody)
	}
	assertSessionCookieCleared(t, report.Cookies(), sessionCookie.Name)
	if cookieNamed(jar.Cookies(baseURL), sessionCookie.Name) != nil {
		t.Fatalf("expired session cookie %q remains in browser jar", sessionCookie.Name)
	}

	for _, collection := range []string{
		store.LostPetsCollection,
		store.ReportContactsCollection,
		store.OutboxCollection,
	} {
		items, err := state.ListState(ctx, collection)
		if err != nil {
			t.Fatalf("list %s after rejected report: %v", collection, err)
		}
		if len(items) != 0 {
			t.Fatalf("%s persisted after rejected report: %#v", collection, items)
		}
	}
}

func httptestServer(
	t *testing.T,
	state store.StateStore,
	sessions identity.SessionManager,
	secureCookies bool,
) *httptest.Server {
	t.Helper()
	return httptest.NewServer(webfrontend.NewServerWithOptions(state, webfrontend.ServerOptions{
		IdentitySessions: sessions, SecureSessionCookie: secureCookies,
	}))
}

func requireLoopbackAuthEmulator(t *testing.T, host, projectID string) {
	t.Helper()
	if projectID != "demo-petspotr-auth" || !strings.HasPrefix(projectID, "demo-") {
		t.Fatalf("refusing to alter a session outside the dedicated demo project: %q", projectID)
	}
	if strings.Contains(host, "://") {
		t.Fatalf("FIREBASE_AUTH_EMULATOR_HOST must not contain a scheme: %q", host)
	}
	hostname, _, err := net.SplitHostPort(host)
	if err != nil {
		t.Fatalf("FIREBASE_AUTH_EMULATOR_HOST must include a port: %q", host)
	}
	ip := net.ParseIP(hostname)
	if ip == nil || !ip.IsLoopback() {
		t.Fatalf("refusing non-loopback Auth emulator host: %q", host)
	}
}

func expireUnsignedEmulatorSessionCookie(t *testing.T, cookie, projectID string) string {
	t.Helper()
	segments := strings.Split(cookie, ".")
	if len(segments) != 3 {
		t.Fatalf("Auth emulator session cookie has %d JWT segments, want 3", len(segments))
	}
	if segments[2] != "" {
		t.Fatal("refusing to alter a signed Auth session cookie")
	}

	var header struct {
		Algorithm string `json:"alg"`
	}
	decodeJWTPart(t, segments[0], &header, "header")
	if header.Algorithm != "none" {
		t.Fatalf("refusing to alter Auth session cookie using algorithm %q", header.Algorithm)
	}

	var claims map[string]interface{}
	decodeJWTPart(t, segments[1], &claims, "claims")
	if claims["aud"] != projectID {
		t.Fatalf("Auth emulator session audience = %#v, want %q", claims["aud"], projectID)
	}
	wantIssuer := "https://session.firebase.google.com/" + projectID
	if claims["iss"] != wantIssuer {
		t.Fatalf("Auth emulator session issuer = %#v, want %q", claims["iss"], wantIssuer)
	}
	if subject, ok := claims["sub"].(string); !ok || strings.TrimSpace(subject) == "" {
		t.Fatalf("Auth emulator session subject = %#v, want non-empty string", claims["sub"])
	}
	expiresAt, ok := claims["exp"].(float64)
	if !ok || expiresAt <= float64(time.Now().Unix()) {
		t.Fatalf("Auth emulator session exp = %#v, want a future timestamp before aging", claims["exp"])
	}
	claims["exp"] = time.Now().Add(-time.Hour).Unix()

	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal aged Auth emulator session claims: %v", err)
	}
	segments[1] = base64.RawURLEncoding.EncodeToString(payload)
	return strings.Join(segments, ".")
}

func decodeJWTPart(t *testing.T, encoded string, target interface{}, part string) {
	t.Helper()
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode Auth emulator session %s: %v", part, err)
	}
	if err := json.Unmarshal(decoded, target); err != nil {
		t.Fatalf("parse Auth emulator session %s: %v", part, err)
	}
}

func requireCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	cookie := cookieNamed(cookies, name)
	if cookie == nil || strings.TrimSpace(cookie.Value) == "" {
		t.Fatalf("cookie %q is missing or empty", name)
	}
	return cookie
}

func cookieNamed(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func assertSessionCookieCleared(t *testing.T, cookies []*http.Cookie, name string) {
	t.Helper()
	cookie := cookieNamed(cookies, name)
	if cookie == nil {
		t.Fatalf("expired-session response did not clear cookie %q", name)
	}
	if cookie.Value != "" || cookie.MaxAge >= 0 || !cookie.Expires.Before(time.Now()) {
		t.Fatalf("expired-session clearing cookie = %#v, want empty expired deletion", cookie)
	}
}
