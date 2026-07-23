import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import {
  ensureStoredOrgUnitFromMembership,
  getStoredOrgUnitId,
  listMyOrgUnitMemberships,
  listOrgUnits,
} from '@/api/org-unit'

/** OrgUnit 切换时派发，供列表侧栏等刷新「所在组织」文案。 */
export const ORG_UNIT_CHANGED_EVENT = 'weknora-org-unit-changed'

const sharedOrgUnitName = ref('')
let loadToken = 0

function findOrgUnitName(
  units: Array<{ id: string; name: string; children?: unknown[] }>,
  targetId: string,
): string {
  for (const unit of units) {
    if (unit.id === targetId) {
      return unit.name?.trim() || ''
    }
    const nested = unit.children as
      | Array<{ id: string; name: string; children?: unknown[] }>
      | undefined
    if (nested?.length) {
      const found = findOrgUnitName(nested, targetId)
      if (found) return found
    }
  }
  return ''
}

async function loadOrgUnitName(
  allowAllOrgsDefault: boolean,
): Promise<void> {
  const token = ++loadToken
  try {
    await ensureStoredOrgUnitFromMembership({
      allowAllOrgsDefault,
    })
    if (token !== loadToken) return

    const storedId = getStoredOrgUnitId()
    if (!storedId) {
      sharedOrgUnitName.value = ''
      return
    }

    // Prefer the name of the *active* (stored) unit — not membership
    // fallback — so a subordinate selection is not mislabeled as home.
    const memberships = await listMyOrgUnitMemberships()
    if (token !== loadToken) return
    const fromMembership = memberships.find(
      (item) => item.org_unit_id === storedId,
    )
    if (fromMembership?.org_unit?.name?.trim()) {
      sharedOrgUnitName.value = fromMembership.org_unit.name.trim()
      return
    }

    try {
      const tree = await listOrgUnits(true)
      if (token !== loadToken) return
      const fromTree = findOrgUnitName(tree, storedId)
      if (fromTree) {
        sharedOrgUnitName.value = fromTree
        return
      }
    } catch {
      // fall through to membership primary name
    }

    const primary =
      memberships.find((item) => item.is_primary) || memberships[0]
    sharedOrgUnitName.value = primary?.org_unit?.name?.trim() || ''
  } catch {
    if (token !== loadToken) return
    sharedOrgUnitName.value = ''
  }
}

/**
 * 知识库 / 智能体列表中「本空间」位的展示名：
 * - 显式未选组织（清空）且可看全林 →「所有」
 * - 其余 → 当前活跃 OrgUnit 名（与 X-Org-Unit-ID 一致）
 */
export function useWorkspaceScopeLabel() {
  const { t } = useI18n()
  const authStore = useAuthStore()

  const canBrowseAllOrgs = computed(
    () => authStore.canAccessAllTenants || authStore.isSystemAdmin,
  )

  const refresh = () => {
    void loadOrgUnitName(authStore.canAccessAllTenants === true)
  }

  onMounted(() => {
    refresh()
    window.addEventListener(ORG_UNIT_CHANGED_EVENT, refresh)
  })

  onUnmounted(() => {
    window.removeEventListener(ORG_UNIT_CHANGED_EVENT, refresh)
  })

  watch(
    () => [
      authStore.effectiveTenantId,
      authStore.canAccessAllTenants,
      authStore.isSystemAdmin,
      authStore.user?.id,
    ],
    () => {
      refresh()
    },
  )

  const workspaceScopeLabel = computed(() => {
    const storedId = getStoredOrgUnitId().trim()
    if (canBrowseAllOrgs.value && !storedId) {
      return t('listSpaceSidebar.allOrgs')
    }
    return (
      sharedOrgUnitName.value ||
      authStore.currentTenantName ||
      t('listSpaceSidebar.workspace')
    )
  })

  return {
    workspaceScopeLabel,
    refreshWorkspaceScopeLabel: refresh,
    isSuperAdminScope: canBrowseAllOrgs,
  }
}
