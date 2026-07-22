package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type orgUnitService struct {
	repo interfaces.OrgUnitRepository
}

func NewOrgUnitService(repo interfaces.OrgUnitRepository) interfaces.OrgUnitService {
	return &orgUnitService{repo: repo}
}

func (s *orgUnitService) Create(
	ctx context.Context,
	tenantID uint64,
	req *types.CreateOrgUnitRequest,
) (*types.OrgUnit, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, apperrors.NewValidationError("name is required")
	}
	parentID := strings.TrimSpace(req.ParentID)
	// Top-level (parent empty) org units are reserved for platform
	// system admins — tenant Owner/Admin may only add child nodes.
	if parentID == "" && !types.IsSystemAdminActor(ctx) {
		return nil, apperrors.NewForbiddenError(
			"only system admin can create a top-level organization",
		)
	}

	depth := 0
	pathPrefix := ""
	if parentID == "" && types.IsSystemAdminActor(ctx) {
		// Platform catalog roots always use tenant_id=0.
		tenantID = types.PlatformOrgTenantID
	} else if parentID != "" {
		parent, err := s.repo.GetByID(ctx, tenantID, parentID)
		if err != nil && types.IsSystemAdminActor(ctx) {
			parent, err = s.repo.GetByIDGlobal(ctx, parentID)
		}
		if err != nil {
			if errors.Is(err, apprepo.ErrOrgUnitNotFound) {
				return nil, apperrors.NewNotFoundError("parent org unit not found")
			}
			return nil, err
		}
		tenantID = parent.TenantID
		depth = parent.Depth + 1
		pathPrefix = parent.Path
	}

	id := uuid.New().String()
	now := time.Now()
	path := "/" + id + "/"
	if parentID != "" {
		path = pathPrefix + id + "/"
	}
	unit := &types.OrgUnit{
		ID:        id,
		TenantID:  tenantID,
		ParentID:  parentID,
		Name:      name,
		Code:      strings.TrimSpace(req.Code),
		Path:      path,
		Depth:     depth,
		SortOrder: req.SortOrder,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(ctx, unit); err != nil {
		return nil, fmt.Errorf("create org unit: %w", err)
	}
	return unit, nil
}

func (s *orgUnitService) Get(
	ctx context.Context,
	tenantID uint64,
	id string,
) (*types.OrgUnit, error) {
	unit, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		if errors.Is(err, apprepo.ErrOrgUnitNotFound) {
			return nil, apperrors.NewNotFoundError("org unit not found")
		}
		return nil, err
	}
	return unit, nil
}

func (s *orgUnitService) Update(
	ctx context.Context,
	tenantID uint64,
	id string,
	req *types.UpdateOrgUnitRequest,
) (*types.OrgUnit, error) {
	unit, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		if errors.Is(err, apprepo.ErrOrgUnitNotFound) {
			return nil, apperrors.NewNotFoundError("org unit not found")
		}
		return nil, err
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, apperrors.NewValidationError("name cannot be empty")
		}
		unit.Name = name
	}
	if req.Code != nil {
		unit.Code = strings.TrimSpace(*req.Code)
	}
	if req.SortOrder != nil {
		unit.SortOrder = *req.SortOrder
	}
	unit.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, unit); err != nil {
		return nil, err
	}
	return unit, nil
}

