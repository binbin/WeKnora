# Org-Unit Single Membership & Space Scoping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce one OrgUnit membership per user per Tenant, add atomic transfer (调岗), and keep tree-internal KB sharing (`share_with_descendants`) clearly separated from cross-space Organization sharing in product copy.

**Architecture:** Keep one Tenant per customer with an OrgUnit tree. Tighten `AddMember` to reject a second unit in the same tenant; add `TransferMember` for move; migrate duplicate rows and add a unique index on `(tenant_id, user_id)`. Frontend stops treating `is_primary` / multi-membership as a product feature; members page gains transfer; i18n clarifies「共享给下级」vs「共享到共享空间」.

**Tech Stack:** Go (Gin, GORM), PostgreSQL + SQLite migrations, Vue 3 + TDesign, existing `org_unit_test.go` stub pattern.

**Spec:** `docs/superpowers/specs/2026-07-23-org-unit-space-scoping-design.md`

---

## File map

| File | Responsibility |
|------|----------------|
| `migrations/versioned/000078_org_unit_single_membership.up.sql` | Dedupe + unique `(tenant_id, user_id)` |
| `migrations/versioned/000078_org_unit_single_membership.down.sql` | Drop unique; restore non-unique index |
| `migrations/sqlite/000008_org_unit_single_membership.up.sql` | SQLite equivalent |
| `migrations/sqlite/000008_org_unit_single_membership.down.sql` | SQLite down |
| `internal/application/repository/org_unit.go` | `RemoveMembersByTenantUser`; transactional transfer helper |
| `internal/types/interfaces/org_unit.go` | `TransferMember`; repo method; update comments |
| `internal/application/service/org_unit.go` | Single-membership `AddMember`; `TransferMember`; simplify resolve |
| `internal/application/service/org_unit_test.go` | Failing→passing membership tests |
| `internal/handler/org_unit.go` | Transfer handler; ignore `is_primary` on add |
| `internal/router/router.go` | Register transfer route |
| `internal/types/org_unit.go` | `TransferOrgUnitMemberRequest`; comment `is_primary` as legacy |
| `internal/application/service/tenant_invitation.go` | Accept path uses transfer/ensure-single |
| `frontend/src/api/org-unit/index.ts` | `transferOrgUnitMember`; stop relying on set-primary |
| `frontend/src/views/settings/OrgUnitSettings.vue` | Scope switch = header only; no set-primary |
| `frontend/src/composables/useWorkspaceScopeLabel.ts` | Single membership / first membership |
| `frontend/src/views/settings/TenantMembers.vue` | Transfer org action for existing members |
| `frontend/src/i18n/locales/{zh-CN,en-US,ko-KR,ru-RU}.ts` | Copy boundaries |

Out of scope (already works / non-goals): `CanReadKB` / `share_with_descendants` logic, Organization + `kb_shares`, per-OrgUnit quotas, dropping `is_primary` column.

---

### Task 1: Migration — dedupe + unique membership

**Files:**
- Create: `migrations/versioned/000078_org_unit_single_membership.up.sql`
- Create: `migrations/versioned/000078_org_unit_single_membership.down.sql`
- Create: `migrations/sqlite/000008_org_unit_single_membership.up.sql`
- Create: `migrations/sqlite/000008_org_unit_single_membership.down.sql`

- [ ] **Step 1: Write PostgreSQL up migration**

Keep one row per `(tenant_id, user_id)`: prefer `is_primary = true`, else oldest `created_at`, else smallest `id`. Then replace non-unique index with unique.

```sql
-- Migration: 000078_org_unit_single_membership
-- One OrgUnit membership per user per tenant.

DO $$ BEGIN RAISE NOTICE
  '[Migration 000078] Enforcing single org_unit membership...';
END $$;

DELETE FROM org_unit_members a
USING org_unit_members b
WHERE a.tenant_id = b.tenant_id
  AND a.user_id = b.user_id
  AND a.id <> b.id
  AND (
    (b.is_primary = TRUE AND a.is_primary = FALSE)
    OR (
      a.is_primary = b.is_primary
      AND (
        a.created_at > b.created_at
        OR (a.created_at = b.created_at AND a.id > b.id)
      )
    )
  );

DROP INDEX IF EXISTS idx_org_unit_members_tenant_user;

CREATE UNIQUE INDEX IF NOT EXISTS idx_org_unit_members_tenant_user_unique
    ON org_unit_members (tenant_id, user_id);

COMMENT ON TABLE org_unit_members IS
    'Maps users to exactly one OrgUnit per tenant; is_primary is legacy';

DO $$ BEGIN RAISE NOTICE '[Migration 000078] done'; END $$;
```

