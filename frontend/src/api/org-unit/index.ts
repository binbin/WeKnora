import { get, post, put, del } from '@/utils/request'

export interface OrgUnit {
  id: string
  tenant_id: number
  parent_id: string
  name: string
  code: string
  path: string
  depth: number
  sort_order: number
  created_at: string
  updated_at: string
  children?: OrgUnit[]
}

export interface OrgUnitMember {
  id: string
  org_unit_id: string
  tenant_id: number
  user_id: string
  is_primary: boolean
  created_at: string
  updated_at: string
  org_unit?: OrgUnit
}

export interface OrgUnitVisibility {
  current_id: string
  readable_ids: string[]
  writable_id: string
  has_hierarchy: boolean
}

const ORG_UNIT_STORAGE_KEY = 'weknora_org_unit_id'
const ORG_UNIT_USER_STORAGE_KEY = 'weknora_org_unit_user_id'

export function getStoredOrgUnitId(): string {
  return localStorage.getItem(ORG_UNIT_STORAGE_KEY) || ''
}

export function clearStoredOrgUnitId(): void {
  localStorage.removeItem(ORG_UNIT_STORAGE_KEY)
  localStorage.removeItem(ORG_UNIT_USER_STORAGE_KEY)
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent('weknora-org-unit-changed'))
  }
}

export function setStoredOrgUnitId(orgUnitId: string): void {
  if (orgUnitId) {
    localStorage.setItem(ORG_UNIT_STORAGE_KEY, orgUnitId)
    try {
      const raw = localStorage.getItem('weknora_user')
      if (raw) {
        const user = JSON.parse(raw) as { id?: string }
        if (user?.id) {
          localStorage.setItem(ORG_UNIT_USER_STORAGE_KEY, user.id)
        }
      }
    } catch {
      // ignore parse errors
    }
  } else {
    localStorage.removeItem(ORG_UNIT_STORAGE_KEY)
    localStorage.removeItem(ORG_UNIT_USER_STORAGE_KEY)
  }
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent('weknora-org-unit-changed'))
  }
}

/**
 * 是否附带 X-Org-Unit-ID。
 * 有选中组织就发送——含系统管理员；清空 localStorage 表示显式「所有」。
 * 旧逻辑对 is_system_admin 一律不发 header，导致超管永远看到下级知识库。
 */
export function shouldSendOrgUnitHeader(): boolean {
  return getStoredOrgUnitId().trim() !== ''
}

export function getRequestOrgUnitId(): string {
  if (!shouldSendOrgUnitHeader()) return ''
  return getStoredOrgUnitId().trim()
}

/**
 * Ensure localStorage has an OrgUnit when the user has memberships.
 * System admins previously stayed on empty「所有」forever and listed
 * every subordinate KB; hydrate to primary/home so lists are scoped.
 * Cross-tenant operators (can_access_all_tenants) keep empty = 所有.
 */
export function allowAllOrgsDefaultFromUserStorage(): boolean {
  try {
    const raw = localStorage.getItem('weknora_user')
    if (!raw) return false
    const user = JSON.parse(raw) as { can_access_all_tenants?: boolean }
    return user?.can_access_all_tenants === true
  } catch {
    return false
  }
}

function currentUserIdFromStorage(): string {
  try {
    const raw = localStorage.getItem('weknora_user')
    if (!raw) return ''
    const user = JSON.parse(raw) as { id?: string }
    return user?.id?.trim() || ''
  } catch {
    return ''
  }
}

/**
 * Ensure localStorage has an OrgUnit for the *current* user.
 * Clears stale IDs left by a previous account (same browser) — that bug
 * made parent-org admins keep a subordinate X-Org-Unit-ID while the UI
 * label fell back to their home name, so subordinate KBs still appeared.
 */
