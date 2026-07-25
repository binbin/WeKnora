<template>
  <div class="guest-link-panel">
    <t-loading :loading="loading" size="small" class="channels-loading-wrap">
      <div v-if="!loading && !guestLink" class="channels-empty">
        <t-empty :description="$t('guestLinkPublish.empty')">
          <template v-if="canManage" #action>
            <t-button theme="primary" @click="openCreateDialog">
              {{ $t('guestLinkPublish.create') }}
            </t-button>
          </template>
        </t-empty>
      </div>

      <div v-else-if="guestLink" class="channel-grid">
        <div class="channel-card channel-card--guest">
          <div class="channel-card__badge">
            <t-icon name="link" size="22px" />
          </div>
          <div class="channel-card__body">
            <div class="channel-card__header">
              <h3 class="channel-card__title">{{ displayName }}</h3>
              <t-tag v-if="!guestLink.enabled" size="small" variant="light" theme="warning">
                {{ $t('guestLinkPublish.disabled') }}
              </t-tag>
            </div>
            <div class="channel-card__web-link">
              <span class="channel-card__web-url">{{ guestLinkUrl }}</span>
              <div class="channel-card__web-actions">
                <t-tooltip :content="$t('common.copy')" placement="top">
                  <t-button variant="text" shape="square" size="small" class="channel-card__action-btn"
                    @click="copyUrl">
                    <template #icon><t-icon name="file-copy" /></template>
                  </t-button>
                </t-tooltip>
                <t-tooltip :content="$t('guestLinkPublish.open')" placement="top">
                  <t-button variant="text" shape="square" size="small" class="channel-card__action-btn"
                    @click="openUrl">
                    <template #icon><t-icon name="browse" /></template>
                  </t-button>
                </t-tooltip>
                <t-tooltip v-if="canManage" :content="$t('guestLinkPublish.settings')" placement="top">
                  <t-button variant="text" shape="square" size="small" class="channel-card__action-btn"
                    @click="openSettings">
                    <template #icon><t-icon name="setting" /></template>
                  </t-button>
                </t-tooltip>
              </div>
            </div>
          </div>
        </div>
      </div>
    </t-loading>

    <t-dialog
      v-model:visible="createVisible"
      :header="$t('guestLinkPublish.createTitle')"
      :confirm-btn="{
        content: $t('guestLinkPublish.create'),
        loading: creating,
      }"
      :cancel-btn="$t('common.cancel')"
      @confirm="submitCreate"
    >
      <!-- 非受控：每次打开用 key 重置默认值，提交时从 FormData 读取 -->
      <form
        :key="createFormKey"
        ref="createFormRef"
        class="guest-link-create-form"
        @submit.prevent="submitCreate"
      >
        <label class="form-label" for="guest-link-create-title">
          {{ $t('guestLinkPublish.title') }}
        </label>
        <input
          id="guest-link-create-title"
          name="title"
          type="text"
          class="guest-link-create-input"
          :defaultValue="defaultCreateTitle"
          :placeholder="$t('guestLinkPublish.titlePlaceholder')"
        />
        <p class="form-desc">{{ $t('guestLinkPublish.titleHint') }}</p>
      </form>
    </t-dialog>

    <SettingDrawer v-model:visible="showDrawer" class="guest-link-drawer" :title="$t('guestLinkPublish.settingsTitle')"
      icon="link" storage-key="setting-drawer:guest-link" width="480px" :confirm-loading="saving"
      :confirm-text="$t('common.save')" :hide-footer="!canManage" @confirm="saveForm" @cancel="closeDrawer">
      <template #footer-left>
        <t-popconfirm v-if="guestLink" theme="warning" :content="$t('guestLinkPublish.deleteConfirm')"
          :confirm-btn="{ content: $t('common.delete'), theme: 'danger' }" :cancel-btn="{ content: $t('common.cancel') }"
          placement="top" @confirm="handleDelete">
          <t-button theme="danger" variant="outline" :loading="deleting">
            {{ $t('common.delete') }}
          </t-button>
        </t-popconfirm>
      </template>

      <section class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ $t('guestLinkPublish.sectionGeneral') }}</h4>

        <div class="setting-row">
          <div class="setting-info">
            <label>{{ $t('guestLinkPublish.enabled') }}</label>
          </div>
          <div class="setting-control">
            <t-switch v-model="form.enabled" size="small" />
          </div>
        </div>

        <div class="form-item">
          <label class="form-label">{{ $t('guestLinkPublish.title') }}</label>
          <t-input v-model="form.name" :placeholder="$t('guestLinkPublish.titlePlaceholder')" />
          <p class="form-desc">{{ $t('guestLinkPublish.titleHint') }}</p>
        </div>

        <div class="form-item">
          <label class="form-label">{{ $t('guestLinkPublish.welcomeMessage') }}</label>
          <t-textarea v-model="form.welcome_message" :placeholder="$t('guestLinkPublish.welcomePlaceholder')"
            :autosize="{ minRows: 2, maxRows: 4 }" />
        </div>
      </section>

      <section class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ $t('guestLinkPublish.sectionLimits') }}</h4>

        <div class="settings-group">
          <div class="setting-row" :class="{ 'setting-row--last': rateLimitUnlimited }">
            <div class="setting-info">
              <label>{{ $t('guestLinkPublish.rateLimitUnlimited') }}</label>
              <p class="setting-desc">{{ $t('guestLinkPublish.rateLimitUnlimitedDesc') }}</p>
            </div>
            <div class="setting-control">
              <t-switch v-model="rateLimitUnlimited" size="small" @change="onRateLimitUnlimitedChange" />
            </div>
          </div>
        </div>

        <div v-if="!rateLimitUnlimited" class="form-item">
          <label class="form-label">{{ $t('guestLinkPublish.rateLimitLabel') }}</label>
          <t-input-number v-model="form.rate_limit_per_minute" :min="1" :max="600" theme="column"
            class="form-number" />
        </div>

        <div v-if="!rateLimitUnlimited" class="form-item">
          <label class="form-label">{{ $t('guestLinkPublish.rateLimitDayLabel') }}</label>
          <t-input-number v-model="form.rate_limit_per_day" :min="1" :max="1000000" theme="column"
            class="form-number" />
        </div>
      </section>

      <section class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ $t('guestLinkPublish.sectionCapabilities') }}</h4>

        <div class="settings-group">
          <div class="setting-row">
            <div class="setting-info">
              <label>{{ $t('guestLinkPublish.showSuggestedQuestions') }}</label>
            </div>
            <div class="setting-control">
              <t-switch v-model="form.show_suggested_questions" size="small" />
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-info">
              <label>{{ $t('guestLinkPublish.allowWebSearch') }}</label>
            </div>
            <div class="setting-control">
              <t-switch v-model="form.allow_web_search" size="small" />
            </div>
          </div>

          <div class="setting-row setting-row--last">
            <div class="setting-info">
              <label>{{ $t('guestLinkPublish.allowFileUpload') }}</label>
            </div>
            <div class="setting-control">
              <t-switch v-model="form.allow_file_upload" size="small" />
            </div>
          </div>
        </div>
      </section>

      <section class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ $t('guestLinkPublish.sectionAppearance') }}</h4>

        <div class="form-item">
          <label class="form-label">{{ $t('guestLinkPublish.pageTitle') }}</label>
          <t-input v-model="form.page_title" :placeholder="$t('guestLinkPublish.pageTitlePlaceholder')" />
        </div>

        <div class="form-item">
          <label class="form-label">{{ $t('guestLinkPublish.defaultLocale') }}</label>
          <t-select v-model="form.default_locale" :options="defaultLocaleOptions" />
        </div>

        <div class="form-item">
          <label class="form-label">{{ $t('guestLinkPublish.primaryColor') }}</label>
          <t-color-picker v-model="form.primary_color" format="HEX" :color-modes="['monochrome']" />
        </div>
      </section>
    </SettingDrawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import SettingDrawer from '@/components/settings/SettingDrawer.vue'