- [ ] **Step 2: Write PostgreSQL down migration**

```sql
DROP INDEX IF EXISTS idx_org_unit_members_tenant_user_unique;

CREATE INDEX IF NOT EXISTS idx_org_unit_members_tenant_user
    ON org_unit_members (tenant_id, user_id);

COMMENT ON TABLE org_unit_members IS
    'Maps users to OrgUnits within a tenant; is_primary marks default unit';
```

- [ ] **Step 3: Write SQLite up/down**

SQLite cannot `DELETE … USING`. Use:

```sql
-- up
DELETE FROM org_unit_members
WHERE id NOT IN (
  SELECT id FROM (
    SELECT id,
           ROW_NUMBER() OVER (
             PARTITION BY tenant_id, user_id
             ORDER BY is_primary DESC, created_at ASC, id ASC
           ) AS rn
    FROM org_unit_members
  ) ranked
  WHERE rn = 1
);

DROP INDEX IF EXISTS idx_org_unit_members_tenant_user;

CREATE UNIQUE INDEX IF NOT EXISTS idx_org_unit_members_tenant_user_unique
    ON org_unit_members (tenant_id, user_id);
```

Down: drop unique, recreate non-unique `idx_org_unit_members_tenant_user`.

If the project’s SQLite version lacks window functions, use a correlated subquery keeping `MIN(id)` among preferred rows — match whatever pattern other SQLite migrations in this repo use.

- [ ] **Step 4: Commit**

```bash
git add migrations/versioned/000078_org_unit_single_membership.*.sql \
  migrations/sqlite/000008_org_unit_single_membership.*.sql
git commit -m "chore(db): enforce one org unit membership per tenant user"
```

---

### Task 2: Repository — remove-by-tenant-user + transactional transfer

**Files:**
- Modify: `internal/types/interfaces/org_unit.go` (OrgUnitRepository)
- Modify: `internal/application/repository/org_unit.go`
- Modify: `internal/application/service/org_unit_test.go` (stub)

- [ ] **Step 1: Extend repository interface**

In `OrgUnitRepository` add:

```go
// RemoveMembersByTenantUser deletes all org_unit_members rows for
// the user in the tenant (0 or 1 row after unique constraint).
RemoveMembersByTenantUser(ctx context.Context, tenantID uint64, userID string) error

// TransferMember atomically moves the user to toOrgUnitID within
// tenantID (delete any existing memberships, then insert).
TransferMember(
	ctx context.Context,
	member *types.OrgUnitMember,
) error
```

`TransferMember` receives the new row to insert (caller fills ID, timestamps, `IsPrimary: true`).

- [ ] **Step 2: Implement in `org_unit.go` repository**

```go
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
```

- [ ] **Step 3: Update stub in `org_unit_test.go`**

Implement stub methods so the file compiles:

```go
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
	_ = r.RemoveMembersByTenantUser(context.Background(), member.TenantID, member.UserID)
	r.members = append(r.members, member)
	return nil
}
```

Also make `AddMember` / `GetMember` / `ListUserMemberships` operate on `r.members` in-memory when writing membership tests in Task 3 (extend stub as needed in that task).

- [ ] **Step 4: Commit**

```bash
git add internal/types/interfaces/org_unit.go \
  internal/application/repository/org_unit.go \
  internal/application/service/org_unit_test.go
git commit -m "feat(org-unit): repo helpers for single-membership transfer"
```

---

### Task 3: Service — AddMember conflict + TransferMember (TDD)

**Files:**
- Modify: `internal/types/interfaces/org_unit.go` (OrgUnitService)
- Modify: `internal/application/service/org_unit.go`
- Modify: `internal/application/service/org_unit_test.go`
- Modify: `internal/types/org_unit.go` (comments / request types)

