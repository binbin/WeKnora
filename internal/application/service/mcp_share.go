package service

import (
	"context"
	"errors"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
)

var (
	ErrMCPShareNotFound             = errors.New("mcp service share not found")
	ErrMCPSharePermissionDenied     = errors.New("permission denied for this share operation")
	ErrMCPServiceNotFoundForShare   = errors.New("mcp service not found")
	ErrNotMCPServiceOwner           = errors.New("only mcp service owner can share")
	ErrMCPServiceBuiltinCannotShare = errors.New("builtin mcp services cannot be shared")
	// ErrOrgRoleCannotShareMCP: only editors and admins (in tenant's org role) may share MCP services to that org; viewers cannot
	ErrOrgRoleCannotShareMCP = errors.New("only editors and admins can share mcp services to this organization")
)

// mcpShareService implements MCPShareService.
//
// Mirrors kbShareService: permission resolution keys on the caller's
// tenant, and the same 3-D cap (share × tenant_org_role × tenant_role_cap)
// applies via applyTenantRoleCap (defined in kbshare.go, same package).
type mcpShareService struct {
	shareRepo interfaces.MCPShareRepository
	orgRepo   interfaces.OrganizationRepository
	mcpRepo   interfaces.MCPServiceRepository
}

// NewMCPShareService creates a new MCP service share service
func NewMCPShareService(
	shareRepo interfaces.MCPShareRepository,
	orgRepo interfaces.OrganizationRepository,
	mcpRepo interfaces.MCPServiceRepository,
) interfaces.MCPShareService {
	return &mcpShareService{
		shareRepo: shareRepo,
		orgRepo:   orgRepo,
		mcpRepo:   mcpRepo,
	}
}

// isValidMCPSharePermission restricts MCP service sharing to viewer|editor
// (unlike KB shares, admin is not a meaningful grant for a shared service).
func isValidMCPSharePermission(p types.OrgMemberRole) bool {
	return p == types.OrgRoleViewer || p == types.OrgRoleEditor
}

// ShareMCPService shares an MCP service to an organization.
// Caller must be in a tenant that owns the service *and* be a member of the
// target org with at least editor role. Builtin services cannot be shared.
func (s *mcpShareService) ShareMCPService(ctx context.Context, serviceID string, orgID string, userID string, tenantID uint64, permission types.OrgMemberRole) (*types.MCPServiceShare, error) {
	logger.Infof(ctx, "Sharing mcp service %s to organization %s", serviceID, orgID)

	mcpService, err := s.mcpRepo.GetByID(ctx, tenantID, serviceID)
	if err != nil {
		return nil, err
	}
	if mcpService == nil {
		return nil, ErrMCPServiceNotFoundForShare
	}
	if mcpService.IsBuiltin {
		return nil, ErrMCPServiceBuiltinCannotShare
	}
	if mcpService.TenantID != tenantID {
		return nil, ErrNotMCPServiceOwner
	}

	_, err = s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		if errors.Is(err, repository.ErrOrganizationNotFound) {
			return nil, ErrOrgNotFound
		}
		return nil, err
	}

	// Caller's tenant must be an org member with editor+ role to share.
	tm, err := s.orgRepo.GetTenantMember(ctx, orgID, tenantID)
	if err != nil {
		if errors.Is(err, repository.ErrOrgMemberNotFound) {
			return nil, ErrTenantNotInOrg
		}
		return nil, err
	}
	if !tm.Role.HasPermission(types.OrgRoleEditor) {
		return nil, ErrOrgRoleCannotShareMCP
	}

	if !isValidMCPSharePermission(permission) {
		return nil, ErrInvalidRole
	}

	share := &types.MCPServiceShare{
		ID:             uuid.New().String(),
		MCPServiceID:   serviceID,
		OrganizationID: orgID,
		SharedByUserID: userID,
		SourceTenantID: tenantID,
		Permission:     permission,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.shareRepo.Create(ctx, share); err != nil {
		if errors.Is(err, repository.ErrMCPShareAlreadyExists) {
			existingShare, err := s.shareRepo.GetByServiceAndOrg(ctx, serviceID, orgID)
			if err != nil {
				return nil, err
			}
			existingShare.Permission = permission
			existingShare.UpdatedAt = time.Now()
			if err := s.shareRepo.Update(ctx, existingShare); err != nil {
				return nil, err
			}
			return existingShare, nil
		}
		return nil, err
	}

	logger.Infof(ctx, "MCP service %s shared successfully to organization %s", serviceID, orgID)
	return share, nil
}

// UpdateSharePermission updates a share's permission.
// Allowed if any one of: (1) the caller is the original sharer; (2) the
// caller's tenant IS the source tenant and the caller is Admin+ in their
// tenant; (3) the caller's tenant is admin in the target org. Mirrors
// kbShareService.UpdateSharePermission's authz envelope.
func (s *mcpShareService) UpdateSharePermission(ctx context.Context, shareID string, permission types.OrgMemberRole, userID string, tenantID uint64) error {
	share, err := s.shareRepo.GetByID(ctx, shareID)
	if err != nil {
		if errors.Is(err, repository.ErrMCPShareNotFound) {
			return ErrMCPShareNotFound
		}
		return err
	}

	if !s.callerCanManageShare(ctx, share.SharedByUserID, share.SourceTenantID, share.OrganizationID, userID, tenantID) {
		return ErrMCPSharePermissionDenied
	}

	if !isValidMCPSharePermission(permission) {
		return ErrInvalidRole
	}

	share.Permission = permission
	share.UpdatedAt = time.Now()

	return s.shareRepo.Update(ctx, share)
}

