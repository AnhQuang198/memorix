package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/memorix/memorix/internal/identity/ports"
	"github.com/memorix/memorix/internal/identity/service"
	"github.com/memorix/memorix/internal/platform/authmw"
)

const refreshCookie = "memorix_refresh"

// Handler là driving adapter Gin cho module identity.
type Handler struct {
	svc        *service.Service
	mailer     ports.Mailer
	oauth      *OAuthDeps
	jwt        *authmw.JWTManager
	refreshTTL time.Duration
	secure     bool // Secure cookie (tắt trong test http)
}

// OAuthDeps tách để test không cần OIDC thật.
type OAuthDeps struct {
	Verifier ports.OIDCVerifier
	Begin    func(provider string) (redirectURL, state, nonce, verifier string, err error)
}

func New(svc *service.Service, mailer ports.Mailer, jwt *authmw.JWTManager, refreshTTL time.Duration, secure bool, oauth *OAuthDeps) *Handler {
	return &Handler{svc: svc, mailer: mailer, jwt: jwt, refreshTTL: refreshTTL, secure: secure, oauth: oauth}
}

// RegisterRoutes gắn route vào group /api/v1. protected group verify JWT (AD-11).
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup) {
	a := v1.Group("/auth")
	a.POST("/register", h.register)
	a.POST("/verify-email", h.verifyEmail)
	a.POST("/login", h.login)
	a.POST("/refresh", h.refresh)
	a.POST("/logout", h.logout)
	a.POST("/password/forgot", h.forgot)
	a.POST("/password/reset", h.reset)
	if h.oauth != nil {
		a.GET("/oauth/:provider/start", h.oauthStart)
		a.POST("/oauth/:provider/callback", h.oauthCallback)
	}

	me := v1.Group("")
	me.Use(authmw.RequireAuth(h.jwt))
	me.GET("/me", h.getMe)
	me.PATCH("/me", h.updateMe)
	me.POST("/account/export", h.exportData)
	me.DELETE("/account", h.deleteAccount)
}

func (h *Handler) setRefresh(c *gin.Context, raw string) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookie, raw, int(h.refreshTTL.Seconds()), "/api/v1/auth", "", h.secure, true)
}

func (h *Handler) register(c *gin.Context) {
	var body struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, errBadJSON)
		return
	}
	res, err := h.svc.Register(c.Request.Context(), service.RegisterInput{
		Email: body.Email, Password: body.Password, DisplayName: body.DisplayName,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	_ = h.mailer.SendVerification(c.Request.Context(), body.Email, res.VerifyToken)
	h.setRefresh(c, res.Tokens.RefreshToken)
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{
		"user_id":      res.UserID,
		"access_token": res.Tokens.AccessToken,
		"expires_at":   res.Tokens.AccessExpiresAt,
	}})
}

func (h *Handler) verifyEmail(c *gin.Context) {
	var body struct {
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, errBadJSON)
		return
	}
	if err := h.svc.VerifyEmail(c.Request.Context(), body.Token); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"verified": true}})
}

func (h *Handler) login(c *gin.Context) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, errBadJSON)
		return
	}
	tok, err := h.svc.Login(c.Request.Context(), body.Email, body.Password)
	if err != nil {
		writeErr(c, err)
		return
	}
	h.setRefresh(c, tok.RefreshToken)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"access_token": tok.AccessToken, "expires_at": tok.AccessExpiresAt,
	}})
}

func (h *Handler) refresh(c *gin.Context) {
	raw, err := c.Cookie(refreshCookie)
	if err != nil || raw == "" {
		writeErr(c, domainTokenInvalid)
		return
	}
	tok, err := h.svc.Refresh(c.Request.Context(), raw)
	if err != nil {
		writeErr(c, err)
		return
	}
	h.setRefresh(c, tok.RefreshToken)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"access_token": tok.AccessToken, "expires_at": tok.AccessExpiresAt,
	}})
}

func (h *Handler) logout(c *gin.Context) {
	// xóa cookie; refresh sẽ tự hết hạn / có thể revoke chủ động ở V1.
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookie, "", -1, "/api/v1/auth", "", h.secure, true)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"logged_out": true}})
}

