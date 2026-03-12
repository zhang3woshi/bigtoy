package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kardianos/service"
)

type stubService struct {
	status          service.Status
	installCalled   bool
	uninstallCalled bool
}

func (s *stubService) String() string                                      { return "stub" }
func (s *stubService) Platform() string                                    { return "stub" }
func (s *stubService) Config() *service.Config                             { return &service.Config{} }
func (s *stubService) Logger(_ chan<- error) (service.Logger, error)       { return nil, nil }
func (s *stubService) Run() error                                          { return nil }
func (s *stubService) Start() error                                        { return nil }
func (s *stubService) Stop() error                                         { return nil }
func (s *stubService) Restart() error                                      { return nil }
func (s *stubService) Install() error                                      { s.installCalled = true; return nil }
func (s *stubService) Uninstall() error                                    { s.uninstallCalled = true; return nil }
func (s *stubService) Status() (service.Status, error)                     { return s.status, nil }
func (s *stubService) SystemLogger(_ chan<- error) (service.Logger, error) { return nil, nil }

func TestFormatServiceStatus(t *testing.T) {
	if value := formatServiceStatus(service.StatusRunning); value != "running" {
		t.Fatalf("unexpected running status text: %s", value)
	}
	if value := formatServiceStatus(service.StatusStopped); value != "stopped" {
		t.Fatalf("unexpected stopped status text: %s", value)
	}
	if value := formatServiceStatus(service.StatusUnknown); value != "unknown" {
		t.Fatalf("unexpected unknown status text: %s", value)
	}
}

func TestHandleServiceCommandRouting(t *testing.T) {
	svc := &stubService{status: service.StatusRunning}

	handled, err := handleServiceCommand(svc, nil)
	if handled || err != nil {
		t.Fatalf("expected no-op for empty args, handled=%v err=%v", handled, err)
	}

	handled, err = handleServiceCommand(svc, []string{"run"})
	if handled || err != nil {
		t.Fatalf("expected non-service prefix to skip, handled=%v err=%v", handled, err)
	}

	handled, err = handleServiceCommand(svc, []string{"service", "install"})
	if !handled || err != nil {
		t.Fatalf("expected install command success, handled=%v err=%v", handled, err)
	}
	if !svc.installCalled {
		t.Fatal("expected install to be called")
	}

	handled, err = handleServiceCommand(svc, []string{"service", "uninstall"})
	if !handled || err != nil {
		t.Fatalf("expected uninstall command success, handled=%v err=%v", handled, err)
	}
	if !svc.uninstallCalled {
		t.Fatal("expected uninstall to be called")
	}

	handled, err = handleServiceCommand(svc, []string{"service", "status"})
	if !handled || err != nil {
		t.Fatalf("expected status command success, handled=%v err=%v", handled, err)
	}

	handled, err = handleServiceCommand(svc, []string{"service", "unknown"})
	if !handled || err == nil {
		t.Fatalf("expected unknown action error, handled=%v err=%v", handled, err)
	}
}

func TestParseSecretsLine(t *testing.T) {
	key, value, skip, err := parseSecretsLine("# comment", 0, "secrets.env")
	if err != nil || !skip {
		t.Fatalf("expected comment line skip, skip=%v err=%v", skip, err)
	}
	if key != "" || value != "" {
		t.Fatalf("expected empty key/value for comment, got %q/%q", key, value)
	}

	key, value, skip, err = parseSecretsLine("export TOKEN='abc123'", 1, "secrets.env")
	if err != nil || skip {
		t.Fatalf("expected parse success, skip=%v err=%v", skip, err)
	}
	if key != "TOKEN" || value != "abc123" {
		t.Fatalf("unexpected parsed key/value: %s=%s", key, value)
	}

	if _, _, _, err := parseSecretsLine("invalid-line", 2, "secrets.env"); err == nil {
		t.Fatal("expected invalid secrets line error")
	}
}

func TestStripOptionalQuotes(t *testing.T) {
	if value := stripOptionalQuotes(`"hello"`); value != "hello" {
		t.Fatalf("unexpected double-quoted value: %s", value)
	}
	if value := stripOptionalQuotes(`'world'`); value != "world" {
		t.Fatalf("unexpected single-quoted value: %s", value)
	}
	if value := stripOptionalQuotes("plain"); value != "plain" {
		t.Fatalf("unexpected plain value: %s", value)
	}
}

func TestReadSecretsLinesAndResolvePath(t *testing.T) {
	root := t.TempDir()
	secretsPath := filepath.Join(root, "secrets.env")

	lines, loaded, err := readSecretsLines(secretsPath)
	if err != nil {
		t.Fatalf("read missing secrets file should not fail: %v", err)
	}
	if loaded || len(lines) != 0 {
		t.Fatalf("expected missing file to report not loaded, got loaded=%v lines=%d", loaded, len(lines))
	}

	content := strings.Join([]string{
		"KEY_A=one",
		"KEY_B=two",
	}, "\n")
	if err := os.WriteFile(secretsPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write secrets file: %v", err)
	}

	lines, loaded, err = readSecretsLines(secretsPath)
	if err != nil {
		t.Fatalf("read existing secrets file: %v", err)
	}
	if !loaded || len(lines) != 2 {
		t.Fatalf("expected loaded file with 2 lines, got loaded=%v lines=%d", loaded, len(lines))
	}

	t.Setenv("BIGTOY_SECRETS_FILE", secretsPath)
	if resolved := resolveSecretsFilePath(); resolved != secretsPath {
		t.Fatalf("unexpected resolved secrets path: %s", resolved)
	}
}

func TestSetEnvIfAbsent(t *testing.T) {
	t.Setenv("BIGTOY_TOKEN", "preset")
	if err := setEnvIfAbsent("BIGTOY_TOKEN", "new-value", "secrets.env"); err != nil {
		t.Fatalf("set env when already present should not fail: %v", err)
	}
	if got := os.Getenv("BIGTOY_TOKEN"); got != "preset" {
		t.Fatalf("expected existing env value to remain unchanged, got %q", got)
	}

	const key = "BIGTOY_ADDED_VALUE"
	_ = os.Unsetenv(key)
	if err := setEnvIfAbsent(key, "added", "secrets.env"); err != nil {
		t.Fatalf("set missing env: %v", err)
	}
	if got := os.Getenv(key); got != "added" {
		t.Fatalf("expected env to be set, got %q", got)
	}
}

func TestExists(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "item.txt")
	if err := os.WriteFile(filePath, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if !exists(filePath) {
		t.Fatal("expected existing file to be detected")
	}
	if exists(filepath.Join(root, "missing.txt")) {
		t.Fatal("expected missing file to return false")
	}
}
