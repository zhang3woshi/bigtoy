package controllers

import (
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/beego/beego/v2/server/web"

	"bigtoy/backend/services"
)

const maxBackupImportBytes = 1024 << 20

type BackupRuntimeConfig struct {
	DBPath         string
	ImagesRoot     string
	BackupDir      string
	LegacyDataPath string
}

var (
	backupRuntimeConfig BackupRuntimeConfig
	backupImportMu      sync.Mutex
)

func SetBackupRuntimeConfig(config BackupRuntimeConfig) {
	backupRuntimeConfig = BackupRuntimeConfig{
		DBPath:         strings.TrimSpace(config.DBPath),
		ImagesRoot:     strings.TrimSpace(config.ImagesRoot),
		BackupDir:      strings.TrimSpace(config.BackupDir),
		LegacyDataPath: strings.TrimSpace(config.LegacyDataPath),
	}
}

type BackupController struct {
	web.Controller
}

func (c *BackupController) Export() {
	if _, ok := sessionFromRequest(c.Ctx.Request); !ok {
		c.Ctx.Output.SetStatus(http.StatusUnauthorized)
		c.Data["json"] = map[string]any{"error": "authentication required"}
		c.ServeJSON()
		return
	}

	config, err := currentBackupRuntimeConfig()
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]any{"error": err.Error()}
		c.ServeJSON()
		return
	}

	backupService, err := newBackupService(config)
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]any{"error": err.Error()}
		c.ServeJSON()
		return
	}

	archivePath, err := backupService.CreateBackupArchive()
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]any{"error": err.Error()}
		c.ServeJSON()
		return
	}

	archiveFile, err := os.Open(archivePath)
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]any{"error": "open backup archive failed"}
		c.ServeJSON()
		return
	}
	defer archiveFile.Close()

	archiveInfo, err := archiveFile.Stat()
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]any{"error": "read backup archive info failed"}
		c.ServeJSON()
		return
	}

	fileName := filepath.Base(archivePath)
	c.Ctx.Output.Header("Content-Type", "application/zip")
	c.Ctx.Output.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Ctx.Output.Header("Content-Length", fmt.Sprintf("%d", archiveInfo.Size()))
	c.Ctx.Output.SetStatus(http.StatusOK)

	if _, err := io.Copy(c.Ctx.ResponseWriter, archiveFile); err != nil {
		log.Printf("[backup] stream backup archive failed: %v", err)
	}
}

func (c *BackupController) Import() {
	if _, ok := sessionFromRequest(c.Ctx.Request); !ok {
		c.Ctx.Output.SetStatus(http.StatusUnauthorized)
		c.Data["json"] = map[string]any{"error": "authentication required"}
		c.ServeJSON()
		return
	}

	config, err := currentBackupRuntimeConfig()
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]any{"error": err.Error()}
		c.ServeJSON()
		return
	}

	uploadHeader, err := parseBackupUpload(c.Ctx.Request)
	if c.Ctx.Request.MultipartForm != nil {
		defer c.Ctx.Request.MultipartForm.RemoveAll()
	}
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.Data["json"] = map[string]any{"error": err.Error()}
		c.ServeJSON()
		return
	}

	uploadedArchivePath, cleanup, err := writeUploadedArchive(config.BackupDir, uploadHeader)
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.Data["json"] = map[string]any{"error": err.Error()}
		c.ServeJSON()
		return
	}
	defer cleanup()

	backupImportMu.Lock()
	defer backupImportMu.Unlock()

	store := modelStore
	if store == nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]any{"error": "model store is not initialized"}
		c.ServeJSON()
		return
	}

	SetModelStore(nil)
	_ = store.Close()

	backupService, err := newBackupService(config)
	if err != nil {
		_ = recoverModelStore(config)
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]any{"error": err.Error()}
		c.ServeJSON()
		return
	}

	if err := backupService.RestoreBackupArchive(uploadedArchivePath); err != nil {
		_ = recoverModelStore(config)
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.Data["json"] = map[string]any{"error": err.Error()}
		c.ServeJSON()
		return
	}

	restoredStore, err := services.NewModelStore(config.DBPath, config.ImagesRoot, config.LegacyDataPath)
	if err != nil {
		_ = recoverModelStore(config)
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]any{"error": fmt.Sprintf("reinitialize model store failed: %v", err)}
		c.ServeJSON()
		return
	}

	SetModelStore(restoredStore)
	c.Data["json"] = map[string]any{
		"data": map[string]any{
			"restored":           true,
			"restartRecommended": true,
		},
	}
	c.ServeJSON()
}

