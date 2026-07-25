<template>
  <t-drawer
    :visible="visible"
    size="720px"
    :footer="false"
    :z-index="2700"
    attach="body"
    class="conversation-detail-drawer"
    @close="close"
  >
    <template #header>
      <div class="detail-header">
        <div class="detail-header__main">
          <h3 class="detail-header__title" :title="title">{{ title }}</h3>
          <span class="detail-header__badge">
            <t-icon name="time" size="14px" />
            {{ $t('agentEditor.logs.recordCount', { count: messages.length }) }}
          </span>
          <span v-if="modelLabel" class="detail-header__model">
            {{ modelLabel }}
          </span>
        </div>
      </div>
    </template>

    <t-loading :loading="loading" size="small" class="detail-body">
      <div v-if="!loading && messages.length === 0" class="detail-empty">
        {{ $t('agentEditor.logs.detailEmpty') }}
      </div>

      <div v-else class="detail-messages">
        <template v-for="(message, index) in messages" :key="message.id || index">
          <div
            v-if="shouldShowTimeDivider(index)"
            class="detail-time-divider"
          >
            {{ formatMessageTime(message.created_at) }}
          </div>

          <div
            :class="[
              'detail-message',
              message.role === 'user'
                ? 'detail-message--user'
                : 'detail-message--assistant',
            ]"
          >
            <div class="detail-message__meta">
              <template v-if="message.role === 'user'">
                <span class="detail-message__time">
                  {{ formatMessageTime(message.created_at) }}
                </span>
                <t-button
                  size="small"
                  variant="text"
                  shape="square"
                  :title="$t('common.copy')"
                  @click="copyMessage(message)"
                >
                  <t-icon name="copy" />
                </t-button>
              </template>
              <template v-else>
                <t-button
                  size="small"
                  variant="text"
                  shape="square"
                  :title="$t('common.copy')"
                  @click="copyMessage(message)"
                >
                  <t-icon name="copy" />
                </t-button>
                <t-button
                  size="small"
                  variant="text"
                  shape="square"
                  theme="danger"
                  :title="$t('common.delete')"
                  :loading="deletingId === message.id"
                  @click="confirmDeleteMessage(message)"
                >
                  <t-icon name="delete" />
                </t-button>
                <span class="detail-message__time">
                  {{ formatMessageTime(message.created_at) }}
                </span>
              </template>
            </div>

            <div
              :class="[
                'detail-message__bubble',
                message.role === 'user'
                  ? 'detail-message__bubble--user'
                  : 'detail-message__bubble--assistant',
              ]"
            >
              <div
                v-if="message.role === 'assistant'"
                class="detail-message__markdown markdown-content"
                v-html="renderContent(message.content)"
              />
              <div v-else class="detail-message__text">
                {{ message.content }}
              </div>

              <div
                v-if="
                  message.role === 'assistant' &&
                  message.knowledge_references?.length
                "
                class="detail-message__refs"
              >
                <docInfo :session="message" embedded-mode />
              </div>
            </div>

            <div
              v-if="message.role === 'assistant' && hasAssistantMeta(message)"
              class="detail-message__stats"
            >
              <span v-if="message.knowledge_references?.length">
                {{
                  $t('agentEditor.logs.citationCount', {
                    count: message.knowledge_references.length,
                  })
                }}
              </span>
              <span v-if="message.agent_duration_ms">
                {{ formatDuration(message.agent_duration_ms) }}
              </span>
            </div>
          </div>
        </template>
      </div>
    </t-loading>

    <div class="detail-feedback">
      <button
        type="button"
        class="detail-feedback__toggle"
        @click="feedbackOpen = !feedbackOpen"
      >
        <span>{{ $t('agentEditor.logs.colFeedback') }}</span>
        <t-icon :name="feedbackOpen ? 'chevron-up' : 'chevron-down'" />
      </button>
      <div v-show="feedbackOpen" class="detail-feedback__body">
        {{ $t('agentEditor.logs.feedbackEmpty') }}
      </div>
    </div>
  </t-drawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import { deleteMessage, getMessageList, getSession } from '@/api/chat'
