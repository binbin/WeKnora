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
func (r *stubOrgUnitRepo) GetByID(_ context.Context, _ uint64, id string) (*types.OrgUnit, error) {
	unit, ok := r.units[id]
	if !ok {
		return nil, apprepo.ErrOrgUnitNotFound
	}
	return unit, nil
}
func (r *stubOrgUnitRepo) Update(context.Context, *types.OrgUnit) error { return nil }
func (r *stubOrgUnitRepo) Delete(context.Context, uint64, string) error  { return nil }
func (r *stubOrgUnitRepo) ListByTenant(context.Context, uint64) ([]*types.OrgUnit, error) {
	out := make([]*types.OrgUnit, 0, len(r.units))
	for _, unit := range r.units {
		out = append(out, unit)
	}
	return out, nil
}
func (r *stubOrgUnitRepo) CountByTenant(context.Context, uint64) (int64, error) {
	return int64(len(r.units)), nil
}
func (r *stubOrgUnitRepo) CountChildren(context.Context, uint64, string) (int64, error) {
	return 0, nil
}
func (r *stubOrgUnitRepo) ListByPathPrefix(
	_ context.Context,
	_ uint64,
	pathPrefix string,
) ([]*types.OrgUnit, error) {
	out := make([]*types.OrgUnit, 0)
	for _, unit := range r.units {
		if unit != nil && strings.HasPrefix(unit.Path, pathPrefix) {
			out = append(out, unit)
		}
	}
	return out, nil
}
func (r *stubOrgUnitRepo) UpdateSubtreePaths(context.Context, uint64, string, string, int) error {
	return nil
}
func (r *stubOrgUnitRepo) AddMember(context.Context, *types.OrgUnitMember) error { return nil }
func (r *stubOrgUnitRepo) RemoveMember(context.Context, string, string) error     { return nil }
func (r *stubOrgUnitRepo) ListMembers(context.Context, string) ([]*types.OrgUnitMember, error) {
	return nil, nil
}
func (r *stubOrgUnitRepo) ListUserMemberships(
	_ context.Context,
	_ uint64,
	userID string,
) ([]*types.OrgUnitMember, error) {
	out := make([]*types.OrgUnitMember, 0)
	for _, membership := range r.members {
		if membership != nil && membership.UserID == userID {
			out = append(out, membership)
		}
	}
	return out, nil
}
func (r *stubOrgUnitRepo) GetMember(context.Context, string, string) (*types.OrgUnitMember, error) {
	return nil, apprepo.ErrOrgUnitMemberNotFound
}
func (r *stubOrgUnitRepo) ClearPrimary(context.Context, uint64, string) error { return nil }
func (r *stubOrgUnitRepo) SetPrimary(context.Context, uint64, string, string) error {
	return nil
}

func newTestOrgUnitService() interfaces.OrgUnitService {
	svc, _ := newTestOrgUnitServiceWithRepo()
	return svc
}

