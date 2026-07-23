import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import {
  getStoredOrgUnitId,
  listMyOrgUnitMemberships,
} from '@/api/org-unit'

/** OrgUnit 切换时派发，供列表侧栏等刷新「所在组织」文案。 */
export const ORG_UNIT_CHANGED_EVENT = 'weknora-org-unit-changed'

const sharedOrgUnitName = ref('')
let loadToken = 0

async function loadOrgUnitName(
  canSkip: boolean,
): Promise<void> {
  const token = ++loadToken
  if (canSkip) {
    sharedOrgUnitName.value = ''
    return
  }
  try {
    const memberships = await listMyOrgUnitMemberships()
    if (token !== loadToken) return
    const storedId = getStoredOrgUnitId()
    const preferred =
      memberships.find((item) => item.org_unit_id === storedId) ||
      memberships.find((item) => item.is_primary) ||
      memberships[0]
    sharedOrgUnitName.value = preferred?.org_unit?.name?.trim() || ''
  } catch {
    if (token !== loadToken) return
    sharedOrgUnitName.value = ''
  }
}

/**
 * 知识库 / 智能体列表中「本空间」位的展示名：
 * - 超管（跨租户或系统管理员）→「所有」
 * - 其余 → 当前所在组织（OrgUnit）名，缺省回退到当前空间名
 */
export function useWorkspaceScopeLabel() {
  const { t } = useI18n()
  const authStore = useAuthStore()

  const isSuperAdmin = computed(
    () => authStore.canAccessAllTenants || authStore.isSystemAdmin,
  )

  const refresh = () => {
    void loadOrgUnitName(isSuperAdmin.value)
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
    ],
    () => {
      refresh()
    },
  )

  const workspaceScopeLabel = computed(() => {
    if (isSuperAdmin.value) {
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
    isSuperAdminScope: isSuperAdmin,
  }
}
