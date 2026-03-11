package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/beego/beego/v2/server/web"

	"bigtoy/backend/services"
)

var authService *services.AuthService

type AuthController struct {
	web.Controller
}

func SetAuthService(service *services.AuthService) {
	authService = service
}

func (c *AuthController) Login() {
	if authService == nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]any{"error": "auth service is not initialized"}
		c.ServeJSON()
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.Data["json"] = map[string]any{"error": "invalid JSON payload"}
		c.ServeJSON()
		return
	}

	token, expiresAt, err := authService.Login(services.ClientIPFromRequest(c.Ctx.Request), req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrTooManyAttempts):
			c.Ctx.Output.SetStatus(http.StatusTooManyRequests)
			c.Data["json"] = map[string]any{"error": err.Error()}
		case errors.Is(err, services.ErrInvalidCredentials):
			c.Ctx.Output.SetStatus(http.StatusUnauthorized)
			c.Data["json"] = map[string]any{"error": err.Error()}
		default:
			c.Ctx.Output.SetStatus(http.StatusInternalServerError)
			c.Data["json"] = map[string]any{"error": "login failed"}
		}
		c.ServeJSON()
		return
	}

	setAuthCookie(c.Ctx.ResponseWriter, c.Ctx.Request, token, expiresAt)
	c.Data["json"] = map[string]any{
		"data": map[string]any{
			"authenticated": true,
			"username":      strings.TrimSpace(req.Username),
			"expiresAt":     expiresAt,
		},
	}
	c.ServeJSON()
}

func (c *AuthController) Logout() {
	if authService == nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]any{"error": "auth service is not initialized"}
		c.ServeJSON()
		return
	}

	token := readSessionToken(c.Ctx.Request)
	authService.Logout(token)
	clearAuthCookie(c.Ctx.ResponseWriter, c.Ctx.Request)

	c.Data["json"] = map[string]any{
		"data": map[string]any{
			"authenticated": false,
		},
	}
	c.ServeJSON()
}

func (c *AuthController) Me() {
	if authService == nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]any{"error": "auth service is not initialized"}
		c.ServeJSON()
		return
	}

	session, authenticated := sessionFromRequest(c.Ctx.Request)
	payload := map[string]any{
		"authenticated": authenticated,
	}
	if authenticated {
		payload["username"] = session.Username
		payload["expiresAt"] = session.ExpiresAt
	}

	c.Data["json"] = map[string]any{"data": payload}
	c.ServeJSON()
}

func sessionFromRequest(r *http.Request) (services.SessionInfo, bool) {
	if authService == nil || r == nil {
		return services.SessionInfo{}, false
	}

	token := readSessionToken(r)
	if token == "" {
		return services.SessionInfo{}, false
	}
	return authService.ValidateSession(token)
}

func readSessionToken(r *http.Request) string {
	if authService == nil || r == nil {
		return ""
	}

	cookie, err := r.Cookie(authService.CookieName())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func setAuthCookie(w http.ResponseWriter, r *http.Request, token string, expiresAt time.Time) {
	if authService == nil {
		return
	}

	ttlSeconds := int(time.Until(expiresAt).Seconds())
	if ttlSeconds < 0 {
		ttlSeconds = 0
	}

	http.SetCookie(w, &http.Cookie{
		Name:     authService.CookieName(),
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   ttlSeconds,
		HttpOnly: true,
		Secure:   authService.ShouldUseSecureCookie(r),
		SameSite: http.SameSiteStrictMode,
	})
}

func clearAuthCookie(w http.ResponseWriter, r *http.Request) {
	if authService == nil {
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     authService.CookieName(),
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   authService.ShouldUseSecureCookie(r),
		SameSite: http.SameSiteStrictMode,
	})
}
