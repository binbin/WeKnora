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
func (r *stubOrgUnitRepo) ListUserMemberships(context.Context, uint64, string) ([]*types.OrgUnitMember, error) {
	return r.members, nil
}
func (r *stubOrgUnitRepo) GetMember(context.Context, string, string) (*types.OrgUnitMember, error) {
	return nil, apprepo.ErrOrgUnitMemberNotFound
}
func (r *stubOrgUnitRepo) ClearPrimary(context.Context, uint64, string) error { return nil }
func (r *stubOrgUnitRepo) SetPrimary(context.Context, uint64, string, string) error {
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
		name     string
		active   string
		kbUnit   string
		wantRead bool
		wantWrite bool
	}{
		{"county reads self", "county", "county", true, true},
		{"county reads city", "county", "city", true, false},
		{"county reads province", "county", "prov", true, false},
		{"county cannot read sibling path", "county", "other", false, false},
		{"city cannot read county", "city", "county", false, false},
		{"province cannot read city", "prov", "city", false, false},
		{"unbound always readable", "county", "", true, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			gotRead, err := svc.CanReadKB(ctx, tenantID, testCase.active, testCase.kbUnit)
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
