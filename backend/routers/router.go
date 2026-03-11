package routers

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/beego/beego/v2/server/web"
	"github.com/beego/beego/v2/server/web/filter/cors"

	"bigtoy/backend/controllers"
	"bigtoy/backend/services"
)

var backupScheduler *services.BackupService

func Register() error {
	backendRoot := resolveBackendRoot()
	dataDir := filepath.Join(backendRoot, "data")
	dbPath := filepath.Join(dataDir, "models.db")
	legacyDataPath := filepath.Join(dataDir, "models.json")
	uploadsPath := filepath.Join(dataDir, "images")
	viewsPath := resolveViewsPath(backendRoot)

	if err := applyTransportSecurityConfig(); err != nil {
		return err
	}

	store, err := services.NewModelStore(dbPath, uploadsPath, legacyDataPath)
	if err != nil {
		return fmt.Errorf("failed to initialize model store: %w", err)
	}
	controllers.SetModelStore(store)

	if err := startBackupScheduler(dbPath, uploadsPath, filepath.Join(dataDir, "backup")); err != nil {
		return fmt.Errorf("failed to initialize backup scheduler: %w", err)
	}

	authConfig, err := resolveAuthConfig()
	if err != nil {
		return fmt.Errorf("failed to initialize auth config: %w", err)
	}
	authService, err := services.NewAuthService(authConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize auth service: %w", err)
	}
	controllers.SetAuthService(authService)

	web.InsertFilter("/*", web.BeforeRouter, cors.Allow(buildCORSOptions()))

	web.Router("/api/models", &controllers.ModelController{})
	web.Router("/api/models/:id", &controllers.ModelController{}, "put:Put;delete:Delete")
	web.Router("/api/auth/login", &controllers.AuthController{}, "post:Login")
	web.Router("/api/auth/logout", &controllers.AuthController{}, "post:Logout")
	web.Router("/api/auth/me", &controllers.AuthController{}, "get:Me")

	web.BConfig.WebConfig.ViewsPath = viewsPath
	web.SetStaticPath("/assets", filepath.Join(viewsPath, "assets"))
	web.SetStaticPath("/uploads", uploadsPath)

	web.Router("/", &controllers.PageController{}, "get:Public")
	web.Router("/index.html", &controllers.PageController{}, "get:Public")
	web.Router("/model", &controllers.PageController{}, "get:Detail")
	web.Router("/model.html", &controllers.PageController{}, "get:Detail")
	web.Router("/login", &controllers.PageController{}, "get:Login")
	web.Router("/login.html", &controllers.PageController{}, "get:Login")
	web.Router("/admin", &controllers.PageController{}, "get:Admin")
	web.Router("/admin.html", &controllers.PageController{}, "get:Admin")

	fmt.Printf("BigToy backend started. DB: %s, views path: %s, uploads path: %s\n", dbPath, viewsPath, uploadsPath)
	return nil
}

