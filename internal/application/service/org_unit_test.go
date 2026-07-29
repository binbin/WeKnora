package service

import (
	"context"
	"strings"
	"testing"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type stubOrgUnitRepo struct {
	units   map[string]*types.OrgUnit
	members []*types.OrgUnitMember
}

func (r *stubOrgUnitRepo) Create(context.Context, *types.OrgUnit) error { return nil }
func (r *stubOrgUnitRepo) GetByID(_ context.Context, tenantID uint64, id string) (*types.OrgUnit, error) {
	unit, ok := r.units[id]
	if !ok || unit == nil || unit.TenantID != tenantID {
		return nil, apprepo.ErrOrgUnitNotFound
	}
	return unit, nil
}
func (r *stubOrgUnitRepo) GetByIDGlobal(_ context.Context, id string) (*types.OrgUnit, error) {
	unit, ok := r.units[id]
	if !ok || unit == nil {
		return nil, apprepo.ErrOrgUnitNotFound
	}
	return unit, nil
}
func (r *stubOrgUnitRepo) Update(context.Context, *types.OrgUnit) error { return nil }
func (r *stubOrgUnitRepo) Delete(context.Context, uint64, string) error  { return nil }
func (r *stubOrgUnitRepo) ListByTenant(_ context.Context, tenantID uint64) ([]*types.OrgUnit, error) {
	out := make([]*types.OrgUnit, 0, len(r.units))
	for _, unit := range r.units {
		if unit != nil && unit.TenantID == tenantID {
			out = append(out, unit)
		}
	}
	return out, nil
}
func (r *stubOrgUnitRepo) ListAll(context.Context) ([]*types.OrgUnit, error) {
	out := make([]*types.OrgUnit, 0, len(r.units))
	for _, unit := range r.units {
		out = append(out, unit)
	}
	return out, nil
}
func (r *stubOrgUnitRepo) ListRoots(_ context.Context, tenantID uint64) ([]*types.OrgUnit, error) {
	out := make([]*types.OrgUnit, 0)
	for _, unit := range r.units {
		if unit != nil && unit.TenantID == tenantID && unit.ParentID == "" {
			out = append(out, unit)
		}
	}
	return out, nil
}
func (r *stubOrgUnitRepo) CountByTenant(_ context.Context, tenantID uint64) (int64, error) {
	var count int64
	for _, unit := range r.units {
		if unit != nil && unit.TenantID == tenantID {
			count++
		}
	}
	return count, nil
}
func (r *stubOrgUnitRepo) CountChildren(context.Context, uint64, string) (int64, error) {
	return 0, nil
}
func (r *stubOrgUnitRepo) ListByPathPrefix(
	_ context.Context,
	tenantID uint64,
	pathPrefix string,
) ([]*types.OrgUnit, error) {
	out := make([]*types.OrgUnit, 0)
	for _, unit := range r.units {
		if unit != nil &&
			unit.TenantID == tenantID &&
			strings.HasPrefix(unit.Path, pathPrefix) {
			out = append(out, unit)
		}
	}
	return out, nil
}
func (r *stubOrgUnitRepo) UpdateSubtreePaths(context.Context, uint64, string, string, int) error {
	return nil
}
func (r *stubOrgUnitRepo) AddMember(_ context.Context, member *types.OrgUnitMember) error {
	r.members = append(r.members, member)
	return nil
}
func (r *stubOrgUnitRepo) RemoveMember(
	_ context.Context, orgUnitID string, userID string,
) error {
	kept := r.members[:0]
	for _, membership := range r.members {
		if membership == nil {
			continue
		}
		if membership.OrgUnitID == orgUnitID && membership.UserID == userID {
			continue
		}
		kept = append(kept, membership)
	}
	r.members = kept
	return nil
}
func (r *stubOrgUnitRepo) ListMembers(
	_ context.Context, orgUnitID string,
) ([]*types.OrgUnitMember, error) {
	out := make([]*types.OrgUnitMember, 0)
	for _, membership := range r.members {
		if membership != nil && membership.OrgUnitID == orgUnitID {
			out = append(out, membership)
		}
	}
	return out, nil
}
func (r *stubOrgUnitRepo) ListMembersByOrgUnitIDs(
	_ context.Context, orgUnitIDs []string,
) ([]*types.OrgUnitMember, error) {
	wanted := make(map[string]struct{}, len(orgUnitIDs))
	for _, orgUnitID := range orgUnitIDs {
		wanted[orgUnitID] = struct{}{}
	}
	out := make([]*types.OrgUnitMember, 0)
	for _, membership := range r.members {
		if membership == nil {
			continue
		}
		if _, ok := wanted[membership.OrgUnitID]; ok {
			out = append(out, membership)
		}
	}
	return out, nil
}
func (r *stubOrgUnitRepo) ListUserMemberships(
	_ context.Context, tenantID uint64, userID string,
) ([]*types.OrgUnitMember, error) {
	out := make([]*types.OrgUnitMember, 0)
	for _, membership := range r.members {
		if membership == nil {
			continue
		}
		if membership.TenantID == tenantID && membership.UserID == userID {
			out = append(out, membership)
		}
	}
	return out, nil
}
func (r *stubOrgUnitRepo) ListUserMembershipsByUser(
	_ context.Context, userID string,
) ([]*types.OrgUnitMember, error) {
	out := make([]*types.OrgUnitMember, 0)
	for _, membership := range r.members {
		if membership != nil && membership.UserID == userID {
			out = append(out, membership)
		}
	}
	return out, nil
}
func (r *stubOrgUnitRepo) GetMember(
	_ context.Context, orgUnitID string, userID string,
) (*types.OrgUnitMember, error) {
	for _, membership := range r.members {
		if membership == nil {
			continue
		}
		if membership.OrgUnitID == orgUnitID && membership.UserID == userID {
			return membership, nil
		}
	}
	return nil, apprepo.ErrOrgUnitMemberNotFound
}
func (r *stubOrgUnitRepo) ClearPrimary(context.Context, uint64, string) error { return nil }
func (r *stubOrgUnitRepo) SetPrimary(context.Context, uint64, string, string) error {
	return nil
}

func (r *stubOrgUnitRepo) RemoveMembersByTenantUser(
	_ context.Context, tenantID uint64, userID string,
) error {
	kept := r.members[:0]
	for _, membership := range r.members {
		if membership == nil {
			continue
		}
		if membership.TenantID == tenantID && membership.UserID == userID {
			continue
		}
		kept = append(kept, membership)
	}
	r.members = kept
	return nil
}

func (r *stubOrgUnitRepo) TransferMember(
	_ context.Context, member *types.OrgUnitMember,
) error {
	kept := r.members[:0]
	for _, membership := range r.members {
		if membership == nil || membership.UserID == member.UserID {
			continue
		}
		kept = append(kept, membership)
	}
	r.members = append(kept, member)
	return nil
}

func newTestOrgUnitService() interfaces.OrgUnitService {
	repo := &stubOrgUnitRepo{
		units: map[string]*types.OrgUnit{
			"prov": {
				ID: "prov", TenantID: 1, ParentID: "",
				Path: "/prov/", Depth: 0, Name: "Province",
			},
			"city": {
				ID: "city", TenantID: 1, ParentID: "prov",
				Path: "/prov/city/", Depth: 1, Name: "City",
			},
			"city2": {
				ID: "city2", TenantID: 1, ParentID: "prov",
				Path: "/prov/city2/", Depth: 1, Name: "CityB",
			},
			"county": {
				ID: "county", TenantID: 1, ParentID: "city",
				Path: "/prov/city/county/", Depth: 2, Name: "County",
			},
		},
	}
	return NewOrgUnitService(repo)
}

func TestOrgUnitAncestorReadSelfWrite(t *testing.T) {
	svc := newTestOrgUnitService()
	ctx := context.Background()
	tenantID := uint64(1)

	ancestors, err := svc.ListAncestorIDs(ctx, tenantID, "county")
	if err != nil {
		t.Fatalf("ListAncestorIDs: %v", err)
	}
	if len(ancestors) != 3 || ancestors[0] != "county" || ancestors[2] != "prov" {
		t.Fatalf("unexpected ancestors: %#v", ancestors)
	}

	cases := []struct {
		name                string
		active              string
		kbUnit              string
		shareWithDescendants bool
		wantRead            bool
		wantWrite           bool
	}{
		{"county reads self", "county", "county", true, true, true},
		{"county reads city", "county", "city", true, true, false},
		{"county reads province", "county", "prov", true, true, false},
		{"county cannot read sibling path", "county", "other", true, false, false},
		{"city cannot read county", "city", "county", true, false, false},
		{"province cannot read city", "prov", "city", true, false, false},
		{"city cannot read peer city2", "city", "city2", true, false, false},
		{"unbound always readable", "county", "", true, true, true},
		{"no share blocks ancestor", "county", "city", false, false, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			gotRead, err := svc.CanReadKB(
				ctx, tenantID, testCase.active, testCase.kbUnit,
				testCase.shareWithDescendants,
			)
			if err != nil {
				t.Fatalf("CanReadKB: %v", err)
			}
			if gotRead != testCase.wantRead {
				t.Fatalf("CanReadKB=%v want %v", gotRead, testCase.wantRead)
			}
			gotWrite, err := svc.CanWriteKB(ctx, tenantID, testCase.active, testCase.kbUnit)
			if err != nil {
				t.Fatalf("CanWriteKB: %v", err)
			}
			if gotWrite != testCase.wantWrite {
				t.Fatalf("CanWriteKB=%v want %v", gotWrite, testCase.wantWrite)
			}
		})
	}
}

