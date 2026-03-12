package routers

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/beego/beego/v2/server/web"

	"bigtoy/backend/services"
)

func resetRouterGlobals(t *testing.T) {
	t.Helper()

	originalConfig := *web.BConfig
	originalBackup := backupScheduler
	t.Cleanup(func() {
		*web.BConfig = originalConfig
		backupScheduler = originalBackup
	})
}

func TestAppendMissingOrigins(t *testing.T) {
	origins := []string{"https://a.example.com"}
	updated := appendMissingOrigins(origins, "https://A.example.com", "https://b.example.com", " ")

	if len(updated) != 2 {
		t.Fatalf("expected 2 unique origins, got %d (%#v)", len(updated), updated)
	}
	if !slices.Contains(updated, "https://a.example.com") || !slices.Contains(updated, "https://b.example.com") {
		t.Fatalf("unexpected origins: %#v", updated)
	}
}

func TestReadEnvOrConfigHelpers(t *testing.T) {
	t.Setenv("BIGTOY_TEST_STRING", "  hello  ")
	if value := readEnvOrConfig("BIGTOY_TEST_STRING", "", "fallback"); value != "hello" {
		t.Fatalf("unexpected env string value: %q", value)
	}

	t.Setenv("BIGTOY_TEST_INT", "90")
	if value := readEnvOrConfigInt("BIGTOY_TEST_INT", "", 7); value != 90 {
		t.Fatalf("unexpected env int value: %d", value)
	}
	t.Setenv("BIGTOY_TEST_INT", "-1")
	if value := readEnvOrConfigInt("BIGTOY_TEST_INT", "", 7); value != 7 {
		t.Fatalf("expected fallback for invalid int, got %d", value)
	}

	t.Setenv("BIGTOY_TEST_BOOL", "true")
	if value := readEnvOrConfigBool("BIGTOY_TEST_BOOL", "", false); !value {
		t.Fatal("expected true bool from env")
	}
	t.Setenv("BIGTOY_TEST_BOOL", "not-a-bool")
	if value := readEnvOrConfigBool("BIGTOY_TEST_BOOL", "", true); !value {
		t.Fatal("expected fallback bool for invalid value")
	}
}

func TestResolveAllowedOrigins(t *testing.T) {
	resetRouterGlobals(t)

	web.BConfig.RunMode = "dev"
	t.Setenv("BIGTOY_ALLOWED_ORIGINS", "")
	defaultOrigins := resolveAllowedOrigins()
	if !slices.Contains(defaultOrigins, "http://localhost:5173") || !slices.Contains(defaultOrigins, "http://127.0.0.1:5173") {
		t.Fatalf("missing dev default origins: %#v", defaultOrigins)
	}

	web.BConfig.RunMode = "prod"
	t.Setenv("BIGTOY_ALLOWED_ORIGINS", "https://a.example.com, https://b.example.com")
	prodOrigins := resolveAllowedOrigins()
	if !slices.Equal(prodOrigins, []string{"https://a.example.com", "https://b.example.com"}) {
		t.Fatalf("unexpected prod origins: %#v", prodOrigins)
	}
}

func TestBuildCORSOptions(t *testing.T) {
	resetRouterGlobals(t)

	web.BConfig.RunMode = "prod"
	t.Setenv("BIGTOY_ALLOWED_ORIGINS", "https://a.example.com")
	options := buildCORSOptions()
	if options.AllowAllOrigins {
		t.Fatal("expected allow-all to be false when origins are configured")
	}
	if !options.AllowCredentials {
		t.Fatal("expected allow credentials to be true when specific origins are configured")
	}
	if !slices.Contains(options.AllowOrigins, "https://a.example.com") {
		t.Fatalf("unexpected allow origins: %#v", options.AllowOrigins)
	}

	t.Setenv("BIGTOY_ALLOWED_ORIGINS", ", ,")
	options = buildCORSOptions()
	if !options.AllowAllOrigins {
		t.Fatal("expected allow-all when no valid origin is configured")
	}
}

func TestResolveViewsPath(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "static")
	frontendDist := filepath.Join(root, "..", "frontend", "dist")

	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		t.Fatalf("create static dir: %v", err)
	}
	if err := os.MkdirAll(frontendDist, 0o755); err != nil {
		t.Fatalf("create frontend dist dir: %v", err)
	}

	viewsPath := resolveViewsPath(root)
	if filepath.Clean(viewsPath) != filepath.Clean(staticDir) {
		t.Fatalf("expected static dir to be preferred, got %s", viewsPath)
	}
}