func applyTransportSecurityConfig() error {
	runMode := strings.ToLower(strings.TrimSpace(web.BConfig.RunMode))
	isDevMode := runMode == "dev"
	forceHTTPSDefault := !isDevMode
	forceHTTPS := readEnvOrConfigBool("BIGTOY_FORCE_HTTPS", "force_https", forceHTTPSDefault)

	enableHTTPDefault := web.BConfig.Listen.EnableHTTP
	enableHTTPSDefault := web.BConfig.Listen.EnableHTTPS
	if isDevMode {
		enableHTTPDefault = true
		enableHTTPSDefault = false
	}

	enableHTTP := readEnvOrConfigBool("BIGTOY_ENABLE_HTTP", "enablehttp", enableHTTPDefault)
	enableHTTPS := readEnvOrConfigBool("BIGTOY_ENABLE_HTTPS", "enablehttps", enableHTTPSDefault)
	if isDevMode {
		if forceHTTPS {
			log.Printf("[dev] BIGTOY_FORCE_HTTPS=true detected; HTTP remains enabled for local debugging")
		}
		enableHTTP = true
	} else if forceHTTPS {
		enableHTTPS = true
		enableHTTP = false
	}

	web.BConfig.Listen.EnableHTTP = enableHTTP
	web.BConfig.Listen.EnableHTTPS = enableHTTPS

	httpsAddr := strings.TrimSpace(readEnvOrConfig("BIGTOY_HTTPS_ADDR", "httpsaddr", web.BConfig.Listen.HTTPSAddr))
	if httpsAddr != "" {
		web.BConfig.Listen.HTTPSAddr = httpsAddr
	}

	httpsPort := readEnvOrConfigInt("BIGTOY_HTTPS_PORT", "httpsport", web.BConfig.Listen.HTTPSPort)
	if httpsPort > 0 {
		web.BConfig.Listen.HTTPSPort = httpsPort
	}

	httpsCertFile := strings.TrimSpace(readEnvOrConfig("BIGTOY_HTTPS_CERT_FILE", "https_cert_file", web.BConfig.Listen.HTTPSCertFile))
	httpsKeyFile := strings.TrimSpace(readEnvOrConfig("BIGTOY_HTTPS_KEY_FILE", "https_key_file", web.BConfig.Listen.HTTPSKeyFile))
	if httpsCertFile != "" {
		web.BConfig.Listen.HTTPSCertFile = httpsCertFile
	}
	if httpsKeyFile != "" {
		web.BConfig.Listen.HTTPSKeyFile = httpsKeyFile
	}

	if !web.BConfig.Listen.EnableHTTP && !web.BConfig.Listen.EnableHTTPS {
		return fmt.Errorf("invalid listen config: both HTTP and HTTPS are disabled")
	}

	if web.BConfig.Listen.EnableHTTPS {
		certPath := strings.TrimSpace(web.BConfig.Listen.HTTPSCertFile)
		keyPath := strings.TrimSpace(web.BConfig.Listen.HTTPSKeyFile)
		if certPath == "" || keyPath == "" {
			if isDevMode {
				log.Printf("[dev] HTTPS is enabled but certificate or key file is missing; disabling HTTPS and keeping HTTP enabled")
				web.BConfig.Listen.EnableHTTPS = false
			} else {
				return fmt.Errorf("HTTPS is enabled but certificate or key file is missing; set BIGTOY_HTTPS_CERT_FILE/BIGTOY_HTTPS_KEY_FILE or https_cert_file/https_key_file")
			}
		}

		if web.BConfig.Listen.EnableHTTPS && !exists(certPath) {
			if isDevMode {
				log.Printf("[dev] HTTPS certificate file not found (%s); disabling HTTPS and keeping HTTP enabled", certPath)
				web.BConfig.Listen.EnableHTTPS = false
			} else {
				return fmt.Errorf("HTTPS certificate file not found: %s", certPath)
			}
		}
		if web.BConfig.Listen.EnableHTTPS && !exists(keyPath) {
			if isDevMode {
				log.Printf("[dev] HTTPS key file not found (%s); disabling HTTPS and keeping HTTP enabled", keyPath)
				web.BConfig.Listen.EnableHTTPS = false
			} else {
				return fmt.Errorf("HTTPS key file not found: %s", keyPath)
			}
		}
	}

	if isDevMode {
		// Keep localhost debugging reliable regardless of inherited production-style env vars.
		web.BConfig.Listen.EnableHTTP = true
	}

	if !web.BConfig.Listen.EnableHTTP && !web.BConfig.Listen.EnableHTTPS {
		return fmt.Errorf("invalid listen config: both HTTP and HTTPS are disabled")
	}

	if web.BConfig.Listen.EnableHTTPS && !web.BConfig.Listen.EnableHTTP {
		log.Printf("[security] HTTPS only mode enabled, listening on %s:%d", web.BConfig.Listen.HTTPSAddr, web.BConfig.Listen.HTTPSPort)
	}

	return nil
}

func resolveBackendRoot() string {
	if exists(filepath.Join("conf", "app.conf")) {
		return "."
	}
	if exists(filepath.Join("backend", "conf", "app.conf")) {
		return "backend"
	}
	return "."
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func resolveViewsPath(backendRoot string) string {
	candidates := []string{
		filepath.Join(backendRoot, "static"),
		filepath.Join(backendRoot, "..", "frontend", "dist"),
	}

	for _, candidate := range candidates {
		if exists(candidate) {
			return filepath.Clean(candidate)
		}
	}

	return filepath.Clean(candidates[0])
}

func buildCORSOptions() *cors.Options {
	allowedOrigins := resolveAllowedOrigins()
	options := &cors.Options{
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
		},
		ExposeHeaders: []string{"Content-Length"},
	}

	if len(allowedOrigins) == 0 {
		options.AllowAllOrigins = true
		return options
	}

	options.AllowOrigins = allowedOrigins
	options.AllowCredentials = true
	return options
}

func resolveAllowedOrigins() []string {
	runMode := strings.ToLower(strings.TrimSpace(web.BConfig.RunMode))
	raw := strings.TrimSpace(readEnvOrConfig("BIGTOY_ALLOWED_ORIGINS", "allowed_origins", ""))
	devDefaults := []string{"http://localhost:5173", "http://127.0.0.1:5173"}

	if raw == "" {
		return devDefaults
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts)+len(devDefaults))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}
		origins = append(origins, origin)
	}

	if runMode == "dev" {
		origins = appendMissingOrigins(origins, devDefaults...)
	}

	return origins
}

func appendMissingOrigins(origins []string, extras ...string) []string {
	for _, extra := range extras {
		candidate := strings.TrimSpace(extra)
		if candidate == "" {
			continue
		}

		exists := false
		for _, origin := range origins {
			if strings.EqualFold(strings.TrimSpace(origin), candidate) {
				exists = true
				break
			}
		}
		if !exists {
			origins = append(origins, candidate)
		}
	}
	return origins
}

