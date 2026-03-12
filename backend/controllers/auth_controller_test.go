package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bigtoy/backend/services"
)

func newControllerAuthService(t *testing.T, requireSecure bool) *services.AuthService {
	t.Helper()

	hash, err := services.GeneratePasswordHash("Secret#12345")
	if err != nil {
		t.Fatalf("generate hash: %v", err)
	}

	svc, err := services.NewAuthService(services.AuthConfig{
		AdminUser:           "admin",
		AdminPasswordHash:   hash,
		SessionTTL:          time.Minute,
		CookieName:          "bigtoy_test_cookie",
		RequireSecureCookie: requireSecure,
	})
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	return svc
}

func TestReadSessionToken(t *testing.T) {
	authService = newControllerAuthService(t, false)
	t.Cleanup(func() { authService = nil })

	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.AddCookie(&http.Cookie{
		Name:  authService.CookieName(),
		Value: " test-token ",
	})

	token := readSessionToken(request)
	if token != "test-token" {
		t.Fatalf("unexpected token value: %q", token)
	}

	if token := readSessionToken(nil); token != "" {
		t.Fatalf("expected empty token for nil request, got %q", token)
	}
}

func TestSessionFromRequest(t *testing.T) {
	authService = newControllerAuthService(t, false)
	t.Cleanup(func() { authService = nil })

	token, _, err := authService.Login("127.0.0.1", "admin", "Secret#12345")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: authService.CookieName(), Value: token})

	session, ok := sessionFromRequest(request)
	if !ok {
		t.Fatal("expected valid session")
	}
	if session.Username != "admin" {
		t.Fatalf("unexpected username: %s", session.Username)
	}
}

func TestSetAndClearAuthCookie(t *testing.T) {
	authService = newControllerAuthService(t, true)
	t.Cleanup(func() { authService = nil })

	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	recorder := httptest.NewRecorder()

	expiresAt := time.Now().Add(time.Minute)
	setAuthCookie(recorder, request, "abc123", expiresAt)

	result := recorder.Result()
	cookies := result.Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected set-cookie header")
	}

	loginCookie := cookies[0]
	if loginCookie.Name != authService.CookieName() || loginCookie.Value != "abc123" {
		t.Fatalf("unexpected login cookie: %#v", loginCookie)
	}
	if !loginCookie.HttpOnly {
		t.Fatal("expected HttpOnly cookie")
	}
	if !loginCookie.Secure {
		t.Fatal("expected Secure cookie")
	}

	clearRecorder := httptest.NewRecorder()
	clearAuthCookie(clearRecorder, request)
	clearCookies := clearRecorder.Result().Cookies()
	if len(clearCookies) == 0 {
		t.Fatal("expected clear-cookie header")
	}
	if clearCookies[0].MaxAge != -1 {
		t.Fatalf("expected Max-Age=-1 on clear cookie, got %d", clearCookies[0].MaxAge)
	}
}
