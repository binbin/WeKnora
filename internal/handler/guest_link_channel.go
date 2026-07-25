package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
)

// GuestLinkChannelHandler manages the guest link channel admin surface and
// the public /w/:slug bootstrap endpoint. Guest links publish an agent chat
// surface directly (no external-site embedding), so — unlike embed channels
// — they have no allowed_origins allowlist: bootstrap only succeeds for
// same-host requests.
type GuestLinkChannelHandler struct {
	guestSvc interfaces.GuestLinkChannelService
	embedSvc interfaces.EmbedChannelService
}

func NewGuestLinkChannelHandler(
	guestSvc interfaces.GuestLinkChannelService,
	embedSvc interfaces.EmbedChannelService,
) *GuestLinkChannelHandler {
	return &GuestLinkChannelHandler{guestSvc: guestSvc, embedSvc: embedSvc}
}

// BootstrapWebLink mints a short-lived session for a direct-open /w/:slug
// page. The slug itself is the public credential; no publish token or embed
// channel id appears in the URL. Cross-origin requests are always rejected:
// guest links have no allowed_origins concept, so only same-host callers
// (the page that /w/:slug itself served) may bootstrap a session.
func (h *GuestLinkChannelHandler) BootstrapWebLink(c *gin.Context) {
	ctx := c.Request.Context()
	slug := strings.TrimSpace(c.Param("slug"))
	gl, err := h.guestSvc.LookupByWebSlug(ctx, slug)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrGuestLinkDisabled):
			c.JSON(http.StatusForbidden, gin.H{"error": "guest link is disabled"})
		case errors.Is(err, service.ErrGuestLinkNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "web link not found"})
		default:
			logger.ErrorWithFields(ctx, err, nil)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve web link"})
		}
		return
	}

	origin := middleware.RequestOrigin(c)
	hostOrigin := middleware.HostOrigin(c)
	sameHost := origin != "" && hostOrigin != "" && strings.EqualFold(origin, hostOrigin)
	if !sameHost {
		c.JSON(http.StatusForbidden, gin.H{"error": "origin not allowed"})
		return
	}

	ch := gl.AsEmbedChannel()
	sessionToken, expiresIn, err := h.embedSvc.IssueSessionToken(ctx, ch.ID)
	if err != nil {
		if errors.Is(err, service.ErrEmbedSessionUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "session tokens unavailable"})
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue session token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"channel_id":    ch.ID,
			"session_token": sessionToken,
			"expires_in":    expiresIn,
			"config":        h.embedSvc.PublicConfig(ctx, ch),
		},
	})
}

type guestLinkChannelRequest struct {
	Name                   string  `json:"name"`
	Enabled                *bool   `json:"enabled"`
	WelcomeMessage         string  `json:"welcome_message"`
	RateLimitPerMinute     int     `json:"rate_limit_per_minute"`
	RateLimitPerDay        int     `json:"rate_limit_per_day"`
	PrimaryColor           string  `json:"primary_color"`
	PageTitle              string  `json:"page_title"`
	HeaderTitleMode        string  `json:"header_title_mode"`
	ShowSuggestedQuestions *bool   `json:"show_suggested_questions"`
	AllowWebSearch         *bool   `json:"allow_web_search"`
	AllowFileUpload        *bool   `json:"allow_file_upload"`
	DefaultLocale          *string `json:"default_locale"`
}

// GetGuestLinkByAgent returns the tenant's guest link for an agent, or a
// null data payload (200) when none has been created yet.
func (h *GuestLinkChannelHandler) GetGuestLinkByAgent(c *gin.Context) {
	agentID := secutils.SanitizeForLog(c.Param("id"))
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	gl, err := h.guestSvc.GetByAgent(c.Request.Context(), tenantID, agentID)
	if err != nil {
		writeGuestLinkMgmtError(c, err)
		return
	}
	if gl == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": guestLinkChannelResponse(c, gl)})
}