func resolveAuthConfig() (services.AuthConfig, error) {
	runMode := strings.ToLower(strings.TrimSpace(web.BConfig.RunMode))
	adminUser := strings.TrimSpace(readEnvOrConfig("BIGTOY_ADMIN_USER", "admin_user", "admin"))
	passwordHash := strings.TrimSpace(readEnvOrConfig("BIGTOY_ADMIN_PASSWORD_HASH", "admin_password_hash", ""))

	if passwordHash == "" {
		if runMode != "dev" {
			return services.AuthConfig{}, fmt.Errorf("missing BIGTOY_ADMIN_PASSWORD_HASH (or app.conf admin_password_hash) in non-dev mode")
		}

		tempPassword, err := services.GenerateRandomPassword(20)
		if err != nil {
			return services.AuthConfig{}, fmt.Errorf("generate temporary admin password: %w", err)
		}
		passwordHash, err = services.GeneratePasswordHash(tempPassword)
		if err != nil {
			return services.AuthConfig{}, fmt.Errorf("hash temporary admin password: %w", err)
		}

		log.Printf("[security] BIGTOY_ADMIN_PASSWORD_HASH is not configured; using one-time dev credentials: user=%s password=%s", adminUser, tempPassword)
	}

	sessionTTLMinutes := readEnvOrConfigInt("BIGTOY_AUTH_SESSION_TTL_MINUTES", "auth_session_ttl_minutes", 120)
	cookieName := strings.TrimSpace(readEnvOrConfig("BIGTOY_AUTH_COOKIE_NAME", "auth_cookie_name", "bigtoy_admin_session"))
	secureDefault := runMode != "dev"
	requireSecureCookie := readEnvOrConfigBool("BIGTOY_AUTH_SECURE_COOKIE", "auth_secure_cookie", secureDefault)
	if runMode == "dev" && web.BConfig.Listen.EnableHTTP && requireSecureCookie {
		log.Printf("[dev] BIGTOY_AUTH_SECURE_COOKIE=true detected while HTTP is enabled; forcing insecure cookie for local debugging")
		requireSecureCookie = false
	}
	maxAttempts := readEnvOrConfigInt("BIGTOY_AUTH_MAX_FAILED_ATTEMPTS", "auth_max_failed_attempts", 5)
	lockoutMinutes := readEnvOrConfigInt("BIGTOY_AUTH_LOCKOUT_MINUTES", "auth_lockout_minutes", 15)

	return services.AuthConfig{
		AdminUser:           adminUser,
		AdminPasswordHash:   passwordHash,
		SessionTTL:          time.Duration(sessionTTLMinutes) * time.Minute,
		CookieName:          cookieName,
		RequireSecureCookie: requireSecureCookie,
		MaxFailedAttempts:   maxAttempts,
		LockoutDuration:     time.Duration(lockoutMinutes) * time.Minute,
	}, nil
}

func readEnvOrConfig(envKey, configKey, fallback string) string {
	if value, ok := os.LookupEnv(envKey); ok {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}

	if configKey != "" {
		value := strings.TrimSpace(web.AppConfig.DefaultString(configKey, ""))
		if value != "" {
			return value
		}
	}

	return fallback
}

func readEnvOrConfigInt(envKey, configKey string, fallback int) int {
	raw := strings.TrimSpace(readEnvOrConfig(envKey, configKey, ""))
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func readEnvOrConfigBool(envKey, configKey string, fallback bool) bool {
	raw := strings.TrimSpace(readEnvOrConfig(envKey, configKey, ""))
	if raw == "" {
		return fallback
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func startBackupScheduler(dbPath, imagesPath, backupDir string) error {
	if backupScheduler != nil {
		return nil
	}

	enabled := readEnvOrConfigBool("BIGTOY_BACKUP_ENABLED", "backup_enabled", true)
	if !enabled {
		log.Printf("[backup] scheduler is disabled by configuration")
		return nil
	}

	intervalMinutes := readEnvOrConfigInt("BIGTOY_BACKUP_INTERVAL_MINUTES", "backup_interval_minutes", 1440)
	maxBackups := readEnvOrConfigInt("BIGTOY_BACKUP_MAX_FILES", "backup_max_files", 3)
	config := services.BackupServiceConfig{
		DBPath:     dbPath,
		ImagesRoot: imagesPath,
		BackupDir:  backupDir,
		Interval:   time.Duration(intervalMinutes) * time.Minute,
		MaxBackups: maxBackups,
	}

	scheduler, err := services.NewBackupService(config)
	if err != nil {
		return err
	}
	scheduler.Start()
	backupScheduler = scheduler

	log.Printf("[backup] scheduler started: interval=%s max_backups=%d backup_dir=%s", config.Interval, config.MaxBackups, backupDir)
	return nil
}
