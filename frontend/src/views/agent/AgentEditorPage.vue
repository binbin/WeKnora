<template>
  <div class="agent-editor-route">
    <t-loading
      v-if="showLoading"
      :loading="true"
      size="medium"
      class="agent-editor-route__loading"
    />
    <div v-else-if="loadError" class="agent-editor-route__error">
      <t-empty :description="loadError">
        <t-button theme="primary" @click="goToList">
          {{ $t('common.back') }}
        </t-button>
      </t-empty>
    </div>
    <AgentEditorModal
      v-else-if="canRenderEditor"
      :mode="editorMode"
      :agent="editorAgent"
      :initial-section="initialSection"
      :initial-highlight-field="initialHighlightField"
      :initial-workspace-tab="initialWorkspaceTab"
      :read-only="readOnly"
      @close="goToList"
      @success="handleSuccess"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { getAgentById, type CustomAgent } from '@/api/agent'
import { useAuthStore } from '@/stores/auth'
import AgentEditorModal from './AgentEditorModal.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const loading = ref(true)
const loadError = ref('')
const editorAgent = ref<CustomAgent | null>(null)
const isCreateRoute = computed(
  () => route.name === 'agentCreate' || route.path.endsWith('/agents/add'),
)

const editorMode = computed<'create' | 'edit'>(() =>
  isCreateRoute.value ? 'create' : 'edit',
)

const showLoading = computed(
  () =>
    loading.value ||
    (editorMode.value === 'edit' && !editorAgent.value && !loadError.value),
)

const canRenderEditor = computed(() => {
  if (loadError.value) return false
  if (editorMode.value === 'create') return !loading.value
  return !!editorAgent.value?.id
})

const initialSection = computed(() => {
  const section = route.query.section
  return typeof section === 'string' && section ? section : 'basic'
})

const initialHighlightField = computed(() => {
  const highlight = route.query.highlight
  return typeof highlight === 'string' ? highlight : ''
})

const initialWorkspaceTab = computed<'config' | 'publish' | 'logs'>(() => {
  const raw = route.query.tab
  const tab = Array.isArray(raw) ? raw[0] : raw
  if (tab === 'publish' || tab === 'logs' || tab === 'config') return tab
  if (route.query.section === 'share') return 'config'
  return 'config'
})

const readOnly = computed(() => {
  if (editorMode.value === 'create' || !editorAgent.value) return false
  return !canManageAgent(editorAgent.value)
})

function canManageAgent(agent: CustomAgent): boolean {
  const userId = authStore.user?.id || ''
  const creatorId = agent.created_by || ''
  if (creatorId && userId && creatorId === userId) return true
  return authStore.hasRole('admin')
}

function goToList(): void {
  router.push({ path: '/platform/agents' })
}

function handleSuccess(agent?: CustomAgent): void {
  if (agent?.id && isCreateRoute.value) {
    router.replace({
      path: `/platform/agents/${agent.id}`,
      query: {
        ...(route.query.section ? { section: route.query.section } : {}),
        ...(route.query.tab ? { tab: route.query.tab } : {}),
      },
    })
    return
  }
  if (agent) {
    editorAgent.value = agent
  }
}

function unwrapAgentPayload(response: unknown): CustomAgent | null {
  if (!response || typeof response !== 'object') return null
  const payload = response as { data?: CustomAgent; id?: string }
  if (payload.data && typeof payload.data === 'object' && payload.data.id) {
    return payload.data
  }
  if (payload.id) {
    return payload as CustomAgent
  }
  return null
}

function resolveRouteAgentId(): string {
  const fromParam = String(route.params.agentId || '').trim()
  if (fromParam && fromParam !== 'add') return fromParam
  const matched = String(route.path || '').match(/\/agents\/([^/]+)\/?$/)
  const fromPath = matched?.[1]?.trim() || ''
  if (fromPath && fromPath !== 'add') return fromPath
  return ''
}

async function loadAgent(): Promise<void> {
  if (isCreateRoute.value) {
    editorAgent.value = null
    loadError.value = ''
    loading.value = false
    return
  }

  const agentId = resolveRouteAgentId()
  if (!agentId) {
    loadError.value = t('agent.messages.loadFailed')
    editorAgent.value = null
    loading.value = false
    return
  }

  loading.value = true
  loadError.value = ''
  try {
    const response = await getAgentById(agentId)
    const agent = unwrapAgentPayload(response)
    if (!agent?.id) {
      loadError.value = t('agent.messages.loadFailed')
      editorAgent.value = null
      return
    }
    editorAgent.value = agent
  } catch (err) {
    console.error('Failed to load agent editor page', err)
    loadError.value = t('agent.messages.loadFailed')
    editorAgent.value = null
  } finally {
    loading.value = false
  }
}

watch(
  () => [route.name, route.params.agentId] as const,
  () => {
    void loadAgent()
  },
  { immediate: true },
)
</script>

<style scoped lang="less">
.agent-editor-route {
  flex: 1;
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.agent-editor-route__loading,
.agent-editor-route__error {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 48px 24px;
}
</style>