import {
  getAgentGuestLink,
  createAgentGuestLink,
  updateGuestLink,
  deleteGuestLink,
  resolveGuestLinkURL,
  type GuestLinkChannel,
} from '@/api/guest-link'
import type { EmbedLocaleTag } from '@/api/embed'

const props = defineProps<{
  agentId: string
  agentName?: string
  canManage?: boolean
}>()

const emit = defineEmits<{
  changed: []
}>()

const { t } = useI18n()

const canManage = computed(() => props.canManage !== false)

const loading = ref(false)
const creating = ref(false)
const saving = ref(false)
const deleting = ref(false)
const showDrawer = ref(false)
const createVisible = ref(false)
const createFormKey = ref(0)
const createFormRef = ref<HTMLFormElement | null>(null)
const guestLink = ref<GuestLinkChannel | null>(null)

const defaultCreateTitle = computed(() => props.agentName?.trim() || '')

const WEKNORA_BRAND_COLOR = '#07C05F'

function getDefaultPrimaryColor(): string {
  if (typeof window === 'undefined') return WEKNORA_BRAND_COLOR
  const css = getComputedStyle(document.documentElement).getPropertyValue('--td-brand-color').trim()
  return css || WEKNORA_BRAND_COLOR
}

const defaultForm = () => ({
  enabled: true,
  name: '',
  welcome_message: '',
  rate_limit_per_minute: 30,
  rate_limit_per_day: 10000,
  show_suggested_questions: true,
  allow_web_search: false,
  allow_file_upload: false,
  page_title: '',
  default_locale: '' as EmbedLocaleTag,
  primary_color: getDefaultPrimaryColor(),
})