func (h *Handler) forgot(c *gin.Context) {
	var body struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, errBadJSON)
		return
	}
	raw, err := h.svc.RequestReset(c.Request.Context(), body.Email)
	if err != nil {
		writeErr(c, err)
		return
	}
	if raw != "" {
		_ = h.mailer.SendPasswordReset(c.Request.Context(), body.Email, raw)
	}
	// Response GIỐNG NHAU dù email tồn tại hay không (Story 1.6).
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"sent": true}})
}

func (h *Handler) reset(c *gin.Context) {
	var body struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, errBadJSON)
		return
	}
	if err := h.svc.ResetPassword(c.Request.Context(), body.Token, body.NewPassword); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"reset": true}})
}

func (h *Handler) oauthStart(c *gin.Context) {
	redirectURL, state, nonce, verifier, err := h.oauth.Begin(c.Param("provider"))
	if err != nil {
		writeErr(c, domainOAuthFailed)
		return
	}
	// state/nonce/verifier lưu cookie ngắn hạn httpOnly để đối chiếu ở callback.
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("oauth_state", state, 600, "/api/v1/auth", "", h.secure, true)
	c.SetCookie("oauth_nonce", nonce, 600, "/api/v1/auth", "", h.secure, true)
	c.SetCookie("oauth_verifier", verifier, 600, "/api/v1/auth", "", h.secure, true)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"authorization_url": redirectURL}})
}

func (h *Handler) oauthCallback(c *gin.Context) {
	var body struct {
		Code        string `json:"code"`
		State       string `json:"state"`
		RedirectURI string `json:"redirect_uri"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, errBadJSON)
		return
	}
	state, _ := c.Cookie("oauth_state")
	nonce, _ := c.Cookie("oauth_nonce")
	verifier, _ := c.Cookie("oauth_verifier")
	if state == "" || body.State != state {
		writeErr(c, domainOAuthFailed)
		return
	}
	tok, err := h.svc.OAuthLogin(c.Request.Context(), c.Param("provider"), body.Code, verifier, body.RedirectURI, nonce)
	if err != nil {
		writeErr(c, err)
		return
	}
	h.setRefresh(c, tok.RefreshToken)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"access_token": tok.AccessToken, "expires_at": tok.AccessExpiresAt,
	}})
}

func (h *Handler) getMe(c *gin.Context) {
	p, _ := authmw.PrincipalFrom(c)
	u, err := h.svc.GetUser(c.Request.Context(), p.UserID)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": u})
}

func (h *Handler) updateMe(c *gin.Context) {
	p, _ := authmw.PrincipalFrom(c)
	var body struct {
		DisplayName *string `json:"display_name"`
		Timezone    *string `json:"timezone"`
		Locale      *string `json:"locale"`
		Theme       *string `json:"theme"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, errBadJSON)
		return
	}
	u, err := h.svc.UpdateProfile(c.Request.Context(), p.UserID, service.ProfileInput{
		DisplayName: body.DisplayName, Timezone: body.Timezone, Locale: body.Locale, Theme: body.Theme,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id": u.ID, "email": u.Email, "display_name": u.DisplayName,
		"timezone": u.Timezone, "locale": u.Locale, "theme": u.Theme,
	}})
}

func (h *Handler) exportData(c *gin.Context) {
	p, _ := authmw.PrincipalFrom(c)
	var body struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, errBadJSON)
		return
	}
	exp, err := h.svc.ExportData(c.Request.Context(), p.UserID, body.Password)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="memorix-export.json"`)
	c.JSON(http.StatusOK, exp)
}

func (h *Handler) deleteAccount(c *gin.Context) {
	p, _ := authmw.PrincipalFrom(c)
	if err := h.svc.DeleteAccount(c.Request.Context(), p.UserID); err != nil {
		writeErr(c, err)
		return
	}
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookie, "", -1, "/api/v1/auth", "", h.secure, true)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"deleted": true}})
}
