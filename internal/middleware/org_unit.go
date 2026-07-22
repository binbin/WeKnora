package middleware

import (
	"strings"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

const (
	// OrgUnitServiceContextKey stashes the OrgUnitService on gin.Context
	// so kb_access can enforce hierarchy without changing RequireKBAccess
	// call signatures.
	OrgUnitServiceContextKey = "org_unit.service"
	// HeaderOrgUnitID is the request header for the active OrgUnit.
	HeaderOrgUnitID = "X-Org-Unit-ID"
)

// OrgUnitServiceProvider injects OrgUnitService into gin.Context.
func OrgUnitServiceProvider(svc interfaces.OrgUnitService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if svc != nil {
			c.Set(OrgUnitServiceContextKey, svc)
		}
		c.Next()
	}
}

// ResolveOrgUnit resolves the caller's active OrgUnit from X-Org-Unit-ID
// or primary membership and writes it into request context.
// No-op when the service is nil or the tenant has no hierarchy.
func ResolveOrgUnit(svc interfaces.OrgUnitService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if svc == nil {
			c.Next()
			return
		}
		ctx := c.Request.Context()
		tenantID, ok := types.TenantIDFromContext(ctx)
		if !ok || tenantID == 0 {
			c.Next()
			return
		}
		userID, _ := types.UserIDFromContext(ctx)
		requested := strings.TrimSpace(c.GetHeader(HeaderOrgUnitID))
		orgUnitID, err := svc.ResolveActiveOrgUnit(ctx, tenantID, userID, requested)
		if err != nil {
			logger.Warnf(ctx, "[org_unit] resolve failed: %v", err)
			_ = c.Error(err)
			c.Abort()
			return
		}
		if orgUnitID != "" {
			c.Set(types.OrgUnitIDContextKey.String(), orgUnitID)
			ctx = types.WithOrgUnitID(ctx, orgUnitID)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	}
}

func orgUnitServiceFromGin(c *gin.Context) interfaces.OrgUnitService {
	if c == nil {
		return nil
	}
	v, ok := c.Get(OrgUnitServiceContextKey)
	if !ok || v == nil {
		return nil
	}
	svc, _ := v.(interfaces.OrgUnitService)
	return svc
}

// enforceOrgUnitKBAccess applies ancestor-read / self-write rules for
// own-tenant KBs. Returns nil when access is allowed, or a sentinel
// error when denied. Shared-space / shared-agent paths skip this.
func enforceOrgUnitKBAccess(
	c *gin.Context,
	kb *types.KnowledgeBase,
	requiredPermission types.OrgMemberRole,
) error {
	svc := orgUnitServiceFromGin(c)
	if svc == nil || kb == nil {
		return nil
	}
	ctx := c.Request.Context()
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 || kb.TenantID != tenantID {
		return nil
	}
	activeID, _ := types.OrgUnitIDFromContext(ctx)
	kbUnitID := strings.TrimSpace(kb.OrgUnitID)

	if requiredPermission == types.OrgRoleViewer {
		okRead, err := svc.CanReadKB(
			ctx, tenantID, activeID, kbUnitID, kb.ShareWithDescendants,
		)
		if err != nil {
			return err
		}
		if !okRead {
			logger.Warnf(ctx,
				"[org_unit] read denied: active=%s kb_unit=%s kb=%s",
				activeID, kbUnitID, kb.ID)
			return errKBAccessForbidden
		}
		return nil
	}

	okWrite, err := svc.CanWriteKB(ctx, tenantID, activeID, kbUnitID)
	if err != nil {
		return err
	}
	if !okWrite {
		logger.Warnf(ctx,
			"[org_unit] write denied: active=%s kb_unit=%s kb=%s",
			activeID, kbUnitID, kb.ID)
		return errKBAccessForbidden
	}
	return nil
}