func TestCanReadKB_ScopedAdminEmptyActiveDenied(t *testing.T) {
	svc := newTestOrgUnitService()
	tenantID := uint64(1)
	adminCtx := context.WithValue(
		context.Background(),
		types.TenantRoleContextKey,
		types.TenantRoleAdmin,
	)
	got, err := svc.CanReadKB(adminCtx, tenantID, "", "city", false)
	if err != nil {
		t.Fatalf("CanReadKB: %v", err)
	}
	if got {
		t.Fatal("scoped admin without active org must not read bound KBs")
	}
	ownerCtx := context.WithValue(
		context.Background(),
		types.TenantRoleContextKey,
		types.TenantRoleOwner,
	)
	got, err = svc.CanReadKB(ownerCtx, tenantID, "", "city", false)
	if err != nil {
		t.Fatalf("CanReadKB owner: %v", err)
	}
	if !got {
		t.Fatal("tenant owner without active org may still browse bound KBs")
	}
}

func TestResolveActiveOrgUnit_ScopedAdminCannotActivatePeer(t *testing.T) {
	repo := &stubOrgUnitRepo{
		units: map[string]*types.OrgUnit{
			"prov": {
				ID: "prov", TenantID: 1, Path: "/prov/", Depth: 0, Name: "Province",
			},
			"city": {
				ID: "city", TenantID: 1, ParentID: "prov",
				Path: "/prov/city/", Depth: 1, Name: "City",
			},
			"city2": {
				ID: "city2", TenantID: 1, ParentID: "prov",
				Path: "/prov/city2/", Depth: 1, Name: "CityB",
			},
			"county": {
				ID: "county", TenantID: 1, ParentID: "city",
				Path: "/prov/city/county/", Depth: 2, Name: "County",
			},
		},
		members: []*types.OrgUnitMember{
			{OrgUnitID: "city", TenantID: 1, UserID: "admin-city", IsPrimary: true},
		},
	}
	svc := NewOrgUnitService(repo)
	tenantID := uint64(1)
	ctx := context.WithValue(
		context.Background(),
		types.TenantRoleContextKey,
		types.TenantRoleAdmin,
	)
	ctx = context.WithValue(ctx, types.UserIDContextKey, "admin-city")

	if _, err := svc.ResolveActiveOrgUnit(ctx, tenantID, "admin-city", "city2"); err == nil {
		t.Fatal("expected peer org unit activation to be denied")
	}
	got, err := svc.ResolveActiveOrgUnit(ctx, tenantID, "admin-city", "county")
	if err != nil {
		t.Fatalf("descendant activation: %v", err)
	}
	if got != "county" {
		t.Fatalf("got %q want county", got)
	}
	got, err = svc.ResolveActiveOrgUnit(ctx, tenantID, "admin-city", "city")
	if err != nil {
		t.Fatalf("home activation: %v", err)
	}
	if got != "city" {
		t.Fatalf("got %q want city", got)
	}
}