func TestResolveBackendRoot(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}

	if got := resolveBackendRoot(); got != "." {
		t.Fatalf("expected default backend root '.', got %q", got)
	}

	if err := os.MkdirAll(filepath.Join("backend", "conf"), 0o755); err != nil {
		t.Fatalf("create backend conf dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join("backend", "conf", "app.conf"), []byte("runmode = dev"), 0o644); err != nil {
		t.Fatalf("write backend app.conf: %v", err)
	}

	if got := resolveBackendRoot(); got != "backend" {
		t.Fatalf("expected backend root 'backend', got %q", got)
	}

	if err := os.MkdirAll("conf", 0o755); err != nil {
		t.Fatalf("create conf dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join("conf", "app.conf"), []byte("runmode = dev"), 0o644); err != nil {
		t.Fatalf("write app.conf: %v", err)
	}

	if got := resolveBackendRoot(); got != "." {
		t.Fatalf("expected conf/app.conf to take precedence, got %q", got)
	}
}

func TestApplyTransportSecurityConfig(t *testing.T) {
	resetRouterGlobals(t)

	web.BConfig.RunMode = "dev"
	web.BConfig.Listen.EnableHTTP = false
	web.BConfig.Listen.EnableHTTPS = true
	t.Setenv("BIGTOY_FORCE_HTTPS", "true")
	t.Setenv("BIGTOY_ENABLE_HTTP", "false")
	t.Setenv("BIGTOY_ENABLE_HTTPS", "true")

	if err := applyTransportSecurityConfig(); err != nil {
		t.Fatalf("expected dev config to succeed, got: %v", err)
	}
	if !web.BConfig.Listen.EnableHTTP {
		t.Fatal("expected HTTP to be forced on in dev mode")
	}

	web.BConfig.RunMode = "prod"
	web.BConfig.Listen.EnableHTTP = true
	web.BConfig.Listen.EnableHTTPS = false
	t.Setenv("BIGTOY_ENABLE_HTTP", "false")
	t.Setenv("BIGTOY_ENABLE_HTTPS", "false")
	t.Setenv("BIGTOY_FORCE_HTTPS", "false")

	if err := applyTransportSecurityConfig(); err == nil {
		t.Fatal("expected error when both HTTP and HTTPS are disabled")
	}
}

func TestResolveAuthConfig(t *testing.T) {
	resetRouterGlobals(t)

	hash, err := services.GeneratePasswordHash("Secret#12345")
	if err != nil {
		t.Fatalf("generate hash: %v", err)
	}

	web.BConfig.RunMode = "prod"
	t.Setenv("BIGTOY_ADMIN_USER", "super-admin")
	t.Setenv("BIGTOY_ADMIN_PASSWORD_HASH", hash)
	t.Setenv("BIGTOY_AUTH_SESSION_TTL_MINUTES", "30")
	t.Setenv("BIGTOY_AUTH_COOKIE_NAME", "admin_cookie")
	t.Setenv("BIGTOY_AUTH_MAX_FAILED_ATTEMPTS", "9")
	t.Setenv("BIGTOY_AUTH_LOCKOUT_MINUTES", "7")

	cfg, err := resolveAuthConfig()
	if err != nil {
		t.Fatalf("resolve auth config: %v", err)
	}
	if cfg.AdminUser != "super-admin" || cfg.CookieName != "admin_cookie" {
		t.Fatalf("unexpected auth config: %#v", cfg)
	}
	if cfg.SessionTTL != 30*time.Minute || cfg.MaxFailedAttempts != 9 || cfg.LockoutDuration != 7*time.Minute {
		t.Fatalf("unexpected auth durations/attempts: %#v", cfg)
	}

	web.BConfig.RunMode = "prod"
	t.Setenv("BIGTOY_ADMIN_PASSWORD_HASH", "")
	if _, err := resolveAuthConfig(); err == nil || !strings.Contains(err.Error(), "missing BIGTOY_ADMIN_PASSWORD_HASH") {
		t.Fatalf("expected missing hash error in prod mode, got: %v", err)
	}
}

func TestStartBackupSchedulerDisabledOrAlreadyStarted(t *testing.T) {
	resetRouterGlobals(t)

	t.Setenv("BIGTOY_BACKUP_ENABLED", "false")
	backupScheduler = nil
	if err := startBackupScheduler("db", "images", "backup"); err != nil {
		t.Fatalf("disabled backup scheduler should not fail: %v", err)
	}
	if backupScheduler != nil {
		t.Fatal("backup scheduler should remain nil when disabled")
	}

	backupScheduler = &services.BackupService{}
	if err := startBackupScheduler("db", "images", "backup"); err != nil {
		t.Fatalf("already initialized scheduler should not fail: %v", err)
	}
}