const form = reactive(defaultForm())
const rateLimitUnlimited = ref(false)
const lastLimitedMinute = ref(30)
const lastLimitedDay = ref(10000)

function onRateLimitUnlimitedChange(unlimited: boolean): void {
  if (unlimited) {
    if (form.rate_limit_per_minute > 0) {
      lastLimitedMinute.value = form.rate_limit_per_minute
    }
    if (form.rate_limit_per_day > 0) {
      lastLimitedDay.value = form.rate_limit_per_day
    }
    return
  }
  form.rate_limit_per_minute = lastLimitedMinute.value || 30
  form.rate_limit_per_day = lastLimitedDay.value || 10000
}

const defaultLocaleOptions = computed(() => ([
  { label: t('embedPublish.defaultLocaleBrowser'), value: '' },
  { label: '简体中文', value: 'zh-CN' },
  { label: 'English', value: 'en-US' },
  { label: '한국어', value: 'ko-KR' },
  { label: 'Русский', value: 'ru-RU' },
]))

const displayName = computed(() => guestLink.value?.name?.trim() || t('guestLinkPublish.defaultName'))

const guestLinkUrl = computed(() => (guestLink.value ? resolveGuestLinkURL(guestLink.value) : ''))

async function load(): Promise<void> {
  if (!props.agentId) {
    guestLink.value = null
    return
  }
  loading.value = true
  try {
    const res = await getAgentGuestLink(props.agentId)
    guestLink.value = res?.data ?? null
  } catch {
    guestLink.value = null
  } finally {
    loading.value = false
    emit('changed')
  }
}

onMounted(() => {
  void load()
})

watch(() => props.agentId, () => {
  void load()
})

defineExpose({
  reload: load,
})

function openCreateDialog(): void {
  createFormKey.value += 1
  createVisible.value = true
}

function readCreateTitle(): string {
  const formElement = createFormRef.value
  if (!formElement) return defaultCreateTitle.value
  const data = new FormData(formElement)
  const raw = String(data.get('title') ?? '').trim()
  return raw || defaultCreateTitle.value
}

async function submitCreate(): Promise<boolean> {
  if (!props.agentId || creating.value) return false
  const title = readCreateTitle()
  creating.value = true
  try {
    const res = await createAgentGuestLink(props.agentId, {
      name: title,
      page_title: title,
    })
    guestLink.value = res?.data ?? null
    createVisible.value = false
    MessagePlugin.success(t('guestLinkPublish.created'))
    emit('changed')
    return true
  } catch (err: any) {
    if (err?.status === 409 || err?.message === 'guest_link_exists') {
      MessagePlugin.warning(t('guestLinkPublish.alreadyExists'))
      createVisible.value = false
      await load()
    } else {
      MessagePlugin.error(err?.message || t('guestLinkPublish.createFailed'))
    }
    return false
  } finally {
    creating.value = false
  }
}

async function copyUrl(): Promise<void> {
  if (!guestLinkUrl.value) return
  try {
    await navigator.clipboard.writeText(guestLinkUrl.value)
    MessagePlugin.success(t('common.copied'))
  } catch {
    MessagePlugin.error(t('common.copyFailed'))
  }
}

function openUrl(): void {
  if (!guestLinkUrl.value) return
  window.open(guestLinkUrl.value, '_blank', 'noopener,noreferrer')
}