- [ ] **Step 1: Write failing tests**

Add to `org_unit_test.go` (extend stub so `GetMember`, `AddMember`, `ListUserMemberships` mutate/read `r.members`):

```go
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
```

- [ ] **Step 2: Run tests — expect fail**

```bash
go test ./internal/application/service/ -run 'TestAddMember|TestTransferMember' -count=1
```

Expected: FAIL (TransferMember undefined and/or AddMember still allows second unit).

- [ ] **Step 3: Implement service API**

Add to `OrgUnitService`:

```go
TransferMember(
	ctx context.Context,
	tenantID uint64,
	userID string,
	toOrgUnitID string,
) (*types.OrgUnitMember, error)
```

Rewrite `AddMember`:

1. Validate org unit exists; trim `userID`.
2. `ListUserMemberships(tenantID, userID)`.
3. If any membership has `OrgUnitID == orgUnitID` → return it (idempotent).
4. If any other membership exists → `apperrors.NewConflictError("user already belongs to another org unit; use transfer")`.
5. Insert with `IsPrimary: true` always (ignore caller `isPrimary` bool for product semantics; keep param for call-site compat or remove in a follow-up — prefer keep signature, ignore value).

Implement `TransferMember`:

1. Validate `toOrgUnitID` exists in tenant.
2. If already sole member of `toOrgUnitID` → return existing.
3. Build new member (`IsPrimary: true`) and call `repo.TransferMember`.

Simplify `ResolveActiveOrgUnit` fallback (no header): use the single membership if present (still tolerate multiple during rollout by preferring `IsPrimary` then first).

- [ ] **Step 4: Re-run tests — expect pass**

```bash
go test ./internal/application/service/ -run 'TestAddMember|TestTransferMember|TestOrgUnit' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/types/interfaces/org_unit.go \
  internal/application/service/org_unit.go \
  internal/application/service/org_unit_test.go \
  internal/types/org_unit.go
git commit -m "feat(org-unit): enforce single membership and transfer"
```

---

### Task 4: HTTP handler + router

**Files:**
- Modify: `internal/types/org_unit.go`
- Modify: `internal/handler/org_unit.go`
- Modify: `internal/router/router.go`

- [ ] **Step 1: Add request type**

```go
// TransferOrgUnitMemberRequest is the body for
// POST /org-units/members/transfer.
type TransferOrgUnitMemberRequest struct {
	UserID      string `json:"user_id"       binding:"required"`
	ToOrgUnitID string `json:"to_org_unit_id" binding:"required"`
}
```

Keep `AddOrgUnitMemberRequest.IsPrimary` for wire compat but document as ignored.

- [ ] **Step 2: Handler**

```go
func (h *OrgUnitHandler) TransferMember(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	var req types.TransferOrgUnitMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError(err.Error()))
		return
	}
	member, err := h.orgUnitService.TransferMember(
		ctx, tenantID, req.UserID, req.ToOrgUnitID,
	)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": member})
}
```

`AddMember` continues to pass `req.IsPrimary` into service (service ignores / forces true).

- [ ] **Step 3: Register route**

Near existing org-unit member routes in `router.go` (same auth as AddMember — admin/manage member capability):

```go
units.POST("/members/transfer", g.Admin() /* or existing manage-member guard */, orgUnitHandler.TransferMember)
```

Place **before** `/:id/...` routes if Gin would otherwise capture `members` as `:id`. Inspect current route block and register so `members/transfer` is not shadowed.

Leave `POST /:id/primary` registered for backward compat; no new UI calls it.

- [ ] **Step 4: Commit**

```bash
git add internal/types/org_unit.go internal/handler/org_unit.go internal/router/router.go
git commit -m "feat(org-unit): add transfer member HTTP API"
```

---

### Task 5: Invitation accept uses single-membership path

**Files:**
- Modify: `internal/application/service/tenant_invitation.go`

- [ ] **Step 1: Change `assignOrgUnitMembership`**

Accepting an invitation must place the user in the invitation’s OrgUnit even if they somehow already had another (re-invite / data repair). Use `TransferMember` instead of `AddMember`:

