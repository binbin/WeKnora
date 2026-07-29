package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type orgWorkspaceService struct {
	orgRepo       interfaces.OrgUnitRepository
	workspaceRepo interfaces.OrgUnitWorkspaceRepository
	tenantService interfaces.TenantService
	memberService interfaces.TenantMemberService
}

func NewOrgWorkspaceService(
	orgRepo interfaces.OrgUnitRepository,
	workspaceRepo interfaces.OrgUnitWorkspaceRepository,
	tenantService interfaces.TenantService,
	memberService interfaces.TenantMemberService,
) interfaces.OrgWorkspaceService {
	return &orgWorkspaceService{
		orgRepo:       orgRepo,
		workspaceRepo: workspaceRepo,
		tenantService: tenantService,
		memberService: memberService,
	}
}

func (s *orgWorkspaceService) EnsureWorkspaceForUser(
	ctx context.Context,
	userID string,
) (uint64, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" || s.orgRepo == nil {
		return 0, nil
	}
	memberships, err := s.orgRepo.ListUserMembershipsByUser(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("list org memberships: %w", err)
	}
	if len(memberships) == 0 {
		return 0, nil
	}
	primary := memberships[0]
	for _, membership := range memberships {
		if membership != nil && membership.IsPrimary {
			primary = membership
			break
		}
	}
	if primary == nil || strings.TrimSpace(primary.OrgUnitID) == "" {
		return 0, nil
	}
	unit, err := s.orgRepo.GetByIDGlobal(ctx, primary.OrgUnitID)
	if err != nil {
		return 0, fmt.Errorf("load org unit: %w", err)
	}
	root, err := s.resolveRootOrgUnit(ctx, unit)
	if err != nil {
		return 0, err
	}
	tenantID, err := s.ensureWorkspaceForRoot(ctx, root)
	if err != nil {
		return 0, err
	}
	if err := s.syncOrgTreeMembers(ctx, root, tenantID, userID); err != nil {
		logger.Warnf(ctx,
			"org workspace sync members failed root=%s tenant=%d: %v",
			root.ID, tenantID, err,
		)
	}
	return tenantID, nil
}

func (s *orgWorkspaceService) EnsureFirstPlatformWorkspace(
	ctx context.Context,
) (uint64, error) {
	if s.orgRepo == nil {
		return 0, nil
	}
	roots, err := s.orgRepo.ListRoots(ctx, types.PlatformOrgTenantID)
	if err != nil {
		return 0, fmt.Errorf("list platform roots: %w", err)
	}
	if len(roots) == 0 || roots[0] == nil {
		return 0, nil
	}
	return s.ensureWorkspaceForRoot(ctx, roots[0])
}

func (s *orgWorkspaceService) resolveRootOrgUnit(
	ctx context.Context,
	unit *types.OrgUnit,
) (*types.OrgUnit, error) {
	if unit == nil {
		return nil, apprepo.ErrOrgUnitNotFound
	}
	current := unit
	for current.ParentID != "" {
		parent, err := s.orgRepo.GetByIDGlobal(ctx, current.ParentID)
		if err != nil {
			return nil, fmt.Errorf("walk parent org unit: %w", err)
		}
		current = parent
	}
	return current, nil
}

