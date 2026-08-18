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
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/identity"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestGoogleIdentityPlatformSessionJourneyWithAuthEmulator(t *testing.T) {
	host := strings.TrimSpace(os.Getenv("FIREBASE_AUTH_EMULATOR_HOST"))
	if host == "" {
		t.Skip("FIREBASE_AUTH_EMULATOR_HOST is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	const projectID = "demo-petspotr-auth"
	email := fmt.Sprintf("google-owner-%d@example.com", time.Now().UnixNano())
	idToken := signInWithGoogleToAuthEmulator(t, ctx, host, email)

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

	csrfResponse := productRequest(t, client, http.MethodGet, srv.URL+"/api/v1/session/csrf", "", "")
	if csrfResponse.StatusCode != http.StatusOK {
		t.Fatalf("CSRF status = %d, want %d", csrfResponse.StatusCode, http.StatusOK)
	}
	var csrfBody map[string]string
	decodeResponseJSON(t, csrfResponse, &csrfBody)
	csrfToken := csrfBody["csrfToken"]

	loginBody, err := json.Marshal(map[string]string{"idToken": idToken})
	if err != nil {
		t.Fatalf("marshal Google session request: %v", err)
	}
	login := productRequest(t, client, http.MethodPost, srv.URL+"/api/v1/session", string(loginBody), csrfToken)
	if login.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(login.Body)
		_ = login.Body.Close()
		t.Fatalf("Google session status = %d, want %d; body = %s", login.StatusCode, http.StatusCreated, body)
	}
	var principal identity.Principal
	decodeResponseJSON(t, login, &principal)
	if principal.Subject == "" || principal.Email != email || !principal.EmailVerified ||
		principal.SignInProvider != identity.ProviderGoogle {
		t.Fatalf("Google principal = %#v, want verified google.com identity for %q", principal, email)
	}

	logout := productRequest(t, client, http.MethodDelete, srv.URL+"/api/v1/session", "", csrfToken)
	if logout.StatusCode != http.StatusNoContent {
		t.Fatalf("Google logout status = %d, want %d", logout.StatusCode, http.StatusNoContent)
	}
	closeResponse(t, logout)
	afterLogout := productRequest(t, client, http.MethodGet, srv.URL+"/api/v1/session", "", "")
	if afterLogout.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Google post-logout status = %d, want %d", afterLogout.StatusCode, http.StatusUnauthorized)
	}
	closeResponse(t, afterLogout)
}

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
	state := store.NewMemoryStore()
	srv := httptest.NewServer(webfrontend.NewServerWithOptions(state, webfrontend.ServerOptions{
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
	anonymousReport := productRequest(
		t, client, http.MethodPost, srv.URL+"/api/v1/lost-pets",
		`{"petId":"lost-anonymous","petName":"Buddy","reporterEmail":"spoofed@example.com","location":"Seattle, WA"}`, "",
	)
	if anonymousReport.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous lost-report status = %d, want %d", anonymousReport.StatusCode, http.StatusUnauthorized)
	}
	closeResponse(t, anonymousReport)
	anonymousFoundReport := productRequest(
		t, client, http.MethodPost, srv.URL+"/api/v1/found-pets",
		`{"petId":"found-anonymous","imageUrl":"https://storage.petspotr.io/found.jpg","finderEmail":"spoofed@example.com","location":"Seattle, WA"}`, "",
	)
	if anonymousFoundReport.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous found-report status = %d, want %d", anonymousFoundReport.StatusCode, http.StatusUnauthorized)
	}
	closeResponse(t, anonymousFoundReport)

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

	missingReportCSRF := productRequest(
		t, client, http.MethodPost, srv.URL+"/api/v1/lost-pets",
		`{"petId":"lost-no-csrf","petName":"Buddy","reporterEmail":"spoofed@example.com","location":"Seattle, WA"}`, "",
	)
	if missingReportCSRF.StatusCode != http.StatusForbidden {
		t.Fatalf("lost-report without CSRF status = %d, want %d", missingReportCSRF.StatusCode, http.StatusForbidden)
	}
	closeResponse(t, missingReportCSRF)

	reportID := fmt.Sprintf("lost-owner-%d", time.Now().UnixNano())
	reportBody, err := json.Marshal(map[string]string{
		"petId": reportID, "petName": "Buddy", "reporterEmail": "spoofed@example.com",
		"phone": "(555) 010-0200", "location": "Seattle, WA",
	})
	if err != nil {
		t.Fatalf("marshal authenticated lost report: %v", err)
	}
	reportResponse := productRequest(
		t, client, http.MethodPost, srv.URL+"/api/v1/lost-pets", string(reportBody), csrfToken,
	)
	if reportResponse.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(reportResponse.Body)
		_ = reportResponse.Body.Close()
		t.Fatalf("authenticated lost-report status = %d, want %d; body = %s", reportResponse.StatusCode, http.StatusCreated, body)
	}
	closeResponse(t, reportResponse)

	reportData, err := state.GetState(ctx, store.LostPetsCollection, reportID)
	if err != nil {
		t.Fatalf("load authenticated lost report: %v", err)
	}
	var report domain.LostPetRecord
	if err := json.Unmarshal(reportData, &report); err != nil {
		t.Fatalf("decode authenticated lost report: %v", err)
	}
	wantOwner := domain.PrincipalRef{Issuer: loggedIn.Issuer, Subject: loggedIn.Subject}
	if report.OwnedBy == nil || *report.OwnedBy != wantOwner {
		t.Fatalf("persisted lost-report owner = %#v, want %#v", report.OwnedBy, wantOwner)
	}
	contactData, err := state.GetState(ctx, store.ReportContactsCollection, report.OwnerIdentityRef)
	if err != nil {
		t.Fatalf("load authenticated lost-report contact: %v", err)
	}
	var contact domain.ReportContact
	if err := json.Unmarshal(contactData, &contact); err != nil {
		t.Fatalf("decode authenticated lost-report contact: %v", err)
	}
	if contact.Email != email || contact.Email == "spoofed@example.com" {
		t.Fatalf("persisted reporter email = %q, want verified email %q", contact.Email, email)
	}

	publicClient := &http.Client{Timeout: 10 * time.Second}
	publicReports := productRequest(t, publicClient, http.MethodGet, srv.URL+"/api/v1/lost-pets", "", "")
	if publicReports.StatusCode != http.StatusOK {
		t.Fatalf("public lost-report status = %d, want %d", publicReports.StatusCode, http.StatusOK)
	}
	publicBody, err := io.ReadAll(publicReports.Body)
	if err != nil {
		t.Fatalf("read public lost reports: %v", err)
	}
	_ = publicReports.Body.Close()
	if bytes.Contains(publicBody, []byte(email)) || bytes.Contains(publicBody, []byte("ownedBy")) {
		t.Fatalf("public lost reports exposed private identity: %s", publicBody)
	}

	privateContactURL := srv.URL + "/api/v1/lost-pets/" + reportID + "/contact"
	privateContactResponse := productRequest(t, client, http.MethodGet, privateContactURL, "", "")
	if privateContactResponse.StatusCode != http.StatusOK {
		t.Fatalf("owner private-contact status = %d, want %d", privateContactResponse.StatusCode, http.StatusOK)
	}
	var privateContact struct {
		Email string `json:"email"`
		Phone string `json:"phone"`
	}
	decodeResponseJSON(t, privateContactResponse, &privateContact)
	if privateContact.Email != email || privateContact.Phone != "(555) 010-0200" {
		t.Fatalf("owner private contact = %#v, want verified email and stored phone", privateContact)
	}

	anonymousPrivateContact := productRequest(t, publicClient, http.MethodGet, privateContactURL, "", "")
	if anonymousPrivateContact.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous private-contact status = %d, want %d", anonymousPrivateContact.StatusCode, http.StatusUnauthorized)
	}
	closeResponse(t, anonymousPrivateContact)

	otherEmail := fmt.Sprintf("other-%d@example.com", time.Now().UnixNano())
	otherClient, otherPrincipal, otherCSRFToken := createAuthenticatedEmulatorClient(
		t, ctx, setupAuth, host, srv.URL, otherEmail, password,
	)
	wrongOwnerContact := productRequest(t, otherClient, http.MethodGet, privateContactURL, "", "")
	if wrongOwnerContact.StatusCode != http.StatusNotFound {
		t.Fatalf("wrong-owner private-contact status = %d, want %d", wrongOwnerContact.StatusCode, http.StatusNotFound)
	}
	wrongOwnerBody, err := io.ReadAll(wrongOwnerContact.Body)
	if err != nil {
		t.Fatalf("read wrong-owner response: %v", err)
	}
	_ = wrongOwnerContact.Body.Close()
	missingContact := productRequest(
		t, otherClient, http.MethodGet, srv.URL+"/api/v1/lost-pets/lost-missing/contact", "", "",
	)
	missingBody, err := io.ReadAll(missingContact.Body)
	if err != nil {
		t.Fatalf("read missing-contact response: %v", err)
	}
	_ = missingContact.Body.Close()
	if missingContact.StatusCode != http.StatusNotFound || !bytes.Equal(missingBody, wrongOwnerBody) {
		t.Fatalf("missing private-contact response = %d %q, want non-enumerating %d %q", missingContact.StatusCode, missingBody, http.StatusNotFound, wrongOwnerBody)
	}

	missingFoundCSRF := productRequest(
		t, client, http.MethodPost, srv.URL+"/api/v1/found-pets",
		`{"petId":"found-no-csrf","imageUrl":"https://storage.petspotr.io/found.jpg","finderEmail":"spoofed@example.com","location":"Seattle, WA"}`, "",
	)
	if missingFoundCSRF.StatusCode != http.StatusForbidden {
		t.Fatalf("found-report without CSRF status = %d, want %d", missingFoundCSRF.StatusCode, http.StatusForbidden)
	}
	closeResponse(t, missingFoundCSRF)

	foundReportID := fmt.Sprintf("found-owner-%d", time.Now().UnixNano())
	foundReportBody, err := json.Marshal(map[string]string{
		"petId": foundReportID, "imageUrl": "https://storage.petspotr.io/found.jpg",
		"finderEmail": "spoofed@example.com", "location": "Seattle, WA",
	})
	if err != nil {
		t.Fatalf("marshal authenticated found report: %v", err)
	}
	foundReportResponse := productRequest(
		t, client, http.MethodPost, srv.URL+"/api/v1/found-pets", string(foundReportBody), csrfToken,
	)
	if foundReportResponse.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(foundReportResponse.Body)
		_ = foundReportResponse.Body.Close()
		t.Fatalf("authenticated found-report status = %d, want %d; body = %s", foundReportResponse.StatusCode, http.StatusCreated, body)
	}
	closeResponse(t, foundReportResponse)

	foundReportData, err := state.GetState(ctx, store.FoundPetsCollection, foundReportID)
	if err != nil {
		t.Fatalf("load authenticated found report: %v", err)
	}
	var foundReport domain.FoundPetRecord
	if err := json.Unmarshal(foundReportData, &foundReport); err != nil {
		t.Fatalf("decode authenticated found report: %v", err)
	}
	if foundReport.OwnedBy == nil || *foundReport.OwnedBy != wantOwner {
		t.Fatalf("persisted found-report owner = %#v, want %#v", foundReport.OwnedBy, wantOwner)
	}
	foundContactData, err := state.GetState(ctx, store.ReportContactsCollection, foundReport.FinderIdentityRef)
	if err != nil {
		t.Fatalf("load authenticated found-report contact: %v", err)
	}
	var foundContact domain.ReportContact
	if err := json.Unmarshal(foundContactData, &foundContact); err != nil {
		t.Fatalf("decode authenticated found-report contact: %v", err)
	}
	if foundContact.Email != email || foundContact.Email == "spoofed@example.com" {
		t.Fatalf("persisted finder email = %q, want verified email %q", foundContact.Email, email)
	}

	publicFoundReports := productRequest(t, publicClient, http.MethodGet, srv.URL+"/api/v1/found-pets", "", "")
	if publicFoundReports.StatusCode != http.StatusOK {
		t.Fatalf("public found-report status = %d, want %d", publicFoundReports.StatusCode, http.StatusOK)
	}
	publicFoundBody, err := io.ReadAll(publicFoundReports.Body)
	if err != nil {
		t.Fatalf("read public found reports: %v", err)
	}
	_ = publicFoundReports.Body.Close()
	if bytes.Contains(publicFoundBody, []byte(email)) || bytes.Contains(publicFoundBody, []byte("ownedBy")) {
		t.Fatalf("public found reports exposed private identity: %s", publicFoundBody)
	}

	privateFoundContactURL := srv.URL + "/api/v1/found-pets/" + foundReportID + "/contact"
	privateFoundContactResponse := productRequest(t, client, http.MethodGet, privateFoundContactURL, "", "")
	if privateFoundContactResponse.StatusCode != http.StatusOK {
		t.Fatalf("finder private-contact status = %d, want %d", privateFoundContactResponse.StatusCode, http.StatusOK)
	}
	var privateFoundContact map[string]string
	decodeResponseJSON(t, privateFoundContactResponse, &privateFoundContact)
	if privateFoundContact["email"] != email || privateFoundContact["phone"] != "" {
		t.Fatalf("finder private contact = %#v, want verified email without phone", privateFoundContact)
	}

	anonymousFoundContact := productRequest(t, publicClient, http.MethodGet, privateFoundContactURL, "", "")
	if anonymousFoundContact.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous found-contact status = %d, want %d", anonymousFoundContact.StatusCode, http.StatusUnauthorized)
	}
	closeResponse(t, anonymousFoundContact)
	wrongOwnerFoundContact := productRequest(t, otherClient, http.MethodGet, privateFoundContactURL, "", "")
	if wrongOwnerFoundContact.StatusCode != http.StatusNotFound {
		t.Fatalf("wrong-owner found-contact status = %d, want %d", wrongOwnerFoundContact.StatusCode, http.StatusNotFound)
	}
	wrongOwnerFoundBody, err := io.ReadAll(wrongOwnerFoundContact.Body)
	if err != nil {
		t.Fatalf("read wrong-owner found-contact response: %v", err)
	}
	_ = wrongOwnerFoundContact.Body.Close()
	missingFoundContact := productRequest(
		t, otherClient, http.MethodGet, srv.URL+"/api/v1/found-pets/found-missing/contact", "", "",
	)
	missingFoundContactBody, err := io.ReadAll(missingFoundContact.Body)
	if err != nil {
		t.Fatalf("read missing found-contact response: %v", err)
	}
	_ = missingFoundContact.Body.Close()
	if missingFoundContact.StatusCode != http.StatusNotFound || !bytes.Equal(missingFoundContactBody, wrongOwnerFoundBody) {
		t.Fatalf("missing found-contact response = %d %q, want non-enumerating %d %q", missingFoundContact.StatusCode, missingFoundContactBody, http.StatusNotFound, wrongOwnerFoundBody)
	}

	sharedMatch := domain.MatchRecord{
		MatchID: "match-shared", FoundPetID: foundReportID, MatchedPetID: reportID,
		Status: domain.MatchStatusPendingReview,
	}
	seedIdentityMatch(t, state, sharedMatch, &domain.MatchParticipantRecord{
		MatchID: sharedMatch.MatchID, FoundPetID: sharedMatch.FoundPetID, LostPetID: sharedMatch.MatchedPetID,
		Reporter: &wantOwner,
		Finder: &domain.PrincipalRef{
			Issuer: otherPrincipal.Issuer, Subject: otherPrincipal.Subject,
		},
	})
	unrelatedMatch := domain.MatchRecord{
		MatchID: "match-unrelated", FoundPetID: "found-unrelated", MatchedPetID: "lost-unrelated",
	}
	unrelatedOwner := &domain.PrincipalRef{Issuer: loggedIn.Issuer, Subject: "unrelated-user"}
	seedIdentityMatch(t, state, unrelatedMatch, &domain.MatchParticipantRecord{
		MatchID: unrelatedMatch.MatchID, FoundPetID: unrelatedMatch.FoundPetID,
		LostPetID: unrelatedMatch.MatchedPetID, Reporter: unrelatedOwner,
	})
	for name, participantClient := range map[string]*http.Client{
		"reporter": client,
		"finder":   otherClient,
	} {
		response := productRequest(t, participantClient, http.MethodGet, srv.URL+"/api/v1/matches", "", "")
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s match-list status = %d, want %d", name, response.StatusCode, http.StatusOK)
		}
		if response.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("%s match-list Cache-Control = %q, want no-store", name, response.Header.Get("Cache-Control"))
		}
		var matches []domain.MatchRecord
		decodeResponseJSON(t, response, &matches)
		if len(matches) != 1 || matches[0].MatchID != sharedMatch.MatchID {
			t.Fatalf("%s matches = %#v, want only %s", name, matches, sharedMatch.MatchID)
		}
	}
	anonymousMatches := productRequest(t, publicClient, http.MethodGet, srv.URL+"/api/v1/matches", "", "")
	if anonymousMatches.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous match-list status = %d, want %d", anonymousMatches.StatusCode, http.StatusUnauthorized)
	}
	closeResponse(t, anonymousMatches)

	matchActionURL := srv.URL + "/api/v1/matches/action"
	actionBody := `{"matchId":"match-shared","action":"confirm"}`
	anonymousAction := productRequest(t, publicClient, http.MethodPost, matchActionURL, actionBody, "")
	if anonymousAction.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous match-action status = %d, want %d", anonymousAction.StatusCode, http.StatusUnauthorized)
	}
	closeResponse(t, anonymousAction)
	missingActionCSRF := productRequest(t, client, http.MethodPost, matchActionURL, actionBody, "")
	if missingActionCSRF.StatusCode != http.StatusForbidden {
		t.Fatalf("match action without CSRF status = %d, want %d", missingActionCSRF.StatusCode, http.StatusForbidden)
	}
	closeResponse(t, missingActionCSRF)

	wrongOwnerAction := productRequest(
		t, client, http.MethodPost, matchActionURL, `{"matchId":"match-unrelated","action":"confirm"}`, csrfToken,
	)
	wrongOwnerActionBody, err := io.ReadAll(wrongOwnerAction.Body)
	if err != nil {
		t.Fatal(err)
	}
	_ = wrongOwnerAction.Body.Close()
	missingAction := productRequest(
		t, client, http.MethodPost, matchActionURL, `{"matchId":"match-missing","action":"confirm"}`, csrfToken,
	)
	missingActionBody, err := io.ReadAll(missingAction.Body)
	if err != nil {
		t.Fatal(err)
	}
	_ = missingAction.Body.Close()
	if wrongOwnerAction.StatusCode != http.StatusNotFound || missingAction.StatusCode != http.StatusNotFound ||
		!bytes.Equal(wrongOwnerActionBody, missingActionBody) {
		t.Fatalf("non-enumerating match actions = %d %q / %d %q",
			wrongOwnerAction.StatusCode, wrongOwnerActionBody, missingAction.StatusCode, missingActionBody)
	}

	reporterAction := productRequest(t, client, http.MethodPost, matchActionURL, actionBody, csrfToken)
	if reporterAction.StatusCode != http.StatusOK || reporterAction.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("reporter match-action response = %d Cache-Control %q",
			reporterAction.StatusCode, reporterAction.Header.Get("Cache-Control"))
	}
	var reporterActionResult map[string]string
	decodeResponseJSON(t, reporterAction, &reporterActionResult)
	if reporterActionResult["status"] != string(domain.MatchStatusPendingReview) {
		t.Fatalf("reporter match-action result = %#v", reporterActionResult)
	}
	exactRetry := productRequest(t, client, http.MethodPost, matchActionURL, actionBody, csrfToken)
	if exactRetry.StatusCode != http.StatusOK {
		t.Fatalf("exact match-action retry status = %d, want %d", exactRetry.StatusCode, http.StatusOK)
	}
	closeResponse(t, exactRetry)

	finderAction := productRequest(t, otherClient, http.MethodPost, matchActionURL, actionBody, otherCSRFToken)
	if finderAction.StatusCode != http.StatusOK {
		t.Fatalf("finder match-action status = %d, want %d", finderAction.StatusCode, http.StatusOK)
	}
	var finderActionResult map[string]string
	decodeResponseJSON(t, finderAction, &finderActionResult)
	if finderActionResult["status"] != string(domain.MatchStatusConfirmed) {
		t.Fatalf("finder match-action result = %#v", finderActionResult)
	}
	conflictingAction := productRequest(
		t, client, http.MethodPost, matchActionURL, `{"matchId":"match-shared","action":"reject"}`, csrfToken,
	)
	if conflictingAction.StatusCode != http.StatusConflict {
		t.Fatalf("changed match-action status = %d, want %d", conflictingAction.StatusCode, http.StatusConflict)
	}
	closeResponse(t, conflictingAction)

	threadURL := srv.URL + "/api/v1/reunions/contact"
	reporterMessageBody := `{"matchId":"match-shared","senderEmail":"spoofed@example.com","message":"Can we compare identifying marks?"}`
	anonymousMessage := mediatedProductRequest(
		t, publicClient, http.MethodPost, threadURL, reporterMessageBody, "", "request-101",
	)
	if anonymousMessage.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous mediated-message status = %d, want %d", anonymousMessage.StatusCode, http.StatusUnauthorized)
	}
	closeResponse(t, anonymousMessage)
	wrongOwnerMessage := mediatedProductRequest(
		t, client, http.MethodPost, threadURL,
		`{"matchId":"match-unrelated","message":"hello"}`, csrfToken, "request-wrong-owner",
	)
	wrongOwnerMessageBody, err := io.ReadAll(wrongOwnerMessage.Body)
	if err != nil {
		t.Fatal(err)
	}
	_ = wrongOwnerMessage.Body.Close()
	missingMessage := mediatedProductRequest(
		t, client, http.MethodPost, threadURL,
		`{"matchId":"match-missing","message":"hello"}`, csrfToken, "request-missing",
	)
	missingMessageBody, err := io.ReadAll(missingMessage.Body)
	if err != nil {
		t.Fatal(err)
	}
	_ = missingMessage.Body.Close()
	if wrongOwnerMessage.StatusCode != http.StatusNotFound || missingMessage.StatusCode != http.StatusNotFound ||
		!bytes.Equal(wrongOwnerMessageBody, missingMessageBody) {
		t.Fatalf("non-enumerating mediated messages = %d %q / %d %q",
			wrongOwnerMessage.StatusCode, wrongOwnerMessageBody, missingMessage.StatusCode, missingMessageBody)
	}

	reporterMessage := mediatedProductRequest(
		t, client, http.MethodPost, threadURL, reporterMessageBody, csrfToken, "request-101",
	)
	if reporterMessage.StatusCode != http.StatusCreated || reporterMessage.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("reporter mediated-message response = %d Cache-Control %q",
			reporterMessage.StatusCode, reporterMessage.Header.Get("Cache-Control"))
	}
	var acceptedReporterMessage map[string]any
	decodeResponseJSON(t, reporterMessage, &acceptedReporterMessage)
	retryMessage := mediatedProductRequest(
		t, client, http.MethodPost, threadURL, reporterMessageBody, csrfToken, "request-101",
	)
	if retryMessage.StatusCode != http.StatusOK {
		t.Fatalf("mediated-message exact retry status = %d, want %d", retryMessage.StatusCode, http.StatusOK)
	}
	closeResponse(t, retryMessage)
	changedMessage := mediatedProductRequest(
		t, client, http.MethodPost, threadURL,
		`{"matchId":"match-shared","message":"Different body"}`, csrfToken, "request-101",
	)
	if changedMessage.StatusCode != http.StatusConflict {
		t.Fatalf("mediated-message changed retry status = %d, want %d", changedMessage.StatusCode, http.StatusConflict)
	}
	closeResponse(t, changedMessage)
	finderMessage := mediatedProductRequest(
		t, otherClient, http.MethodPost, threadURL,
		`{"matchId":"match-shared","message":"Yes, there is a white spot on the left paw."}`,
		otherCSRFToken, "request-202",
	)
	if finderMessage.StatusCode != http.StatusCreated {
		t.Fatalf("finder mediated-message status = %d, want %d", finderMessage.StatusCode, http.StatusCreated)
	}
	closeResponse(t, finderMessage)
	for name, participantClient := range map[string]*http.Client{"reporter": client, "finder": otherClient} {
		thread := productRequest(t, participantClient, http.MethodGet, threadURL+"?matchId=match-shared", "", "")
		if thread.StatusCode != http.StatusOK || thread.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("%s mediated-thread response = %d Cache-Control %q",
				name, thread.StatusCode, thread.Header.Get("Cache-Control"))
		}
		threadBody, err := io.ReadAll(thread.Body)
		if err != nil {
			t.Fatal(err)
		}
		_ = thread.Body.Close()
		if bytes.Contains(threadBody, []byte(email)) || bytes.Contains(threadBody, []byte(otherEmail)) ||
			bytes.Contains(threadBody, []byte(loggedIn.Subject)) || bytes.Contains(threadBody, []byte(otherPrincipal.Subject)) ||
			bytes.Count(threadBody, []byte(`"messageId"`)) != 2 {
			t.Fatalf("%s mediated thread exposed identity or omitted messages: %s", name, threadBody)
		}
	}

	storedMatchData, err := state.GetState(ctx, store.MatchesCollection, sharedMatch.MatchID)
	if err != nil {
		t.Fatal(err)
	}
	storedParticipantsData, err := state.GetState(ctx, store.MatchParticipantsCollection, sharedMatch.MatchID)
	if err != nil {
		t.Fatal(err)
	}
	var storedMatch domain.MatchRecord
	var storedParticipants domain.MatchParticipantRecord
	if err := json.Unmarshal(storedMatchData, &storedMatch); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(storedParticipantsData, &storedParticipants); err != nil {
		t.Fatal(err)
	}
	if storedMatch.Status != domain.MatchStatusConfirmed ||
		storedParticipants.ReporterDecision != domain.MatchDecisionConfirm ||
		storedParticipants.FinderDecision != domain.MatchDecisionConfirm || len(storedParticipants.DecisionAudit) != 2 ||
		len(storedParticipants.Messages) != 2 {
		t.Fatalf("stored bilateral decision = %#v / %#v", storedMatch, storedParticipants)
	}
	if bytes.Contains(storedParticipantsData, []byte("spoofed@example.com")) {
		t.Fatalf("mediated thread persisted caller-supplied contact: %s", storedParticipantsData)
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
	afterLogoutContact := productRequest(t, client, http.MethodGet, privateContactURL, "", "")
	if afterLogoutContact.StatusCode != http.StatusUnauthorized {
		t.Fatalf("post-logout private-contact status = %d, want %d", afterLogoutContact.StatusCode, http.StatusUnauthorized)
	}
	closeResponse(t, afterLogoutContact)
	afterLogoutFoundContact := productRequest(t, client, http.MethodGet, privateFoundContactURL, "", "")
	if afterLogoutFoundContact.StatusCode != http.StatusUnauthorized {
		t.Fatalf("post-logout found-contact status = %d, want %d", afterLogoutFoundContact.StatusCode, http.StatusUnauthorized)
	}
	closeResponse(t, afterLogoutFoundContact)
	afterLogoutMatches := productRequest(t, client, http.MethodGet, srv.URL+"/api/v1/matches", "", "")
	if afterLogoutMatches.StatusCode != http.StatusUnauthorized {
		t.Fatalf("post-logout match-list status = %d, want %d", afterLogoutMatches.StatusCode, http.StatusUnauthorized)
	}
	closeResponse(t, afterLogoutMatches)
	afterLogoutAction := productRequest(t, client, http.MethodPost, matchActionURL, actionBody, csrfToken)
	if afterLogoutAction.StatusCode != http.StatusUnauthorized {
		t.Fatalf("post-logout match-action status = %d, want %d", afterLogoutAction.StatusCode, http.StatusUnauthorized)
	}
	closeResponse(t, afterLogoutAction)
	afterLogoutThread := productRequest(t, client, http.MethodGet, threadURL+"?matchId=match-shared", "", "")
	if afterLogoutThread.StatusCode != http.StatusUnauthorized {
		t.Fatalf("post-logout mediated-thread status = %d, want %d", afterLogoutThread.StatusCode, http.StatusUnauthorized)
	}
	closeResponse(t, afterLogoutThread)
	afterLogoutReport := productRequest(
		t, client, http.MethodPost, srv.URL+"/api/v1/lost-pets",
		`{"petId":"lost-after-logout","petName":"Buddy","reporterEmail":"spoofed@example.com","location":"Seattle, WA"}`, csrfToken,
	)
	if afterLogoutReport.StatusCode != http.StatusUnauthorized {
		t.Fatalf("post-logout lost-report status = %d, want %d", afterLogoutReport.StatusCode, http.StatusUnauthorized)
	}
	closeResponse(t, afterLogoutReport)
	afterLogoutFoundReport := productRequest(
		t, client, http.MethodPost, srv.URL+"/api/v1/found-pets",
		`{"petId":"found-after-logout","imageUrl":"https://storage.petspotr.io/found.jpg","finderEmail":"spoofed@example.com","location":"Seattle, WA"}`, csrfToken,
	)
	if afterLogoutFoundReport.StatusCode != http.StatusUnauthorized {
		t.Fatalf("post-logout found-report status = %d, want %d", afterLogoutFoundReport.StatusCode, http.StatusUnauthorized)
	}
	closeResponse(t, afterLogoutFoundReport)
}

