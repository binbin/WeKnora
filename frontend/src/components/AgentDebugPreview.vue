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
          :key="String(message.id || `${message.role}-${index}`)"
          class="debug-preview__msg"
        >
          <EmbedUserMessage
            v-if="message.role === 'user'"
            :content="String(message.content || '')"
            :images="asDebugImages(message.images)"
            :attachments="asDebugAttachments(message.attachments)"
            :embedded-mode="false"
          />
          <EmbedBotMessage
            v-else-if="
              message.role === 'assistant' &&
              shouldRenderAssistantMessage(message)
            "
            :content="String(message.content || '')"
            :session="message"
            :session-id="sessionId"
            :user-query="getUserQuery(index)"
            :embedded-mode="false"
          />
        </div>
        <div
          v-if="showTypingIndicator"
          class="debug-preview__typing"
          role="status"
          :aria-label="$t('chat.thinkingAlt')"
        >
          <span class="debug-preview__typing-spinner" aria-hidden="true" />
        </div>
      </div>

      <div class="debug-preview__composer">
        <div
          v-if="imageUploadEnabled && uploadedAttachments.length"
          class="debug-preview__files"
        >
          <div
            v-for="(attachment, index) in uploadedAttachments"
            :key="`${attachment.file.name}-${index}`"
            class="debug-preview__file-chip"
          >
            <t-icon name="file" size="14px" />
            <span>{{ attachment.file.name }}</span>
            <button
              type="button"
              class="debug-preview__remove"
              @click="removeAttachment(index)"
            >
              <t-icon name="close" size="12px" />
            </button>
          </div>
        </div>
        <div
          v-if="imageUploadEnabled && uploadedImages.length"
          class="debug-preview__images"
        >
          <div
            v-for="(image, index) in uploadedImages"
            :key="image.preview"
            class="debug-preview__image-thumb"
          >
            <img :src="image.preview" :alt="image.file.name" />
            <button
              type="button"
              class="debug-preview__remove"
              @click="removeImage(index)"
            >
              <t-icon name="close" size="12px" />
            </button>
          </div>
        </div>

        <form class="debug-preview__input" @submit.prevent="handleSend">
          <input
            v-if="imageUploadEnabled"
            ref="imageInputRef"
            type="file"
            accept="image/jpeg,image/png,image/gif,image/webp"
            multiple
            class="debug-preview__hidden-input"
            @change="handleImageSelect"
          />
          <input
            v-if="imageUploadEnabled"
            ref="fileInputRef"
            type="file"
            accept=".pdf,.doc,.docx,.txt,.md,.csv,.xlsx,.xls,.ppt,.pptx,application/pdf,text/plain"
            multiple
            class="debug-preview__hidden-input"
            @change="handleFileSelect"
          />
          <div v-if="imageUploadEnabled" class="debug-preview__upload-btns">
            <t-tooltip :content="$t('chat.imageUploadTooltip')" placement="top">
              <button
                type="button"
                class="debug-preview__upload-btn"
                :class="{ active: uploadedImages.length > 0 }"
                :aria-label="$t('chat.imageUploadTooltip')"
                @click="triggerImageUpload"
              >
                <t-icon name="image" size="18px" />
              </button>
            </t-tooltip>
            <t-tooltip
              :content="$t('chat.attachmentUploadTooltip')"
              placement="top"
            >
              <button
                type="button"
                class="debug-preview__upload-btn"
                :class="{ active: uploadedAttachments.length > 0 }"
                :aria-label="$t('chat.attachmentUploadTooltip')"
                @click="triggerFileUpload"
              >
                <t-icon name="attach" size="18px" />
              </button>
            </t-tooltip>
          </div>
          <t-textarea
            ref="inputRef"
            v-model="draft"
            :placeholder="$t('agentEditor.workspace.debugPlaceholder')"
            :disabled="isReplying"
            :autosize="{ minRows: 2, maxRows: 4 }"
            @keydown="onDebugKeydown"
          />
          <t-button
            theme="primary"
            type="submit"
            :loading="isReplying"
            :disabled="!canSend || isReplying"
          >
            {{ $t('agentEditor.workspace.debugSend') }}
          </t-button>
        </form>
      </div>
    </template>
    <ChatReferencesDrawer />
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
import { provideChatReferencesDrawer } from '@/composables/useChatReferencesDrawer'
import ChatReferencesDrawer from '@/components/ChatReferencesDrawer.vue'
import EmbedBotMessage from '@/views/embed/EmbedBotMessage.vue'
import EmbedUserMessage from '@/views/embed/EmbedUserMessage.vue'
import { fileToDataURI, isEmbedImageFile } from '@/utils/embedFile'

