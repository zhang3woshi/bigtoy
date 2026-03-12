package services

import (
	"crypto/tls"
	"errors"
	"net/http"
	"testing"
	"time"
)

func newAuthServiceForTest(t *testing.T, cfg AuthConfig) *AuthService {
	t.Helper()

	hash, err := GeneratePasswordHash("Secret#12345")
	if err != nil {
		t.Fatalf("generate hash: %v", err)
	}

	if cfg.AdminUser == "" {
		cfg.AdminUser = "admin"
	}
	if cfg.AdminPasswordHash == "" {
		cfg.AdminPasswordHash = hash
	}

	service, err := NewAuthService(cfg)
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	return service
}

func TestNewAuthServiceValidationAndDefaults(t *testing.T) {
	hash, err := GeneratePasswordHash("Secret#12345")
	if err != nil {
		t.Fatalf("generate hash: %v", err)
	}

	if _, err := NewAuthService(AuthConfig{AdminPasswordHash: hash}); err == nil {
		t.Fatal("expected missing admin username error")
	}
	if _, err := NewAuthService(AuthConfig{AdminUser: "admin"}); err == nil {
		t.Fatal("expected missing admin password hash error")
	}
	if _, err := NewAuthService(AuthConfig{AdminUser: "admin", AdminPasswordHash: "not-bcrypt"}); err == nil {
		t.Fatal("expected invalid bcrypt hash error")
	}

	service, err := NewAuthService(AuthConfig{
		AdminUser:         " admin ",
		AdminPasswordHash: hash,
	})
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}

	if service.CookieName() != "bigtoy_admin_session" {
		t.Fatalf("unexpected default cookie name: %s", service.CookieName())
	}
	if service.CookieMaxAgeSeconds() != int((2*time.Hour)/time.Second) {
		t.Fatalf("unexpected default session ttl seconds: %d", service.CookieMaxAgeSeconds())
	}
}

func TestAuthServiceLoginValidateLogout(t *testing.T) {
	service := newAuthServiceForTest(t, AuthConfig{
		SessionTTL:          time.Minute,
		CookieName:          "session_cookie",
		RequireSecureCookie: false,
	})

	token, expiresAt, err := service.Login(" 127.0.0.1 ", " admin ", "Secret#12345")
	if err != nil {
		t.Fatalf("login should succeed: %v", err)
	}
	if token == "" {
		t.Fatal("expected session token")
	}
	if expiresAt.Before(time.Now()) {
		t.Fatal("expected future expiry")
	}

	session, ok := service.ValidateSession(token)
	if !ok {
		t.Fatal("expected session to be valid")
	}
	if session.Username != "admin" {
		t.Fatalf("unexpected username: %s", session.Username)
	}

	service.Logout(token)
	if _, ok := service.ValidateSession(token); ok {
		t.Fatal("expected session to be removed after logout")
	}
}

func TestAuthServiceLockoutAndRecovery(t *testing.T) {
	service := newAuthServiceForTest(t, AuthConfig{
		SessionTTL:        time.Minute,
		MaxFailedAttempts: 1,
		LockoutDuration:   time.Second,
	})

	if _, _, err := service.Login("192.168.1.8", "admin", "bad-password-1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials on first failure, got: %v", err)
	}
	if _, _, err := service.Login("192.168.1.8", "admin", "bad-password-2"); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("expected lockout error, got: %v", err)
	}

	time.Sleep(1100 * time.Millisecond)
	if _, _, err := service.Login("192.168.1.8", "admin", "Secret#12345"); err != nil {
		t.Fatalf("expected successful login after lockout expiry, got: %v", err)
	}
}

func TestAuthServiceSessionExpires(t *testing.T) {
	service := newAuthServiceForTest(t, AuthConfig{
		SessionTTL: 30 * time.Millisecond,
	})

	token, _, err := service.Login("127.0.0.1", "admin", "Secret#12345")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	if _, ok := service.ValidateSession(token); ok {
		t.Fatal("expected session to expire")
	}
}

