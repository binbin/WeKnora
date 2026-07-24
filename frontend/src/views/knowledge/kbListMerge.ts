// Pure helpers backing the "All" scope of the knowledge-base list.
//
// The list template renders every card with `:key="kb.id"`. Vue requires
// `v-for` keys to be unique within a list — duplicate keys corrupt the
// virtual-DOM patch and the list renders empty or partial. The same
// knowledge base can legitimately show up more than once in the source
// data:
//
//   1. A KB the caller owns can also be shared back to them (e.g. they
//      belong to an org the KB was shared into), so it appears in both the
//      owned list and `sharedKnowledgeBases`.
//   2. The very same KB can be shared into the caller's view through more
//      than one organization. Each share is a distinct row (its own
//      `share_id`) but carries the identical `knowledge_base.id`.
//
// Either case yields two entries with the same `kb.id` once there are ≥2
// knowledge bases, which is the #795 symptom: a single KB renders fine
// (no collision) while two or more blank the page. De-duplicating by KB
// id here keeps the keys unique. Owned rows win over shared ones, and
// among multiple shares of the same KB we keep the most-privileged
// permission so the card stays as capable as the caller's best grant.

export interface OwnedKnowledgeBase {
  id: string;
  is_pinned?: boolean;
  pinned_at?: string;
  created_at?: string;
  creator_id?: string;
  org_unit_id?: string;
  org_unit_name?: string;
  [key: string]: unknown;
}

export interface SharedKnowledgeBaseLike {
  knowledge_base: {
    id: string;
    knowledge_count?: number;
    chunk_count?: number;
    [key: string]: unknown;
  } | null;
  permission: string;
  shared_at: string;
  share_id: string;
  organization_id?: string;
  org_name?: string;
  [key: string]: unknown;
}

export type MergedOwnedKnowledgeBase = OwnedKnowledgeBase & { isMine: true };
export type MergedSharedKnowledgeBase = Record<string, unknown> & {
  id: string;
  isMine: false;
  permission: string;
  shared_at: string;
  share_id: string;
  organization_id?: string;
  org_name?: string;
};
export type MergedKnowledgeBase =
  | MergedOwnedKnowledgeBase
  | MergedSharedKnowledgeBase;

// Permissions that grant write access. Mirrors EDITABLE_PERMS in
// KnowledgeBaseList.vue — kept local so this module stays free of any
// component/store imports and remains trivially unit-testable.
const EDITABLE_PERMS = new Set(['admin', 'editor']);

export function isSharedKbEditable(perm: string | undefined): boolean {
  return !!perm && EDITABLE_PERMS.has(perm);
}

// Higher rank = more privileged. Unknown permissions rank lowest so they
// never shadow a real grant when collapsing duplicate shares.
const PERMISSION_RANK: Record<string, number> = {
  admin: 3,
  editor: 2,
  viewer: 1,
};

function permissionRank(perm: string | undefined): number {
  return (perm && PERMISSION_RANK[perm]) || 0;
}

function isMyKb(
  kb: { creator_id?: string },
  currentUserId: string | undefined,
): boolean {
  return !!(
    kb.creator_id &&
    currentUserId &&
    kb.creator_id === currentUserId
  );
}

function pinnedTime(kb: OwnedKnowledgeBase): number {
  return kb.pinned_at ? Date.parse(kb.pinned_at) : 0;
}

/** Stable identity for a shared-from-org (协作空间) section. */
export function sharedOrgSectionId(
  entry: { organization_id?: string; org_name?: string },
): string {
  const orgId = entry.organization_id?.trim();
  if (orgId) return orgId;
  const orgName = entry.org_name?.trim();
  if (orgName) return `name:${orgName}`;
  return 'unknown';
}

function sharedOrgSortKey(
  entry: { organization_id?: string; org_name?: string },
): string {
  return (
    entry.org_name?.trim() ||
    entry.organization_id?.trim() ||
    ''
  ).toLowerCase();
}

/**
 * Stable identity for a same-tenant "other members / ancestor-shared"
 * section keyed by the KB's OrgUnit. Empty org_unit_id (legacy unbound)
 * collapses to one bucket so the list can still render a single header.
 */
export function tenantOrgSectionId(
  kb: { org_unit_id?: string },
): string {
  const orgUnitId = kb.org_unit_id?.trim();
  return orgUnitId || 'unbound';
}