import { useConfirmDelete } from '@/components/settings/useConfirmDelete'
import { useChatResourcesStore } from '@/stores/chatResources'
import {
  collectAllSessionMessages,
  type SessionExportMessage,
} from '@/utils/sessionMarkdown'
import { copyTextToClipboard } from '@/utils/chatMessageShared'
import {
  createChatMarkdownRenderer,
  renderChatMarkdown,
} from '@/utils/chatMarkdownRenderer'
import {
  sanitizeMarkdownHTML,
  safeMarkdownToHTML,
} from '@/utils/security'
import docInfo from '@/views/chat/components/docInfo.vue'

interface DetailMessage extends SessionExportMessage {
  model_id?: string
  agent_duration_ms?: number
  knowledge_references?: Array<Record<string, unknown>>
}

const props = defineProps<{
  visible: boolean
  sessionId: string
  fallbackTitle?: string
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
}>()

const { t } = useI18n()
const confirmDelete = useConfirmDelete()
const chatResources = useChatResourcesStore()

const loading = ref(false)
const messages = ref<DetailMessage[]>([])
const sessionTitle = ref('')
const sessionModelId = ref('')
const deletingId = ref('')
const feedbackOpen = ref(false)

const markdownRenderer = createChatMarkdownRenderer()

const title = computed(
  () =>
    sessionTitle.value.trim() ||
    props.fallbackTitle?.trim() ||
    t('createChat.newSessionTitle'),
)

const modelLabel = computed(() => {
  const modelId =
    sessionModelId.value ||
    [...messages.value]
      .reverse()
      .find((message) => message.model_id)?.model_id ||
    ''
  if (!modelId) return ''
  const matched = chatResources.allModels.find(
    (model) => model.id === modelId,
  )
  return matched?.display_name || matched?.name || modelId
})

watch(
  () => [props.visible, props.sessionId] as const,
  ([open, sessionId]) => {
    if (open && sessionId) {
      void loadDetail(sessionId)
    }
    if (!open) {
      messages.value = []
      sessionTitle.value = ''
      sessionModelId.value = ''
      feedbackOpen.value = false
    }
  },
)

function close(): void {
  emit('update:visible', false)
}

async function loadDetail(sessionId: string): Promise<void> {
  loading.value = true
  try {
    await chatResources.ensureModels().catch(() => undefined)
    const [sessionResp, loadedMessages] = await Promise.all([
      getSession(sessionId),
      collectAllSessionMessages(async (beforeTime, limit) => {
        const response = await getMessageList({
          session_id: sessionId,
          limit,
          created_at: beforeTime,
        })
        return (response?.data || []) as DetailMessage[]
      }),
    ])
    const session = sessionResp?.data as
      | {
          title?: string
          last_request_state?: { model_id?: string }
        }
      | undefined
    sessionTitle.value = String(session?.title || '')
    sessionModelId.value = String(
      session?.last_request_state?.model_id || '',
    )
    messages.value = loadedMessages as DetailMessage[]
  } catch (err) {
    console.error('Failed to load conversation detail', err)
    MessagePlugin.error(t('agentEditor.logs.detailLoadFailed'))
    messages.value = []
  } finally {
    loading.value = false
  }
}

function renderContent(content?: string): string {
  return renderChatMarkdown(content || '', {
    renderer: markdownRenderer,
    escapeMarkdown: safeMarkdownToHTML,
    sanitizeHtml: sanitizeMarkdownHTML,
  })
}

function formatMessageTime(value?: string): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  return `${month}-${day} ${hours}:${minutes}`
}

function shouldShowTimeDivider(index: number): boolean {
  if (index === 0) return false
  const current = messages.value[index]?.created_at
  const previous = messages.value[index - 1]?.created_at
  if (!current || !previous) return false
  return formatMessageTime(current) !== formatMessageTime(previous)
}

function formatDuration(durationMs: number): string {
  return `${(durationMs / 1000).toFixed(2)}s`
}

function hasAssistantMeta(message: DetailMessage): boolean {
  return !!(
    message.knowledge_references?.length ||
    message.agent_duration_ms
  )
}