provideChatReferencesDrawer()

const MAX_IMAGE_COUNT = 5
const MAX_ATTACHMENT_COUNT = 5
const MAX_IMAGE_BYTES = 10 * 1024 * 1024
const MAX_ATTACHMENT_BYTES = 20 * 1024 * 1024
const ALLOWED_IMAGE_TYPES = [
  'image/jpeg',
  'image/png',
  'image/gif',
  'image/webp',
]

type DebugImage = { url?: string; data?: string }
type DebugAttachment = { file_name: string; file_size?: number }

const props = withDefaults(
  defineProps<{
    agentId: string
    agentMode: 'quick-answer' | 'smart-reasoning'
    disabledReason?: string
    imageUploadEnabled?: boolean
  }>(),
  {
    imageUploadEnabled: false,
  },
)

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
const imageInputRef = ref<HTMLInputElement | null>(null)
const fileInputRef = ref<HTMLInputElement | null>(null)
const uploadedImages = ref<Array<{ file: File; preview: string }>>([])
const uploadedAttachments = ref<Array<{ file: File }>>([])

const canSend = computed(
  () =>
    draft.value.trim().length > 0 ||
    uploadedImages.value.length > 0 ||
    uploadedAttachments.value.length > 0,
)

const isAgentMode = computed(() => props.agentMode === 'smart-reasoning')

const asDebugImages = (value: unknown): DebugImage[] =>
  Array.isArray(value) ? (value as DebugImage[]) : []

const asDebugAttachments = (value: unknown): DebugAttachment[] =>
  Array.isArray(value) ? (value as DebugAttachment[]) : []

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
  shouldRenderAssistantMessage,
  shouldShowGlobalTypingIndicator,
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

const showTypingIndicator = computed(() =>
  shouldShowGlobalTypingIndicator(messagesList, isReplying.value),
)

const getUserQuery = (index: number): string => {
  for (let cursor = index - 1; cursor >= 0; cursor -= 1) {
    const item = messagesList[cursor]
    if (item?.role === 'user') {
      return String(item.content || '')
    }
  }
  return ''
}

const revokeImagePreviews = (): void => {
  for (const image of uploadedImages.value) {
    URL.revokeObjectURL(image.preview)
  }
}

const clearUploads = (): void => {
  revokeImagePreviews()
  uploadedImages.value = []
  uploadedAttachments.value = []
}

const triggerImageUpload = (): void => {
  imageInputRef.value?.click()
}

const triggerFileUpload = (): void => {
  fileInputRef.value?.click()
}

const addImageFiles = (files: File[]): void => {
  for (const file of files) {
    if (!isEmbedImageFile(file)) continue
    if (uploadedImages.value.length >= MAX_IMAGE_COUNT) {
      MessagePlugin.warning(t('chat.imageTooMany'))
      break
    }
    if (!ALLOWED_IMAGE_TYPES.includes(file.type)) {
      MessagePlugin.warning(t('chat.imageTypeSizeError'))
      continue
    }
    if (file.size > MAX_IMAGE_BYTES) {
      MessagePlugin.warning(t('chat.imageTypeSizeError'))
      continue
    }
    uploadedImages.value.push({
      file,
      preview: URL.createObjectURL(file),
    })
  }
}

const addAttachmentFiles = (files: File[]): void => {
  for (const file of files) {
    if (isEmbedImageFile(file)) continue
    if (uploadedAttachments.value.length >= MAX_ATTACHMENT_COUNT) {
      MessagePlugin.warning(
        t('chat.attachmentTooMany', { max: MAX_ATTACHMENT_COUNT }),
      )
      break
    }
    if (file.size > MAX_ATTACHMENT_BYTES) {
      MessagePlugin.warning(
        t('chat.attachmentTooLarge', {
          name: file.name,
          max: MAX_ATTACHMENT_BYTES / (1024 * 1024),
        }),
      )
      continue
    }
    uploadedAttachments.value.push({ file })
  }
}

const handleImageSelect = (event: Event): void => {
  const input = event.target as HTMLInputElement
  if (!input.files) return
  addImageFiles(Array.from(input.files))
  input.value = ''
}

const handleFileSelect = (event: Event): void => {
  const input = event.target as HTMLInputElement
  if (!input.files) return
  addAttachmentFiles(Array.from(input.files))
  input.value = ''
}

const removeImage = (index: number): void => {
  const [removed] = uploadedImages.value.splice(index, 1)
  if (removed) URL.revokeObjectURL(removed.preview)
}

const removeAttachment = (index: number): void => {
  uploadedAttachments.value.splice(index, 1)
}

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

