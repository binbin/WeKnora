<template>
  <div class="debug-preview">
    <div class="debug-preview__header">
      <div class="debug-preview__title-row">
        <h3 class="debug-preview__title">
          {{ $t('agentEditor.workspace.debugTitle') }}
        </h3>
        <t-button
          variant="outline"
          size="small"
          :disabled="!canChat || isReplying || !sessionId"
          @click="clearSession"
        >
          {{ $t('agentEditor.workspace.debugClear') }}
        </t-button>
      </div>
      <p class="debug-preview__hint">
        {{ $t('agentEditor.workspace.debugHint') }}
      </p>
    </div>

    <div v-if="!canChat" class="debug-preview__empty">
      <t-icon name="chat" size="28px" />
      <p>{{ disabledReason || $t('agentEditor.workspace.debugNeedSave') }}</p>
    </div>

    <template v-else>
      <div ref="scrollContainer" class="debug-preview__messages" @scroll="onScroll">
        <div v-if="messagesList.length === 0" class="debug-preview__placeholder">
          {{ $t('agentEditor.workspace.debugEmpty') }}
        </div>
        <div
          v-for="(message, index) in messagesList"
          :key="String(message.id || index)"
          :class="[
            'debug-preview__bubble',
            message.role === 'user'
              ? 'debug-preview__bubble--user'
              : 'debug-preview__bubble--assistant',
          ]"
        >
          <div class="debug-preview__bubble-role">
            {{
              message.role === 'user'
                ? $t('agentEditor.workspace.debugYou')
                : $t('agentEditor.workspace.debugAgent')
            }}
          </div>
          <div class="debug-preview__bubble-content">
            {{ String(message.content || '') }}
            <span
              v-if="
                message.role === 'assistant' &&
                !message.is_completed &&
                isReplying
              "
              class="debug-preview__cursor"
            >|</span>
          </div>
        </div>
        <div
          v-if="isReplying && !hasTrailingAssistant"
          class="debug-preview__typing"
        >
          {{ $t('agentEditor.workspace.debugThinking') }}
        </div>
      </div>

      <form class="debug-preview__input" @submit.prevent="handleSend">
        <t-textarea
          ref="inputRef"
          v-model="draft"
          :placeholder="$t('agentEditor.workspace.debugPlaceholder')"
          :disabled="isReplying"
          :autosize="{ minRows: 2, maxRows: 4 }"
          @keydown.enter.exact.prevent="handleSend"
        />
        <t-button
          theme="primary"
          type="submit"
          :loading="isReplying"
          :disabled="!draft.trim() || isReplying"
        >
          {{ $t('agentEditor.workspace.debugSend') }}
        </t-button>
      </form>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import { createSessions } from '@/api/chat'
import { useStream } from '@/api/chat/streame'
import {
  useChatStreamHandler,
  type ChatMessage,
} from '@/composables/useChatStreamHandler'

const props = defineProps<{
  agentId: string
  agentMode: 'quick-answer' | 'smart-reasoning'
  disabledReason?: string
}>()

const { t } = useI18n()
const { onChunk, error, startStream, stopStream } = useStream()

const canChat = computed(() => !!props.agentId && !props.disabledReason)
const draft = ref('')
const sessionId = ref('')
const messagesList = reactive<ChatMessage[]>([])
const isReplying = ref(false)
const loading = ref(false)
const currentAssistantMessageId = ref('')
const fullContent = ref('')
const scrollContainer = ref<HTMLElement | null>(null)
const userHasScrolledUp = ref(false)
const SCROLL_BOTTOM_THRESHOLD = 80

const isAgentMode = computed(() => props.agentMode === 'smart-reasoning')

const hasTrailingAssistant = computed(() => {
  const last = messagesList[messagesList.length - 1]
  return !!(last && last.role === 'assistant')
})

const isNearBottom = (): boolean => {
  if (!scrollContainer.value) return true
  const { scrollTop, scrollHeight, clientHeight } = scrollContainer.value
  return scrollHeight - scrollTop - clientHeight < SCROLL_BOTTOM_THRESHOLD
}

const scrollToBottom = (force = false): void => {
  if (!force && userHasScrolledUp.value) return
  nextTick(() => {
    if (scrollContainer.value) {
      scrollContainer.value.scrollTop = scrollContainer.value.scrollHeight
    }
  })
}

