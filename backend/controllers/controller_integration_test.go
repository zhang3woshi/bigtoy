package controllers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/beego/beego/v2/server/web/context"
	"github.com/google/uuid"

	"bigtoy/backend/models"
	"bigtoy/backend/services"
)

func setupControllerDeps(t *testing.T) (token string, cookieName string) {
	t.Helper()

	hash, err := services.GeneratePasswordHash("Secret#12345")
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}

	auth, err := services.NewAuthService(services.AuthConfig{
		AdminUser:         "admin",
		AdminPasswordHash: hash,
		SessionTTL:        time.Hour,
	})
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}

	root := t.TempDir()
	dbPath := filepath.Join(root, "data", "models.db")
	imagesRoot := filepath.Join(root, "images")
	backupDir := filepath.Join(root, "backup")
	store, err := services.NewModelStore(
		dbPath,
		imagesRoot,
		"",
	)
	if err != nil {
		t.Fatalf("new model store: %v", err)
	}

	SetAuthService(auth)
	SetModelStore(store)
	SetBackupRuntimeConfig(BackupRuntimeConfig{
		DBPath:         dbPath,
		ImagesRoot:     imagesRoot,
		BackupDir:      backupDir,
		LegacyDataPath: "",
	})
	t.Cleanup(func() {
		SetAuthService(nil)
		currentStore := modelStore
		SetModelStore(nil)
		SetBackupRuntimeConfig(BackupRuntimeConfig{})
		if currentStore != nil {
			_ = currentStore.Close()
		}
	})

	token, _, err = auth.Login("127.0.0.1", "admin", "Secret#12345")
	if err != nil {
		t.Fatalf("seed login: %v", err)
	}

	return token, auth.CookieName()
}

func newControllerContext(method, target string, body []byte, contentType string) (*context.Context, *httptest.ResponseRecorder) {
	request := httptest.NewRequest(method, target, bytes.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	recorder := httptest.NewRecorder()
	ctx := context.NewContext()
	ctx.Reset(recorder, request)
	ctx.Input.RequestBody = body
	return ctx, recorder
}

func decodeJSONBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode json body: %v; body=%s", err, recorder.Body.String())
	}
	return payload
}

func newMultipartContext(t *testing.T, method, target, fieldName, fileName string, content []byte) (*context.Context, *httptest.ResponseRecorder) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fileWriter, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := fileWriter.Write(content); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	return newControllerContext(method, target, body.Bytes(), writer.FormDataContentType())
}

func TestAuthControllerLoginLogoutAndMe(t *testing.T) {
	setupControllerDeps(t)

	invalidCtx, invalidRecorder := newControllerContext(http.MethodPost, "/api/auth/login", []byte("{invalid"), "application/json")
	invalidController := &AuthController{}
	invalidController.Init(invalidCtx, "AuthController", "Login", nil)
	invalidController.Login()
	if invalidRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid json, got %d", invalidRecorder.Code)
	}

	badPayload := []byte(`{"username":"admin","password":"wrong"}`)
	badCtx, badRecorder := newControllerContext(http.MethodPost, "/api/auth/login", badPayload, "application/json")
	badController := &AuthController{}
	badController.Init(badCtx, "AuthController", "Login", nil)
	badController.Login()
	if badRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid credentials, got %d", badRecorder.Code)
	}

	goodPayload := []byte(`{"username":"admin","password":"Secret#12345"}`)
	goodCtx, goodRecorder := newControllerContext(http.MethodPost, "/api/auth/login", goodPayload, "application/json")
	goodController := &AuthController{}
	goodController.Init(goodCtx, "AuthController", "Login", nil)
	goodController.Login()
	if goodRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for successful login, got %d", goodRecorder.Code)
	}
	if len(goodRecorder.Result().Cookies()) == 0 {
		t.Fatal("expected auth cookie after login")
	}

	loginCookie := goodRecorder.Result().Cookies()[0]

	meCtx, meRecorder := newControllerContext(http.MethodGet, "/api/auth/me", nil, "")
	meCtx.Request.AddCookie(loginCookie)
	meController := &AuthController{}
	meController.Init(meCtx, "AuthController", "Me", nil)
	meController.Me()
	if meRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for me endpoint, got %d", meRecorder.Code)
	}
	mePayload := decodeJSONBody(t, meRecorder)
	data := mePayload["data"].(map[string]any)
	if authenticated, _ := data["authenticated"].(bool); !authenticated {
		t.Fatalf("expected authenticated response, payload=%#v", data)
	}

	logoutCtx, logoutRecorder := newControllerContext(http.MethodPost, "/api/auth/logout", nil, "")
	logoutCtx.Request.AddCookie(loginCookie)
	logoutController := &AuthController{}
	logoutController.Init(logoutCtx, "AuthController", "Logout", nil)
	logoutController.Logout()
	if logoutRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for logout, got %d", logoutRecorder.Code)
	}
}

