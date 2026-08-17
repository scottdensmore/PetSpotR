package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
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

func TestIdentityPlatformSessionJourneyWithAuthEmulator(t *testing.T) {
	host := strings.TrimSpace(os.Getenv("FIREBASE_AUTH_EMULATOR_HOST"))
	if host == "" {
		t.Skip("FIREBASE_AUTH_EMULATOR_HOST is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	projectID := "demo-petspotr-auth"
	email := fmt.Sprintf("owner-%d@example.com", time.Now().UnixNano())
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
	srv := httptest.NewServer(webfrontend.NewServerWithOptions(store.NewMemoryStore(), webfrontend.ServerOptions{
		IdentitySessions: identityRuntime.Sessions, SecureSessionCookie: identityRuntime.SecureCookies,
	}))
	defer srv.Close()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}

	anonymous := productRequest(t, client, http.MethodGet, srv.URL+"/api/v1/session", "", "")
	if anonymous.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous session status = %d, want %d", anonymous.StatusCode, http.StatusUnauthorized)
	}
	closeResponse(t, anonymous)

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

	wrongCSRF := productRequest(t, client, http.MethodPost, srv.URL+"/api/v1/session", `{"idToken":"`+idToken+`"}`, "wrong-token")
	if wrongCSRF.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong-CSRF login status = %d, want %d", wrongCSRF.StatusCode, http.StatusForbidden)
	}
	closeResponse(t, wrongCSRF)

	login := productRequest(t, client, http.MethodPost, srv.URL+"/api/v1/session", `{"idToken":"`+idToken+`"}`, csrfToken)
	if login.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(login.Body)
		_ = login.Body.Close()
		t.Fatalf("session login status = %d, want %d; body = %s", login.StatusCode, http.StatusCreated, body)
	}
	var loggedIn identity.Principal
	decodeResponseJSON(t, login, &loggedIn)
	if loggedIn.Subject != created.UID || loggedIn.Email != email || !loggedIn.EmailVerified {
		t.Fatalf("login principal = %#v, want UID %q and verified email %q", loggedIn, created.UID, email)
	}

	current := productRequest(t, client, http.MethodGet, srv.URL+"/api/v1/session", "", "")
	if current.StatusCode != http.StatusOK {
		t.Fatalf("current session status = %d, want %d", current.StatusCode, http.StatusOK)
	}
	var verified identity.Principal
	decodeResponseJSON(t, current, &verified)
	if verified != loggedIn {
		t.Fatalf("verified principal = %#v, want %#v", verified, loggedIn)
	}

	logout := productRequest(t, client, http.MethodDelete, srv.URL+"/api/v1/session", "", csrfToken)
	if logout.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d", logout.StatusCode, http.StatusNoContent)
	}
	closeResponse(t, logout)

	afterLogout := productRequest(t, client, http.MethodGet, srv.URL+"/api/v1/session", "", "")
	if afterLogout.StatusCode != http.StatusUnauthorized {
		t.Fatalf("post-logout session status = %d, want %d", afterLogout.StatusCode, http.StatusUnauthorized)
	}
	closeResponse(t, afterLogout)
}

func signInToAuthEmulator(t *testing.T, ctx context.Context, host, email, password string) string {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{
		"email": email, "password": password, "returnSecureToken": true,
	})
	if err != nil {
		t.Fatalf("marshal emulator sign-in: %v", err)
	}
	url := "http://" + host + "/identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=fake-api-key"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create emulator sign-in request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("sign in through Auth emulator: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(response.Body)
		t.Fatalf("Auth emulator sign-in status = %d; body = %s", response.StatusCode, responseBody)
	}
	var signedIn struct {
		IDToken string `json:"idToken"`
	}
	if err := json.NewDecoder(response.Body).Decode(&signedIn); err != nil {
		t.Fatalf("decode Auth emulator sign-in: %v", err)
	}
	if signedIn.IDToken == "" {
		t.Fatal("Auth emulator returned an empty ID token")
	}
	return signedIn.IDToken
}

func productRequest(t *testing.T, client *http.Client, method, url, body, csrfToken string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("create product request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if csrfToken != "" {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}
	response, err := client.Do(req)
	if err != nil {
		t.Fatalf("execute product request: %v", err)
	}
	return response
}

func decodeResponseJSON(t *testing.T, response *http.Response, target interface{}) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatalf("decode product response: %v", err)
	}
}

func closeResponse(t *testing.T, response *http.Response) {
	t.Helper()
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		t.Errorf("drain response body: %v", err)
	}
	if err := response.Body.Close(); err != nil {
		t.Errorf("close response body: %v", err)
	}
}
