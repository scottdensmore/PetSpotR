package webfrontend

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/identity"
)

const (
	secureSessionCookieName = "__Host-petspotr_session"
	localSessionCookieName  = "petspotr_session"
	csrfCookieName          = "__Host-petspotr_csrf"
	localCSRFCookieName     = "petspotr_csrf"
	csrfHeaderName          = "X-CSRF-Token"
	sessionLifetime         = 5 * 24 * time.Hour
	maximumSessionBodyBytes = 16 * 1024
)

type sessionLoginRequest struct {
	IDToken string `json:"idToken"`
}

func (s *Server) handleApiIdentityClientConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.identityClientConfig)
}

func (s *Server) handleApiSessionCSRF(w http.ResponseWriter, r *http.Request) {
	if s.identitySessions == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token, err := newCSRFToken()
	if err != nil {
		http.Error(w, "Unable to start a session", http.StatusServiceUnavailable)
		return
	}
	s.setCSRFCookie(w, token, int(sessionLifetime.Seconds()), time.Now().Add(sessionLifetime))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"csrfToken": token})
}

func (s *Server) handleApiSession(w http.ResponseWriter, r *http.Request) {
	if s.identitySessions == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodPost:
		s.handleSessionLogin(w, r)
	case http.MethodGet:
		s.handleCurrentSession(w, r)
	case http.MethodDelete:
		s.handleSessionLogout(w, r)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSessionLogin(w http.ResponseWriter, r *http.Request) {
	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maximumSessionBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var login sessionLoginRequest
	if err := decoder.Decode(&login); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(login.IDToken) == "" {
		http.Error(w, "Identity token is required", http.StatusBadRequest)
		return
	}

	session, err := s.identitySessions.CreateSession(r.Context(), login.IDToken, sessionLifetime)
	switch {
	case errors.Is(err, identity.ErrEmailNotVerified):
		http.Error(w, "Verified email is required", http.StatusForbidden)
		return
	case errors.Is(err, identity.ErrUnauthenticated), errors.Is(err, identity.ErrRecentSignInRequired):
		http.Error(w, "Unable to authenticate", http.StatusUnauthorized)
		return
	case err != nil:
		http.Error(w, "Unable to create session", http.StatusServiceUnavailable)
		return
	}

	s.setSessionCookie(w, session.Cookie, int(sessionLifetime.Seconds()), time.Now().Add(sessionLifetime))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(session.Principal)
}

func (s *Server) handleCurrentSession(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.verifiedRequestPrincipal(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(principal)
}

func (s *Server) verifiedRequestPrincipal(w http.ResponseWriter, r *http.Request) (identity.Principal, bool) {
	cookie, err := r.Cookie(s.sessionCookieName())
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return identity.Principal{}, false
	}
	principal, err := s.identitySessions.VerifySession(r.Context(), cookie.Value)
	if err != nil {
		s.setSessionCookie(w, "", -1, time.Unix(1, 0))
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return identity.Principal{}, false
	}
	return principal, true
}

func (s *Server) handleSessionLogout(w http.ResponseWriter, r *http.Request) {
	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	s.setSessionCookie(w, "", -1, time.Unix(1, 0))
	s.setCSRFCookie(w, "", -1, time.Unix(1, 0))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) validCSRF(r *http.Request) bool {
	header := r.Header.Get(csrfHeaderName)
	cookie, err := r.Cookie(s.csrfCookieName())
	if err != nil || header == "" || cookie.Value == "" || len(header) != len(cookie.Value) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(header), []byte(cookie.Value)) == 1
}

func (s *Server) setSessionCookie(w http.ResponseWriter, value string, maxAge int, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: s.sessionCookieName(), Value: value, Path: "/", Expires: expires,
		MaxAge: maxAge, HttpOnly: true, Secure: s.secureSessionCookie,
		SameSite: http.SameSiteStrictMode,
	})
}

func (s *Server) setCSRFCookie(w http.ResponseWriter, value string, maxAge int, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: s.csrfCookieName(), Value: value, Path: "/", Expires: expires,
		MaxAge: maxAge, HttpOnly: true, Secure: s.secureSessionCookie,
		SameSite: http.SameSiteStrictMode,
	})
}

func (s *Server) sessionCookieName() string {
	if s.secureSessionCookie {
		return secureSessionCookieName
	}
	return localSessionCookieName
}

func (s *Server) csrfCookieName() string {
	if s.secureSessionCookie {
		return csrfCookieName
	}
	return localCSRFCookieName
}

func newCSRFToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
