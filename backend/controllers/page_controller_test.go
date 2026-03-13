package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/beego/beego/v2/server/web/context"

	"bigtoy/backend/services"
)

func newPageControllerForTest(request *http.Request) (*PageController, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx := context.NewContext()
	ctx.Reset(recorder, request)

	controller := &PageController{}
	controller.Ctx = ctx
	return controller, recorder
}

func setupPageAuthService(t *testing.T) string {
	t.Helper()

	hash, err := services.GeneratePasswordHash("Secret#12345")
	if err != nil {
		t.Fatalf("generate hash: %v", err)
	}
	svc, err := services.NewAuthService(services.AuthConfig{
		AdminUser:         "admin",
		AdminPasswordHash: hash,
		SessionTTL:        time.Minute,
	})
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	authService = svc
	t.Cleanup(func() {
		authService = nil
	})

	token, _, err := svc.Login("127.0.0.1", "admin", "Secret#12345")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	return token
}

func TestPageControllerPublicAndDetail(t *testing.T) {
	controller, _ := newPageControllerForTest(httptest.NewRequest(http.MethodGet, "/", nil))
	controller.Public()
	if controller.TplName != "index.html" {
		t.Fatalf("unexpected public tpl: %s", controller.TplName)
	}

	controller.Detail()
	if controller.TplName != "model.html" {
		t.Fatalf("unexpected detail tpl: %s", controller.TplName)
	}
}

func TestPageControllerAdminRedirectWithoutSession(t *testing.T) {
	setupPageAuthService(t)

	request := httptest.NewRequest(http.MethodGet, "/admin.html", nil)
	controller, recorder := newPageControllerForTest(request)
	controller.Admin()

	if recorder.Code != http.StatusFound {
		t.Fatalf("expected redirect status, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "/login.html" {
		t.Fatalf("unexpected redirect location: %s", location)
	}

	editRequest := httptest.NewRequest(http.MethodGet, "/admin-edit.html", nil)
	editController, editRecorder := newPageControllerForTest(editRequest)
	editController.AdminEdit()

	if editRecorder.Code != http.StatusFound {
		t.Fatalf("expected edit redirect status, got %d", editRecorder.Code)
	}
	if location := editRecorder.Header().Get("Location"); location != "/login.html" {
		t.Fatalf("unexpected edit redirect location: %s", location)
	}
}

func TestPageControllerAdminAndLoginWithSession(t *testing.T) {
	token := setupPageAuthService(t)

	adminRequest := httptest.NewRequest(http.MethodGet, "/admin.html", nil)
	adminRequest.AddCookie(&http.Cookie{
		Name:  authService.CookieName(),
		Value: token,
	})
	adminController, adminRecorder := newPageControllerForTest(adminRequest)
	adminController.Admin()
	if adminRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected admin status: %d", adminRecorder.Code)
	}
	if adminController.TplName != "admin.html" {
		t.Fatalf("unexpected admin tpl: %s", adminController.TplName)
	}

	editRequest := httptest.NewRequest(http.MethodGet, "/admin-edit.html", nil)
	editRequest.AddCookie(&http.Cookie{
		Name:  authService.CookieName(),
		Value: token,
	})
	editController, editRecorder := newPageControllerForTest(editRequest)
	editController.AdminEdit()
	if editRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected admin edit status: %d", editRecorder.Code)
	}
	if editController.TplName != "admin-edit.html" {
		t.Fatalf("unexpected admin edit tpl: %s", editController.TplName)
	}

	loginRequest := httptest.NewRequest(http.MethodGet, "/login.html", nil)
	loginRequest.AddCookie(&http.Cookie{
		Name:  authService.CookieName(),
		Value: token,
	})
	loginController, loginRecorder := newPageControllerForTest(loginRequest)
	loginController.Login()
	if loginRecorder.Code != http.StatusFound {
		t.Fatalf("expected redirect for authenticated login page, got %d", loginRecorder.Code)
	}
	if location := loginRecorder.Header().Get("Location"); location != "/admin.html" {
		t.Fatalf("unexpected login redirect location: %s", location)
	}
}