// RemoveShare removes a share. Same authz envelope as UpdateSharePermission.
func (s *mcpShareService) RemoveShare(ctx context.Context, shareID string, userID string, tenantID uint64) error {
	share, err := s.shareRepo.GetByID(ctx, shareID)
	if err != nil {
		if errors.Is(err, repository.ErrMCPShareNotFound) {
			return ErrMCPShareNotFound
		}
		return err
	}

	if !s.callerCanManageShare(ctx, share.SharedByUserID, share.SourceTenantID, share.OrganizationID, userID, tenantID) {
		return ErrMCPSharePermissionDenied
	}

	return s.shareRepo.Delete(ctx, shareID)
}

// callerCanManageShare encapsulates the "who can mutate this share" rule,
// identical in shape to kbShareService.callerCanManageShare: original
// sharer, OR source-tenant Admin+, OR target-org admin.
func (s *mcpShareService) callerCanManageShare(
	ctx context.Context,
	shareSharedByUserID string,
	shareSourceTenantID uint64,
	shareOrgID string,
	callerUserID string,
	callerTenantID uint64,
) bool {
	if shareSharedByUserID == callerUserID {
		return true
	}
	if callerTenantID != 0 && callerTenantID == shareSourceTenantID {
		if types.TenantRoleFromContext(ctx).HasPermission(types.TenantRoleAdmin) {
			return true
		}
	}
	if tm, err := s.orgRepo.GetTenantMember(ctx, shareOrgID, callerTenantID); err == nil && tm.Role == types.OrgRoleAdmin {
		return true
	}
	return false
}

// ListSharesByService lists shares for an MCP service; caller's tenant must own the service.
func (s *mcpShareService) ListSharesByService(ctx context.Context, serviceID string, tenantID uint64) ([]*types.MCPServiceShare, error) {
	mcpService, err := s.mcpRepo.GetByID(ctx, tenantID, serviceID)
	if err != nil {
		return nil, err
	}
	if mcpService == nil {
		return nil, ErrMCPServiceNotFoundForShare
	}
	if mcpService.TenantID != tenantID {
		return nil, ErrNotMCPServiceOwner
	}
	return s.shareRepo.ListByService(ctx, serviceID)
}

// ListSharesByOrganization lists all MCP service shares for an organization
func (s *mcpShareService) ListSharesByOrganization(ctx context.Context, orgID string) ([]*types.MCPServiceShare, error) {
	return s.shareRepo.ListByOrganization(ctx, orgID)
}

// ListSharedMCPServices lists all MCP services reachable from the caller's
// tenant via cross-tenant org shares, excluding shares sourced from the
// caller's own tenant. Permission per service is computed via the 3-D cap.
func (s *mcpShareService) ListSharedMCPServices(ctx context.Context, tenantID uint64, callerTenantRole types.TenantRole) ([]*types.SharedMCPServiceInfo, error) {
	shares, err := s.shareRepo.ListSharedMCPServicesForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	infoMap := make(map[string]*types.SharedMCPServiceInfo)

	for _, share := range shares {
		if share.SourceTenantID == tenantID {
			continue
		}
		if share.MCPService == nil {
			continue
		}

		serviceID := share.MCPService.ID

		tm, err := s.orgRepo.GetTenantMember(ctx, share.OrganizationID, tenantID)
		if err != nil {
			continue
		}

		// 3-D cap: share × tenant_org_role × tenant_role_cap.
		effective := types.MinOrgRole(share.Permission, tm.Role)
		effective = applyTenantRoleCap(effective, callerTenantRole)

		info := &types.SharedMCPServiceInfo{
			MCPService:     share.MCPService,
			ShareID:        share.ID,
			OrganizationID: share.OrganizationID,
			OrgName:        "",
			Permission:     effective,
			SourceTenantID: share.SourceTenantID,
			SharedAt:       share.CreatedAt,
		}

		if share.Organization != nil {
			info.OrgName = share.Organization.Name
		}

		existing, exists := infoMap[serviceID]
		if !exists {
			infoMap[serviceID] = info
		} else if effective.HasPermission(existing.Permission) && effective != existing.Permission {
			infoMap[serviceID] = info
		}
	}

	result := make([]*types.SharedMCPServiceInfo, 0, len(infoMap))
	for _, info := range infoMap {
		result = append(result, info)
	}

	return result, nil
}

// GetShare gets a share by ID
func (s *mcpShareService) GetShare(ctx context.Context, shareID string) (*types.MCPServiceShare, error) {
	share, err := s.shareRepo.GetByID(ctx, shareID)
	if err != nil {
		if errors.Is(err, repository.ErrMCPShareNotFound) {
			return nil, ErrMCPShareNotFound
		}
		return nil, err
	}
	return share, nil
}

// CountByOrganizations returns share counts per organization (for list sidebar); excludes deleted MCP services
func (s *mcpShareService) CountByOrganizations(ctx context.Context, orgIDs []string) (map[string]int64, error) {
	return s.shareRepo.CountByOrganizations(ctx, orgIDs)
}
