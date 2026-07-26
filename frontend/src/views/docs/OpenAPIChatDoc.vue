<template>
  <div class="openapi-doc-page" data-testid="openapi-chat-doc-page">
    <header class="openapi-doc-page__header">
      <t-button variant="text" theme="default" @click="goBack">
        <template #icon><t-icon name="chevron-left" /></template>
        {{ $t('agentEditor.publish.apiDocBack') }}
      </t-button>
      <h1>{{ $t('agentEditor.publish.apiDocTitle') }}</h1>
      <p class="openapi-doc-page__subtitle">
        {{ $t('agentEditor.publish.apiDocSubtitle') }}
      </p>
    </header>
    <article
      class="openapi-doc-page__body markdown-body"
      v-html="docHtml"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { marked } from 'marked'
import { sanitizeMarkdownHTML } from '@/utils/security'
import openapiChatMarkdown from '@/docs/openapi-chat.md?raw'

const route = useRoute()
const router = useRouter()

const docHtml = computed(() => {
  const parsed = marked.parse(openapiChatMarkdown, {
    breaks: true,
    async: false,
  }) as string
  return sanitizeMarkdownHTML(parsed)
})

function goBack(): void {
  const fromAgent = String(route.query.fromAgent || '').trim()
  if (fromAgent) {
    router.push({
      path: `/platform/agents/${fromAgent}`,
      query: { tab: 'publish' },
    })
    return
  }
  if (window.history.length > 1) {
    router.back()
    return
  }
  router.push({ path: '/platform/agents' })
}
</script>

<style scoped lang="less">
.openapi-doc-page {
  max-width: 880px;
  margin: 0 auto;
  padding: 24px 28px 64px;
  min-height: 100%;
  overflow: auto;
}

.openapi-doc-page__header {
  margin-bottom: 20px;

  h1 {
    margin: 8px 0 6px;
    font-size: 24px;
    font-weight: 600;
    color: var(--td-text-color-primary);
  }
}

.openapi-doc-page__subtitle {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-size: 14px;
  line-height: 1.5;
}

.openapi-doc-page__body {
  color: var(--td-text-color-primary);
  font-size: 14px;
  line-height: 1.7;

  :deep(h1) {
    display: none; /* page header already shows title */
  }

  :deep(h2) {
    margin: 28px 0 12px;
    font-size: 18px;
    font-weight: 600;
  }

  :deep(h3) {
    margin: 20px 0 8px;
    font-size: 16px;
    font-weight: 600;
  }

  :deep(p),
  :deep(ul),
  :deep(ol),
  :deep(table) {
    margin: 0 0 12px;
  }

  :deep(code) {
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 12.5px;
    padding: 1px 4px;
    border-radius: 4px;
    background: var(--td-bg-color-secondarycontainer);
  }

  :deep(pre) {
    margin: 0 0 16px;
    padding: 12px 14px;
    overflow: auto;
    border-radius: 8px;
    background: var(--td-bg-color-secondarycontainer);
    border: 1px solid var(--td-component-border);

    code {
      padding: 0;
      background: transparent;
    }
  }

  :deep(table) {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;

    th,
    td {
      border: 1px solid var(--td-component-border);
      padding: 8px 10px;
      text-align: left;
      vertical-align: top;
    }

    th {
      background: var(--td-bg-color-secondarycontainer);
      font-weight: 600;
    }
  }

  :deep(blockquote) {
    margin: 0 0 12px;
    padding: 8px 12px;
    border-left: 3px solid var(--td-brand-color);
    background: var(--td-bg-color-secondarycontainer);
    color: var(--td-text-color-secondary);
  }

  :deep(a) {
    color: var(--td-brand-color);
  }
}
</style>