func TestOrgUnitInviteableScope(t *testing.T) {
	svc := newTestOrgUnitService()
	ctx := context.Background()
	tenantID := uint64(1)

	// City actor inviting contributor: 本级 + 下级 (no peer city2)
	units, err := svc.ListInviteableOrgUnits(
		ctx, tenantID, "city", types.TenantRoleContributor,
	)
	if err != nil {
		t.Fatalf("ListInviteableOrgUnits: %v", err)
	}
	ids := map[string]bool{}
	for _, unit := range units {
		ids[unit.ID] = true
	}
	if !ids["city"] || !ids["county"] {
		t.Fatalf("expected city+county, got %#v", ids)
	}
	if ids["city2"] {
		t.Fatalf("must not include peer city2: %#v", ids)
	}
	if ids["prov"] {
		t.Fatalf("must not include ancestor province: %#v", ids)
	}

	// Owner role from city: only county (下级), not self
	ownerUnits, err := svc.ListInviteableOrgUnits(
		ctx, tenantID, "city", types.TenantRoleOwner,
	)
	if err != nil {
		t.Fatalf("owner list: %v", err)
	}
	if len(ownerUnits) != 1 || ownerUnits[0].ID != "county" {
		t.Fatalf("owner should only get county, got %#v", ownerUnits)
	}

	ok, err := svc.CanInviteToOrgUnit(
		ctx, tenantID, "city", "county", types.TenantRoleOwner,
	)
	if err != nil || !ok {
		t.Fatalf("owner can invite county: ok=%v err=%v", ok, err)
	}
	ok, err = svc.CanInviteToOrgUnit(
		ctx, tenantID, "city", "city", types.TenantRoleOwner,
	)
	if err != nil || ok {
		t.Fatalf("owner must not invite self: ok=%v err=%v", ok, err)
	}
	ok, err = svc.CanInviteToOrgUnit(
		ctx, tenantID, "city", "city2", types.TenantRoleContributor,
	)
	if err != nil || ok {
		t.Fatalf("must not invite peer: ok=%v err=%v", ok, err)
	}
}