func (s *orgUnitService) Delete(
	ctx context.Context,
	tenantID uint64,
	id string,
) error {
	childCount, err := s.repo.CountChildren(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if childCount > 0 {
		return apperrors.NewBadRequestError(
			"cannot delete org unit with children; move or delete children first",
		)
	}
	if err := s.repo.Delete(ctx, tenantID, id); err != nil {
		if errors.Is(err, apprepo.ErrOrgUnitNotFound) {
			return apperrors.NewNotFoundError("org unit not found")
		}
		return err
	}
	return nil
}

func (s *orgUnitService) ListFlat(
	ctx context.Context,
	tenantID uint64,
) ([]*types.OrgUnit, error) {
	return s.repo.ListByTenant(ctx, tenantID)
}

func (s *orgUnitService) ListTree(
	ctx context.Context,
	tenantID uint64,
) ([]*types.OrgUnit, error) {
	units, err := s.repo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return buildOrgUnitTree(units), nil
}

func buildOrgUnitTree(units []*types.OrgUnit) []*types.OrgUnit {
	byID := make(map[string]*types.OrgUnit, len(units))
	for _, unit := range units {
		if unit == nil {
			continue
		}
		unit.Children = nil
		byID[unit.ID] = unit
	}
	roots := make([]*types.OrgUnit, 0)
	for _, unit := range units {
		if unit == nil {
			continue
		}
		if unit.ParentID == "" {
			roots = append(roots, unit)
			continue
		}
		parent, ok := byID[unit.ParentID]
		if !ok {
			roots = append(roots, unit)
			continue
		}
		parent.Children = append(parent.Children, unit)
	}
	return roots
}

func (s *orgUnitService) Move(
	ctx context.Context,
	tenantID uint64,
	id string,
	newParentID string,
) (*types.OrgUnit, error) {
	unit, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		if errors.Is(err, apprepo.ErrOrgUnitNotFound) {
			return nil, apperrors.NewNotFoundError("org unit not found")
		}
		return nil, err
	}
	newParentID = strings.TrimSpace(newParentID)
	if newParentID == unit.ID {
		return nil, apperrors.NewBadRequestError("cannot move org unit under itself")
	}
	// Promoting a node to top-level (empty parent) is the same privilege
	// boundary as creating a root org unit.
	if newParentID == "" && unit.ParentID != "" && !types.IsSystemAdminActor(ctx) {
		return nil, apperrors.NewForbiddenError(
			"only system admin can move an organization to the top level",
		)
	}

	newDepth := 0
	newPathPrefix := "/"
	if newParentID != "" {
		parent, parentErr := s.repo.GetByID(ctx, tenantID, newParentID)
		if parentErr != nil {
			if errors.Is(parentErr, apprepo.ErrOrgUnitNotFound) {
				return nil, apperrors.NewNotFoundError("parent org unit not found")
			}
			return nil, parentErr
		}
		// Prevent cycles: new parent must not be under the moved node.
		if strings.HasPrefix(parent.Path, unit.Path) {
			return nil, apperrors.NewBadRequestError(
				"cannot move org unit under its descendant",
			)
		}
		newDepth = parent.Depth + 1
		newPathPrefix = parent.Path
	}

	oldPath := unit.Path
	newPath := newPathPrefix + unit.ID + "/"
	depthDelta := newDepth - unit.Depth

	unit.ParentID = newParentID
	unit.Path = newPath
	unit.Depth = newDepth
	unit.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, unit); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateSubtreePaths(
		ctx, tenantID, oldPath, newPath, depthDelta,
	); err != nil {
		return nil, err
	}
	return unit, nil
}