func createAuthenticatedEmulatorClient(
	t *testing.T,
	ctx context.Context,
	setupAuth *auth.Client,
	host string,
	baseURL string,
	email string,
	password string,
) (*http.Client, identity.Principal, string) {
	t.Helper()
	created, err := setupAuth.CreateUser(ctx, (&auth.UserToCreate{}).
		Email(email).
		EmailVerified(true).
		Password(password))
	if err != nil {
		t.Fatalf("create additional verified emulator user: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = setupAuth.DeleteUser(cleanupCtx, created.UID)
	})
	idToken := signInToAuthEmulator(t, ctx, host, email, password)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create additional cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	csrfResponse := productRequest(t, client, http.MethodGet, baseURL+"/api/v1/session/csrf", "", "")
	if csrfResponse.StatusCode != http.StatusOK {
		t.Fatalf("additional CSRF status = %d, want %d", csrfResponse.StatusCode, http.StatusOK)
	}
	var csrfBody map[string]string
	decodeResponseJSON(t, csrfResponse, &csrfBody)
	login := productRequest(
		t, client, http.MethodPost, baseURL+"/api/v1/session", `{"idToken":"`+idToken+`"}`, csrfBody["csrfToken"],
	)
	if login.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(login.Body)
		_ = login.Body.Close()
		t.Fatalf("additional session login status = %d, want %d; body = %s", login.StatusCode, http.StatusCreated, body)
	}
	var principal identity.Principal
	decodeResponseJSON(t, login, &principal)
	return client, principal, csrfBody["csrfToken"]
}

