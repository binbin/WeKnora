<template>
  <div class="publish-channels" data-testid="publish-channels-root">
    <div class="channel-type-grid" data-testid="publish-channel-type-grid">
      <button
        v-for="item in channelTypes"
        :key="item.key"
        type="button"
        :class="['channel-type-card', { 'is-selected': selectedType === item.key }]"
        @click="selectedType = item.key"
      >
        <span class="channel-type-card__check" aria-hidden="true">
          <t-icon v-if="selectedType === item.key" name="check-circle-filled" />
          <span v-else class="channel-type-card__radio" />
        </span>
        <div class="channel-type-card__icon" :class="`channel-type-card__icon--${item.key}`">
          <img v-if="item.logo" :src="item.logo" :alt="item.title" />
          <t-icon v-else :name="item.icon" size="22px" />
        </div>
        <div class="channel-type-card__body">
          <div class="channel-type-card__title">{{ item.title }}</div>
          <div class="channel-type-card__desc">{{ item.desc }}</div>
        </div>
      </button>
    </div>

    <section class="channel-detail">
      <!-- 免登录窗口 -->
      <template v-if="selectedType === 'web'">
        <div class="channel-detail__header">
          <div class="channel-detail__title-wrap">
            <h3>{{ $t('agentEditor.publish.types.web') }}</h3>
            <t-tooltip :content="$t('agentEditor.publish.types.webDesc')" placement="top">
              <t-icon name="help-circle" class="channel-detail__help" />
            </t-tooltip>
          </div>
          <t-button
            v-if="canManage"
            theme="primary"
            variant="outline"
            @click="embedPanelRef?.openCreate()"
          >
            {{ $t('agentEditor.publish.createLink') }}
          </t-button>
        </div>
        <t-loading :loading="embedLoading" size="small">
          <t-table
            row-key="id"
            :data="embedRows"
            :columns="embedColumns"
            hover
          >
            <template #empty>
              <div class="table-empty">{{ embedEmpty }}</div>
            </template>
            <template #name="{ row }">
              <button type="button" class="link-btn" @click="embedPanelRef?.openDrawer(row.raw)">
                {{ row.name }}
              </button>
            </template>
            <template #enabled="{ row }">
              <t-icon
                :name="row.enabled ? 'check' : 'close'"
                :style="{ color: row.enabled ? 'var(--td-success-color)' : 'var(--td-text-color-placeholder)' }"
              />
            </template>
            <template #ops="{ row }">
              <div class="row-ops">
                <t-button size="small" theme="primary" variant="outline" @click="startEmbed(row.raw)">
                  {{ $t('agentEditor.publish.startUse') }}
                </t-button>
                <t-dropdown
                  v-if="canManage"
                  trigger="click"
                  :options="embedMenuOptions"
                  @click="(value) => onEmbedMenu(value, row.raw)"
                >
                  <t-button size="small" variant="outline" shape="square">
                    <template #icon><t-icon name="ellipsis" /></template>
                  </t-button>
                </t-dropdown>
              </div>
            </template>
          </t-table>
        </t-loading>
        <AgentEmbedChannelPanel
          ref="embedPanelRef"
          v-model:filter-agent-id="lockedAgentId"
          hide-channel-list
          lock-agent-filter
          class="publish-host-panel"
          @changed="loadEmbedRows"
        />
      </template>

      <!-- API -->
      <template v-else-if="selectedType === 'api'">
        <div class="channel-detail__header">
          <div class="channel-detail__title-wrap">
            <h3>{{ $t('agentEditor.publish.apiKeysTitle') }}</h3>
            <a
              class="channel-detail__doc"
              href="https://github.com/Tencent/WeKnora"
              target="_blank"
              rel="noopener noreferrer"
            >
              {{ $t('agentEditor.publish.viewDocs') }}
            </a>
          </div>
          <div class="channel-detail__aside">
            <div class="api-root">
              <span>{{ $t('agentEditor.publish.apiRoot') }}</span>
              <code>{{ apiBaseUrl }}</code>
              <t-button size="small" variant="text" @click="copyText(apiBaseUrl)">
                <t-icon name="file-copy" />
              </t-button>
            </div>
            <t-button v-if="canManage" theme="primary" @click="openCreateApiKey">
              + {{ $t('common.create') }}
            </t-button>
          </div>
        </div>
        <p class="channel-detail__hint">
          {{ $t('agentEditor.publish.apiHint', { agentId }) }}
        </p>
        <t-loading :loading="apiLoading" size="small">
          <t-table
            row-key="id"
            :data="apiRows"
            :columns="apiColumns"
            hover
          >
            <template #empty>
              <div class="table-empty">{{ $t('agentEditor.publish.apiEmpty') }}</div>
            </template>
          </t-table>
        </t-loading>
      </template>

      <!-- IM: 飞书 / 钉钉 / 微信 -->
      <template v-else-if="isImType">
        <div class="channel-detail__header">
          <div class="channel-detail__title-wrap">
            <h3>{{ currentTypeMeta?.title }}</h3>
          </div>
          <t-button
            v-if="canManage"
            theme="primary"
            variant="outline"
            @click="imPanelRef?.openCreate(primaryImPlatform)"
          >
            {{ $t('agentEditor.publish.createBot') }}
          </t-button>
        </div>
        <t-loading :loading="imLoading" size="small">
          <t-table
            row-key="id"
            :data="imRows"
            :columns="imColumns"
            hover
          >
            <template #empty>
              <div class="table-empty">{{ $t('agentEditor.publish.imEmpty') }}</div>
            </template>
            <template #name="{ row }">
              <button type="button" class="link-btn" @click="imPanelRef?.openDrawer(row.raw)">
                {{ row.name }}
              </button>
            </template>
            <template #enabled="{ row }">
              <t-tag size="small" :theme="row.enabled ? 'success' : 'warning'" variant="light">
                {{ row.enabled ? $t('common.on') : $t('common.off') }}
              </t-tag>
            </template>
            <template #ops="{ row }">
              <t-button size="small" variant="outline" @click="imPanelRef?.openDrawer(row.raw)">
                {{ $t('common.edit') }}
              </t-button>
            </template>
          </t-table>
        </t-loading>
        <IMChannelPanel
          ref="imPanelRef"
          v-model:filter-agent-id="lockedAgentId"
          :platform-filter="imPlatformFilter"
          hide-channel-list
          lock-agent-filter
          class="publish-host-panel"
          @changed="loadImRows"
        />
      </template>

      <!-- 门户 -->
      <template v-else>
        <div class="channel-detail__header">
          <div class="channel-detail__title-wrap">
            <h3>{{ $t('agentEditor.publish.types.portal') }}</h3>
          </div>
        </div>
        <div class="portal-empty">
          <t-empty :description="$t('agentEditor.publish.portalDesc')">
            <t-button theme="primary" variant="outline" @click="emit('goto-share')">
              {{ $t('agentEditor.publish.gotoShare') }}
            </t-button>
          </t-empty>
        </div>
      </template>
    </section>

    <t-dialog
      v-model:visible="createApiKeyVisible"
      :header="$t('integrations.api.createApiKey')"
      :confirm-btn="{
        content: $t('integrations.api.createApiKey'),
        loading: createApiKeyLoading,
      }"
      :cancel-btn="$t('common.cancel')"
      @confirm="createApiKey"
    >
      <t-form>
        <t-form-item :label="$t('integrations.api.apiKeyName')">
          <t-input
            v-model="createApiKeyName"
            :placeholder="$t('integrations.api.apiKeyNamePlaceholder')"
          />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import AgentEmbedChannelPanel from '@/components/AgentEmbedChannelPanel.vue'