function openSettings(): void {
  if (!guestLink.value) return
  const gl = guestLink.value
  form.enabled = gl.enabled
  form.name = gl.name || ''
  form.welcome_message = gl.welcome_message || ''
  const unlimited =
    gl.rate_limit_per_minute === 0 && gl.rate_limit_per_day === 0
  rateLimitUnlimited.value = unlimited
  if (unlimited) {
    form.rate_limit_per_minute = lastLimitedMinute.value || 30
    form.rate_limit_per_day = lastLimitedDay.value || 10000
  } else {
    form.rate_limit_per_minute =
      gl.rate_limit_per_minute > 0 ? gl.rate_limit_per_minute : 30
    form.rate_limit_per_day =
      gl.rate_limit_per_day > 0 ? gl.rate_limit_per_day : 10000
    lastLimitedMinute.value = form.rate_limit_per_minute
    lastLimitedDay.value = form.rate_limit_per_day
  }
  form.show_suggested_questions = gl.show_suggested_questions !== false
  form.allow_web_search = gl.allow_web_search === true
  form.allow_file_upload = gl.allow_file_upload === true
  form.page_title = gl.page_title || ''
  form.default_locale = (gl.default_locale || '') as EmbedLocaleTag
  form.primary_color = gl.primary_color || getDefaultPrimaryColor()
  showDrawer.value = true
}

function closeDrawer(): void {
  showDrawer.value = false
}

async function saveForm(): Promise<void> {
  if (!canManage.value || !guestLink.value) return
  saving.value = true
  try {
    const rateLimitPerMinute = rateLimitUnlimited.value
      ? 0
      : form.rate_limit_per_minute
    const rateLimitPerDay = rateLimitUnlimited.value
      ? 0
      : form.rate_limit_per_day
    const res = await updateGuestLink(guestLink.value.id, {
      name: form.name,
      enabled: form.enabled,
      welcome_message: form.welcome_message,
      rate_limit_per_minute: rateLimitPerMinute,
      rate_limit_per_day: rateLimitPerDay,
      show_suggested_questions: form.show_suggested_questions,
      allow_web_search: form.allow_web_search,
      allow_file_upload: form.allow_file_upload,
      page_title: form.page_title.trim() || form.name.trim(),
      default_locale: form.default_locale || '',
      primary_color: form.primary_color,
    })
    guestLink.value = res?.data ?? guestLink.value
    MessagePlugin.success(t('guestLinkPublish.updated'))
    showDrawer.value = false
    emit('changed')
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('guestLinkPublish.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function handleDelete(): Promise<void> {
  if (!guestLink.value) return
  deleting.value = true
  try {
    await deleteGuestLink(guestLink.value.id)
    guestLink.value = null
    showDrawer.value = false
    MessagePlugin.success(t('guestLinkPublish.deleted'))
    emit('changed')
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('guestLinkPublish.deleteFailed'))
  } finally {
    deleting.value = false
  }
}
</script>

<style scoped lang="less">
@import './css/channel-panel-list.less';

.guest-link-panel {
  display: flex;
  flex-direction: column;
}

.channel-card--guest {
  cursor: default;
}

.form-item {
  margin-bottom: 0;
}

.form-label {
  display: block;
  margin-bottom: 6px;
  font-size: 13px;
  font-weight: 500;
  color: var(--td-text-color-primary);
  line-height: 1.4;
}

.form-number {
  width: 100%;
  max-width: 200px;
}

.form-desc {
  margin: 6px 0 0;
  font-size: 12px;
  line-height: 1.5;
  color: var(--td-text-color-placeholder);
}

.guest-link-create-form {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.guest-link-create-input {
  width: 100%;
  height: 32px;
  padding: 0 12px;
  border: 1px solid var(--td-component-border);
  border-radius: var(--td-radius-default);
  background: var(--td-bg-color-container);
  color: var(--td-text-color-primary);
  font-size: 14px;
  outline: none;
  box-sizing: border-box;
}

.guest-link-create-input:focus {
  border-color: var(--td-brand-color);
  box-shadow: 0 0 0 2px var(--td-brand-color-focus);
}

.settings-group {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.setting-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px solid var(--td-component-stroke);

  &--last {
    border-bottom: none;
    padding-bottom: 0;
  }
}

.setting-info {
  flex: 1;
  min-width: 0;

  label {
    display: block;
    margin: 0;
    font-size: 13px;
    font-weight: 500;
    color: var(--td-text-color-primary);
    line-height: 1.4;
  }
}

.setting-desc {
  margin: 4px 0 0;
  font-size: 12px;
  line-height: 1.5;
  color: var(--td-text-color-placeholder);
}

.setting-control {
  flex-shrink: 0;
  padding-top: 2px;
}
</style>
