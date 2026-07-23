<template>
  <div class="members-page">
    <TenantMembers />
  </div>
</template>

<script setup lang="ts">
import TenantMembers from '@/views/settings/TenantMembers.vue'
</script>

<style scoped lang="less">
.members-page {
  height: 100%;
  box-sizing: border-box;
  overflow: auto;
  padding: 24px 28px 32px;
}
</style>
