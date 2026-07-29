<template>
  <t-dialog
    v-model:visible="visibleModel"
    :header="$t('auth.changePassword.title')"
    width="440px"
    :confirm-btn="{
      content: $t('auth.changePassword.confirmBtn'),
      loading: submitting,
    }"
    :cancel-btn="{
      content: $t('common.cancel'),
      disabled: submitting,
    }"
    :close-on-overlay-click="!submitting"
    :close-btn="!submitting"
    @confirm="onConfirm"
    @close="resetForm"
  >
    <t-alert
      theme="info"
      :message="$t('auth.changePassword.reloginHint')"
      class="change-password-hint"
    />
    <t-form
      ref="formRef"
      :data="form"
      :rules="rules"
      label-align="top"
      @submit.prevent
    >
      <t-form-item
        :label="$t('auth.changePassword.oldPasswordLabel')"
        name="oldPassword"
      >
        <t-input
          v-model="form.oldPassword"
          type="password"
          autocomplete="current-password"
          :disabled="submitting"
          :placeholder="$t('auth.changePassword.oldPasswordPlaceholder')"
        />
      </t-form-item>
      <t-form-item
        :label="$t('auth.changePassword.newPasswordLabel')"
        name="newPassword"
      >
        <t-input
          v-model="form.newPassword"
          type="password"
          autocomplete="new-password"
          :disabled="submitting"
          :placeholder="$t('auth.passwordPlaceholder')"
        />
      </t-form-item>
      <t-form-item
        :label="$t('auth.changePassword.confirmPasswordLabel')"
        name="confirmPassword"
      >
        <t-input
          v-model="form.confirmPassword"
          type="password"
          autocomplete="new-password"
          :disabled="submitting"
          :placeholder="$t('auth.confirmPasswordPlaceholder')"
        />
      </t-form-item>
    </t-form>
  </t-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  MessagePlugin,
  type FormInstanceFunctions,
  type FormRule,
} from 'tdesign-vue-next'
import { changePassword } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{ (e: 'update:visible', value: boolean): void }>()

const visibleModel = computed({
  get: () => props.visible,
  set: (value: boolean) => emit('update:visible', value),
})

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()

const formRef = ref<FormInstanceFunctions>()
const submitting = ref(false)
const form = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
})

const rules: Record<string, FormRule[]> = {
  oldPassword: [
    {
      required: true,
      message: t('auth.changePassword.oldPasswordRequired'),
      trigger: 'blur',
    },
  ],
  newPassword: [
    {
      required: true,
      message: t('auth.passwordRequired'),
      trigger: 'blur',
    },
    {
      min: 8,
      message: t('auth.passwordMinLength'),
      trigger: 'blur',
    },
    {
      max: 32,
      message: t('auth.passwordMaxLength'),
      trigger: 'blur',
    },
    {
      pattern: /[a-zA-Z]/,
      message: t('auth.passwordMustContainLetter'),
      trigger: 'blur',
    },
    {
      pattern: /\d/,
      message: t('auth.passwordMustContainNumber'),
      trigger: 'blur',
    },
  ],
  confirmPassword: [
    {
      required: true,
      message: t('auth.confirmPasswordRequired'),
      trigger: 'blur',
    },
    {
      validator: (value: string) => value === form.newPassword,
      message: t('auth.passwordMismatch'),
      trigger: 'blur',
    },
  ],
}

function resetForm(): void {
  form.oldPassword = ''
  form.newPassword = ''
  form.confirmPassword = ''
  formRef.value?.clearValidate?.()
}

watch(
  () => props.visible,
  (isVisible) => {
    if (isVisible) {
      resetForm()
    }
  },
)

async function onConfirm(): Promise<boolean> {
  if (submitting.value) return false
  const valid = await formRef.value?.validate?.()
  if (valid !== true) return false

  submitting.value = true
  try {
    const result = await changePassword({
      old_password: form.oldPassword,
      new_password: form.newPassword,
    })
    if (!result.success) {
      MessagePlugin.error(result.message || t('auth.changePassword.failed'))
      return false
    }

    visibleModel.value = false
    // 服务端会吊销全部 token，本地会话一并清掉并回到登录页。
    authStore.logout()
    MessagePlugin.success(t('auth.changePassword.success'))
    router.push('/login')
    return true
  } finally {
    submitting.value = false
  }
}
</script>

<style lang="less" scoped>
.change-password-hint {
  margin-bottom: 16px;
}
</style>