watch(
  () => props.imageUploadEnabled,
  (enabled) => {
    if (!enabled) clearUploads()
  },
)

function resetLocalState(): void {
  stopStream()
  sessionId.value = ''
  messagesList.splice(0, messagesList.length)
  draft.value = ''
  clearUploads()
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

/** TDesign textarea 的 keydown 首参是 value 字符串，不能用 Vue 的 .enter 修饰符。 */
function onDebugKeydown(
  _value: string,
  context: { e: KeyboardEvent },
): void {
  const event = context.e
  if (
    event.key === 'Enter' &&
    !event.shiftKey &&
    !event.altKey &&
    !event.ctrlKey &&
    !event.metaKey
  ) {
    event.preventDefault()
    void handleSend()
  }
}

async function handleSend(): Promise<void> {
  const query = draft.value.trim()
  const imageFiles = props.imageUploadEnabled
    ? uploadedImages.value.map((item) => item.file)
    : []
  const attachmentFiles = props.imageUploadEnabled
    ? uploadedAttachments.value.map((item) => item.file)
    : []
  if (
    (!query && imageFiles.length === 0 && attachmentFiles.length === 0) ||
    !canChat.value ||
    isReplying.value
  ) {
    return
  }

  try {
    const activeSessionId = await ensureSession()
    const imageAttachments: Array<{ data: string }> = []
    const displayImages: Array<{ url: string }> = []
    const attachmentUploads: Array<{
      data: string
      file_name: string
      file_size: number
    }> = []
    const displayAttachments: Array<{ file_name: string; file_size: number }> =
      []

    for (const file of imageFiles) {
      const dataURI = await fileToDataURI(file)
      imageAttachments.push({ data: dataURI })
      displayImages.push({ url: dataURI })
    }
    for (const file of attachmentFiles) {
      const dataURI = await fileToDataURI(file)
      attachmentUploads.push({
        data: dataURI,
        file_name: file.name,
        file_size: file.size,
      })
      displayAttachments.push({
        file_name: file.name,
        file_size: file.size,
      })
    }

    draft.value = ''
    clearUploads()
    userHasScrolledUp.value = false
    prepareForNewOutgoingMessage()
    messagesList.push({
      content: query,
      role: 'user',
      images: displayImages,
      attachments: displayAttachments,
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
      images: imageAttachments.length > 0 ? imageAttachments : undefined,
      attachment_uploads:
        attachmentUploads.length > 0 ? attachmentUploads : undefined,
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
  clearUploads()
})
</script>

<style scoped lang="less">
.debug-preview {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.debug-preview__header {
  flex-shrink: 0;
  margin-bottom: 12px;
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
}

.debug-preview__hint {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.debug-preview__empty {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--td-text-color-placeholder);
  text-align: center;
  padding: 24px;
}

.debug-preview__messages {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 8px 0;
}

.debug-preview__placeholder {
  padding: 24px 12px;
  text-align: center;
  color: var(--td-text-color-placeholder);
  font-size: 13px;
}

.debug-preview__msg {
  margin-bottom: 12px;
}

.debug-preview__typing {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  color: var(--td-text-color-secondary);
  font-size: 13px;
}

.debug-preview__typing-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid var(--td-component-border);
  border-top-color: var(--td-brand-color);
  border-radius: 50%;
  animation: debug-spin 0.7s linear infinite;
}

.debug-preview__composer {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--td-component-border);
}

.debug-preview__files,
.debug-preview__images {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.debug-preview__file-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  max-width: 100%;
  padding: 4px 8px;
  border-radius: 6px;
  background: var(--td-bg-color-secondarycontainer);
  font-size: 12px;

  span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.debug-preview__image-thumb {
  position: relative;
  width: 48px;
  height: 48px;
  border-radius: 6px;
  overflow: hidden;

  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
}

.debug-preview__remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin-left: 2px;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--td-text-color-secondary);
  cursor: pointer;
}

.debug-preview__image-thumb .debug-preview__remove {
  position: absolute;
  top: 2px;
  right: 2px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  margin: 0;
}

.debug-preview__input {
  display: flex;
  align-items: flex-end;
  gap: 8px;
}

.debug-preview__upload-btns {
  display: flex;
  flex-shrink: 0;
  gap: 2px;
  padding-bottom: 4px;
}

.debug-preview__upload-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--td-text-color-secondary);
  cursor: pointer;

  &:hover,
  &.active {
    color: var(--td-brand-color);
    background: var(--td-bg-color-container-hover);
  }
}

.debug-preview__hidden-input {
  display: none;
}

.debug-preview__input :deep(.t-textarea) {
  flex: 1;
  min-width: 0;
}

@keyframes debug-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
