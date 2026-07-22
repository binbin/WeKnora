<template>
  <div class="conversation-logs">
    <div v-if="!agentId" class="conversation-logs__empty">
      <t-empty :description="$t('agentEditor.workspace.logsNeedSave')" />
    </div>

    <template v-else>
      <div class="logs-subnav">
        <button
          type="button"
          :class="['logs-subnav__item', { 'is-active': subView === 'dashboard' }]"
          @click="subView = 'dashboard'"
        >
          <t-icon name="chart" />
          <span>{{ $t('agentEditor.logs.dashboard') }}</span>
        </button>
        <button
          type="button"
          :class="['logs-subnav__item', { 'is-active': subView === 'details' }]"
          @click="subView = 'details'"
        >
          <t-icon name="root-list" />
          <span>{{ $t('agentEditor.logs.details') }}</span>
        </button>
      </div>

      <div v-if="subView === 'dashboard'" class="logs-dashboard">
        <div class="stat-card">
          <div class="stat-card__label">{{ $t('agentEditor.logs.statTotal') }}</div>
          <div class="stat-card__value">{{ filteredRows.length }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-card__label">{{ $t('agentEditor.logs.statWeb') }}</div>
          <div class="stat-card__value">{{ countBySource('web') }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-card__label">{{ $t('agentEditor.logs.statEmbed') }}</div>
          <div class="stat-card__value">{{ countBySource('embed') }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-card__label">{{ $t('agentEditor.logs.statIm') }}</div>
          <div class="stat-card__value">{{ countBySource('im') }}</div>
        </div>
      </div>

      <template v-else>
        <div class="logs-toolbar">
          <div class="logs-toolbar__filters">
            <t-select
              v-model="sourceFilter"
              :options="sourceOptions"
              :placeholder="$t('agentEditor.logs.source')"
              clearable
              style="width: 140px"
            />
            <t-date-range-picker
              v-model="dateRange"
              allow-input
              clearable
              style="width: 260px"
            />
            <t-input
              v-model="keyword"
              clearable
              :placeholder="$t('agentEditor.logs.searchPlaceholder')"
              style="width: 220px"
            >
              <template #prefix-icon><t-icon name="search" /></template>
            </t-input>
          </div>
          <div class="logs-toolbar__actions">
            <t-button variant="outline" :loading="loading" @click="reload">
              {{ $t('common.refresh') }}
            </t-button>
            <t-button theme="primary" variant="outline" @click="exportCsv">
              {{ $t('agentEditor.logs.export') }}
            </t-button>
          </div>
        </div>

        <t-loading :loading="loading" size="small" class="logs-table-wrap">
          <t-table
            row-key="id"
            :data="filteredRows"
            :columns="columns"
            hover
            :selected-row-keys="selectedKeys"
            @select-change="onSelectChange"
          >
            <template #empty>
              <div class="logs-empty">
                <t-icon name="folder-open" size="40px" />
                <p>{{ $t('agentEditor.logs.emptyCute') }}</p>
              </div>
            </template>
            <template #ops="{ row }">
              <t-button size="small" variant="text" theme="primary" @click="openSession(row)">
                {{ $t('agentEditor.logs.openChat') }}
              </t-button>
            </template>
          </t-table>
        </t-loading>
      </template>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { getSessionsList } from '@/api/chat'
import { listEmbedChannels } from '@/api/embed'
import { useAuthStore } from '@/stores/auth'

interface LogRow {
  id: string
  userLabel: string
  title: string
  updatedAt: string
  updatedAtRaw: string
  messageCount: string
  feedback: string
  ipLabel: string
  sourceKey: string
  sourceLabel: string
}

const props = defineProps<{
  agentId: string
}>()

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()

const loading = ref(false)
const rows = ref<LogRow[]>([])
const subView = ref<'dashboard' | 'details'>('details')
const sourceFilter = ref('')
const keyword = ref('')
const dateRange = ref<string[]>([])
const selectedKeys = ref<string[]>([])

const PAGE_SIZE = 50
const MAX_PAGES = 4

const sourceOptions = computed(() => [
  { label: t('agentEditor.logs.sourceAll'), value: '' },
  { label: t('agentEditor.workspace.logsSourceWeb'), value: 'web' },
  { label: t('agentEditor.workspace.logsSourceEmbed'), value: 'embed' },
  { label: t('agentEditor.logs.sourceIm'), value: 'im' },
])

const columns = computed(() => [
  { colKey: 'row-select', type: 'multiple' as const, width: 46 },
  { colKey: 'userLabel', title: t('agentEditor.logs.colUser'), width: 140 },
  { colKey: 'title', title: t('agentEditor.logs.colTitle'), ellipsis: true },
  { colKey: 'updatedAt', title: t('agentEditor.logs.colLastChat'), width: 180 },
  { colKey: 'messageCount', title: t('agentEditor.logs.colMessages'), width: 100 },
  { colKey: 'feedback', title: t('agentEditor.logs.colFeedback'), width: 110 },
  { colKey: 'sourceLabel', title: t('agentEditor.logs.colSource'), width: 100 },
  { colKey: 'ipLabel', title: t('agentEditor.logs.colIp'), width: 140 },
  { colKey: 'ops', title: t('common.edit'), cell: 'ops', width: 100 },
])

const filteredRows = computed(() => {
  return rows.value.filter((row) => {
    if (sourceFilter.value && row.sourceKey !== sourceFilter.value) return false
    if (keyword.value.trim()) {
      const query = keyword.value.trim().toLowerCase()
      const haystack = `${row.title} ${row.id} ${row.userLabel}`.toLowerCase()
      if (!haystack.includes(query)) return false
    }
    if (dateRange.value?.length === 2) {
      const start = new Date(dateRange.value[0]).getTime()
      const end = new Date(dateRange.value[1]).getTime() + 24 * 60 * 60 * 1000 - 1
      const stamp = new Date(row.updatedAtRaw).getTime()
      if (!Number.isNaN(start) && !Number.isNaN(end) && !Number.isNaN(stamp)) {
        if (stamp < start || stamp > end) return false
      }
    }
    return true
  })
})

watch(
  () => props.agentId,
  () => {
    void reload()
  },
)

onMounted(() => {
  void reload()
})

function countBySource(sourceKey: string): number {
  return rows.value.filter((row) => row.sourceKey === sourceKey).length
}

function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function sessionMatchesAgent(session: Record<string, unknown>): boolean {
  const state = session.last_request_state as { agent_id?: string } | undefined
  return !!(state?.agent_id && state.agent_id === props.agentId)
}

function toRow(
  session: Record<string, unknown>,
  sourceKey: string,
  sourceLabel: string,
): LogRow {
  const title =
    String(session.title || '').trim() || t('createChat.newSessionTitle')
  const userId = String(session.user_id || '')
  return {
    id: String(session.id),
    userLabel: userId ? userId.slice(0, 12) : t('common.me'),
    title,
    updatedAt: formatTime(String(session.updated_at || session.created_at || '')),
    updatedAtRaw: String(session.updated_at || session.created_at || ''),
    messageCount: '—',
    feedback: '—',
    ipLabel: '—',
    sourceKey,
    sourceLabel,
  }
}

async function loadWebSessions(): Promise<LogRow[]> {
  const matched: LogRow[] = []
  for (let page = 1; page <= MAX_PAGES; page += 1) {
    const response = await getSessionsList(page, PAGE_SIZE)
    const batch = (response?.data || []) as Record<string, unknown>[]
    for (const session of batch) {
      if (sessionMatchesAgent(session)) {
        matched.push(
          toRow(session, 'web', t('agentEditor.workspace.logsSourceWeb')),
        )
      }
    }
    if (batch.length < PAGE_SIZE) break
  }
  return matched
}

async function loadEmbedSessions(): Promise<LogRow[]> {
  if (!authStore.hasRole('admin') || !props.agentId) return []
  const channelResp = await listEmbedChannels(props.agentId)
  const channels = channelResp?.data || []
  const matched: LogRow[] = []
  for (const channel of channels) {
    const source = `embed:${channel.id}`
    for (let page = 1; page <= MAX_PAGES; page += 1) {
      try {
        const response = await getSessionsList(page, PAGE_SIZE, source)
        const batch = (response?.data || []) as Record<string, unknown>[]
        for (const session of batch) {
          matched.push(
            toRow(session, 'embed', t('agentEditor.workspace.logsSourceEmbed')),
          )
        }
        if (batch.length < PAGE_SIZE) break
      } catch {
        break
      }
    }
  }
  return matched
}

async function reload(): Promise<void> {
  if (!props.agentId) {
    rows.value = []
    return
  }
  loading.value = true
  try {
    const [webRows, embedRows] = await Promise.all([
      loadWebSessions(),
      loadEmbedSessions(),
    ])
    const byId = new Map<string, LogRow>()
    for (const row of [...webRows, ...embedRows]) {
      byId.set(row.id, row)
    }
    rows.value = Array.from(byId.values()).sort((left, right) =>
      String(right.updatedAtRaw).localeCompare(String(left.updatedAtRaw)),
    )
  } catch (err) {
    console.error('Failed to load agent conversation logs', err)
    MessagePlugin.error(t('agentEditor.workspace.logsLoadFailed'))
    rows.value = []
  } finally {
    loading.value = false
  }
}

function onSelectChange(keys: Array<string | number>): void {
  selectedKeys.value = keys.map(String)
}

function openSession(row: LogRow): void {
  emit('close')
  router.push({
    path: `/platform/chat/${row.id}`,
    query: { agent_id: props.agentId },
  })
}

function exportCsv(): void {
  if (filteredRows.value.length === 0) {
    MessagePlugin.warning(t('agentEditor.logs.exportEmpty'))
    return
  }
  const header = [
    t('agentEditor.logs.colUser'),
    t('agentEditor.logs.colTitle'),
    t('agentEditor.logs.colLastChat'),
    t('agentEditor.logs.colSource'),
    'ID',
  ]
  const lines = [header.join(',')]
  for (const row of filteredRows.value) {
    lines.push(
      [
        csvEscape(row.userLabel),
        csvEscape(row.title),
        csvEscape(row.updatedAt),
        csvEscape(row.sourceLabel),
        csvEscape(row.id),
      ].join(','),
    )
  }
  const blob = new Blob([lines.join('\n')], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = `agent-${props.agentId}-logs.csv`
  anchor.click()
  URL.revokeObjectURL(url)
}

function csvEscape(value: string): string {
  if (value.includes(',') || value.includes('"') || value.includes('\n')) {
    return `"${value.replace(/"/g, '""')}"`
  }
  return value
}
</script>

<style scoped lang="less">
.conversation-logs {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  padding: 12px 20px 20px;
  gap: 12px;
}

.logs-subnav {
  display: flex;
  align-items: center;
  gap: 8px;
}

.logs-subnav__item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: none;
  background: transparent;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  padding: 6px 10px;
  border-radius: 6px;
  cursor: pointer;

  &.is-active {
    color: var(--td-brand-color);
    background: color-mix(in srgb, var(--td-brand-color) 10%, transparent);
  }
}

.logs-dashboard {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.stat-card {
  border: 1px solid var(--td-component-stroke);
  border-radius: 10px;
  padding: 16px;
  background: var(--td-bg-color-container);
}

.stat-card__label {
  font-size: 12px;
  color: var(--td-text-color-secondary);
  margin-bottom: 8px;
}

.stat-card__value {
  font-size: 28px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.logs-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.logs-toolbar__filters,
.logs-toolbar__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.logs-table-wrap {
  flex: 1;
  min-height: 0;
  overflow: auto;
}

.logs-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 56px 12px;
  color: var(--td-text-color-placeholder);

  p {
    margin: 0;
    font-size: 13px;
  }
}

.conversation-logs__empty {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 48px 12px;
}

@media (max-width: 960px) {
  .logs-dashboard {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