func TestOrgUnitInviteableUnscopedOwner(t *testing.T) {
	svc := newTestOrgUnitService()
	ctx := context.WithValue(
		context.Background(),
		types.TenantRoleContextKey,
		types.TenantRoleOwner,
	)
	units, err := svc.ListInviteableOrgUnits(
		ctx, 1, "", types.TenantRoleContributor,
	)
	if err != nil {
		t.Fatalf("unscoped owner list: %v", err)
	}
	if len(units) < 3 {
		t.Fatalf("expected full tree for owner without current unit, got %d", len(units))
	}
	ok, err := svc.CanInviteToOrgUnit(
		ctx, 1, "", "prov", types.TenantRoleContributor,
	)
	if err != nil || !ok {
		t.Fatalf("owner may invite any unit: ok=%v err=%v", ok, err)
	}
}

func TestOrgUnitInviteableRequiresActorForNonOwner(t *testing.T) {
	svc := newTestOrgUnitService()
	ctx := context.WithValue(
		context.Background(),
		types.TenantRoleContextKey,
		types.TenantRoleContributor,
	)
	_, err := svc.ListInviteableOrgUnits(
		ctx, 1, "", types.TenantRoleContributor,
	)
	if err == nil {
		t.Fatal("expected validation error without current org unit")
	}
}

func TestBuildOrgUnitTree(t *testing.T) {
	units := []*types.OrgUnit{
		{ID: "prov", ParentID: "", Name: "P"},
		{ID: "city", ParentID: "prov", Name: "C"},
		{ID: "county", ParentID: "city", Name: "X"},
	}
	roots := buildOrgUnitTree(units)
	if len(roots) != 1 || roots[0].ID != "prov" {
		t.Fatalf("roots=%#v", roots)
	}
	if len(roots[0].Children) != 1 || roots[0].Children[0].ID != "city" {
		t.Fatalf("city children=%#v", roots[0].Children)
	}
	if len(roots[0].Children[0].Children) != 1 {
		t.Fatalf("county missing")
	}
}

func adminCtxWithHome(userID string) context.Context {
	ctx := context.WithValue(
		context.Background(),
		types.TenantRoleContextKey,
		types.TenantRoleAdmin,
	)
	return context.WithValue(ctx, types.UserIDContextKey, userID)
}