```go
func (s *tenantInvitationService) assignOrgUnitMembership(
	ctx context.Context,
	tenantID uint64,
	userID string,
	orgUnitID string,
) {
	if s.orgUnitService == nil || orgUnitID == "" || userID == "" {
		return
	}
	if _, err := s.orgUnitService.TransferMember(
		ctx, tenantID, userID, orgUnitID,
	); err != nil {
		logger.Errorf(ctx,
			"invitation accepted but org_unit membership failed: tenant=%d user=%s unit=%s err=%v",
			tenantID, userID, orgUnitID, err)
	}
}
```

Update the comment above the function accordingly.

- [ ] **Step 2: Compile / run related tests**

```bash
go test ./internal/application/service/ -count=1 -run 'Invitation|OrgUnit|AddMember|Transfer'
```

Expected: PASS (or only pre-existing failures unrelated to this change).

- [ ] **Step 3: Commit**

```bash
git add internal/application/service/tenant_invitation.go
git commit -m "fix(invitation): assign org unit via transfer for single membership"
```

---

### Task 6: Frontend API + scope label + OrgUnit settings

**Files:**
- Modify: `frontend/src/api/org-unit/index.ts`
- Modify: `frontend/src/composables/useWorkspaceScopeLabel.ts`
- Modify: `frontend/src/views/settings/OrgUnitSettings.vue`

- [ ] **Step 1: API**

Add:

```typescript
export async function transferOrgUnitMember(
  userId: string,
  toOrgUnitId: string,
): Promise<OrgUnitMember> {
  const response = await post('/org-units/members/transfer', {
    user_id: userId,
    to_org_unit_id: toOrgUnitId,
  })
  return unwrap(response) // match existing unwrap helpers in this file
}
```

Keep `setPrimaryOrgUnit` exported but unused by product pages (or mark deprecated in comment). Prefer not deleting until no callers.

- [ ] **Step 2: `useWorkspaceScopeLabel.ts`**

When resolving label from memberships, use the sole membership (or `[0]`), do not prefer `is_primary` as a multi-org concept:

```typescript
const membership = memberships[0]
```

- [ ] **Step 3: `OrgUnitSettings.vue` — scope switch without SetPrimary**

In `onActiveChange` / select handler:

- Call `setStoredOrgUnitId(value)` only (drives `X-Org-Unit-ID`).
- **Do not** call `setPrimaryOrgUnit`.
- For non-admin users with a single membership: show read-only current org name (disable/hide the select), since they cannot belong to multiple units.

Admins/Owners may still pick any unit for browsing (existing middleware allows that without membership).

- [ ] **Step 4: Manual smoke (dev)**

1. Login as admin with hierarchy.
2. Change「当前组织」— lists filter by header; membership row count still 1.
3. Confirm network tab has no `POST .../primary`.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/api/org-unit/index.ts \
  frontend/src/composables/useWorkspaceScopeLabel.ts \
  frontend/src/views/settings/OrgUnitSettings.vue
git commit -m "fix(frontend): scope switch without multi-org primary"
```

---

### Task 7: Members page — transfer (调岗)

**Files:**
- Modify: `frontend/src/views/settings/TenantMembers.vue` (and `TenantMembersPage.vue` wrapper if needed only for i18n keys)
- Modify: `frontend/src/i18n/locales/zh-CN.ts` (and en/ko/ru keys used)

- [ ] **Step 1: UI**

On the members table, for rows that have `org_unit_id` (when hierarchy exists), add an action「调整组织」:

- Dialog / select of inviteable or full tree (same options source as invite: prefer `listInviteableOrgUnits` for the actor’s role, or `listOrgUnits` for owner/admin).
- On confirm: `transferOrgUnitMember(row.user_id, selectedOrgUnitId)` then reload members.
- Disable if target equals current `org_unit_id`.
- Show conflict/error message from API as-is via `MessagePlugin`.

- [ ] **Step 2: i18n keys (zh-CN example)**

```typescript
tenantMember: {
  // ...
  transferOrgUnit: '调整组织',
  transferOrgUnitTitle: '将成员调整到其他组织',
  transferOrgUnitConfirm: '确认调整',
  transferOrgUnitSuccess: '已调整组织',
}
```

Mirror in `en-US`, `ko-KR`, `ru-RU` with equivalent meaning (not machine-garbled placeholders).

- [ ] **Step 3: Smoke**

1. Two org units under same tenant.
2. Member in unit A → transfer to B → table shows B; member lists only B’s KBs + ancestor shared.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/views/settings/TenantMembers.vue \
  frontend/src/i18n/locales/zh-CN.ts \
  frontend/src/i18n/locales/en-US.ts \
  frontend/src/i18n/locales/ko-KR.ts \
  frontend/src/i18n/locales/ru-RU.ts
git commit -m "feat(frontend): transfer member between org units"
```

