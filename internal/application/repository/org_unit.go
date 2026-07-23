package repository

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

var (
	ErrOrgUnitNotFound       = errors.New("org unit not found")
	ErrOrgUnitMemberNotFound = errors.New("org unit member not found")
)

type orgUnitRepository struct {
	db *gorm.DB
}

func NewOrgUnitRepository(db *gorm.DB) interfaces.OrgUnitRepository {
	return &orgUnitRepository{db: db}
}

func (r *orgUnitRepository) Create(ctx context.Context, unit *types.OrgUnit) error {
	return r.db.WithContext(ctx).Create(unit).Error
}

func (r *orgUnitRepository) GetByID(
	ctx context.Context,
	tenantID uint64,
	id string,
) (*types.OrgUnit, error) {
	var unit types.OrgUnit
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&unit).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrgUnitNotFound
	}
	if err != nil {
		return nil, err
	}
	return &unit, nil
}

func (r *orgUnitRepository) GetByIDGlobal(
	ctx context.Context,
	id string,
) (*types.OrgUnit, error) {
	var unit types.OrgUnit
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&unit).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrgUnitNotFound
	}
	if err != nil {
		return nil, err
	}
	return &unit, nil
}

func (r *orgUnitRepository) Update(ctx context.Context, unit *types.OrgUnit) error {
	return r.db.WithContext(ctx).Save(unit).Error
}

func (r *orgUnitRepository) Delete(
	ctx context.Context,
	tenantID uint64,
	id string,
) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&types.OrgUnit{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrOrgUnitNotFound
	}
	return nil
}

func (r *orgUnitRepository) ListByTenant(
	ctx context.Context,
	tenantID uint64,
) ([]*types.OrgUnit, error) {
	var units []*types.OrgUnit
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("depth ASC, sort_order ASC, name ASC").
		Find(&units).Error
	return units, err
}

func (r *orgUnitRepository) ListRoots(
	ctx context.Context,
	tenantID uint64,
) ([]*types.OrgUnit, error) {
	var units []*types.OrgUnit
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND parent_id = ?", tenantID, "").
		Order("created_at ASC, sort_order ASC, name ASC").
		Find(&units).Error
	return units, err
}

func (r *orgUnitRepository) CountByTenant(
	ctx context.Context,
	tenantID uint64,
) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&types.OrgUnit{}).
		Where("tenant_id = ?", tenantID).
		Count(&count).Error
	return count, err
}

func (r *orgUnitRepository) CountChildren(
	ctx context.Context,
	tenantID uint64,
	parentID string,
) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&types.OrgUnit{}).
		Where("tenant_id = ? AND parent_id = ?", tenantID, parentID).
		Count(&count).Error
	return count, err
}

func (r *orgUnitRepository) ListByPathPrefix(
	ctx context.Context,
	tenantID uint64,
	pathPrefix string,
) ([]*types.OrgUnit, error) {
	var units []*types.OrgUnit
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND path LIKE ?", tenantID, pathPrefix+"%").
		Order("depth ASC").
		Find(&units).Error
	return units, err
}