func TestListTreeScopedToAdminHome(t *testing.T) {
	repo := &stubOrgUnitRepo{
		units: map[string]*types.OrgUnit{
			"prov": {
				ID: "prov", TenantID: 1, ParentID: "",
				Path: "/prov/", Depth: 0, Name: "Province",
			},
			"city": {
				ID: "city", TenantID: 1, ParentID: "prov",
				Path: "/prov/city/", Depth: 1, Name: "City",
			},
			"city2": {
				ID: "city2", TenantID: 1, ParentID: "prov",
				Path: "/prov/city2/", Depth: 1, Name: "CityB",
			},
			"county": {
				ID: "county", TenantID: 1, ParentID: "city",
				Path: "/prov/city/county/", Depth: 2, Name: "County",
			},
		},
		members: []*types.OrgUnitMember{
			{OrgUnitID: "city", TenantID: 1, UserID: "admin-city"},
		},
	}
	svc := NewOrgUnitService(repo)
	ctx := adminCtxWithHome("admin-city")

	tree, err := svc.ListTree(ctx, 1)
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	if len(tree) != 1 || tree[0].ID != "city" {
		t.Fatalf("want tree rooted at city, got %#v", tree)
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].ID != "county" {
		t.Fatalf("want county under city, got %#v", tree[0].Children)
	}

	// Owner still sees the full forest.
	ownerCtx := context.WithValue(
		context.Background(),
		types.TenantRoleContextKey,
		types.TenantRoleOwner,
	)
	ownerCtx = context.WithValue(ownerCtx, types.UserIDContextKey, "owner")
	ownerTree, err := svc.ListTree(ownerCtx, 1)
	if err != nil {
		t.Fatalf("owner ListTree: %v", err)
	}
	if len(ownerTree) != 1 || ownerTree[0].ID != "prov" {
		t.Fatalf("owner should see province root, got %#v", ownerTree)
	}
}

func TestAdminCreateDeleteOrgUnitScope(t *testing.T) {
	repo := &stubOrgUnitRepo{
		units: map[string]*types.OrgUnit{
			"prov": {
				ID: "prov", TenantID: 1, ParentID: "",
				Path: "/prov/", Depth: 0, Name: "Province",
			},
			"city": {
				ID: "city", TenantID: 1, ParentID: "prov",
				Path: "/prov/city/", Depth: 1, Name: "City",
			},
			"city2": {
				ID: "city2", TenantID: 1, ParentID: "prov",
				Path: "/prov/city2/", Depth: 1, Name: "CityB",
			},
			"county": {
				ID: "county", TenantID: 1, ParentID: "city",
				Path: "/prov/city/county/", Depth: 2, Name: "County",
			},
		},
		members: []*types.OrgUnitMember{
			{OrgUnitID: "city", TenantID: 1, UserID: "admin-city"},
		},
	}
	svc := NewOrgUnitService(repo)
	ctx := adminCtxWithHome("admin-city")

	// Cannot create under a peer (city2).
	_, err := svc.Create(ctx, 1, &types.CreateOrgUnitRequest{
		Name: "X", ParentID: "city2",
	})
	if err == nil {
		t.Fatal("expected create under peer to fail")
	}

	// Can create under self.
	child, err := svc.Create(ctx, 1, &types.CreateOrgUnitRequest{
		Name: "District", ParentID: "city",
	})
	if err != nil {
		t.Fatalf("create under self: %v", err)
	}
	if child.ParentID != "city" {
		t.Fatalf("parent=%s", child.ParentID)
	}

	// Cannot delete own home.
	if err := svc.Delete(ctx, 1, "city"); err == nil {
		t.Fatal("expected delete home to fail")
	}

	// Can delete a descendant.
	if err := svc.Delete(ctx, 1, "county"); err != nil {
		t.Fatalf("delete descendant: %v", err)
	}
}