---

### Task 8: Product copy — two kinds of「共享」

**Files:**
- Modify: `frontend/src/i18n/locales/zh-CN.ts`
- Modify: `frontend/src/i18n/locales/en-US.ts`
- Modify: `frontend/src/i18n/locales/ko-KR.ts`
- Modify: `frontend/src/i18n/locales/ru-RU.ts`

- [ ] **Step 1: Align strings**

Ensure knowledge editor uses **共享给下级 / Share with subordinate organizations** (already largely correct).

Change Organization share CTA where it still says ambiguous「共享到空间」to **共享到共享空间** / **Share to shared space**:

| Key area | zh-CN target |
|----------|----------------|
| `knowledgeEditor.basic.shareWithDescendantsLabel` | `是否共享给下级机构` (keep) |
| `knowledgeEditor.basic.shareWithDescendantsTip` | Keep: 下级只读引用；默认不共享 |
| Organization share button `shareToSpace` | `共享到共享空间` |
| en `shareToSpace` | `Share to shared space` |

Do not rename menu `organizations: "共享空间"` — already correct.

- [ ] **Step 2: Quick grep**

```bash
rg -n "共享到空间|Share to space" frontend/src/i18n/locales/
```

Expected: remaining hits only intentional or updated.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/i18n/locales/*.ts
git commit -m "docs(i18n): distinguish subordinate share vs shared space"
```

---

### Task 9: KB create path sanity check (no behavior change expected)

**Files:**
- Read-only verify: `internal/application/service/knowledgebase.go` (create sets `OrgUnitID` from context)
- Read-only verify: `frontend/src/utils/request.ts` sends `X-Org-Unit-ID` from `getStoredOrgUnitId()`

- [ ] **Step 1: Confirm create path**

If create already does:

```go
if orgUnitID, ok := types.OrgUnitIDFromContext(ctx); ok {
    kb.OrgUnitID = orgUnitID
}
```

and the request interceptor always attaches the header when stored — **no code change**.

If header is missing for normal members who have a membership, fix interceptor to fall back to `listMyOrgUnitMemberships()[0]` once per session (only if a real gap is found).

- [ ] **Step 2: Add a focused regression test only if a gap is found**

Example name: `TestCreateKnowledgeBaseBindsActiveOrgUnit` in existing KB service tests. Skip this step if verification shows binding already correct.

- [ ] **Step 3: Commit only if code changed**

```bash
git commit -m "fix(kb): ensure new knowledge bases bind active org unit"
```

---

## Spec coverage checklist

| Spec requirement | Task |
|------------------|------|
| One Tenant + OrgUnit tree (no split for subordinates) | Already architecture; no task |
| `share_with_descendants` for downward share | Already implemented; Task 8 copy only |
| Organization only for cross-tenant | Already; Task 8 copy |
| Single OrgUnit membership per tenant user | Tasks 1–3 |
| Transfer = atomic rebind | Tasks 2–4, 7 |
| Invitation places user in one unit | Task 5 |
| Scope UI without multi-org primary | Task 6 |
| New KB binds current org unit | Task 9 |
| No per-OrgUnit quotas / sibling auto-share | Explicit non-goals |

## Self-review notes

- No TBD placeholders in tasks.
- `TransferMember` naming consistent across repo/service/handler/API.
- Unique index is `(tenant_id, user_id)` so platform catalog (`tenant_id=0`) and business tenant remain independent.
- `is_primary` column retained; product always writes `true` on the sole row.
