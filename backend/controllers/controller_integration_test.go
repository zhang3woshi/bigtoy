package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/beego/beego/v2/server/web/context"

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
	store, err := services.NewModelStore(
		filepath.Join(root, "data", "models.db"),
		filepath.Join(root, "images"),
		"",
	)
	if err != nil {
		t.Fatalf("new model store: %v", err)
	}

	SetAuthService(auth)
	SetModelStore(store)
	t.Cleanup(func() {
		SetAuthService(nil)
		SetModelStore(nil)
		_ = store.Close()
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
	modelID := int64(created["id"].(float64))
	if modelID <= 0 {
		t.Fatalf("invalid created model id: %d", modelID)
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
	putCtx, putRecorder := newControllerContext(http.MethodPut, "/api/models/1", updateBody, "application/json")
	putCtx.Request.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	putCtx.Input.SetParam(":id", "1")
	putController := &ModelController{}
	putController.Init(putCtx, "ModelController", "Put", nil)
	putController.Put()
	if putRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for update, got %d body=%s", putRecorder.Code, putRecorder.Body.String())
	}

	deleteCtx, deleteRecorder := newControllerContext(http.MethodDelete, "/api/models/1", nil, "")
	deleteCtx.Request.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	deleteCtx.Input.SetParam(":id", "1")
	deleteController := &ModelController{}
	deleteController.Init(deleteCtx, "ModelController", "Delete", nil)
	deleteController.Delete()
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for delete, got %d body=%s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	deleteMissingCtx, deleteMissingRecorder := newControllerContext(http.MethodDelete, "/api/models/999", nil, "")
	deleteMissingCtx.Request.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	deleteMissingCtx.Input.SetParam(":id", "999")
	deleteMissingController := &ModelController{}
	deleteMissingController.Init(deleteMissingCtx, "ModelController", "Delete", nil)
	deleteMissingController.Delete()
	if deleteMissingRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing model delete, got %d", deleteMissingRecorder.Code)
	}
}