func (r *orgUnitRepository) UpdateSubtreePaths(
	ctx context.Context,
	tenantID uint64,
	oldPrefix string,
	newPrefix string,
	depthDelta int,
) error {
	// Portable rewrite: load matching rows and update in Go so Postgres
	// and SQLite share one code path.
	units, err := r.ListByPathPrefix(ctx, tenantID, oldPrefix)
	if err != nil {
		return err
	}
	for _, unit := range units {
		if unit == nil {
			continue
		}
		if len(unit.Path) < len(oldPrefix) || unit.Path[:len(oldPrefix)] != oldPrefix {
			continue
		}
		unit.Path = newPrefix + unit.Path[len(oldPrefix):]
		unit.Depth += depthDelta
		if err := r.db.WithContext(ctx).
			Model(unit).
			Select("path", "depth", "updated_at").
			Updates(unit).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *orgUnitRepository) AddMember(
	ctx context.Context,
	member *types.OrgUnitMember,
) error {
	return r.db.WithContext(ctx).Create(member).Error
}

func (r *orgUnitRepository) RemoveMember(
	ctx context.Context,
	orgUnitID string,
	userID string,
) error {
	result := r.db.WithContext(ctx).
		Where("org_unit_id = ? AND user_id = ?", orgUnitID, userID).
		Delete(&types.OrgUnitMember{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrOrgUnitMemberNotFound
	}
	return nil
}

func (r *orgUnitRepository) ListMembers(
	ctx context.Context,
	orgUnitID string,
) ([]*types.OrgUnitMember, error) {
	var members []*types.OrgUnitMember
	err := r.db.WithContext(ctx).
		Where("org_unit_id = ?", orgUnitID).
		Order("created_at ASC").
		Find(&members).Error
	return members, err
}

func (r *orgUnitRepository) ListMembersByOrgUnitIDs(
	ctx context.Context,
	orgUnitIDs []string,
) ([]*types.OrgUnitMember, error) {
	if len(orgUnitIDs) == 0 {
		return nil, nil
	}
	var members []*types.OrgUnitMember
	err := r.db.WithContext(ctx).
		Where("org_unit_id IN ?", orgUnitIDs).
		Order("created_at ASC").
		Find(&members).Error
	return members, err
}

func (r *orgUnitRepository) ListUserMemberships(
	ctx context.Context,
	tenantID uint64,
	userID string,
) ([]*types.OrgUnitMember, error) {
	var members []*types.OrgUnitMember
	err := r.db.WithContext(ctx).
		Preload("OrgUnit").
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Order("is_primary DESC, created_at ASC").
		Find(&members).Error
	return members, err
}

func (r *orgUnitRepository) ListUserMembershipsByUser(
	ctx context.Context,
	userID string,
) ([]*types.OrgUnitMember, error) {
	var members []*types.OrgUnitMember
	err := r.db.WithContext(ctx).
		Preload("OrgUnit").
		Where("user_id = ?", userID).
		Order("is_primary DESC, created_at ASC").
		Find(&members).Error
	return members, err
}

func (r *orgUnitRepository) GetMember(
	ctx context.Context,
	orgUnitID string,
	userID string,
) (*types.OrgUnitMember, error) {
	var member types.OrgUnitMember
	err := r.db.WithContext(ctx).
		Where("org_unit_id = ? AND user_id = ?", orgUnitID, userID).
		First(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrgUnitMemberNotFound
	}
	if err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *orgUnitRepository) ClearPrimary(
	ctx context.Context,
	tenantID uint64,
	userID string,
) error {
	return r.db.WithContext(ctx).
		Model(&types.OrgUnitMember{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Update("is_primary", false).Error
}

func (r *orgUnitRepository) SetPrimary(
	ctx context.Context,
	tenantID uint64,
	userID string,
	orgUnitID string,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&types.OrgUnitMember{}).
			Where("tenant_id = ? AND user_id = ?", tenantID, userID).
			Update("is_primary", false).Error; err != nil {
			return err
		}
		result := tx.Model(&types.OrgUnitMember{}).
			Where(
				"tenant_id = ? AND user_id = ? AND org_unit_id = ?",
				tenantID, userID, orgUnitID,
			).
			Update("is_primary", true)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrOrgUnitMemberNotFound
		}
		return nil
	})
}

func (r *orgUnitRepository) RemoveMembersByTenantUser(
	ctx context.Context,
	tenantID uint64,
	userID string,
) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Delete(&types.OrgUnitMember{}).Error
}

func (r *orgUnitRepository) TransferMember(
	ctx context.Context,
	member *types.OrgUnitMember,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("tenant_id = ? AND user_id = ?", member.TenantID, member.UserID).
			Delete(&types.OrgUnitMember{}).Error; err != nil {
			return err
		}
		return tx.Create(member).Error
	})
}
