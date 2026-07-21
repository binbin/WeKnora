<template>
  <li class="tree-node">
    <div
      class="row"
      :class="{ active: node.id === activeId }"
      :style="{ paddingLeft: `${node.depth * 16}px` }"
    >
      <button type="button" class="name" @click="$emit('select', node.id)">
        {{ node.name }}
        <span v-if="node.code" class="code">{{ node.code }}</span>
      </button>
      <t-button
        v-if="canManage"
        size="small"
        variant="text"
        theme="danger"
        @click="$emit('delete', node.id)"
      >
        删除
      </t-button>
    </div>
    <ul v-if="node.children?.length" class="children">
      <OrgUnitTreeNode
        v-for="child in node.children"
        :key="child.id"
        :node="child"
        :can-manage="canManage"
        :active-id="activeId"
        @select="$emit('select', $event)"
        @delete="$emit('delete', $event)"
      />
    </ul>
  </li>
</template>

<script setup lang="ts">
import type { OrgUnit } from '@/api/org-unit'

defineProps<{
  node: OrgUnit
  canManage: boolean
  activeId: string
}>()

defineEmits<{
  select: [id: string]
  delete: [id: string]
}>()
</script>

<script lang="ts">
export default {
  name: 'OrgUnitTreeNode',
}
</script>

<style scoped lang="less">
.tree-node {
  list-style: none;
}
.children {
  list-style: none;
  margin: 0;
  padding: 0;
}
.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 36px;
  border-radius: 6px;
  padding-right: 8px;
}
.row.active {
  background: var(--td-bg-color-container-hover);
}
.name {
  border: none;
  background: transparent;
  cursor: pointer;
  text-align: left;
  font-size: 14px;
  color: var(--td-text-color-primary);
  padding: 6px 8px;
}
.code {
  margin-left: 8px;
  font-size: 12px;
  color: var(--td-text-color-secondary);
}
</style>