func (s *orgUnitService) AddMember(
	ctx context.Context,
	tenantID uint64,
	orgUnitID string,
	userID string,
	isPrimary bool,
) (*types.OrgUnitMember, error) {
	if _, err := s.repo.GetByID(ctx, tenantID, orgUnitID); err != nil {
		if errors.Is(err, apprepo.ErrOrgUnitNotFound) {
			return nil, apperrors.NewNotFoundError("org unit not found")
		}
		return nil, err
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, apperrors.NewValidationError("user_id is required")
	}
	if existing, err := s.repo.GetMember(ctx, orgUnitID, userID); err == nil && existing != nil {
		return existing, nil
	} else if err != nil && !errors.Is(err, apprepo.ErrOrgUnitMemberNotFound) {
		return nil, err
	}

	now := time.Now()
	member := &types.OrgUnitMember{
		ID:        uuid.New().String(),
		OrgUnitID: orgUnitID,
		TenantID:  tenantID,
		UserID:    userID,
		IsPrimary: isPrimary,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if isPrimary {
		if err := s.repo.ClearPrimary(ctx, tenantID, userID); err != nil {
			return nil, err
		}
	}
	if err := s.repo.AddMember(ctx, member); err != nil {
		return nil, err
	}
	return member, nil
}

func (s *orgUnitService) RemoveMember(
	ctx context.Context,
	tenantID uint64,
	orgUnitID string,
	userID string,
) error {
	if _, err := s.repo.GetByID(ctx, tenantID, orgUnitID); err != nil {
		if errors.Is(err, apprepo.ErrOrgUnitNotFound) {
			return apperrors.NewNotFoundError("org unit not found")
		}
		return err
	}
	if err := s.repo.RemoveMember(ctx, orgUnitID, userID); err != nil {
		if errors.Is(err, apprepo.ErrOrgUnitMemberNotFound) {
			return apperrors.NewNotFoundError("org unit member not found")
		}
		return err
	}
	return nil
}

func (s *orgUnitService) ListMembers(
	ctx context.Context,
	tenantID uint64,
	orgUnitID string,
) ([]*types.OrgUnitMember, error) {
	if _, err := s.repo.GetByID(ctx, tenantID, orgUnitID); err != nil {
		if errors.Is(err, apprepo.ErrOrgUnitNotFound) {
			return nil, apperrors.NewNotFoundError("org unit not found")
		}
		return nil, err
	}
	return s.repo.ListMembers(ctx, orgUnitID)
}

func (s *orgUnitService) ListUserMemberships(
	ctx context.Context,
	tenantID uint64,
	userID string,
) ([]*types.OrgUnitMember, error) {
	return s.repo.ListUserMemberships(ctx, tenantID, userID)
}

func (s *orgUnitService) SetPrimary(
	ctx context.Context,
	tenantID uint64,
	userID string,
	orgUnitID string,
) error {
	if _, err := s.repo.GetByID(ctx, tenantID, orgUnitID); err != nil {
		if errors.Is(err, apprepo.ErrOrgUnitNotFound) {
			return apperrors.NewNotFoundError("org unit not found")
		}
		return err
	}
	if err := s.repo.SetPrimary(ctx, tenantID, userID, orgUnitID); err != nil {
		if errors.Is(err, apprepo.ErrOrgUnitMemberNotFound) {
			return apperrors.NewNotFoundError("org unit member not found")
		}
		return err
	}
	return nil
}

func (s *orgUnitService) ResolveActiveOrgUnit(
	ctx context.Context,
	tenantID uint64,
	userID string,
	requestedID string,
) (string, error) {
	has, err := s.HasHierarchy(ctx, tenantID)
	if err != nil {
		return "", err
	}
	if !has {
		return "", nil
	}

	requestedID = strings.TrimSpace(requestedID)
	if requestedID != "" {
		if _, err := s.repo.GetByID(ctx, tenantID, requestedID); err != nil {
			if errors.Is(err, apprepo.ErrOrgUnitNotFound) {
				return "", apperrors.NewBadRequestError("invalid X-Org-Unit-ID")
			}
			return "", err
		}
		// Tenant Admins/Owners may switch to any unit; members may only
		// use units they belong to. Role is checked by caller middleware
		// via membership when userID is set and not synthetic.
		if userID != "" && !types.IsSyntheticUserID(userID) {
			role := types.TenantRoleFromContext(ctx)
			if role != types.TenantRoleAdmin && role != types.TenantRoleOwner {
				if _, memberErr := s.repo.GetMember(ctx, requestedID, userID); memberErr != nil {
					if errors.Is(memberErr, apprepo.ErrOrgUnitMemberNotFound) {
						return "", apperrors.NewForbiddenError(
							"not a member of the requested org unit",
						)
					}
					return "", memberErr
				}
			}
		}
		return requestedID, nil
	}

	if userID == "" || types.IsSyntheticUserID(userID) {
		return "", nil
	}
	memberships, err := s.repo.ListUserMemberships(ctx, tenantID, userID)
	if err != nil {
		return "", err
	}
	for _, membership := range memberships {
		if membership != nil && membership.IsPrimary {
			return membership.OrgUnitID, nil
		}
	}
	if len(memberships) > 0 && memberships[0] != nil {
		return memberships[0].OrgUnitID, nil
	}
	return "", nil
}

func (s *orgUnitService) ResolveVisibility(
	ctx context.Context,
	tenantID uint64,
	orgUnitID string,
) (*types.OrgUnitVisibility, error) {
	has, err := s.HasHierarchy(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	vis := &types.OrgUnitVisibility{
		CurrentID:    orgUnitID,
		ReadableIDs:  nil,
		WritableID:   orgUnitID,
		HasHierarchy: has,
	}
	if !has || orgUnitID == "" {
		return vis, nil
	}
	ancestors, err := s.ListAncestorIDs(ctx, tenantID, orgUnitID)
	if err != nil {
		return nil, err
	}
	vis.ReadableIDs = ancestors
	return vis, nil
}

func (s *orgUnitService) ListAncestorIDs(
	ctx context.Context,
	tenantID uint64,
	orgUnitID string,
) ([]string, error) {
	unit, err := s.repo.GetByID(ctx, tenantID, orgUnitID)
	if err != nil {
		if errors.Is(err, apprepo.ErrOrgUnitNotFound) {
			return nil, apperrors.NewNotFoundError("org unit not found")
		}
		return nil, err
	}
	// path = /a/b/c/ → ["a","b","c"] with self last; return self-first
	// for readability (current first, then ancestors).
	parts := strings.Split(strings.Trim(unit.Path, "/"), "/")
	ids := make([]string, 0, len(parts))
	for index := len(parts) - 1; index >= 0; index-- {
		part := strings.TrimSpace(parts[index])
		if part != "" {
			ids = append(ids, part)
		}
	}
	return ids, nil
}

func (s *orgUnitService) CanReadKB(
	ctx context.Context,
	tenantID uint64,
	activeOrgUnitID string,
	kbOrgUnitID string,
	shareWithDescendants bool,
) (bool, error) {
	has, err := s.HasHierarchy(ctx, tenantID)
	if err != nil {
		return false, err
	}
	if !has {
		return true, nil
	}
	// Unbound KBs stay tenant-wide readable.
	if kbOrgUnitID == "" {
		return true, nil
	}
	if activeOrgUnitID == "" {
		// No active unit: only tenant Admin/Owner may browse bound KBs
		// (they can still pick a unit via X-Org-Unit-ID).
		role := types.TenantRoleFromContext(ctx)
		return role == types.TenantRoleAdmin || role == types.TenantRoleOwner, nil
	}
	if kbOrgUnitID == activeOrgUnitID {
		return true, nil
	}
	// Descendants may read an ancestor KB only when the owner opted in.
	if !shareWithDescendants {
		return false, nil
	}
	ancestors, err := s.ListAncestorIDs(ctx, tenantID, activeOrgUnitID)
	if err != nil {
		return false, err
	}
	for _, ancestorID := range ancestors {
		if ancestorID == kbOrgUnitID {
			return true, nil
		}
	}
	return false, nil
}

func (s *orgUnitService) CanWriteKB(
	ctx context.Context,
	tenantID uint64,
	activeOrgUnitID string,
	kbOrgUnitID string,
) (bool, error) {
	has, err := s.HasHierarchy(ctx, tenantID)
	if err != nil {
		return false, err
	}
	if !has {
		return true, nil
	}
	// Unbound KBs remain writable via normal tenant RBAC when hierarchy
	// exists (Admin managing legacy content). Callers still enforce RBAC.
	if kbOrgUnitID == "" {
		return true, nil
	}
	if activeOrgUnitID == "" {
		return false, nil
	}
	return kbOrgUnitID == activeOrgUnitID, nil
}

func (s *orgUnitService) HasHierarchy(
	ctx context.Context,
	tenantID uint64,
) (bool, error) {
	count, err := s.repo.CountByTenant(ctx, tenantID)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListInviteableOrgUnits implements peer/self + descendant scope for
// contributor/viewer invites. Admin and Owner roles are restricted to
// descendants only (同级不可再任命管理员).
//
// When the actor has no current OrgUnit, system admins / cross-tenant
// superusers / tenant Owners (bootstrap first user) may list the full
// tree — they are not bound to a hierarchy node yet.
func (s *orgUnitService) ListInviteableOrgUnits(
	ctx context.Context,
	tenantID uint64,
	actorOrgUnitID string,
	role types.TenantRole,
) ([]*types.OrgUnit, error) {
	actorOrgUnitID = strings.TrimSpace(actorOrgUnitID)
	if actorOrgUnitID == "" {
		if !isUnscopedOrgInviter(ctx) {
			return nil, apperrors.NewValidationError(
				"current org unit is required to list inviteable organizations",
			)
		}
		all, err := s.repo.ListByTenant(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		sort.SliceStable(all, func(left, right int) bool {
			if all[left].Depth != all[right].Depth {
				return all[left].Depth < all[right].Depth
			}
			if all[left].SortOrder != all[right].SortOrder {
				return all[left].SortOrder < all[right].SortOrder
			}
			return all[left].Name < all[right].Name
		})
		return all, nil
	}
	actor, err := s.repo.GetByID(ctx, tenantID, actorOrgUnitID)
	if err != nil {
		if errors.Is(err, apprepo.ErrOrgUnitNotFound) {
			return nil, apperrors.NewNotFoundError("org unit not found")
		}
		return nil, err
	}

	// 管理员/所有者只能挂到下级，避免同级再产生管理员。
	descendantsOnly := role == types.TenantRoleOwner ||
		role == types.TenantRoleAdmin
	out := make([]*types.OrgUnit, 0)

	if !descendantsOnly {
		out = append(out, actor)
		all, listErr := s.repo.ListByTenant(ctx, tenantID)
		if listErr != nil {
			return nil, listErr
		}
		for _, unit := range all {
			if unit == nil || unit.ID == actor.ID {
				continue
			}
			// 平级: same parent (including other roots when actor is root).
			if unit.ParentID == actor.ParentID {
				out = append(out, unit)
			}
		}
	}

	descendants, err := s.repo.ListByPathPrefix(ctx, tenantID, actor.Path)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(out))
	for _, unit := range out {
		if unit != nil {
			seen[unit.ID] = struct{}{}
		}
	}
	for _, unit := range descendants {
		if unit == nil || unit.ID == actor.ID {
			continue
		}
		if _, ok := seen[unit.ID]; ok {
			continue
		}
		out = append(out, unit)
		seen[unit.ID] = struct{}{}
	}

	// Stable order: depth then sort_order then name.
	sort.SliceStable(out, func(left, right int) bool {
		if out[left].Depth != out[right].Depth {
			return out[left].Depth < out[right].Depth
		}
		if out[left].SortOrder != out[right].SortOrder {
			return out[left].SortOrder < out[right].SortOrder
		}
		return out[left].Name < out[right].Name
	})
	return out, nil
}

// isUnscopedOrgInviter reports callers who may invite without binding to
// a current OrgUnit: platform system admins, cross-tenant operators,
// and tenant Owners (covers the first-install bootstrap user).
func isUnscopedOrgInviter(ctx context.Context) bool {
	if types.IsSystemAdminFromContext(ctx) {
		return true
	}
	if user, ok := ctx.Value(types.UserContextKey).(*types.User); ok && user != nil {
		if user.CanAccessAllTenants || user.IsSystemAdmin {
			return true
		}
	}
	return types.TenantRoleFromContext(ctx) == types.TenantRoleOwner
}

func (s *orgUnitService) CanInviteToOrgUnit(
	ctx context.Context,
	tenantID uint64,
	actorOrgUnitID string,
	targetOrgUnitID string,
	role types.TenantRole,
) (bool, error) {
	targetOrgUnitID = strings.TrimSpace(targetOrgUnitID)
	if targetOrgUnitID == "" {
		return false, nil
	}
	actorOrgUnitID = strings.TrimSpace(actorOrgUnitID)
	if actorOrgUnitID == "" {
		if !isUnscopedOrgInviter(ctx) {
			return false, nil
		}
		_, err := s.repo.GetByID(ctx, tenantID, targetOrgUnitID)
		if err != nil {
			if errors.Is(err, apprepo.ErrOrgUnitNotFound) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
	units, err := s.ListInviteableOrgUnits(ctx, tenantID, actorOrgUnitID, role)
	if err != nil {
		return false, err
	}
	for _, unit := range units {
		if unit != nil && unit.ID == targetOrgUnitID {
			return true, nil
		}
	}
	return false, nil
}

// Member-management scope errors for org-scoped admins.
var (
	// ErrMemberOutsideManageScope is returned when the target member is
	// neither a peer non-admin nor a subordinate.
	ErrMemberOutsideManageScope = errors.New(
		"can only manage same-level non-admins or subordinate members",
	)

	// ErrCannotManagePeerAdmin is returned when the target is another
	// admin/owner at the same org level (本级/平级).
	ErrCannotManagePeerAdmin = errors.New(
		"cannot manage another same-level admin",
	)

	// ErrCannotPromotePeerToAdmin is returned when promoting a peer
	// non-admin to admin (or owner).
	ErrCannotPromotePeerToAdmin = errors.New(
		"cannot promote a same-level member to admin",
	)

	// ErrActorOrgUnitRequired is returned when a scoped admin has no
	// active OrgUnit selected while the tenant has a hierarchy.
	ErrActorOrgUnitRequired = errors.New(
		"current org unit is required to manage members",
	)
)

func isAdminOrHigherRole(role types.TenantRole) bool {
	return role == types.TenantRoleAdmin || role == types.TenantRoleOwner
}

func primaryOrgUnitIDFromMemberships(
	memberships []*types.OrgUnitMember,
) string {
	for _, membership := range memberships {
		if membership != nil && membership.IsPrimary &&
			strings.TrimSpace(membership.OrgUnitID) != "" {
			return membership.OrgUnitID
		}
	}
	for _, membership := range memberships {
		if membership != nil && strings.TrimSpace(membership.OrgUnitID) != "" {
			return membership.OrgUnitID
		}
	}
	return ""
}

// orgUnitRelation classifies target relative to actor.
type orgUnitRelation int

const (
	orgUnitRelationOutside orgUnitRelation = iota
	orgUnitRelationPeer
	orgUnitRelationDescendant
)

func classifyOrgUnitRelation(
	actor *types.OrgUnit,
	target *types.OrgUnit,
) orgUnitRelation {
	if actor == nil || target == nil {
		return orgUnitRelationOutside
	}
	if target.ID == actor.ID || target.ParentID == actor.ParentID {
		return orgUnitRelationPeer
	}
	if actor.Path != "" &&
		strings.HasPrefix(target.Path, actor.Path) &&
		target.ID != actor.ID {
		return orgUnitRelationDescendant
	}
	return orgUnitRelationOutside
}

// AssertCanManageTenantMember enforces org-scoped admin member limits.
// newRole may be "" for remove-only operations.
func (s *orgUnitService) AssertCanManageTenantMember(
	ctx context.Context,
	tenantID uint64,
	targetUserID string,
	targetRole types.TenantRole,
	newRole types.TenantRole,
) error {
	if isUnscopedOrgInviter(ctx) {
		return nil
	}
	hasHierarchy, err := s.HasHierarchy(ctx, tenantID)
	if err != nil {
		return err
	}
	if !hasHierarchy {
		return nil
	}

	actorOrgUnitID, _ := types.OrgUnitIDFromContext(ctx)
	actorOrgUnitID = strings.TrimSpace(actorOrgUnitID)
	if actorOrgUnitID == "" {
		return ErrActorOrgUnitRequired
	}

	actorUnit, err := s.repo.GetByID(ctx, tenantID, actorOrgUnitID)
	if err != nil {
		if errors.Is(err, apprepo.ErrOrgUnitNotFound) {
			return ErrActorOrgUnitRequired
		}
		return err
	}

	targetUserID = strings.TrimSpace(targetUserID)
	memberships, err := s.repo.ListUserMemberships(ctx, tenantID, targetUserID)
	if err != nil {
		return err
	}
	targetOrgUnitID := primaryOrgUnitIDFromMemberships(memberships)
	if targetOrgUnitID == "" {
		return ErrMemberOutsideManageScope
	}
	targetUnit, err := s.repo.GetByID(ctx, tenantID, targetOrgUnitID)
	if err != nil {
		if errors.Is(err, apprepo.ErrOrgUnitNotFound) {
			return ErrMemberOutsideManageScope
		}
		return err
	}

	relation := classifyOrgUnitRelation(actorUnit, targetUnit)
	switch relation {
	case orgUnitRelationDescendant:
		return nil
	case orgUnitRelationPeer:
		if isAdminOrHigherRole(targetRole) {
			return ErrCannotManagePeerAdmin
		}
		if newRole != "" && isAdminOrHigherRole(newRole) {
			return ErrCannotPromotePeerToAdmin
		}
		return nil
	default:
		return ErrMemberOutsideManageScope
	}
}