func TestBackupControllerExportAndImportFlow(t *testing.T) {
	token, cookieName := setupControllerDeps(t)

	if _, err := modelStore.Add(models.CreateModelRequest{Name: "Export A", Year: 2020}); err != nil {
		t.Fatalf("seed export model A: %v", err)
	}
	if _, err := modelStore.Add(models.CreateModelRequest{Name: "Export B", Year: 2021}); err != nil {
		t.Fatalf("seed export model B: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(backupRuntimeConfig.ImagesRoot, "before"), 0o755); err != nil {
		t.Fatalf("create source image dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupRuntimeConfig.ImagesRoot, "before", "keep.png"), []byte("keep"), 0o644); err != nil {
		t.Fatalf("write source image file: %v", err)
	}

	unauthorizedExportCtx, unauthorizedExportRecorder := newControllerContext(http.MethodGet, "/api/backup/export", nil, "")
	unauthorizedExport := &BackupController{}
	unauthorizedExport.Init(unauthorizedExportCtx, "BackupController", "Export", nil)
	unauthorizedExport.Export()
	if unauthorizedExportRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthorized backup export, got %d", unauthorizedExportRecorder.Code)
	}

	exportCtx, exportRecorder := newControllerContext(http.MethodGet, "/api/backup/export", nil, "")
	exportCtx.Request.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	exportController := &BackupController{}
	exportController.Init(exportCtx, "BackupController", "Export", nil)
	exportController.Export()
	if exportRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for backup export, got %d body=%s", exportRecorder.Code, exportRecorder.Body.String())
	}
	if got := exportRecorder.Header().Get("Content-Type"); !strings.Contains(strings.ToLower(got), "application/zip") {
		t.Fatalf("expected zip content-type, got %q", got)
	}
	if exportRecorder.Header().Get("Content-Disposition") == "" {
		t.Fatal("expected Content-Disposition header in export response")
	}

	exportReader, err := zip.NewReader(bytes.NewReader(exportRecorder.Body.Bytes()), int64(exportRecorder.Body.Len()))
	if err != nil {
		t.Fatalf("decode exported zip: %v", err)
	}
	exportedEntries := make([]string, 0, len(exportReader.File))
	for _, file := range exportReader.File {
		exportedEntries = append(exportedEntries, file.Name)
	}
	if !slices.Contains(exportedEntries, "db/models.db") {
		t.Fatalf("expected exported zip to contain db/models.db, got %#v", exportedEntries)
	}
	if !slices.Contains(exportedEntries, "images/before/keep.png") {
		t.Fatalf("expected exported zip to contain images/before/keep.png, got %#v", exportedEntries)
	}

	if _, err := modelStore.Add(models.CreateModelRequest{Name: "Export C", Year: 2022}); err != nil {
		t.Fatalf("seed extra model before import: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(backupRuntimeConfig.ImagesRoot, "after"), 0o755); err != nil {
		t.Fatalf("create extra image dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupRuntimeConfig.ImagesRoot, "after", "drop.png"), []byte("drop"), 0o644); err != nil {
		t.Fatalf("write extra image file: %v", err)
	}
	if got := len(modelStore.List()); got != 3 {
		t.Fatalf("expected 3 models before import restore, got %d", got)
	}

	unauthorizedImportCtx, unauthorizedImportRecorder := newMultipartContext(t, http.MethodPost, "/api/backup/import", "file", "backup.zip", exportRecorder.Body.Bytes())
	unauthorizedImport := &BackupController{}
	unauthorizedImport.Init(unauthorizedImportCtx, "BackupController", "Import", nil)
	unauthorizedImport.Import()
	if unauthorizedImportRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthorized backup import, got %d", unauthorizedImportRecorder.Code)
	}

	importCtx, importRecorder := newMultipartContext(t, http.MethodPost, "/api/backup/import", "file", "backup.zip", exportRecorder.Body.Bytes())
	importCtx.Request.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	importController := &BackupController{}
	importController.Init(importCtx, "BackupController", "Import", nil)
	importController.Import()
	if importRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for backup import, got %d body=%s", importRecorder.Code, importRecorder.Body.String())
	}

	importedPayload := decodeJSONBody(t, importRecorder)
	importedData, ok := importedPayload["data"].(map[string]any)
	if !ok {
		t.Fatalf("invalid backup import response payload: %#v", importedPayload)
	}
	if restored, ok := importedData["restored"].(bool); !ok || !restored {
		t.Fatalf("expected restored=true, got %#v", importedData["restored"])
	}

	items := modelStore.List()
	if len(items) != 2 {
		t.Fatalf("expected 2 models after backup restore, got %d", len(items))
	}
	names := []string{items[0].Name, items[1].Name}
	if !slices.Contains(names, "Export A") || !slices.Contains(names, "Export B") {
		t.Fatalf("expected restored model names Export A and Export B, got %#v", names)
	}
	if slices.Contains(names, "Export C") {
		t.Fatalf("unexpected stale model remains after restore: %#v", names)
	}

	if _, err := os.Stat(filepath.Join(backupRuntimeConfig.ImagesRoot, "before", "keep.png")); err != nil {
		t.Fatalf("expected restored image keep.png to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backupRuntimeConfig.ImagesRoot, "after", "drop.png")); !os.IsNotExist(err) {
		t.Fatalf("expected stale image drop.png to be removed, got err=%v", err)
	}
}

func TestModelControllerCRUDFlow(t *testing.T) {
	token, cookieName := setupControllerDeps(t)

	unauthorizedBody := []byte(`{"name":"Model A"}`)
	unauthorizedCtx, unauthorizedRecorder := newControllerContext(http.MethodPost, "/api/models", unauthorizedBody, "application/json")
	unauthorized := &ModelController{}
	unauthorized.Init(unauthorizedCtx, "ModelController", "Post", nil)
	unauthorized.Post()
	if unauthorizedRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthorized create, got %d", unauthorizedRecorder.Code)
	}

	createBody := []byte(`{"name":"Model A","brand":"Brand A","year":2020}`)
	createCtx, createRecorder := newControllerContext(http.MethodPost, "/api/models", createBody, "application/json")
	createCtx.Request.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	createController := &ModelController{}
	createController.Init(createCtx, "ModelController", "Post", nil)
	createController.Post()
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 for create, got %d body=%s", createRecorder.Code, createRecorder.Body.String())
	}

	createdPayload := decodeJSONBody(t, createRecorder)
	created := createdPayload["data"].(map[string]any)
	modelID, ok := created["id"].(string)
	if !ok {
		t.Fatalf("invalid created model id type: %#v", created["id"])
	}
	if _, err := uuid.Parse(modelID); err != nil {
		t.Fatalf("invalid created model id value: %q", modelID)
	}

	getCtx, getRecorder := newControllerContext(http.MethodGet, "/api/models", nil, "")
	getController := &ModelController{}
	getController.Init(getCtx, "ModelController", "Get", nil)
	getController.Get()
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for get, got %d", getRecorder.Code)
	}

	invalidPutCtx, invalidPutRecorder := newControllerContext(http.MethodPut, "/api/models/bad", createBody, "application/json")
	invalidPutCtx.Request.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	invalidPutCtx.Input.SetParam(":id", "bad")
	invalidPut := &ModelController{}
	invalidPut.Init(invalidPutCtx, "ModelController", "Put", nil)
	invalidPut.Put()
	if invalidPutRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid id, got %d", invalidPutRecorder.Code)
	}

	updateBody := []byte(`{"name":"Model B","brand":"Brand B","year":2021}`)
	putCtx, putRecorder := newControllerContext(http.MethodPut, "/api/models/"+modelID, updateBody, "application/json")
	putCtx.Request.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	putCtx.Input.SetParam(":id", modelID)
	putController := &ModelController{}
	putController.Init(putCtx, "ModelController", "Put", nil)
	putController.Put()
	if putRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for update, got %d body=%s", putRecorder.Code, putRecorder.Body.String())
	}

	deleteCtx, deleteRecorder := newControllerContext(http.MethodDelete, "/api/models/"+modelID, nil, "")
	deleteCtx.Request.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	deleteCtx.Input.SetParam(":id", modelID)
	deleteController := &ModelController{}
	deleteController.Init(deleteCtx, "ModelController", "Delete", nil)
	deleteController.Delete()
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for delete, got %d body=%s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	missingID := uuid.NewString()
	deleteMissingCtx, deleteMissingRecorder := newControllerContext(http.MethodDelete, "/api/models/"+missingID, nil, "")
	deleteMissingCtx.Request.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	deleteMissingCtx.Input.SetParam(":id", missingID)
	deleteMissingController := &ModelController{}
	deleteMissingController.Init(deleteMissingCtx, "ModelController", "Delete", nil)
	deleteMissingController.Delete()
	if deleteMissingRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing model delete, got %d", deleteMissingRecorder.Code)
	}
}
