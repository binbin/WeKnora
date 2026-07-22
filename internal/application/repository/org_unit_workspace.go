package repository

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

var ErrOrgUnitWorkspaceNotFound = errors.New("org unit workspace not found")

type orgUnitWorkspaceRepository struct {
	db *gorm.DB
}

func NewOrgUnitWorkspaceRepository(db *gorm.DB) interfaces.OrgUnitWorkspaceRepository {
	return &orgUnitWorkspaceRepository{db: db}
}

func (r *orgUnitWorkspaceRepository) GetByRootOrgUnitID(
	ctx context.Context,
	rootOrgUnitID string,
) (*types.OrgUnitWorkspace, error) {
	var binding types.OrgUnitWorkspace
	err := r.db.WithContext(ctx).
		Where("root_org_unit_id = ?", rootOrgUnitID).
		First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrgUnitWorkspaceNotFound
	}
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

func (r *orgUnitWorkspaceRepository) GetByTenantID(
	ctx context.Context,
	tenantID uint64,
) (*types.OrgUnitWorkspace, error) {
	var binding types.OrgUnitWorkspace
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrgUnitWorkspaceNotFound
	}
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

func (r *orgUnitWorkspaceRepository) Create(
	ctx context.Context,
	binding *types.OrgUnitWorkspace,
) error {
	return r.db.WithContext(ctx).Create(binding).Error
}