const onScroll = (): void => {
  userHasScrolledUp.value = !isNearBottom()
}

const {
  processStreamChunk,
  prepareForNewOutgoingMessage,
} = useChatStreamHandler({
  messagesList,
  loading,
  isReplying,
  currentAssistantMessageId,
  fullContent,
  isAgentStreamSession: () => isAgentMode.value,
  scrollToBottom,
  onError: (message) => MessagePlugin.error(message),
})

onChunk((data) => {
  processStreamChunk(data)
})

watch(error, (message) => {
  if (!message) return
  MessagePlugin.error(message)
  isReplying.value = false
})

watch(
  () => props.agentId,
  () => {
    resetLocalState()
  },
)

function resetLocalState(): void {
  stopStream()
  sessionId.value = ''
  messagesList.splice(0, messagesList.length)
  draft.value = ''
  isReplying.value = false
  loading.value = false
  currentAssistantMessageId.value = ''
  fullContent.value = ''
  userHasScrolledUp.value = false
}

async function ensureSession(): Promise<string> {
  if (sessionId.value) return sessionId.value
  const response = await createSessions({})
  const nextId = response?.data?.id
  if (!nextId) {
    throw new Error(t('createChat.messages.createFailed'))
  }
  sessionId.value = String(nextId)
  return sessionId.value
}

async function handleSend(): Promise<void> {
  const query = draft.value.trim()
  if (!query || !canChat.value || isReplying.value) return

  try {
    const activeSessionId = await ensureSession()
    draft.value = ''
    userHasScrolledUp.value = false
    prepareForNewOutgoingMessage()
    messagesList.push({
      content: query,
      role: 'user',
      channel: 'web',
    })
    scrollToBottom(true)

    const endpoint = isAgentMode.value
      ? '/api/v1/agent-chat'
      : '/api/v1/knowledge-chat'

    await startStream({
      session_id: activeSessionId,
      query,
      agent_enabled: isAgentMode.value,
      agent_id: props.agentId,
      method: 'POST',
      url: endpoint,
    })
  } catch (err) {
    const message =
      err instanceof Error ? err.message : t('createChat.messages.createError')
    MessagePlugin.error(message)
    isReplying.value = false
  }
}

function clearSession(): void {
  resetLocalState()
}

onBeforeUnmount(() => {
  stopStream()
})
</script>

<style scoped lang="less">
.debug-preview {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  background: var(--td-bg-color-container);
}

.debug-preview__header {
  flex-shrink: 0;
  padding: 14px 16px 10px;
  border-bottom: 1px solid var(--td-component-stroke);
}

.debug-preview__title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.debug-preview__title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.debug-preview__hint {
  margin: 6px 0 0;
  font-size: 12px;
  line-height: 1.4;
  color: var(--td-text-color-secondary);
}

.debug-preview__empty {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 24px;
  color: var(--td-text-color-secondary);
  text-align: center;
  font-size: 13px;
}

.debug-preview__messages {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.debug-preview__placeholder {
  margin: auto;
  font-size: 13px;
  color: var(--td-text-color-placeholder);
}

.debug-preview__bubble {
  max-width: 92%;
  padding: 10px 12px;
  border-radius: 10px;
  font-size: 13px;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-word;
}

.debug-preview__bubble--user {
  align-self: flex-end;
  background: color-mix(
    in srgb,
    var(--td-brand-color) 12%,
    var(--td-bg-color-secondarycontainer)
  );
}

.debug-preview__bubble--assistant {
  align-self: flex-start;
  background: var(--td-bg-color-secondarycontainer);
}

.debug-preview__bubble-role {
  margin-bottom: 4px;
  font-size: 11px;
  font-weight: 600;
  color: var(--td-text-color-placeholder);
}

.debug-preview__cursor {
  display: inline-block;
  margin-left: 2px;
  animation: debug-blink 1s steps(1) infinite;
  color: var(--td-brand-color);
}

.debug-preview__typing {
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.debug-preview__input {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px 16px 16px;
  border-top: 1px solid var(--td-component-stroke);
}

@keyframes debug-blink {
  50% {
    opacity: 0;
  }
}
</style>
