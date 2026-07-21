package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// OrgUnitService manages the tenant-scoped administrative hierarchy and
// resolves ancestor-readable / self-writable visibility for knowledge.
type OrgUnitService interface {
	Create(ctx context.Context, tenantID uint64, req *types.CreateOrgUnitRequest) (*types.OrgUnit, error)
	Get(ctx context.Context, tenantID uint64, id string) (*types.OrgUnit, error)
	Update(ctx context.Context, tenantID uint64, id string, req *types.UpdateOrgUnitRequest) (*types.OrgUnit, error)
	Delete(ctx context.Context, tenantID uint64, id string) error
	ListFlat(ctx context.Context, tenantID uint64) ([]*types.OrgUnit, error)
	ListTree(ctx context.Context, tenantID uint64) ([]*types.OrgUnit, error)
	Move(ctx context.Context, tenantID uint64, id string, newParentID string) (*types.OrgUnit, error)

	AddMember(ctx context.Context, tenantID uint64, orgUnitID string, userID string, isPrimary bool) (*types.OrgUnitMember, error)
	RemoveMember(ctx context.Context, tenantID uint64, orgUnitID string, userID string) error
	ListMembers(ctx context.Context, tenantID uint64, orgUnitID string) ([]*types.OrgUnitMember, error)
	ListUserMemberships(ctx context.Context, tenantID uint64, userID string) ([]*types.OrgUnitMember, error)
	SetPrimary(ctx context.Context, tenantID uint64, userID string, orgUnitID string) error

	// ResolveActiveOrgUnit picks the caller's active unit from an
	// explicit header value or primary membership. Returns "" when the
	// tenant has no hierarchy or the user has no membership (legacy mode).
	ResolveActiveOrgUnit(ctx context.Context, tenantID uint64, userID string, requestedID string) (string, error)

	// ResolveVisibility returns readable (self+ancestors) and writable
	// (self) OrgUnit IDs for the active unit.
	ResolveVisibility(ctx context.Context, tenantID uint64, orgUnitID string) (*types.OrgUnitVisibility, error)

	// CanReadKB reports whether the active OrgUnit may read a KB bound
	// to kbOrgUnitID (empty kbOrgUnitID = unbound = always readable
	// within the tenant when hierarchy is inactive or as shared root).
	CanReadKB(ctx context.Context, tenantID uint64, activeOrgUnitID string, kbOrgUnitID string) (bool, error)

	// CanWriteKB reports whether the active OrgUnit may mutate a KB.
	CanWriteKB(ctx context.Context, tenantID uint64, activeOrgUnitID string, kbOrgUnitID string) (bool, error)

	// ListAncestorIDs returns [self, parent, ..., root] for path closure.
	ListAncestorIDs(ctx context.Context, tenantID uint64, orgUnitID string) ([]string, error)

	// HasHierarchy is true when the tenant has at least one live OrgUnit.
	HasHierarchy(ctx context.Context, tenantID uint64) (bool, error)

	// ListInviteableOrgUnits returns units the actor may assign when
	// inviting a member: own unit + siblings (平级) + descendants (下级).
	// When role is Owner, only descendants are returned.
	ListInviteableOrgUnits(
		ctx context.Context,
		tenantID uint64,
		actorOrgUnitID string,
		role types.TenantRole,
	) ([]*types.OrgUnit, error)

	// CanInviteToOrgUnit reports whether targetOrgUnitID is within the
	// inviteable scope for actorOrgUnitID and the granted tenant role.
	CanInviteToOrgUnit(
		ctx context.Context,
		tenantID uint64,
		actorOrgUnitID string,
		targetOrgUnitID string,
		role types.TenantRole,
	) (bool, error)
}

// OrgUnitRepository persists OrgUnits and memberships.
type OrgUnitRepository interface {
	Create(ctx context.Context, unit *types.OrgUnit) error
	GetByID(ctx context.Context, tenantID uint64, id string) (*types.OrgUnit, error)
	Update(ctx context.Context, unit *types.OrgUnit) error
	Delete(ctx context.Context, tenantID uint64, id string) error
	ListByTenant(ctx context.Context, tenantID uint64) ([]*types.OrgUnit, error)
	CountByTenant(ctx context.Context, tenantID uint64) (int64, error)
	CountChildren(ctx context.Context, tenantID uint64, parentID string) (int64, error)
	ListByPathPrefix(ctx context.Context, tenantID uint64, pathPrefix string) ([]*types.OrgUnit, error)
	UpdateSubtreePaths(ctx context.Context, tenantID uint64, oldPrefix string, newPrefix string, depthDelta int) error

	AddMember(ctx context.Context, member *types.OrgUnitMember) error
	RemoveMember(ctx context.Context, orgUnitID string, userID string) error
	ListMembers(ctx context.Context, orgUnitID string) ([]*types.OrgUnitMember, error)
	ListUserMemberships(ctx context.Context, tenantID uint64, userID string) ([]*types.OrgUnitMember, error)
	GetMember(ctx context.Context, orgUnitID string, userID string) (*types.OrgUnitMember, error)
	ClearPrimary(ctx context.Context, tenantID uint64, userID string) error
	SetPrimary(ctx context.Context, tenantID uint64, userID string, orgUnitID string) error
}