func seedIdentityMatch(
	t *testing.T,
	state store.StateStore,
	match domain.MatchRecord,
	participants *domain.MatchParticipantRecord,
) {
	t.Helper()
	matchData, err := json.Marshal(match)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(context.Background(), store.MatchesCollection, match.MatchID, matchData); err != nil {
		t.Fatal(err)
	}
	if participants == nil {
		return
	}
	participantData, err := json.Marshal(participants)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(
		context.Background(), store.MatchParticipantsCollection, participants.MatchID, participantData,
	); err != nil {
		t.Fatal(err)
	}
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

func signInWithGoogleToAuthEmulator(t *testing.T, ctx context.Context, host, email string) string {
	t.Helper()
	credential, err := json.Marshal(map[string]interface{}{
		"sub":            "google-" + strings.ReplaceAll(email, "@", "-"),
		"email":          email,
		"email_verified": true,
	})
	if err != nil {
		t.Fatalf("marshal Google emulator credential: %v", err)
	}
	postBody := url.Values{
		"id_token":   []string{string(credential)},
		"providerId": []string{identity.ProviderGoogle},
	}.Encode()
	body, err := json.Marshal(map[string]interface{}{
		"postBody": postBody, "requestUri": "http://localhost",
		"returnIdpCredential": true, "returnSecureToken": true,
	})
	if err != nil {
		t.Fatalf("marshal Google emulator sign-in: %v", err)
	}
	endpoint := "http://" + host + "/identitytoolkit.googleapis.com/v1/accounts:signInWithIdp?key=fake-api-key"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create Google emulator sign-in request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("sign in with Google through Auth emulator: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(response.Body)
		t.Fatalf("Google Auth emulator sign-in status = %d; body = %s", response.StatusCode, responseBody)
	}
	var signedIn struct {
		IDToken string `json:"idToken"`
	}
	if err := json.NewDecoder(response.Body).Decode(&signedIn); err != nil {
		t.Fatalf("decode Google Auth emulator sign-in: %v", err)
	}
	if signedIn.IDToken == "" {
		t.Fatal("Google Auth emulator returned an empty ID token")
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

func mediatedProductRequest(
	t *testing.T,
	client *http.Client,
	method, url, body, csrfToken, idempotencyKey string,
) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("create mediated product request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if csrfToken != "" {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response, err := client.Do(req)
	if err != nil {
		t.Fatalf("execute mediated product request: %v", err)
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
