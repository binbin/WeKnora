package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// PublishAPIKeyAuth authenticates OpenAI-compatible routes with
// Authorization: Bearer wkpub_… only. It intentionally does not set
// TenantAPIKeyScope so APIKeyRouteAuthorizer will not treat the caller as a
// workspace API key.
func PublishAPIKeyAuth(
	svc interfaces.AgentPublishAPIKeyService,
	tenantService interfaces.TenantService,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			abortOpenAPIUnauthorized(c, "unauthorized", "missing bearer token")
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if token == "" {
			abortOpenAPIUnauthorized(c, "unauthorized", "missing bearer token")
			return
		}

		key, err := svc.Authenticate(c.Request.Context(), token)
		if err != nil || key == nil {
			abortOpenAPIUnauthorized(c, "unauthorized", "invalid api key")
			return
		}

		tenant, err := tenantService.GetTenantByID(c.Request.Context(), key.TenantID)
		if err != nil || tenant == nil {
			logger.Warnf(
				c.Request.Context(),
				"publish api key auth: tenant unavailable (tenant_id=%d key_id=%d): %v",
				key.TenantID,
				key.ID,
				err,
			)
			abortOpenAPIError(
				c,
				http.StatusInternalServerError,
				"server_error",
				"workspace_unavailable",
				"workspace unavailable",
			)
			return
		}

		principal := types.Principal{
			Type: types.PrincipalAPIPublish,
			ID:   strconv.FormatUint(key.ID, 10),
		}
		userID := principal.StorageID()
		user := &types.User{
			ID:       userID,
			Username: userID,
			Email:    fmt.Sprintf("publish-api-key-%d@api-key.local", key.ID),
			TenantID: key.TenantID,
			IsActive: true,
		}
		pubCtx := types.AgentPublishAPIKeyContext{
			KeyID:    key.ID,
			TenantID: key.TenantID,
			AgentID:  key.AgentID,
		}

		c.Set(types.TenantIDContextKey.String(), key.TenantID)
		c.Set(types.TenantInfoContextKey.String(), tenant)
		c.Set(types.UserContextKey.String(), user)
		c.Set(types.UserIDContextKey.String(), user.ID)
		c.Set(types.PrincipalContextKey.String(), principal)
		c.Set(types.TenantRoleContextKey.String(), types.TenantRoleViewer)
		c.Set(types.SystemAdminContextKey.String(), false)

		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, types.TenantIDContextKey, key.TenantID)
		ctx = context.WithValue(ctx, types.TenantInfoContextKey, tenant)
		ctx = context.WithValue(ctx, types.UserContextKey, user)
		ctx = context.WithValue(ctx, types.UserIDContextKey, user.ID)
		ctx = types.WithPrincipal(ctx, principal)
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleViewer)
		ctx = context.WithValue(ctx, types.SystemAdminContextKey, false)
		ctx = types.WithAgentPublishAPIKeyContext(ctx, pubCtx)
		// Deliberately omit TenantAPIKeyScope — publish keys are not workspace keys.
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func abortOpenAPIUnauthorized(c *gin.Context, code, message string) {
	abortOpenAPIError(
		c,
		http.StatusUnauthorized,
		"authentication_error",
		code,
		message,
	)
}

func abortOpenAPIError(c *gin.Context, status int, typ, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    typ,
			"code":    code,
		},
	})
}
