package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
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