func TestClientIPAndCookieSecurity(t *testing.T) {
	service := newAuthServiceForTest(t, AuthConfig{
		RequireSecureCookie: true,
	})

	if ip := ClientIPFromRequest(nil); ip != "unknown" {
		t.Fatalf("unexpected client ip for nil request: %s", ip)
	}

	request := &http.Request{
		Header:     make(http.Header),
		RemoteAddr: "10.0.0.1:12345",
	}
	request.Header.Set("X-Forwarded-For", " 1.2.3.4, 5.6.7.8 ")
	if ip := ClientIPFromRequest(request); ip != "1.2.3.4" {
		t.Fatalf("unexpected forwarded ip: %s", ip)
	}

	request.Header.Del("X-Forwarded-For")
	request.Header.Set("X-Real-IP", " 9.9.9.9 ")
	if ip := ClientIPFromRequest(request); ip != "9.9.9.9" {
		t.Fatalf("unexpected real ip: %s", ip)
	}

	request.Header.Del("X-Real-IP")
	if ip := ClientIPFromRequest(request); ip != "10.0.0.1" {
		t.Fatalf("unexpected remote host ip: %s", ip)
	}

	request.RemoteAddr = "invalid-remote-addr"
	if ip := ClientIPFromRequest(request); ip != "invalid-remote-addr" {
		t.Fatalf("unexpected fallback remote addr value: %s", ip)
	}

	request.RemoteAddr = ":1234"
	if ip := ClientIPFromRequest(request); ip != "unknown" {
		t.Fatalf("expected unknown for empty remote host, got: %s", ip)
	}

	if !service.ShouldUseSecureCookie(nil) {
		t.Fatal("expected secure cookie when RequireSecureCookie=true")
	}

	httpsRequest := &http.Request{TLS: &tls.ConnectionState{}}
	if !service.ShouldUseSecureCookie(httpsRequest) {
		t.Fatal("expected secure cookie for TLS request")
	}

	insecureService := newAuthServiceForTest(t, AuthConfig{RequireSecureCookie: false})
	if insecureService.ShouldUseSecureCookie(&http.Request{}) {
		t.Fatal("expected non-secure cookie when not configured and no TLS")
	}
}

func TestRandomPasswordAndSecureStringEqual(t *testing.T) {
	password, err := GenerateRandomPassword(8)
	if err != nil {
		t.Fatalf("generate random password: %v", err)
	}
	if len(password) < 16 {
		t.Fatalf("expected minimum generated length 16, got %d", len(password))
	}

	if !secureStringEqual("admin", "admin") {
		t.Fatal("expected equal strings")
	}
	if secureStringEqual("admin", "admin1") {
		t.Fatal("expected unequal strings when lengths differ")
	}
	if secureStringEqual("admin", "root!") {
		t.Fatal("expected unequal strings")
	}
}

func TestAuthHelpersAndEdgeCases(t *testing.T) {
	if _, err := GeneratePasswordHash("   "); err == nil {
		t.Fatal("expected error for blank password")
	}

	service := newAuthServiceForTest(t, AuthConfig{
		SessionTTL: time.Minute,
	})
	now := time.Now()
	service.sessions["expired"] = sessionRecord{Username: "admin", ExpiresAt: now.Add(-time.Minute)}
	service.sessions["active"] = sessionRecord{Username: "admin", ExpiresAt: now.Add(time.Minute)}
	service.cleanupExpiredSessionsLocked(now)

	if _, ok := service.sessions["expired"]; ok {
		t.Fatal("expected expired session to be removed")
	}
	if _, ok := service.sessions["active"]; !ok {
		t.Fatal("expected active session to remain")
	}

	if key := normalizeClientKey("  "); key != "unknown" {
		t.Fatalf("expected unknown normalized key, got %q", key)
	}
}
