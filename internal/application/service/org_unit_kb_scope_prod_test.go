package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

func TestCanReadKB_ParentCannotSeeSharedDescendantKB(t *testing.T) {
	// Mirrors production: 内蒙古人社厅 → 赤峰市 → 赤峰市本级,
	// KB bound to 赤峰市本级 with share_with_descendants=true.
	const (
		prov   = "105970e2-6acd-4204-863e-7f56a021d402"
		city   = "d60fec9c-f805-4c45-a900-f2df87eead8b"
		cityL1 = "2b04ec32-9da7-4714-b4da-df3df7411a66"
	)
	repo := &stubOrgUnitRepo{
		units: map[string]*types.OrgUnit{
			prov: {
				ID: prov, TenantID: 10000, Path: "/" + prov + "/", Depth: 0, Name: "内蒙古人社厅",
			},
			city: {
				ID: city, TenantID: 10000, ParentID: prov,
				Path: "/" + prov + "/" + city + "/", Depth: 1, Name: "赤峰市",
			},
			cityL1: {
				ID: cityL1, TenantID: 10000, ParentID: city,
				Path: "/" + prov + "/" + city + "/" + cityL1 + "/", Depth: 2, Name: "赤峰市本级",
			},
		},
	}
	svc := NewOrgUnitService(repo)
	ctx := context.Background()

	ok, err := svc.CanReadKB(ctx, 10000, prov, cityL1, true)
	if err != nil {
		t.Fatalf("CanReadKB: %v", err)
	}
	if ok {
		t.Fatal("parent must not read descendant KB even when share_with_descendants=true")
	}
}