function tenantOrgSortKey(kb: OwnedKnowledgeBase): string {
  return (kb.org_unit_id?.trim() || '').toLowerCase();
}

function createdTime(kb: OwnedKnowledgeBase): number {
  return kb.created_at ? Date.parse(kb.created_at) : 0;
}

/**
 * Build the de-duplicated, ordered list rendered by the "All" scope.
 *
 * Ordering:
 *   1. pinned KBs (any creator the caller pinned), newest pin first
 *   2. the caller's own non-pinned KBs
 *   3. teammate / ancestor-shared non-pinned KBs, grouped by org_unit_id
 *   4. cross-tenant shared KBs grouped by organization name, editable
 *      grants first within each organization
 *
 * On top of that, every entry is unique by knowledge-base id: owned rows
 * win over shared duplicates, and repeated shares of one KB collapse to
 * the most-privileged permission.
 */
export function mergeAllScopeKnowledgeBases(
  owned: OwnedKnowledgeBase[],
  shared: SharedKnowledgeBaseLike[],
  currentUserId: string | undefined,
): MergedKnowledgeBase[] {
  const result: MergedKnowledgeBase[] = [];

  const pinned: OwnedKnowledgeBase[] = [];
  const ownMine: OwnedKnowledgeBase[] = [];
  const teammateMine: OwnedKnowledgeBase[] = [];
  const ownedIds = new Set<string>();
  for (const kb of owned) {
    ownedIds.add(kb.id);
    if (kb.is_pinned) pinned.push(kb);
    else if (isMyKb(kb, currentUserId)) ownMine.push(kb);
    else teammateMine.push(kb);
  }
  pinned.sort((left, right) => pinnedTime(right) - pinnedTime(left));

  // Keep KBs from the same OrgUnit contiguous so the list can insert one
  // section header per organization (ancestor shares vs local teammates).
  teammateMine.sort((left, right) => {
    const orgCmp = tenantOrgSortKey(left).localeCompare(
      tenantOrgSortKey(right),
      undefined,
      { sensitivity: 'base' },
    );
    if (orgCmp !== 0) return orgCmp;
    return createdTime(right) - createdTime(left);
  });

  for (const kb of pinned) result.push({ ...kb, isMine: true as const });
  for (const kb of ownMine) result.push({ ...kb, isMine: true as const });
  for (const kb of teammateMine) {
    result.push({ ...kb, isMine: true as const });
  }

  // Collapse the shared rows by KB id, keeping the most-privileged grant,
  // and drop any KB the caller already owns. This is what guarantees a
  // unique `:key` per rendered card.
  const dedupedShared = new Map<string, SharedKnowledgeBaseLike>();
  for (const entry of shared) {
    const kb = entry?.knowledge_base;
    if (!kb) continue;
    if (ownedIds.has(kb.id)) continue;
    const existing = dedupedShared.get(kb.id);
    if (
      !existing ||
      permissionRank(entry.permission) > permissionRank(existing.permission)
    ) {
      dedupedShared.set(kb.id, entry);
    }
  }

  // Group by organization so the list can render one section header per
  // shared space; within an org keep editable grants ahead of view-only.
  const sortedShared = [...dedupedShared.values()].sort((left, right) => {
    const orgCmp = sharedOrgSortKey(left).localeCompare(
      sharedOrgSortKey(right),
      undefined,
      { sensitivity: 'base' },
    );
    if (orgCmp !== 0) return orgCmp;
    const leftEditable = isSharedKbEditable(left.permission) ? 0 : 1;
    const rightEditable = isSharedKbEditable(right.permission) ? 0 : 1;
    return leftEditable - rightEditable;
  });

  for (const sharedEntry of sortedShared) {
    const kb = sharedEntry.knowledge_base!;
    result.push({
      ...kb,
      isMine: false as const,
      permission: sharedEntry.permission,
      shared_at: sharedEntry.shared_at,
      share_id: sharedEntry.share_id,
      organization_id: sharedEntry.organization_id,
      org_name: sharedEntry.org_name,
      knowledge_count: kb.knowledge_count,
      chunk_count: kb.chunk_count,
    } as MergedSharedKnowledgeBase);
  }

  return result;
}
