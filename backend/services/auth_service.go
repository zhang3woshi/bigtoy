package services

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrTooManyAttempts    = errors.New("too many failed login attempts, please try again later")
)

type AuthConfig struct {
	AdminUser           string
	AdminPasswordHash   string
	SessionTTL          time.Duration
	CookieName          string
	RequireSecureCookie bool
	MaxFailedAttempts   int
	LockoutDuration     time.Duration
}

type SessionInfo struct {
	Username  string
	ExpiresAt time.Time
}

type sessionRecord struct {
	Username  string
	ExpiresAt time.Time
}

type loginAttempt struct {
	FailedCount int
	LastFailed  time.Time
	LockedUntil time.Time
}

type AuthService struct {
	mu       sync.Mutex
	config   AuthConfig
	sessions map[string]sessionRecord
	attempts map[string]loginAttempt
}

func NewAuthService(cfg AuthConfig) (*AuthService, error) {
	cfg.AdminUser = strings.TrimSpace(cfg.AdminUser)
	cfg.AdminPasswordHash = strings.TrimSpace(cfg.AdminPasswordHash)
	cfg.CookieName = strings.TrimSpace(cfg.CookieName)

	if cfg.AdminUser == "" {
		return nil, errors.New("admin username is required")
	}
	if cfg.AdminPasswordHash == "" {
		return nil, errors.New("admin password hash is required")
	}
	if _, err := bcrypt.Cost([]byte(cfg.AdminPasswordHash)); err != nil {
		return nil, errors.New("admin password hash must be a valid bcrypt hash")
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = 2 * time.Hour
	}
	if cfg.CookieName == "" {
		cfg.CookieName = "bigtoy_admin_session"
	}
	if cfg.MaxFailedAttempts <= 0 {
		cfg.MaxFailedAttempts = 5
	}
	if cfg.LockoutDuration <= 0 {
		cfg.LockoutDuration = 15 * time.Minute
	}

	return &AuthService{
		config:   cfg,
		sessions: make(map[string]sessionRecord),
		attempts: make(map[string]loginAttempt),
	}, nil
}

func (s *AuthService) Login(clientIP, username, password string) (string, time.Time, error) {
	now := time.Now()
	key := normalizeClientKey(clientIP)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isLockedOutLocked(key, now) {
		return "", time.Time{}, ErrTooManyAttempts
	}

	inputUser := strings.TrimSpace(username)
	if !secureStringEqual(inputUser, s.config.AdminUser) || bcrypt.CompareHashAndPassword([]byte(s.config.AdminPasswordHash), []byte(password)) != nil {
		s.recordFailedAttemptLocked(key, now)
		return "", time.Time{}, ErrInvalidCredentials
	}

	delete(s.attempts, key)
	token, err := generateSessionToken()
	if err != nil {
		return "", time.Time{}, err
	}

	expiresAt := now.Add(s.config.SessionTTL)
	s.cleanupExpiredSessionsLocked(now)
	s.sessions[token] = sessionRecord{
		Username:  s.config.AdminUser,
		ExpiresAt: expiresAt,
	}

	return token, expiresAt, nil
}

func (s *AuthService) ValidateSession(token string) (SessionInfo, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return SessionInfo{}, false
	}

	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.sessions[token]
	if !ok {
		return SessionInfo{}, false
	}
	if now.After(record.ExpiresAt) {
		delete(s.sessions, token)
		return SessionInfo{}, false
	}

	return SessionInfo{
		Username:  record.Username,
		ExpiresAt: record.ExpiresAt,
	}, true
}

func (s *AuthService) Logout(token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

func (s *AuthService) CookieName() string {
	return s.config.CookieName
}

func (s *AuthService) CookieMaxAgeSeconds() int {
	return int(s.config.SessionTTL / time.Second)
}

func (s *AuthService) ShouldUseSecureCookie(r *http.Request) bool {
	if r != nil && r.TLS != nil {
		return true
	}
	return s.config.RequireSecureCookie
}

func GeneratePasswordHash(password string) (string, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return "", errors.New("password is required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func GenerateRandomPassword(length int) (string, error) {
	if length < 16 {
		length = 16
	}

	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	encoded := base64.RawURLEncoding.EncodeToString(buf)
	if len(encoded) < length {
		return encoded, nil
	}
	return encoded[:length], nil
}

func ClientIPFromRequest(r *http.Request) string {
	if r == nil {
		return "unknown"
	}

	for _, headerName := range []string{"CF-Connecting-IP", "X-Forwarded-For", "X-Real-IP"} {
		value := strings.TrimSpace(r.Header.Get(headerName))
		if value == "" {
			continue
		}

		parts := strings.Split(value, ",")
		candidate := strings.TrimSpace(parts[0])
		if candidate != "" {
			return candidate
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	if host == "" {
		return "unknown"
	}
	return host
}

func (s *AuthService) isLockedOutLocked(key string, now time.Time) bool {
	attempt, ok := s.attempts[key]
	if !ok {
		return false
	}
	if !attempt.LockedUntil.IsZero() && now.Before(attempt.LockedUntil) {
		return true
	}
	if now.Sub(attempt.LastFailed) > s.config.LockoutDuration {
		delete(s.attempts, key)
	}
	return false
}

func (s *AuthService) recordFailedAttemptLocked(key string, now time.Time) {
	attempt := s.attempts[key]
	if now.Sub(attempt.LastFailed) > s.config.LockoutDuration {
		attempt = loginAttempt{}
	}

	attempt.FailedCount++
	attempt.LastFailed = now
	if attempt.FailedCount >= s.config.MaxFailedAttempts {
		attempt.LockedUntil = now.Add(s.config.LockoutDuration)
		attempt.FailedCount = 0
	}

	s.attempts[key] = attempt
}

func (s *AuthService) cleanupExpiredSessionsLocked(now time.Time) {
	for token, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, token)
		}
	}
}

func normalizeClientKey(clientIP string) string {
	key := strings.TrimSpace(clientIP)
	if key == "" {
		return "unknown"
	}
	return key
}

func secureStringEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func generateSessionToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
