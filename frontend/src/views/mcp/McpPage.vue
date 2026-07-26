<template>
  <div class="mcp-page">
    <McpSettings />
  </div>
</template>

<script setup lang="ts">
import McpSettings from '@/views/settings/McpSettings.vue'
</script>

<style scoped lang="less">
.mcp-page {
  height: 100%;
  box-sizing: border-box;
  overflow: auto;
  padding: 24px 28px 32px;
}
</style>
