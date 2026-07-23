<template>
  <div class="org-unit-settings">
    <div class="section-header">
      <h3 class="section-title">组织层级</h3>
      <p class="section-desc">
        平台组织树：根组织由系统管理员创建；成员登录后自动进入「组织名的空间」，
        本组织与下级组织用户共享该空间。
      </p>
    </div>

    <div class="active-unit" v-if="memberships.length || tree.length">
      <label class="field-label">当前组织</label>
      <t-select
        v-if="canSwitchScope"
        v-model="activeOrgUnitId"
        :options="flatOptions"
        placeholder="选择当前组织"
        clearable
        @change="onActiveChange"
      />
      <div v-else class="readonly-org">
        {{ currentOrgUnitLabel }}
      </div>
    </div>

    <div class="toolbar" v-if="canManage">
      <t-input
        v-model="newName"
        placeholder="新组织名称"
        class="name-input"
      />
      <t-select
        v-model="newParentId"
        :options="parentOptions"
        :placeholder="canCreateRoot ? '上级（空=根，仅超管）' : '上级组织（必选）'"
        :clearable="canCreateRoot"
        class="parent-select"
      />
      <t-button theme="primary" :loading="creating" @click="onCreate">
        添加
      </t-button>
    </div>

    <t-loading :loading="loading">
      <div v-if="!tree.length" class="empty">
        <template v-if="canCreateRoot">
          尚未配置组织树。超级管理员可在此创建根组织（如省级）。
        </template>
        <template v-else>
          尚未配置组织树。根组织仅超级管理员可创建；管理员可在根下添加下级。
        </template>
      </div>
      <ul v-else class="tree">
        <OrgUnitTreeNode
          v-for="node in tree"
          :key="node.id"
          :node="node"
          :can-manage="canManage"
          :active-id="activeOrgUnitId"
          @select="selectUnit"
          @delete="onDelete"
        />
      </ul>
    </t-loading>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useAuthStore } from '@/stores/auth'
import {
  createOrgUnit,
  deleteOrgUnit,
  getStoredOrgUnitId,
  listMyOrgUnitMemberships,
  listOrgUnits,
  setStoredOrgUnitId,
  type OrgUnit,
  type OrgUnitMember,
} from '@/api/org-unit'
import OrgUnitTreeNode from './OrgUnitTreeNode.vue'

const authStore = useAuthStore()
const canManage = computed(() => authStore.hasRole('admin'))
/** 根组织（无上级）仅平台超级管理员可创建 */
const canCreateRoot = computed(() => authStore.isSystemAdmin === true)
/** 非管理员单归属：只读展示，不可切换浏览范围 */
const canSwitchScope = computed(
  () => canManage.value || memberships.value.length !== 1,
)

const loading = ref(false)
const creating = ref(false)
const tree = ref<OrgUnit[]>([])
const memberships = ref<OrgUnitMember[]>([])
const activeOrgUnitId = ref(getStoredOrgUnitId())
const newName = ref('')
const newParentId = ref('')

const flatten = (nodes: OrgUnit[], depth = 0): Array<{ label: string; value: string }> => {
  const rows: Array<{ label: string; value: string }> = []
  for (const node of nodes) {
    rows.push({
      label: `${'—'.repeat(depth)} ${node.name}`.trim(),
      value: node.id,
    })
    if (node.children?.length) {
      rows.push(...flatten(node.children, depth + 1))
    }
  }
  return rows
}

const flatOptions = computed(() => flatten(tree.value))
const parentOptions = computed(() => [
  { label: '（根）', value: '' },
  ...flatOptions.value,
])

const currentOrgUnitLabel = computed(() => {
  const sole = memberships.value[0]
  if (sole?.org_unit?.name) {
    return sole.org_unit.name
  }
  const match = flatOptions.value.find(
    (option) => option.value === (activeOrgUnitId.value || sole?.org_unit_id),
  )
  return match?.label?.replace(/^—+\s*/, '') || sole?.org_unit_id || '—'
})

async function reload() {
  loading.value = true
  try {
    const [units, mine] = await Promise.all([
      listOrgUnits(true, { platform: authStore.isSystemAdmin === true }),
      listMyOrgUnitMemberships(),
    ])
    tree.value = units
    memberships.value = mine
    if (!activeOrgUnitId.value) {
      const sole = mine[0]
      if (sole) {
        activeOrgUnitId.value = sole.org_unit_id
        setStoredOrgUnitId(sole.org_unit_id)
      }
    }
  } catch (error: unknown) {
    MessagePlugin.error((error as Error)?.message || '加载组织树失败')
  } finally {
    loading.value = false
  }
}

async function onCreate() {
  const name = newName.value.trim()
  if (!name) {
    MessagePlugin.warning('请输入组织名称')
    return
  }
  const parentId = (newParentId.value || '').trim()
  if (!parentId && !canCreateRoot.value) {
    MessagePlugin.warning('请选择上级组织；根组织仅超级管理员可创建')
    return
  }
  creating.value = true
  try {
    await createOrgUnit({
      name,
      parent_id: parentId,
    })
    newName.value = ''
    newParentId.value = ''
    MessagePlugin.success('已创建')
    await reload()
  } catch (error: unknown) {
    MessagePlugin.error((error as Error)?.message || '创建失败')
  } finally {
    creating.value = false
  }
}

async function onDelete(id: string) {
  try {
    await deleteOrgUnit(id)
    if (activeOrgUnitId.value === id) {
      activeOrgUnitId.value = ''
      setStoredOrgUnitId('')
    }
    MessagePlugin.success('已删除')
    await reload()
  } catch (error: unknown) {
    MessagePlugin.error((error as Error)?.message || '删除失败')
  }
}

function selectUnit(id: string) {
  if (!canSwitchScope.value) {
    return
  }
  activeOrgUnitId.value = id
  setStoredOrgUnitId(id)
}

function onActiveChange(value: string) {
  // Scope switch is header-only (X-Org-Unit-ID); do not call SetPrimary.
  setStoredOrgUnitId(value || '')
}

onMounted(reload)
</script>

<style scoped lang="less">
.org-unit-settings {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.section-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}
.section-desc {
  margin: 4px 0 0;
  color: var(--td-text-color-secondary);
  font-size: 13px;
}
.active-unit {
  max-width: 360px;
}
.readonly-org {
  padding: 8px 0;
  font-size: 14px;
  color: var(--td-text-color-primary);
}
.field-label {
  display: block;
  margin-bottom: 6px;
  font-size: 13px;
  color: var(--td-text-color-secondary);
}
.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}
.name-input {
  width: 200px;
}
.parent-select {
  width: 220px;
}
.empty {
  padding: 24px 0;
  color: var(--td-text-color-secondary);
}
.tree {
  list-style: none;
  margin: 0;
  padding: 0;
}
</style>