// CreateGuestLink creates the (at most one) guest link for an agent, 409ing
// if one already exists.
func (h *GuestLinkChannelHandler) CreateGuestLink(c *gin.Context) {
	agentID := secutils.SanitizeForLog(c.Param("id"))
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	var req guestLinkChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	showSuggested := true
	if req.ShowSuggestedQuestions != nil {
		showSuggested = *req.ShowSuggestedQuestions
	}
	allowWebSearch := false
	if req.AllowWebSearch != nil {
		allowWebSearch = *req.AllowWebSearch
	}
	allowFileUpload := false
	if req.AllowFileUpload != nil {
		allowFileUpload = *req.AllowFileUpload
	}
	gl, err := h.guestSvc.Create(c.Request.Context(), tenantID, agentID, &types.GuestLinkChannel{
		Name:                   req.Name,
		Enabled:                enabled,
		WelcomeMessage:         req.WelcomeMessage,
		RateLimitPerMinute:     req.RateLimitPerMinute,
		RateLimitPerDay:        req.RateLimitPerDay,
		PrimaryColor:           req.PrimaryColor,
		PageTitle:              req.PageTitle,
		HeaderTitleMode:        req.HeaderTitleMode,
		ShowSuggestedQuestions: showSuggested,
		AllowWebSearch:         allowWebSearch,
		AllowFileUpload:        allowFileUpload,
		DefaultLocale:          stringOrEmpty(req.DefaultLocale),
	})
	if err != nil {
		writeGuestLinkMgmtError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": guestLinkChannelResponse(c, gl)})
}

// GetGuestLink returns a single guest link for management.
func (h *GuestLinkChannelHandler) GetGuestLink(c *gin.Context) {
	id := secutils.SanitizeForLog(c.Param("id"))
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	gl, err := h.guestSvc.Get(c.Request.Context(), tenantID, id)
	if err != nil {
		writeGuestLinkMgmtError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": guestLinkChannelResponse(c, gl)})
}

// UpdateGuestLink updates a guest link's chat surface configuration.
func (h *GuestLinkChannelHandler) UpdateGuestLink(c *gin.Context) {
	id := secutils.SanitizeForLog(c.Param("id"))
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	var req guestLinkChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	update := &types.GuestLinkChannel{
		Name:               req.Name,
		WelcomeMessage:     req.WelcomeMessage,
		RateLimitPerMinute: req.RateLimitPerMinute,
		RateLimitPerDay:    req.RateLimitPerDay,
		PrimaryColor:       req.PrimaryColor,
		PageTitle:          req.PageTitle,
		HeaderTitleMode:    req.HeaderTitleMode,
		DefaultLocale:      stringOrEmpty(req.DefaultLocale),
	}
	if req.ShowSuggestedQuestions != nil {
		update.ShowSuggestedQuestions = *req.ShowSuggestedQuestions
	}
	if req.AllowWebSearch != nil {
		update.AllowWebSearch = *req.AllowWebSearch
	}
	if req.AllowFileUpload != nil {
		update.AllowFileUpload = *req.AllowFileUpload
	}
	gl, err := h.guestSvc.Update(c.Request.Context(), tenantID, id, update, req.Enabled)
	if err != nil {
		writeGuestLinkMgmtError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": guestLinkChannelResponse(c, gl)})
}

// DeleteGuestLink removes a guest link, freeing the agent to create a new one.
func (h *GuestLinkChannelHandler) DeleteGuestLink(c *gin.Context) {
	id := secutils.SanitizeForLog(c.Param("id"))
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if err := h.guestSvc.Delete(c.Request.Context(), tenantID, id); err != nil {
		writeGuestLinkMgmtError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// guestLinkChannelResponse renders a guest link for the admin UI. It
// includes web_url — the direct-open chat page at `{host}/w/{slug}` (see
// frontend buildWebChannelURL and the embed-main.ts `/w/:slug` SPA route) —
// built from the current admin request's host, alongside the bare web_slug
// so the frontend can rebuild the URL itself if the admin origin differs
// from the public embed origin.
func guestLinkChannelResponse(c *gin.Context, gl *types.GuestLinkChannel) gin.H {
	webURL := ""
	if host := middleware.HostOrigin(c); host != "" && gl.WebSlug != "" {
		webURL = host + "/w/" + url.PathEscape(gl.WebSlug)
	}
	return gin.H{
		"id":                       gl.ID,
		"tenant_id":                gl.TenantID,
		"agent_id":                 gl.AgentID,
		"name":                     gl.Name,
		"enabled":                  gl.Enabled,
		"web_slug":                 gl.WebSlug,
		"web_url":                  webURL,
		"welcome_message":          gl.WelcomeMessage,
		"rate_limit_per_minute":    gl.RateLimitPerMinute,
		"rate_limit_per_day":       gl.RateLimitPerDay,
		"primary_color":            gl.PrimaryColor,
		"page_title":               gl.PageTitle,
		"header_title_mode":        types.NormalizeEmbedHeaderTitleMode(gl.HeaderTitleMode),
		"show_suggested_questions": gl.ShowSuggestedQuestions,
		"allow_web_search":         gl.AllowWebSearch,
		"allow_file_upload":        gl.AllowFileUpload,
		"default_locale":           gl.DefaultLocale,
		"created_at":               gl.CreatedAt,
		"updated_at":               gl.UpdatedAt,
	}
}

func writeGuestLinkMgmtError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrGuestLinkNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "guest link not found"})
	case errors.Is(err, service.ErrGuestLinkExists):
		c.JSON(http.StatusConflict, gin.H{"error": "guest_link_exists"})
	default:
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			c.JSON(appErr.HTTPCode, gin.H{"error": appErr.Message})
			return
		}
		logger.Error(c.Request.Context(), "guest link management failed", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "operation failed"})
	}
}