import IMChannelPanel from '@/components/IMChannelPanel.vue'
import { listEmbedChannels, type EmbedChannel } from '@/api/embed'
import { listIMChannels, type IMChannel } from '@/api/agent'
import {
  createTenantAPIKey,
  listTenantAPIKeys,
  type TenantAPIKey,
} from '@/api/tenant'
import { getApiBaseUrl } from '@/utils/api-base'
import { useAuthStore } from '@/stores/auth'
import feishuLogo from '@/assets/img/im/feishu.svg'
import dingtalkLogo from '@/assets/img/im/dingtalk.svg'
import wechatLogo from '@/assets/img/im/wechat.svg'

type ChannelTypeKey = 'web' | 'api' | 'feishu' | 'dingtalk' | 'wechat' | 'portal'

const props = defineProps<{
  agentId: string
  canManage?: boolean
}>()

const emit = defineEmits<{
  'goto-share': []
}>()

const { t } = useI18n()
const authStore = useAuthStore()

const selectedType = ref<ChannelTypeKey>('web')
const lockedAgentId = ref(props.agentId)
const embedPanelRef = ref<InstanceType<typeof AgentEmbedChannelPanel> | null>(null)
const imPanelRef = ref<InstanceType<typeof IMChannelPanel> | null>(null)
const createApiKeyVisible = ref(false)
const createApiKeyLoading = ref(false)
const createApiKeyName = ref('')

