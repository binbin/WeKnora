package types

import (
	"time"

	"gorm.io/gorm"
)

// OrgUnit is a tenant-scoped administrative hierarchy node
// (e.g. province → city → county). Orthogonal to Organization
// (cross-tenant SharedSpace).
type OrgUnit struct {
	ID        string         `json:"id"         gorm:"type:varchar(36);primaryKey"`
	TenantID  uint64         `json:"tenant_id"  gorm:"index"`
	ParentID  string         `json:"parent_id"  gorm:"type:varchar(36);default:''"`
	Name      string         `json:"name"       gorm:"type:varchar(255);not null"`
	Code      string         `json:"code"       gorm:"type:varchar(64);default:''"`
	Path      string         `json:"path"       gorm:"type:varchar(1024);default:''"`
	Depth     int            `json:"depth"      gorm:"default:0"`
	SortOrder int            `json:"sort_order" gorm:"default:0"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"          gorm:"index"`

	// Children is populated by tree APIs; not a DB column.
	Children []*OrgUnit `json:"children,omitempty" gorm:"-"`
}

func (OrgUnit) TableName() string { return "org_units" }

// OrgUnitMember binds a user to an OrgUnit within a tenant.
// Product rule: at most one membership per (tenant_id, user_id);
// IsPrimary is always true for new rows (legacy multi-membership
// fields remain for rollout compatibility).
type OrgUnitMember struct {
	ID        string    `json:"id"          gorm:"type:varchar(36);primaryKey"`
	OrgUnitID string    `json:"org_unit_id" gorm:"type:varchar(36);index;not null"`
	TenantID  uint64    `json:"tenant_id"   gorm:"index"`
	UserID    string    `json:"user_id"     gorm:"type:varchar(36);index;not null"`
	IsPrimary bool      `json:"is_primary"  gorm:"default:false"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// OrgUnit is optionally joined for list responses.
	OrgUnit *OrgUnit `json:"org_unit,omitempty" gorm:"foreignKey:OrgUnitID"`
}

func (OrgUnitMember) TableName() string { return "org_unit_members" }

// PlatformOrgTenantID is the catalog tenant for the global OrgUnit tree.
// Root nodes (ParentID="") created by system admins live here; each root
// binds to a real business Tenant via OrgUnitWorkspace.
const PlatformOrgTenantID uint64 = 0

// OrgUnitWorkspace binds a platform root OrgUnit to its business workspace.
type OrgUnitWorkspace struct {
	RootOrgUnitID string    `json:"root_org_unit_id" gorm:"type:varchar(36);primaryKey"`
	TenantID      uint64    `json:"tenant_id"        gorm:"uniqueIndex;not null"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (OrgUnitWorkspace) TableName() string { return "org_unit_workspaces" }

// CreateOrgUnitRequest is the body for POST /org-units.
type CreateOrgUnitRequest struct {
	Name      string `json:"name"       binding:"required,min=1,max=255"`
	Code      string `json:"code"`
	ParentID  string `json:"parent_id"`
	SortOrder int    `json:"sort_order"`
}

// UpdateOrgUnitRequest is the body for PUT /org-units/:id.
type UpdateOrgUnitRequest struct {
	Name      *string `json:"name"`
	Code      *string `json:"code"`
	SortOrder *int    `json:"sort_order"`
}

// AddOrgUnitMemberRequest is the body for POST /org-units/:id/members.
// IsPrimary is ignored by the service (new memberships are always
// primary under the single-membership model); kept for API compat.
type AddOrgUnitMemberRequest struct {
	UserID    string `json:"user_id"    binding:"required"`
	IsPrimary bool   `json:"is_primary"`
}

// TransferOrgUnitMemberRequest is the body for transferring a user
// to another OrgUnit within the same tenant.
type TransferOrgUnitMemberRequest struct {
	UserID      string `json:"user_id"        binding:"required"`
	ToOrgUnitID string `json:"to_org_unit_id" binding:"required"`
}

// OrgUnitVisibility describes which OrgUnit IDs the current caller may
// read (self + ancestors) and write (self only). Empty strings in
// ReadableIDs are never used; unbound KBs (org_unit_id="") are handled
// separately by the access layer.
type OrgUnitVisibility struct {
	CurrentID   string   `json:"current_id"`
	ReadableIDs []string `json:"readable_ids"`
	WritableID  string   `json:"writable_id"`
	// HasHierarchy is true when the tenant has at least one OrgUnit.
	// Callers use this to decide whether to fall back to legacy
	// tenant-wide KB visibility.
	HasHierarchy bool `json:"has_hierarchy"`
}
