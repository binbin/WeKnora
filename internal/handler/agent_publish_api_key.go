package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
)

// AgentPublishAPIKeyHandler manages admin CRUD for agent-bound publish API keys.
type AgentPublishAPIKeyHandler struct {
	svc   interfaces.AgentPublishAPIKeyService
	agent interfaces.CustomAgentService
}

// NewAgentPublishAPIKeyHandler constructs the admin publish API key handler.
func NewAgentPublishAPIKeyHandler(
	svc interfaces.AgentPublishAPIKeyService,
	agent interfaces.CustomAgentService,
) *AgentPublishAPIKeyHandler {
	return &AgentPublishAPIKeyHandler{svc: svc, agent: agent}
}

type agentPublishAPIKeyCreateRequest struct {
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// List returns non-revoked publish API keys for an agent (masked).
func (h *AgentPublishAPIKeyHandler) List(c *gin.Context) {
	agentID := secutils.SanitizeForLog(c.Param("id"))
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if err := h.ensureAgentOwned(c.Request.Context(), tenantID, agentID); err != nil {
		writeAgentPublishAPIKeyError(c, err)
		return
	}
	keys, err := h.svc.ListByAgent(c.Request.Context(), tenantID, agentID)
	if err != nil {
		logger.ErrorWithFields(c.Request.Context(), err, nil)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list publish API keys"})
		return
	}
	items := make([]gin.H, 0, len(keys))
	for _, key := range keys {
		items = append(items, agentPublishAPIKeyResponse(key))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

// Create mints a new publish API key. The plaintext token is returned once
// in the top-level "plaintext" field and never again.
func (h *AgentPublishAPIKeyHandler) Create(c *gin.Context) {
	agentID := secutils.SanitizeForLog(c.Param("id"))
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if err := h.ensureAgentOwned(c.Request.Context(), tenantID, agentID); err != nil {
		writeAgentPublishAPIKeyError(c, err)
		return
	}
	var req agentPublishAPIKeyCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		utc := req.ExpiresAt.UTC()
		if !utc.After(time.Now().UTC()) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "expires_at must be in the future"})
			return
		}
		expiresAt = &utc
	}
	createdBy, _ := types.UserIDFromContext(c.Request.Context())
	result, err := h.svc.Create(c.Request.Context(), interfaces.AgentPublishAPIKeyCreateRequest{
		TenantID:  tenantID,
		AgentID:   agentID,
		Name:      name,
		CreatedBy: createdBy,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		writeAgentPublishAPIKeyError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"success":   true,
		"data":      agentPublishAPIKeyResponse(result.APIKey),
		"plaintext": result.Token,
	})
}

// Delete revokes a publish API key for the given agent.
func (h *AgentPublishAPIKeyHandler) Delete(c *gin.Context) {
	agentID := secutils.SanitizeForLog(c.Param("id"))
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if err := h.ensureAgentOwned(c.Request.Context(), tenantID, agentID); err != nil {
		writeAgentPublishAPIKeyError(c, err)
		return
	}
	keyID, err := strconv.ParseUint(c.Param("key_id"), 10, 64)
	if err != nil || keyID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid key_id"})
		return
	}
	if err := h.svc.Revoke(c.Request.Context(), tenantID, agentID, keyID); err != nil {
		writeAgentPublishAPIKeyError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ensureAgentOwned verifies the path agent belongs to the current tenant.
func (h *AgentPublishAPIKeyHandler) ensureAgentOwned(
	ctx context.Context, tenantID uint64, agentID string,
) error {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return apperrors.NewBadRequestError("agent_id is required")
	}
	if types.IsBuiltinAgentID(agentID) {
		return apperrors.NewBadRequestError(
			"built-in agents cannot use publish API keys",
		)
	}
	agent, err := h.agent.GetAgentByID(ctx, agentID)
	if err != nil {
		return err
	}
	if agent == nil || agent.TenantID != tenantID {
		return apperrors.NewNotFoundError("agent not found")
	}
	return nil
}

func agentPublishAPIKeyResponse(key *types.AgentPublishAPIKey) gin.H {
	if key == nil {
		return gin.H{}
	}
	return gin.H{
		"id":           key.ID,
		"name":         key.Name,
		"api_key":      key.MaskedKey(),
		"created_at":   key.CreatedAt,
		"last_used_at": key.LastUsedAt,
		"expires_at":   key.ExpiresAt,
	}
}

func writeAgentPublishAPIKeyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, apprepo.ErrAgentPublishAPIKeyNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "publish API key not found"})
	default:
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			c.JSON(appErr.HTTPCode, gin.H{"error": appErr.Message})
			return
		}
		logger.Error(c.Request.Context(), "agent publish API key operation failed", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "operation failed"})
	}
}
