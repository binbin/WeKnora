package repository

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

var (
	ErrMCPShareNotFound      = errors.New("mcp service share not found")
	ErrMCPShareAlreadyExists = errors.New("mcp service already shared to this organization")
)

// mcpShareRepository implements MCPShareRepository interface
type mcpShareRepository struct {
	db *gorm.DB
}

// NewMCPShareRepository creates a new MCP service share repository
func NewMCPShareRepository(db *gorm.DB) interfaces.MCPShareRepository {
	return &mcpShareRepository{db: db}
}

// Create creates a new share record
func (r *mcpShareRepository) Create(ctx context.Context, share *types.MCPServiceShare) error {
	var count int64
	r.db.WithContext(ctx).Model(&types.MCPServiceShare{}).
		Where("mcp_service_id = ? AND organization_id = ? AND deleted_at IS NULL",
			share.MCPServiceID, share.OrganizationID).
		Count(&count)

	if count > 0 {
		return ErrMCPShareAlreadyExists
	}

	return r.db.WithContext(ctx).Create(share).Error
}

// GetByID gets a share record by ID
func (r *mcpShareRepository) GetByID(ctx context.Context, id string) (*types.MCPServiceShare, error) {
	var share types.MCPServiceShare
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&share).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMCPShareNotFound
		}
		return nil, err
	}
	return &share, nil
}

// GetByServiceAndOrg gets a share record by MCP service ID and organization ID
func (r *mcpShareRepository) GetByServiceAndOrg(ctx context.Context, serviceID string, orgID string) (*types.MCPServiceShare, error) {
	var share types.MCPServiceShare
	err := r.db.WithContext(ctx).
		Where("mcp_service_id = ? AND organization_id = ?", serviceID, orgID).
		First(&share).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMCPShareNotFound
		}
		return nil, err
	}
	return &share, nil
}

// Update updates a share record
func (r *mcpShareRepository) Update(ctx context.Context, share *types.MCPServiceShare) error {
	return r.db.WithContext(ctx).Model(&types.MCPServiceShare{}).
		Where("id = ?", share.ID).
		Updates(share).Error
}

// Delete soft deletes a share record
func (r *mcpShareRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&types.MCPServiceShare{}).Error
}

// DeleteByServiceID soft deletes all share records for an MCP service (e.g. when the service is deleted)
func (r *mcpShareRepository) DeleteByServiceID(ctx context.Context, serviceID string) error {
	return r.db.WithContext(ctx).Where("mcp_service_id = ?", serviceID).Delete(&types.MCPServiceShare{}).Error
}

// DeleteByOrganizationID soft deletes all share records for an organization (e.g. when the org is deleted)
func (r *mcpShareRepository) DeleteByOrganizationID(ctx context.Context, orgID string) error {
	return r.db.WithContext(ctx).Where("organization_id = ?", orgID).Delete(&types.MCPServiceShare{}).Error
}

// ListByService lists all share records for an MCP service
func (r *mcpShareRepository) ListByService(ctx context.Context, serviceID string) ([]*types.MCPServiceShare, error) {
	var shares []*types.MCPServiceShare
	err := r.db.WithContext(ctx).
		Preload("Organization").
		Where("mcp_service_id = ?", serviceID).
		Order("created_at DESC").
		Find(&shares).Error

	if err != nil {
		return nil, err
	}
	return shares, nil
}

// ListByOrganization lists all share records for an organization.
// Excludes shares whose MCP service has been soft-deleted.
func (r *mcpShareRepository) ListByOrganization(ctx context.Context, orgID string) ([]*types.MCPServiceShare, error) {
	var shares []*types.MCPServiceShare
	err := r.db.WithContext(ctx).
		Joins("JOIN mcp_services ON mcp_services.id = mcp_shares.mcp_service_id AND mcp_services.deleted_at IS NULL").
		Preload("MCPService").
		Preload("Organization").
		Where("mcp_shares.organization_id = ? AND mcp_shares.deleted_at IS NULL", orgID).
		Order("mcp_shares.created_at DESC").
		Find(&shares).Error

	if err != nil {
		return nil, err
	}
	return shares, nil
}

// ListByOrganizations lists all share records for the given organizations (batch).
func (r *mcpShareRepository) ListByOrganizations(ctx context.Context, orgIDs []string) ([]*types.MCPServiceShare, error) {
	if len(orgIDs) == 0 {
		return nil, nil
	}
	var shares []*types.MCPServiceShare
	err := r.db.WithContext(ctx).
		Joins("JOIN mcp_services ON mcp_services.id = mcp_shares.mcp_service_id AND mcp_services.deleted_at IS NULL").
		Preload("MCPService").
		Preload("Organization").
		Where("mcp_shares.organization_id IN ? AND mcp_shares.deleted_at IS NULL", orgIDs).
		Order("mcp_shares.created_at DESC").
		Find(&shares).Error
	if err != nil {
		return nil, err
	}
	return shares, nil
}

// ListSharedMCPServicesForTenant lists all MCP services shared to organizations
// that the given tenant participates in. Excludes shares for soft-deleted
// organizations and soft-deleted MCP services.
func (r *mcpShareRepository) ListSharedMCPServicesForTenant(ctx context.Context, tenantID uint64) ([]*types.MCPServiceShare, error) {
	var shares []*types.MCPServiceShare

	err := r.db.WithContext(ctx).
		Joins("JOIN mcp_services ON mcp_services.id = mcp_shares.mcp_service_id AND mcp_services.deleted_at IS NULL").
		Preload("MCPService").
		Preload("Organization").
		Joins("JOIN organization_tenant_members otm ON otm.organization_id = mcp_shares.organization_id").
		Joins("JOIN organizations ON organizations.id = mcp_shares.organization_id AND organizations.deleted_at IS NULL").
		Where("otm.tenant_id = ?", tenantID).
		Where("mcp_shares.deleted_at IS NULL").
		Order("mcp_shares.created_at DESC").
		Find(&shares).Error

	if err != nil {
		return nil, err
	}
	return shares, nil
}

// CountByOrganizations returns share counts per organization (only orgs in orgIDs). Excludes deleted MCP services.
func (r *mcpShareRepository) CountByOrganizations(ctx context.Context, orgIDs []string) (map[string]int64, error) {
	if len(orgIDs) == 0 {
		return make(map[string]int64), nil
	}
	type row struct {
		OrgID string `gorm:"column:organization_id"`
		Count int64  `gorm:"column:count"`
	}
	var rows []row
	err := r.db.WithContext(ctx).Model(&types.MCPServiceShare{}).
		Joins("JOIN mcp_services ON mcp_services.id = mcp_shares.mcp_service_id AND mcp_services.deleted_at IS NULL").
		Select("mcp_shares.organization_id as organization_id, COUNT(*) as count").
		Where("mcp_shares.organization_id IN ? AND mcp_shares.deleted_at IS NULL", orgIDs).
		Group("mcp_shares.organization_id").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64)
	for _, o := range orgIDs {
		out[o] = 0
	}
	for _, r := range rows {
		out[r.OrgID] = r.Count
	}
	return out, nil
}
