<template>
  <div class="org-unit-page">
    <OrgUnitSettings />
  </div>
</template>

<script setup lang="ts">
import OrgUnitSettings from '@/views/settings/OrgUnitSettings.vue'
</script>

<style scoped lang="less">
.org-unit-page {
  height: 100%;
  box-sizing: border-box;
  overflow: auto;
  padding: 24px 28px 32px;
}
</style>