func newTestOrgUnitServiceWithRepo() (interfaces.OrgUnitService, *stubOrgUnitRepo) {
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
	return NewOrgUnitService(repo), repo
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
		name                 string
		active               string
		kbUnit               string
		shareWithDescendants bool
		wantRead             bool
		wantWrite            bool
	}{
		{"county reads self", "county", "county", false, true, true},
		{"county cannot read city without share", "county", "city", false, false, false},
		{"county reads city when shared", "county", "city", true, true, false},
		{"county reads province when shared", "county", "prov", true, true, false},
		{"county cannot read sibling path", "county", "other", true, false, false},
		{"city cannot read county even if shared", "city", "county", true, false, false},
		{"province cannot read city even if shared", "prov", "city", true, false, false},
		{"unbound always readable", "county", "", false, true, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			gotRead, err := svc.CanReadKB(
				ctx, tenantID, testCase.active, testCase.kbUnit, testCase.shareWithDescendants,
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

func TestOrgUnitInviteableScope(t *testing.T) {
	svc := newTestOrgUnitService()
	ctx := context.Background()
	tenantID := uint64(1)

	// City actor: self + sibling cities + descendants (county)
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
	if !ids["city"] || !ids["county"] || !ids["city2"] {
		t.Fatalf("expected city+city2+county, got %#v", ids)
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

	// Admin role from city: also descendants only (同级不可任命管理员)
	adminUnits, err := svc.ListInviteableOrgUnits(
		ctx, tenantID, "city", types.TenantRoleAdmin,
	)
	if err != nil {
		t.Fatalf("admin list: %v", err)
	}
	if len(adminUnits) != 1 || adminUnits[0].ID != "county" {
		t.Fatalf("admin should only get county, got %#v", adminUnits)
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
		ctx, tenantID, "city", "city2", types.TenantRoleAdmin,
	)
	if err != nil || ok {
		t.Fatalf("admin must not invite peer as admin: ok=%v err=%v", ok, err)
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

func TestAssertCanManageTenantMember(t *testing.T) {
	svc, repo := newTestOrgUnitServiceWithRepo()
	repo.members = []*types.OrgUnitMember{
		{UserID: "peer-contrib", OrgUnitID: "city2", IsPrimary: true},
		{UserID: "peer-admin", OrgUnitID: "city2", IsPrimary: true},
		{UserID: "self-contrib", OrgUnitID: "city", IsPrimary: true},
		{UserID: "child-admin", OrgUnitID: "county", IsPrimary: true},
		{UserID: "ancestor", OrgUnitID: "prov", IsPrimary: true},
	}
	ctx := types.WithOrgUnitID(context.Background(), "city")
	tenantID := uint64(1)

	cases := []struct {
		name       string
		target     string
		targetRole types.TenantRole
		newRole    types.TenantRole
		wantErr    error
	}{
		{
			name: "peer contributor removable",
			target: "peer-contrib", targetRole: types.TenantRoleContributor,
			wantErr: nil,
		},
		{
			name: "peer contributor cannot promote to admin",
			target: "peer-contrib", targetRole: types.TenantRoleContributor,
			newRole: types.TenantRoleAdmin, wantErr: ErrCannotPromotePeerToAdmin,
		},
		{
			name: "peer admin not manageable",
			target: "peer-admin", targetRole: types.TenantRoleAdmin,
			wantErr: ErrCannotManagePeerAdmin,
		},
		{
			name: "same-unit contributor ok",
			target: "self-contrib", targetRole: types.TenantRoleContributor,
			wantErr: nil,
		},
		{
			name: "same-unit cannot promote to admin",
			target: "self-contrib", targetRole: types.TenantRoleContributor,
			newRole: types.TenantRoleAdmin, wantErr: ErrCannotPromotePeerToAdmin,
		},
		{
			name: "subordinate admin manageable",
			target: "child-admin", targetRole: types.TenantRoleAdmin,
			wantErr: nil,
		},
		{
			name: "subordinate can promote to admin",
			target: "child-admin", targetRole: types.TenantRoleContributor,
			newRole: types.TenantRoleAdmin, wantErr: nil,
		},
		{
			name: "ancestor outside scope",
			target: "ancestor", targetRole: types.TenantRoleContributor,
			wantErr: ErrMemberOutsideManageScope,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := svc.AssertCanManageTenantMember(
				ctx, tenantID, testCase.target, testCase.targetRole, testCase.newRole,
			)
			if testCase.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				return
			}
			if err == nil || err.Error() != testCase.wantErr.Error() {
				t.Fatalf("got %v want %v", err, testCase.wantErr)
			}
		})
	}

	// Unscoped owner bypasses hierarchy limits.
	ownerCtx := context.WithValue(
		types.WithOrgUnitID(context.Background(), "city"),
		types.TenantRoleContextKey,
		types.TenantRoleOwner,
	)
	if err := svc.AssertCanManageTenantMember(
		ownerCtx, tenantID, "peer-admin", types.TenantRoleAdmin, "",
	); err != nil {
		t.Fatalf("owner should manage peer admin: %v", err)
	}
}
