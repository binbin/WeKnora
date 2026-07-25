<template>
  <div class="guest-link-panel">
    <t-loading :loading="loading" size="small" class="channels-loading-wrap">
      <div v-if="!loading && !guestLink" class="channels-empty">
        <t-empty :description="$t('guestLinkPublish.empty')">
          <template v-if="canManage" #action>
            <t-button theme="primary" :loading="creating" @click="handleCreate">
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
          <label class="form-label">{{ $t('guestLinkPublish.name') }}</label>
          <t-input v-model="form.name" :placeholder="$t('guestLinkPublish.namePlaceholder')" />
        </div>

        <div class="form-item">
          <label class="form-label">{{ $t('guestLinkPublish.welcomeMessage') }}</label>
          <t-textarea v-model="form.welcome_message" :placeholder="$t('guestLinkPublish.welcomePlaceholder')"
            :autosize="{ minRows: 2, maxRows: 4 }" />
        </div>
      </section>

      <section class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ $t('guestLinkPublish.sectionLimits') }}</h4>

        <div class="form-item">
          <label class="form-label">{{ $t('guestLinkPublish.rateLimitLabel') }}</label>
          <t-input-number v-model="form.rate_limit_per_minute" :min="1" :max="600" theme="column"
            class="form-number" />
        </div>

        <div class="form-item">
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
const guestLink = ref<GuestLinkChannel | null>(null)

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

async function handleCreate(): Promise<void> {
  if (!props.agentId || creating.value) return
  creating.value = true
  try {
    const res = await createAgentGuestLink(props.agentId, {})
    guestLink.value = res?.data ?? null
    MessagePlugin.success(t('guestLinkPublish.created'))
    emit('changed')
  } catch (err: any) {
    if (err?.status === 409 || err?.message === 'guest_link_exists') {
      MessagePlugin.warning(t('guestLinkPublish.alreadyExists'))
      await load()
    } else {
      MessagePlugin.error(err?.message || t('guestLinkPublish.createFailed'))
    }
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
  form.rate_limit_per_minute = gl.rate_limit_per_minute || 30
  form.rate_limit_per_day = gl.rate_limit_per_day || 10000
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
    const res = await updateGuestLink(guestLink.value.id, {
      name: form.name,
      enabled: form.enabled,
      welcome_message: form.welcome_message,
      rate_limit_per_minute: form.rate_limit_per_minute,
      rate_limit_per_day: form.rate_limit_per_day,
      show_suggested_questions: form.show_suggested_questions,
      allow_web_search: form.allow_web_search,
      allow_file_upload: form.allow_file_upload,
      page_title: form.page_title,
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

.setting-control {
  flex-shrink: 0;
  padding-top: 2px;
}
</style>