const embedLoading = ref(false)
const embedRows = ref<Array<{
  id: string
  name: string
  enabled: boolean
  updatedAt: string
  raw: EmbedChannel
}>>([])

const apiLoading = ref(false)
const apiRows = ref<Array<{
  id: number
  name: string
  apiKey: string
  createdAt: string
  lastUsedAt: string
}>>([])

const imLoading = ref(false)
const imRows = ref<Array<{
  id: string
  name: string
  platform: string
  enabled: boolean
  raw: IMChannel
}>>([])

const apiBaseUrl = getApiBaseUrl()
const canManage = computed(() => props.canManage !== false && authStore.hasRole('admin'))

const channelTypes = computed(() => [
  {
    key: 'web' as const,
    title: t('agentEditor.publish.types.web'),
    desc: t('agentEditor.publish.types.webDesc'),
    icon: 'link',
  },
  {
    key: 'api' as const,
    title: t('agentEditor.publish.types.api'),
    desc: t('agentEditor.publish.types.apiDesc'),
    icon: 'code',
  },
  {
    key: 'feishu' as const,
    title: t('agentEditor.publish.types.feishu'),
    desc: t('agentEditor.publish.types.feishuDesc'),
    icon: 'chat',
    logo: feishuLogo,
  },
  {
    key: 'dingtalk' as const,
    title: t('agentEditor.publish.types.dingtalk'),
    desc: t('agentEditor.publish.types.dingtalkDesc'),
    icon: 'chat',
    logo: dingtalkLogo,
  },
  {
    key: 'wechat' as const,
    title: t('agentEditor.publish.types.wechat'),
    desc: t('agentEditor.publish.types.wechatDesc'),
    icon: 'chat',
    logo: wechatLogo,
  },
  {
    key: 'portal' as const,
    title: t('agentEditor.publish.types.portal'),
    desc: t('agentEditor.publish.types.portalDesc'),
    icon: 'internet',
  },
])

const currentTypeMeta = computed(() =>
  channelTypes.value.find((item) => item.key === selectedType.value),
)

const isImType = computed(() =>
  selectedType.value === 'feishu'
  || selectedType.value === 'dingtalk'
  || selectedType.value === 'wechat',
)

const imPlatformFilter = computed(() => {
  if (selectedType.value === 'feishu') return ['feishu', 'lark']
  if (selectedType.value === 'dingtalk') return 'dingtalk'
  if (selectedType.value === 'wechat') return ['wechat', 'wecom']
  return undefined
})

const primaryImPlatform = computed(() => {
  if (selectedType.value === 'feishu') return 'feishu'
  if (selectedType.value === 'dingtalk') return 'dingtalk'
  if (selectedType.value === 'wechat') return 'wecom'
  return 'wecom'
})

const embedColumns = [
  { colKey: 'name', title: () => t('agentEditor.publish.colName'), cell: 'name' },
  { colKey: 'enabled', title: () => t('agentEditor.publish.colEnabled'), cell: 'enabled', width: 100 },
  { colKey: 'updatedAt', title: () => t('agentEditor.publish.colLastUsed'), width: 160 },
  { colKey: 'ops', title: () => t('common.edit'), cell: 'ops', width: 180 },
]

const apiColumns = [
  { colKey: 'name', title: () => t('agentEditor.publish.colName') },
  { colKey: 'apiKey', title: () => t('agentEditor.publish.colApiKey') },
  { colKey: 'createdAt', title: () => t('agentEditor.publish.colCreatedAt') },
  { colKey: 'lastUsedAt', title: () => t('agentEditor.publish.colLastUsed') },
]

