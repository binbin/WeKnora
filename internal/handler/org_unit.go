package handler

import (
	"net/http"
	"strings"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// OrgUnitHandler exposes /org-units CRUD and membership APIs for the
// tenant-scoped administrative hierarchy.
type OrgUnitHandler struct {
	orgUnitService interfaces.OrgUnitService
}

func NewOrgUnitHandler(
	orgUnitService interfaces.OrgUnitService,
) *OrgUnitHandler {
	return &OrgUnitHandler{orgUnitService: orgUnitService}
}

func (h *OrgUnitHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	var req types.CreateOrgUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError(err.Error()))
		return
	}
	unit, err := h.orgUnitService.Create(ctx, tenantID, &req)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"tenant_id": tenantID,
		})
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": unit})
}

func (h *OrgUnitHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	id := strings.TrimSpace(c.Param("id"))
	unit, err := h.orgUnitService.Get(ctx, tenantID, id)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": unit})
}

func (h *OrgUnitHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	id := strings.TrimSpace(c.Param("id"))
	var req types.UpdateOrgUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError(err.Error()))
		return
	}
	unit, err := h.orgUnitService.Update(ctx, tenantID, id, &req)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": unit})
}

func (h *OrgUnitHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	id := strings.TrimSpace(c.Param("id"))
	if err := h.orgUnitService.Delete(ctx, tenantID, id); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *OrgUnitHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	asTree := c.Query("tree") == "1" || c.Query("tree") == "true"
	var (
		units []*types.OrgUnit
		err   error
	)
	if asTree {
		units, err = h.orgUnitService.ListTree(ctx, tenantID)
	} else {
		units, err = h.orgUnitService.ListFlat(ctx, tenantID)
	}
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": units})
}

type moveOrgUnitRequest struct {
	ParentID string `json:"parent_id"`
}

func (h *OrgUnitHandler) Move(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	id := strings.TrimSpace(c.Param("id"))
	var req moveOrgUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError(err.Error()))
		return
	}
	unit, err := h.orgUnitService.Move(ctx, tenantID, id, req.ParentID)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": unit})
}

func (h *OrgUnitHandler) AddMember(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	orgUnitID := strings.TrimSpace(c.Param("id"))
	var req types.AddOrgUnitMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError(err.Error()))
		return
	}
	member, err := h.orgUnitService.AddMember(
		ctx, tenantID, orgUnitID, req.UserID, req.IsPrimary,
	)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": member})
}

func (h *OrgUnitHandler) RemoveMember(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	orgUnitID := strings.TrimSpace(c.Param("id"))
	userID := strings.TrimSpace(c.Param("user_id"))
	if err := h.orgUnitService.RemoveMember(ctx, tenantID, orgUnitID, userID); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *OrgUnitHandler) ListMembers(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	orgUnitID := strings.TrimSpace(c.Param("id"))
	members, err := h.orgUnitService.ListMembers(ctx, tenantID, orgUnitID)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": members})
}

func (h *OrgUnitHandler) ListMyMemberships(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	userID := c.GetString(types.UserIDContextKey.String())
	members, err := h.orgUnitService.ListUserMemberships(ctx, tenantID, userID)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": members})
}

func (h *OrgUnitHandler) SetPrimary(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	userID := c.GetString(types.UserIDContextKey.String())
	orgUnitID := strings.TrimSpace(c.Param("id"))
	if err := h.orgUnitService.SetPrimary(ctx, tenantID, userID, orgUnitID); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *OrgUnitHandler) GetVisibility(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	orgUnitID, _ := types.OrgUnitIDFromContext(ctx)
	vis, err := h.orgUnitService.ResolveVisibility(ctx, tenantID, orgUnitID)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": vis})
}

// ListInviteable returns OrgUnits the current actor may assign on an
// invitation for the given tenant role (peer/self + descendants;
// Owner → descendants only).
func (h *OrgUnitHandler) ListInviteable(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	actorOrgUnitID, _ := types.OrgUnitIDFromContext(ctx)
	role := types.TenantRole(strings.TrimSpace(c.Query("role")))
	if role == "" {
		role = types.TenantRoleContributor
	}
	if !role.IsValid() {
		c.Error(apperrors.NewValidationError("role must be one of owner/admin/contributor/viewer"))
		return
	}
	units, err := h.orgUnitService.ListInviteableOrgUnits(
		ctx, tenantID, actorOrgUnitID, role,
	)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": units})
}