func (s *orgWorkspaceService) ensureWorkspaceForRoot(
	ctx context.Context,
	root *types.OrgUnit,
) (uint64, error) {
	if root == nil {
		return 0, apprepo.ErrOrgUnitNotFound
	}
	if s.workspaceRepo != nil {
		binding, err := s.workspaceRepo.GetByRootOrgUnitID(ctx, root.ID)
		if err == nil && binding != nil && binding.TenantID > 0 {
			return binding.TenantID, nil
		}
		if err != nil && !errors.Is(err, apprepo.ErrOrgUnitWorkspaceNotFound) {
			return 0, err
		}
	}

	// Legacy in-tenant trees already sit inside a business Tenant — reuse it.
	if root.TenantID != types.PlatformOrgTenantID && root.TenantID > 0 {
		if s.workspaceRepo != nil {
			now := time.Now()
			_ = s.workspaceRepo.Create(ctx, &types.OrgUnitWorkspace{
				RootOrgUnitID: root.ID,
				TenantID:      root.TenantID,
				CreatedAt:     now,
				UpdatedAt:     now,
			})
		}
		return root.TenantID, nil
	}

	if s.tenantService == nil {
		return 0, fmt.Errorf("tenant service unavailable")
	}
	// Single shared workspace: never mint a new tenant per org root.
	// Bind the earliest existing tenant, or fail closed if none exist yet
	// (empty install should create the workspace via registration bootstrap).
	tenants, listErr := s.tenantService.ListTenants(ctx)
	if listErr != nil {
		return 0, fmt.Errorf("list tenants for org workspace: %w", listErr)
	}
	if len(tenants) == 0 || tenants[0] == nil {
		return 0, fmt.Errorf("no shared workspace available to bind org %s", root.ID)
	}
	earliest := tenants[0]
	for _, tenant := range tenants[1:] {
		if tenant == nil {
			continue
		}
		if earliest == nil || tenant.CreatedAt.Before(earliest.CreatedAt) {
			earliest = tenant
		}
	}
	if earliest == nil {
		return 0, fmt.Errorf("no shared workspace available to bind org %s", root.ID)
	}
	if s.workspaceRepo != nil {
		now := time.Now()
		if bindErr := s.workspaceRepo.Create(ctx, &types.OrgUnitWorkspace{
			RootOrgUnitID: root.ID,
			TenantID:      earliest.ID,
			CreatedAt:     now,
			UpdatedAt:     now,
		}); bindErr != nil {
			logger.Warnf(ctx,
				"failed to bind org %s to shared tenant %d: %v",
				root.ID, earliest.ID, bindErr,
			)
		}
	}
	logger.Infof(ctx,
		"bound platform root org=%s to shared workspace tenant=%d",
		root.ID, earliest.ID,
	)
	return earliest.ID, nil
}

func (s *orgWorkspaceService) syncOrgTreeMembers(
	ctx context.Context,
	root *types.OrgUnit,
	tenantID uint64,
	priorityUserID string,
) error {
	if s.memberService == nil || s.orgRepo == nil || root == nil || tenantID == 0 {
		return nil
	}
	units, err := s.orgRepo.ListByPathPrefix(ctx, root.TenantID, root.Path)
	if err != nil {
		return err
	}
	orgUnitIDs := make([]string, 0, len(units)+1)
	orgUnitIDs = append(orgUnitIDs, root.ID)
	for _, unit := range units {
		if unit == nil || unit.ID == root.ID {
			continue
		}
		orgUnitIDs = append(orgUnitIDs, unit.ID)
	}
	members, err := s.orgRepo.ListMembersByOrgUnitIDs(ctx, orgUnitIDs)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(members)+1)
	if priorityUserID != "" {
		if err := s.ensureTenantMembership(ctx, priorityUserID, tenantID, true); err != nil {
			return err
		}
		seen[priorityUserID] = struct{}{}
	}
	for _, member := range members {
		if member == nil || member.UserID == "" {
			continue
		}
		if _, ok := seen[member.UserID]; ok {
			continue
		}
		seen[member.UserID] = struct{}{}
		if err := s.ensureTenantMembership(ctx, member.UserID, tenantID, false); err != nil {
			logger.Warnf(ctx,
				"ensure tenant membership user=%s tenant=%d: %v",
				member.UserID, tenantID, err,
			)
		}
	}
	return nil
}

func (s *orgWorkspaceService) ensureTenantMembership(
	ctx context.Context,
	userID string,
	tenantID uint64,
	preferOwnerIfEmpty bool,
) error {
	existing, err := s.memberService.GetMembership(ctx, userID, tenantID)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	hasMembers, err := s.memberService.HasAnyMembers(ctx, tenantID)
	if err != nil {
		return err
	}
	if preferOwnerIfEmpty && !hasMembers {
		_, err = s.memberService.EnsureOwner(ctx, userID, tenantID)
		return err
	}
	_, err = s.memberService.AddMember(
		ctx, userID, tenantID, types.TenantRoleContributor, nil,
	)
	if err != nil && errors.Is(err, ErrMembershipAlreadyExists) {
		return nil
	}
	return err
}
