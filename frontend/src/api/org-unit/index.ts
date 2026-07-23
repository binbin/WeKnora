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

export function getStoredOrgUnitId(): string {
  return localStorage.getItem(ORG_UNIT_STORAGE_KEY) || ''
}

export function setStoredOrgUnitId(orgUnitId: string): void {
  if (orgUnitId) {
    localStorage.setItem(ORG_UNIT_STORAGE_KEY, orgUnitId)
  } else {
    localStorage.removeItem(ORG_UNIT_STORAGE_KEY)
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