const imColumns = [
  { colKey: 'name', title: () => t('agentEditor.publish.colName'), cell: 'name' },
  { colKey: 'platform', title: () => t('agentEditor.im.platform') },
  { colKey: 'enabled', title: () => t('agentEditor.publish.colEnabled'), cell: 'enabled', width: 100 },
  { colKey: 'ops', title: () => t('common.edit'), cell: 'ops', width: 120 },
]

const embedMenuOptions = [
  { content: t('common.edit'), value: 'edit' },
  { content: t('common.delete'), value: 'delete' },
]

const embedEmpty = computed(() => t('agentEditor.publish.webEmpty'))

watch(
  () => props.agentId,
  (agentId) => {
    lockedAgentId.value = agentId
    void refreshAll()
  },
)

watch(selectedType, () => {
  void refreshCurrent()
})

onMounted(() => {
  void refreshAll()
})

function formatTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

async function refreshAll(): Promise<void> {
  await Promise.all([loadEmbedRows(), loadApiRows(), loadImRows()])
}

async function refreshCurrent(): Promise<void> {
  if (selectedType.value === 'web') await loadEmbedRows()
  else if (selectedType.value === 'api') await loadApiRows()
  else if (isImType.value) await loadImRows()
}

async function loadEmbedRows(): Promise<void> {
  if (!props.agentId) {
    embedRows.value = []
    return
  }
  embedLoading.value = true
  try {
    const response = await listEmbedChannels(props.agentId)
    embedRows.value = (response?.data || []).map((channel) => ({
      id: channel.id,
      name: channel.name || t('embedPublish.unnamed'),
      enabled: !!channel.enabled,
      updatedAt: formatTime(channel.updated_at || channel.created_at),
      raw: channel,
    }))
  } catch {
    embedRows.value = []
  } finally {
    embedLoading.value = false
  }
}

async function loadApiRows(): Promise<void> {
  const tenantId = authStore.selectedTenantId || Number(authStore.tenant?.id || 0)
  if (!tenantId) {
    apiRows.value = []
    return
  }
  apiLoading.value = true
  try {
    const response = await listTenantAPIKeys(tenantId)
    apiRows.value = (response?.data || []).map((key: TenantAPIKey) => ({
      id: key.id,
      name: key.name,
      apiKey: maskKey(key.api_key),
      createdAt: formatTime(key.created_at),
      lastUsedAt: formatTime(key.last_used_at),
    }))
  } catch {
    apiRows.value = []
  } finally {
    apiLoading.value = false
  }
}

async function loadImRows(): Promise<void> {
  if (!props.agentId) {
    imRows.value = []
    return
  }
  imLoading.value = true
  try {
    const response = await listIMChannels(props.agentId)
    const platforms = imPlatformFilter.value
    const allowed = platforms
      ? new Set((Array.isArray(platforms) ? platforms : [platforms]).map((item) => item.toLowerCase()))
      : null
    imRows.value = (response?.data || [])
      .filter((channel) => !allowed || allowed.has(String(channel.platform || '').toLowerCase()))
      .map((channel) => ({
        id: channel.id,
        name: channel.name || t('agentEditor.im.unnamed'),
        platform: channel.platform,
        enabled: !!channel.enabled,
        raw: channel,
      }))
  } catch {
    imRows.value = []
  } finally {
    imLoading.value = false
  }
}

function maskKey(value: string): string {
  if (!value) return '—'
  if (value.length <= 6) return '******'
  return `******${value.slice(-4)}`
}

async function copyText(value: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(value)
    MessagePlugin.success(t('common.copied'))
  } catch {
    MessagePlugin.error(t('common.copyFailed'))
  }
}

function startEmbed(channel: EmbedChannel): void {
  embedPanelRef.value?.openDrawer(channel)
}

function onEmbedMenu(value: string | number | Record<string, unknown>, channel: EmbedChannel): void {
  const action = typeof value === 'object' && value && 'value' in value
    ? String((value as { value: string }).value)
    : String(value)
  if (action === 'edit') {
    embedPanelRef.value?.openDrawer(channel)
    return
  }
  if (action === 'delete') {
    embedPanelRef.value?.openDrawer(channel)
  }
}