func currentBackupRuntimeConfig() (BackupRuntimeConfig, error) {
	config := BackupRuntimeConfig{
		DBPath:         strings.TrimSpace(backupRuntimeConfig.DBPath),
		ImagesRoot:     strings.TrimSpace(backupRuntimeConfig.ImagesRoot),
		BackupDir:      strings.TrimSpace(backupRuntimeConfig.BackupDir),
		LegacyDataPath: strings.TrimSpace(backupRuntimeConfig.LegacyDataPath),
	}

	switch {
	case config.DBPath == "":
		return BackupRuntimeConfig{}, errors.New("backup db path is not configured")
	case config.ImagesRoot == "":
		return BackupRuntimeConfig{}, errors.New("backup images root path is not configured")
	case config.BackupDir == "":
		return BackupRuntimeConfig{}, errors.New("backup directory is not configured")
	default:
		return config, nil
	}
}

func newBackupService(config BackupRuntimeConfig) (*services.BackupService, error) {
	return services.NewBackupService(services.BackupServiceConfig{
		DBPath:     config.DBPath,
		ImagesRoot: config.ImagesRoot,
		BackupDir:  config.BackupDir,
		Interval:   time.Minute,
		MaxBackups: 3,
	})
}

func parseBackupUpload(request *http.Request) (*multipart.FileHeader, error) {
	if request == nil {
		return nil, errors.New("invalid request")
	}
	if err := request.ParseMultipartForm(maxBackupImportBytes); err != nil {
		return nil, errors.New("invalid multipart payload")
	}

	file := firstValidFile(request.MultipartForm.File["file"])
	if file == nil {
		file = firstValidFile(request.MultipartForm.File["backupFile"])
	}
	if file == nil {
		return nil, errors.New("backup zip file is required")
	}
	if file.Size <= 0 {
		return nil, errors.New("backup zip file is empty")
	}
	if file.Size > maxBackupImportBytes {
		return nil, errors.New("backup zip file exceeds max size")
	}

	return file, nil
}

func writeUploadedArchive(backupDir string, fileHeader *multipart.FileHeader) (string, func(), error) {
	if fileHeader == nil {
		return "", nil, errors.New("backup zip file is required")
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", nil, fmt.Errorf("ensure backup directory failed: %w", err)
	}

	tempFile, err := os.CreateTemp(backupDir, "import_*.zip")
	if err != nil {
		return "", nil, fmt.Errorf("create temp backup file failed: %w", err)
	}
	tempPath := tempFile.Name()
	cleanup := func() { _ = os.Remove(tempPath) }

	source, err := fileHeader.Open()
	if err != nil {
		tempFile.Close()
		cleanup()
		return "", nil, errors.New("open uploaded backup file failed")
	}
	defer source.Close()

	written, err := io.Copy(tempFile, io.LimitReader(source, maxBackupImportBytes+1))
	closeErr := tempFile.Close()
	if err != nil {
		cleanup()
		return "", nil, errors.New("write uploaded backup file failed")
	}
	if closeErr != nil {
		cleanup()
		return "", nil, errors.New("close uploaded backup file failed")
	}
	if written > maxBackupImportBytes {
		cleanup()
		return "", nil, errors.New("backup zip file exceeds max size")
	}

	return tempPath, cleanup, nil
}

func recoverModelStore(config BackupRuntimeConfig) error {
	recoveredStore, err := services.NewModelStore(config.DBPath, config.ImagesRoot, config.LegacyDataPath)
	if err != nil {
		log.Printf("[backup] recover model store failed: %v", err)
		SetModelStore(nil)
		return err
	}
	SetModelStore(recoveredStore)
	return nil
}