export async function ensureStoredOrgUnitFromMembership(options?: {
  allowAllOrgsDefault?: boolean
}): Promise<string> {
  const allowAll =
    options?.allowAllOrgsDefault ?? allowAllOrgsDefaultFromUserStorage()
  const currentUserId = currentUserIdFromStorage()
  const storedUserId = (
    localStorage.getItem(ORG_UNIT_USER_STORAGE_KEY) || ''
  ).trim()
  // Different user (or missing owner tag from older builds): drop stale scope.
  if (
    currentUserId &&
    storedUserId &&
    storedUserId !== currentUserId
  ) {
    clearStoredOrgUnitId()
  } else if (currentUserId && !storedUserId && getStoredOrgUnitId()) {
    // Legacy rows without owner tag — treat as untrusted across accounts.
    clearStoredOrgUnitId()
  }

  const existing = getStoredOrgUnitId().trim()
  if (existing) {
    if (currentUserId && !storedUserId) {
      localStorage.setItem(ORG_UNIT_USER_STORAGE_KEY, currentUserId)
    }
    return existing
  }
  if (allowAll) {
    return ''
  }
  try {
    const memberships = await listMyOrgUnitMemberships()
    const preferred =
      memberships.find((item) => item.is_primary) || memberships[0]
    const orgUnitId = preferred?.org_unit_id?.trim() || ''
    if (orgUnitId) {
      setStoredOrgUnitId(orgUnitId)
    }
    return orgUnitId
  } catch {
    return ''
  }
}

export async function listOrgUnits(
  asTree = true,
  options?: { platform?: boolean },
): Promise<OrgUnit[]> {
  const params = new URLSearchParams()
  params.set('tree', asTree ? '1' : '0')
  if (options?.platform) {
    params.set('scope', 'platform')
  }
  const response = await get(`/api/v1/org-units?${params.toString()}`)
  return response?.data ?? []
}

export async function listInviteableOrgUnits(
  role: string,
): Promise<OrgUnit[]> {
  const response = await get(
    `/api/v1/org-units/inviteable?role=${encodeURIComponent(role || 'contributor')}`,
  )
  return response?.data ?? []
}

export async function createOrgUnit(payload: {
  name: string
  code?: string
  parent_id?: string
  sort_order?: number
}): Promise<OrgUnit> {
  const response = await post('/api/v1/org-units', payload)
  return response.data
}

export async function updateOrgUnit(
  id: string,
  payload: { name?: string; code?: string; sort_order?: number },
): Promise<OrgUnit> {
  const response = await put(`/api/v1/org-units/${id}`, payload)
  return response.data
}

export async function deleteOrgUnit(id: string): Promise<void> {
  await del(`/api/v1/org-units/${id}`)
}

export async function moveOrgUnit(
  id: string,
  parentId: string,
): Promise<OrgUnit> {
  const response = await post(`/api/v1/org-units/${id}/move`, {
    parent_id: parentId,
  })
  return response.data
}

export async function listMyOrgUnitMemberships(): Promise<OrgUnitMember[]> {
  const response = await get('/api/v1/org-units/me')
  return response?.data ?? []
}

/** @deprecated Prefer header-only scope via setStoredOrgUnitId; kept for API compat. */
export async function setPrimaryOrgUnit(id: string): Promise<void> {
  await post(`/api/v1/org-units/${id}/primary`, {})
}

export async function transferOrgUnitMember(
  userId: string,
  toOrgUnitId: string,
): Promise<OrgUnitMember> {
  const response = await post('/api/v1/org-units/members/transfer', {
    user_id: userId,
    to_org_unit_id: toOrgUnitId,
  })
  return response.data
}

export async function getOrgUnitVisibility(): Promise<OrgUnitVisibility> {
  const response = await get('/api/v1/org-units/visibility')
  return response.data
}

export async function addOrgUnitMember(
  orgUnitId: string,
  userId: string,
  isPrimary = false,
): Promise<OrgUnitMember> {
  const response = await post(`/api/v1/org-units/${orgUnitId}/members`, {
    user_id: userId,
    is_primary: isPrimary,
  })
  return response.data
}

export async function removeOrgUnitMember(
  orgUnitId: string,
  userId: string,
): Promise<void> {
  await del(`/api/v1/org-units/${orgUnitId}/members/${userId}`)
}