func TestCreateUnderPlatformParentFromBusinessTenant(t *testing.T) {
	repo := &stubOrgUnitRepo{
		units: map[string]*types.OrgUnit{
			"neimeng": {
				ID: "neimeng", TenantID: types.PlatformOrgTenantID,
				ParentID: "", Path: "/neimeng/", Depth: 0,
				Name: "内蒙古自治区",
			},
			"renshe": {
				ID: "renshe", TenantID: types.PlatformOrgTenantID,
				ParentID: "neimeng", Path: "/neimeng/renshe/", Depth: 1,
				Name: "人社厅",
			},
		},
		members: []*types.OrgUnitMember{
			{
				OrgUnitID: "renshe",
				TenantID:  types.PlatformOrgTenantID,
				UserID:    "admin-renshe",
				IsPrimary: true,
			},
		},
	}
	svc := NewOrgUnitService(repo)
	ctx := adminCtxWithHome("admin-renshe")

	// Workspace-scoped create must resolve platform catalog parents —
	// regression for "parent org unit not found" under 人社厅.
	child, err := svc.Create(ctx, 10000, &types.CreateOrgUnitRequest{
		Name: "自治区社保中心", ParentID: "renshe",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if child.TenantID != types.PlatformOrgTenantID {
		t.Fatalf("tenant=%d, want platform catalog", child.TenantID)
	}
	if child.ParentID != "renshe" || child.Depth != 2 {
		t.Fatalf("child=%#v", child)
	}
	if !strings.HasPrefix(child.Path, "/neimeng/renshe/") {
		t.Fatalf("path=%q", child.Path)
	}
}

func TestResolveMemberListScopeSelfAndDescendants(t *testing.T) {
	repo := &stubOrgUnitRepo{
		units: map[string]*types.OrgUnit{
			"prov": {
				ID: "prov", TenantID: 1, ParentID: "",
				Path: "/prov/", Depth: 0, Name: "Province",
			},
			"city": {
				ID: "city", TenantID: 1, ParentID: "prov",
				Path: "/prov/city/", Depth: 1, Name: "City",
			},
			"city2": {
				ID: "city2", TenantID: 1, ParentID: "prov",
				Path: "/prov/city2/", Depth: 1, Name: "CityB",
			},
			"county": {
				ID: "county", TenantID: 1, ParentID: "city",
				Path: "/prov/city/county/", Depth: 2, Name: "County",
			},
		},
		members: []*types.OrgUnitMember{
			{OrgUnitID: "prov", TenantID: 1, UserID: "u-prov"},
			{OrgUnitID: "city", TenantID: 1, UserID: "admin-city"},
			{OrgUnitID: "city2", TenantID: 1, UserID: "u-city2"},
			{OrgUnitID: "county", TenantID: 1, UserID: "u-county"},
		},
	}
	svc := NewOrgUnitService(repo)
	ctx := adminCtxWithHome("admin-city")

	ids, restricted, err := svc.ResolveMemberListScope(ctx, 1)
	if err != nil {
		t.Fatalf("ResolveMemberListScope: %v", err)
	}
	if !restricted {
		t.Fatal("expected restricted scope for admin")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	if !seen["admin-city"] || !seen["u-county"] {
		t.Fatalf("want self+descendant, got %#v", ids)
	}
	if seen["u-prov"] {
		t.Fatalf("must not include ancestor member: %#v", ids)
	}
	if seen["u-city2"] {
		t.Fatalf("must not include peer member: %#v", ids)
	}

	ownerCtx := context.WithValue(
		context.Background(),
		types.TenantRoleContextKey,
		types.TenantRoleOwner,
	)
	ownerCtx = context.WithValue(ownerCtx, types.UserIDContextKey, "owner")
	_, ownerRestricted, err := svc.ResolveMemberListScope(ownerCtx, 1)
	if err != nil {
		t.Fatalf("owner scope: %v", err)
	}
	if ownerRestricted {
		t.Fatal("owner must not be restricted")
	}
}

func TestOrgUnitCreateRootRequiresSystemAdmin(t *testing.T) {
	svc := newTestOrgUnitService()
	_, err := svc.Create(context.Background(), 1, &types.CreateOrgUnitRequest{
		Name: "Root",
	})
	if err == nil {
		t.Fatal("expected forbidden creating root without system admin")
	}
	adminCtx := context.WithValue(
		context.Background(),
		types.SystemAdminContextKey,
		true,
	)
	unit, err := svc.Create(adminCtx, 1, &types.CreateOrgUnitRequest{Name: "Root"})
	if err != nil {
		t.Fatalf("system admin should create root: %v", err)
	}
	if unit == nil || unit.ParentID != "" {
		t.Fatalf("unexpected unit: %#v", unit)
	}
}

func TestAddMemberRejectsSecondOrgUnit(t *testing.T) {
	repo := &stubOrgUnitRepo{
		units: map[string]*types.OrgUnit{
			"city":  {ID: "city", TenantID: 1, Path: "/city/", Name: "City"},
			"city2": {ID: "city2", TenantID: 1, Path: "/city2/", Name: "City2"},
		},
		members: []*types.OrgUnitMember{
			{ID: "m1", TenantID: 1, OrgUnitID: "city", UserID: "u1", IsPrimary: true},
		},
	}
	svc := NewOrgUnitService(repo)
	_, err := svc.AddMember(context.Background(), 1, "city2", "u1", true)
	if err == nil {
		t.Fatal("expected conflict when user already in another org unit")
	}
}

func TestAddMemberIdempotentSameUnit(t *testing.T) {
	repo := &stubOrgUnitRepo{
		units: map[string]*types.OrgUnit{
			"city": {ID: "city", TenantID: 1, Path: "/city/", Name: "City"},
		},
		members: []*types.OrgUnitMember{
			{ID: "m1", TenantID: 1, OrgUnitID: "city", UserID: "u1", IsPrimary: true},
		},
	}
	svc := NewOrgUnitService(repo)
	member, err := svc.AddMember(context.Background(), 1, "city", "u1", false)
	if err != nil {
		t.Fatalf("AddMember same unit: %v", err)
	}
	if member.OrgUnitID != "city" {
		t.Fatalf("got %#v", member)
	}
}

func TestTransferMemberMovesUser(t *testing.T) {
	repo := &stubOrgUnitRepo{
		units: map[string]*types.OrgUnit{
			"city":  {ID: "city", TenantID: 1, Path: "/city/", Name: "City"},
			"city2": {ID: "city2", TenantID: 1, Path: "/city2/", Name: "City2"},
		},
		members: []*types.OrgUnitMember{
			{ID: "m1", TenantID: 1, OrgUnitID: "city", UserID: "u1", IsPrimary: true},
		},
	}
	svc := NewOrgUnitService(repo)
	member, err := svc.TransferMember(context.Background(), 1, "u1", "city2")
	if err != nil {
		t.Fatalf("TransferMember: %v", err)
	}
	if member.OrgUnitID != "city2" {
		t.Fatalf("got %#v", member)
	}
	list, err := svc.ListUserMemberships(context.Background(), 1, "u1")
	if err != nil || len(list) != 1 || list[0].OrgUnitID != "city2" {
		t.Fatalf("memberships=%#v err=%v", list, err)
	}
}

func TestListPlatformTreeIncludesLegacyTenantTrees(t *testing.T) {
	repo := &stubOrgUnitRepo{
		units: map[string]*types.OrgUnit{
			"platform-root": {
				ID: "platform-root", TenantID: types.PlatformOrgTenantID,
				ParentID: "", Path: "/platform-root/", Depth: 0,
				Name: "PlatformRoot",
			},
			"legacy-root": {
				ID: "legacy-root", TenantID: 10000,
				ParentID: "", Path: "/legacy-root/", Depth: 0,
				Name: "LegacyRoot",
			},
			"legacy-child": {
				ID: "legacy-child", TenantID: 10000,
				ParentID: "legacy-root", Path: "/legacy-root/legacy-child/",
				Depth: 1, Name: "LegacyChild",
			},
		},
	}
	svc := NewOrgUnitService(repo)

	// Tenant-scoped list for platform catalog alone must not hide that
	// legacy trees exist elsewhere — regression for /platform/org-units.
	platformOnly, err := svc.ListTree(context.Background(), types.PlatformOrgTenantID)
	if err != nil {
		t.Fatalf("ListTree platform: %v", err)
	}
	if len(platformOnly) != 1 || platformOnly[0].ID != "platform-root" {
		t.Fatalf("platform-only tree=%#v", platformOnly)
	}

	forest, err := svc.ListPlatformTree(context.Background())
	if err != nil {
		t.Fatalf("ListPlatformTree: %v", err)
	}
	if len(forest) != 2 {
		t.Fatalf("want 2 roots, got %#v", forest)
	}
	byID := map[string]*types.OrgUnit{}
	for _, root := range forest {
		byID[root.ID] = root
	}
	if byID["platform-root"] == nil {
		t.Fatal("missing platform-root")
	}
	legacy := byID["legacy-root"]
	if legacy == nil {
		t.Fatal("missing legacy-root")
	}
	if len(legacy.Children) != 1 || legacy.Children[0].ID != "legacy-child" {
		t.Fatalf("legacy children=%#v", legacy.Children)
	}
}

// Platform-catalog roots (tenant_id=0) must still drive invite UI when the
// actor is browsing a business workspace with no in-tenant OrgUnits.
func TestOrgUnitPlatformCatalogInviteHierarchy(t *testing.T) {
	repo := &stubOrgUnitRepo{
		units: map[string]*types.OrgUnit{
			"platform-root": {
				ID: "platform-root", TenantID: types.PlatformOrgTenantID,
				ParentID: "", Path: "/platform-root/", Depth: 0,
				Name: "人社厅",
			},
			"platform-child": {
				ID: "platform-child", TenantID: types.PlatformOrgTenantID,
				ParentID: "platform-root",
				Path:     "/platform-root/platform-child/",
				Depth:    1,
				Name:     "赤峰市",
			},
		},
	}
	svc := NewOrgUnitService(repo)
	workspaceTenant := uint64(10000)

	has, err := svc.HasHierarchy(context.Background(), workspaceTenant)
	if err != nil {
		t.Fatalf("HasHierarchy: %v", err)
	}
	if !has {
		t.Fatal("platform catalog must count as hierarchy for workspace tenant")
	}

	ownerCtx := context.WithValue(
		context.Background(),
		types.TenantRoleContextKey,
		types.TenantRoleOwner,
	)
	units, err := svc.ListInviteableOrgUnits(
		ownerCtx, workspaceTenant, "", types.TenantRoleContributor,
	)
	if err != nil {
		t.Fatalf("ListInviteableOrgUnits: %v", err)
	}
	if len(units) != 2 {
		t.Fatalf("want platform forest, got %#v", units)
	}

	ok, err := svc.CanInviteToOrgUnit(
		ownerCtx, workspaceTenant, "", "platform-child",
		types.TenantRoleContributor,
	)
	if err != nil || !ok {
		t.Fatalf("CanInviteToOrgUnit platform child: ok=%v err=%v", ok, err)
	}

	flat, err := svc.ListFlat(ownerCtx, workspaceTenant)
	if err != nil {
		t.Fatalf("ListFlat: %v", err)
	}
	if len(flat) != 2 {
		t.Fatalf("ListFlat should surface platform catalog, got %#v", flat)
	}
}

func TestTransferMemberAcceptsPlatformCatalogUnit(t *testing.T) {
	repo := &stubOrgUnitRepo{
		units: map[string]*types.OrgUnit{
			"platform-child": {
				ID: "platform-child", TenantID: types.PlatformOrgTenantID,
				ParentID: "platform-root",
				Path:     "/platform-root/platform-child/",
				Depth:    1,
				Name:     "赤峰市",
			},
			"platform-root": {
				ID: "platform-root", TenantID: types.PlatformOrgTenantID,
				ParentID: "", Path: "/platform-root/", Depth: 0,
				Name: "人社厅",
			},
		},
	}
	svc := NewOrgUnitService(repo)
	ownerCtx := context.WithValue(
		context.Background(),
		types.TenantRoleContextKey,
		types.TenantRoleOwner,
	)
	member, err := svc.TransferMember(
		ownerCtx, 10000, "user-1", "platform-child",
	)
	if err != nil {
		t.Fatalf("TransferMember platform unit: %v", err)
	}
	if member.OrgUnitID != "platform-child" {
		t.Fatalf("org_unit_id=%q", member.OrgUnitID)
	}
	if member.TenantID != types.PlatformOrgTenantID {
		t.Fatalf("membership tenant_id=%d want platform", member.TenantID)
	}

	listed, err := svc.ListUserMemberships(ownerCtx, 10000, "user-1")
	if err != nil {
		t.Fatalf("ListUserMemberships: %v", err)
	}
	if len(listed) != 1 || listed[0].OrgUnitID != "platform-child" {
		t.Fatalf("workspace list must include platform binding, got %#v", listed)
	}
}