async function copyMessage(message: DetailMessage): Promise<void> {
  try {
    await copyTextToClipboard(String(message.content || ''))
    MessagePlugin.success(t('common.copied'))
  } catch {
    MessagePlugin.error(t('chatHeader.copyFailed'))
  }
}

function confirmDeleteMessage(message: DetailMessage): void {
  if (!message.id || !props.sessionId) return
  confirmDelete({
    title: t('agentEditor.logs.deleteMessageTitle'),
    body: t('agentEditor.logs.deleteMessageBody'),
    onConfirm: async () => {
      deletingId.value = message.id || ''
      try {
        const response = await deleteMessage(props.sessionId, message.id!)
        if (!response?.success) {
          throw new Error(response?.message || 'delete failed')
        }
        messages.value = messages.value.filter(
          (item) => item.id !== message.id,
        )
        MessagePlugin.success(t('agentEditor.logs.deleteMessageSuccess'))
      } catch (err) {
        console.error('Failed to delete message', err)
        MessagePlugin.error(t('agentEditor.logs.deleteMessageFailed'))
      } finally {
        deletingId.value = ''
      }
    },
  })
}
</script>

<style scoped lang="less">
.detail-header {
  display: flex;
  align-items: center;
  min-width: 0;
  padding-right: 8px;
}

.detail-header__main {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  flex: 1;
}

.detail-header__title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 280px;
}

.detail-header__badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
  padding: 2px 10px;
  border-radius: 999px;
  font-size: 12px;
  color: var(--td-brand-color);
  background: color-mix(in srgb, var(--td-brand-color) 12%, transparent);
}

.detail-header__model {
  flex-shrink: 0;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  color: var(--td-success-color);
  background: color-mix(in srgb, var(--td-success-color) 12%, transparent);
}

.detail-body {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: calc(100% - 52px);
  overflow: auto;
}

.detail-empty {
  padding: 48px 16px;
  text-align: center;
  color: var(--td-text-color-placeholder);
  font-size: 13px;
}

.detail-messages {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 4px 4px 20px;
}

.detail-time-divider {
  align-self: center;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
  padding: 4px 0;
}

.detail-message {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-width: 92%;

  &--user {
    align-self: flex-end;
    align-items: flex-end;
  }

  &--assistant {
    align-self: flex-start;
    align-items: flex-start;
  }
}

.detail-message__meta {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  color: var(--td-text-color-placeholder);
}

.detail-message__time {
  font-size: 12px;
  padding: 0 4px;
}

.detail-message__bubble {
  border-radius: 12px;
  padding: 12px 14px;
  font-size: 14px;
  line-height: 1.6;
  word-break: break-word;

  &--user {
    background: color-mix(in srgb, var(--td-brand-color) 12%, transparent);
    color: var(--td-text-color-primary);
  }

  &--assistant {
    background: var(--td-bg-color-container);
    border: 1px solid var(--td-component-stroke);
    color: var(--td-text-color-primary);
  }
}

.detail-message__text {
  white-space: pre-wrap;
}

.detail-message__markdown {
  :deep(p) {
    margin: 0 0 0.6em;

    &:last-child {
      margin-bottom: 0;
    }
  }

  :deep(ul),
  :deep(ol) {
    margin: 0.4em 0;
    padding-left: 1.4em;
  }
}

.detail-message__refs {
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px dashed var(--td-component-stroke);
}

.detail-message__stats {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.detail-feedback {
  border-top: 1px solid var(--td-component-stroke);
  margin-top: 8px;
}

.detail-feedback__toggle {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  border: none;
  background: transparent;
  padding: 12px 4px;
  cursor: pointer;
  color: var(--td-text-color-primary);
  font-size: 13px;
}

.detail-feedback__body {
  padding: 0 4px 12px;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}
</style>

<style lang="less">
.conversation-detail-drawer {
  .t-drawer__body {
    display: flex;
    flex-direction: column;
    padding: 12px 20px 16px;
    overflow: hidden;
  }
}
</style>