function openCreateApiKey(): void {
  createApiKeyName.value = ''
  createApiKeyVisible.value = true
}

async function createApiKey(): Promise<boolean> {
  const name = createApiKeyName.value.trim()
  if (!name) {
    MessagePlugin.error(t('integrations.api.apiKeyNameRequired'))
    return false
  }
  const tenantId = authStore.selectedTenantId || Number(authStore.tenant?.id || 0)
  if (!tenantId) {
    MessagePlugin.error(t('integrations.api.createApiKeyFailed'))
    return false
  }
  createApiKeyLoading.value = true
  try {
    const response = await createTenantAPIKey(tenantId, {
      name,
      full_access: true,
    })
    if (!response.success) {
      throw new Error(response.message || t('integrations.api.createApiKeyFailed'))
    }
    MessagePlugin.success(t('integrations.api.apiKeyCreated'))
    await loadApiRows()
    return true
  } catch (error: unknown) {
    const message = error instanceof Error
      ? error.message
      : t('integrations.api.createApiKeyFailed')
    MessagePlugin.error(message)
    return false
  } finally {
    createApiKeyLoading.value = false
  }
}
</script>

<style scoped lang="less">
.publish-channels {
  display: flex;
  flex-direction: column;
  gap: 20px;
  height: 100%;
  min-height: 0;
  padding: 16px 20px 24px;
  overflow: auto;
}

.channel-type-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.channel-type-card {
  position: relative;
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 10px;
  background: var(--td-bg-color-container);
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;

  &:hover {
    border-color: var(--td-brand-color);
  }

  &.is-selected {
    border-color: var(--td-brand-color);
    box-shadow: 0 0 0 1px var(--td-brand-color);
  }
}

.channel-type-card__check {
  position: absolute;
  top: 10px;
  right: 10px;
  color: var(--td-brand-color);
  font-size: 18px;
  line-height: 1;
}

.channel-type-card__radio {
  display: inline-block;
  width: 14px;
  height: 14px;
  border: 1.5px solid var(--td-component-border);
  border-radius: 50%;
}

.channel-type-card__icon {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--td-bg-color-secondarycontainer);
  flex-shrink: 0;
  overflow: hidden;

  img {
    width: 22px;
    height: 22px;
    object-fit: contain;
  }
}

.channel-type-card__title {
  font-size: 14px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  margin-bottom: 4px;
}

.channel-type-card__desc {
  font-size: 12px;
  line-height: 1.45;
  color: var(--td-text-color-secondary);
}

.channel-detail {
  border: 1px solid var(--td-component-stroke);
  border-radius: 10px;
  padding: 16px;
  background: var(--td-bg-color-container);
  min-height: 280px;
}

.channel-detail__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.channel-detail__title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;

  h3 {
    margin: 0;
    font-size: 15px;
    font-weight: 600;
  }
}

.channel-detail__help {
  color: var(--td-text-color-placeholder);
}

.channel-detail__doc {
  font-size: 12px;
  color: var(--td-brand-color);
  text-decoration: none;
}

.channel-detail__aside {
  display: flex;
  align-items: center;
  gap: 12px;
}

.channel-detail__hint {
  margin: 0 0 12px;
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.api-root {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--td-text-color-secondary);

  code {
    font-family: var(--app-font-family-mono, ui-monospace, monospace);
    color: var(--td-text-color-primary);
    max-width: 280px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.link-btn {
  border: none;
  background: none;
  padding: 0;
  color: var(--td-brand-color);
  cursor: pointer;
  font: inherit;
}

.row-ops {
  display: flex;
  align-items: center;
  gap: 8px;
}

.portal-empty {
  padding: 48px 12px;
}

.table-empty {
  padding: 32px 12px;
  text-align: center;
  color: var(--td-text-color-placeholder);
  font-size: 13px;
}

.publish-host-panel {
  /* Host drawer/preview only; list is rendered by the table above. */
  height: 0;
  overflow: visible;
}

@media (max-width: 960px) {
  .channel-type-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
