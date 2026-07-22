<template>
  <aside class="embed-session-sidebar" :class="{ 'is-open': open }">
    <div class="embed-session-sidebar__head">
      <h2 class="embed-session-sidebar__title">{{ $t('embedPublish.chatHistory') }}</h2>
      <t-button
        theme="primary"
        size="small"
        class="embed-session-sidebar__new"
        :disabled="!canCreate"
        @click="$emit('new-chat')"
      >
        <template #icon><t-icon name="add" /></template>
        {{ $t('embedPublish.newChat') }}
      </t-button>
    </div>

    <div class="embed-session-sidebar__list" role="list">
      <button
        v-for="session in sessions"
        :key="session.id"
        type="button"
        role="listitem"
        class="embed-session-item"
        :class="{ 'is-active': session.id === currentId }"
        @click="$emit('select', session.id)"
      >
        <span class="embed-session-item__title">
          {{ sessionTitle(session) }}
        </span>
        <span class="embed-session-item__meta">{{ formatTime(session.updatedAt) }}</span>
        <t-button
          class="embed-session-item__delete"
          variant="text"
          shape="square"
          size="small"
          :title="$t('embedPublish.deleteChat')"
          :aria-label="$t('embedPublish.deleteChat')"
          @click.stop="$emit('delete', session.id)"
        >
          <template #icon><t-icon name="delete" size="14px" /></template>
        </t-button>
      </button>

      <p v-if="sessions.length === 0" class="embed-session-sidebar__empty">
        {{ $t('embedPublish.chatHistoryEmpty') }}
      </p>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { EmbedStoredSessionEntry } from '@/api/embed'

defineProps<{
  sessions: EmbedStoredSessionEntry[]
  currentId: string
  open: boolean
  canCreate: boolean
}>()

defineEmits<{
  (event: 'new-chat'): void
  (event: 'select', sessionId: string): void
  (event: 'delete', sessionId: string): void
}>()

const { t, locale } = useI18n()

function sessionTitle(session: EmbedStoredSessionEntry): string {
  const title = session.title?.trim()
  return title || t('embedPublish.untitledChat')
}

function formatTime(timestamp: number): string {
  if (!timestamp) return ''
  try {
    return new Intl.DateTimeFormat(locale.value || undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(timestamp))
  } catch {
    return ''
  }
}
</script>

<style scoped lang="less">
.embed-session-sidebar {
  display: flex;
  flex-direction: column;
  width: 260px;
  flex-shrink: 0;
  border-right: 1px solid var(--td-component-stroke);
  background: var(--td-bg-color-container);
  min-height: 0;

  &__head {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 14px 12px 12px;
    border-bottom: 1px solid var(--td-component-stroke);
  }

  &__title {
    margin: 0;
    font-size: 13px;
    font-weight: 600;
    color: var(--td-text-color-secondary);
  }

  &__new {
    width: 100%;
  }

  &__list {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: 8px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  &__empty {
    margin: 24px 8px;
    text-align: center;
    font-size: 13px;
    color: var(--td-text-color-placeholder);
  }
}

.embed-session-item {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  width: 100%;
  padding: 10px 32px 10px 10px;
  border: none;
  border-radius: 8px;
  background: transparent;
  text-align: left;
  cursor: pointer;
  color: var(--td-text-color-primary);

  &:hover {
    background: var(--td-bg-color-container-hover, var(--td-bg-color-secondarycontainer));
  }

  &.is-active {
    background: color-mix(in srgb, var(--td-brand-color) 10%, transparent);
  }

  &__title {
    width: 100%;
    font-size: 13px;
    font-weight: 500;
    line-height: 1.4;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  &__meta {
    font-size: 11px;
    color: var(--td-text-color-placeholder);
  }

  &__delete {
    position: absolute;
    top: 6px;
    right: 4px;
    opacity: 0;
    color: var(--td-text-color-placeholder);
  }

  &:hover &__delete,
  &:focus-within &__delete {
    opacity: 1;
  }
}

@media (max-width: 768px) {
  .embed-session-sidebar {
    position: absolute;
    inset: 0 auto 0 0;
    z-index: 20;
    height: 100%;
    box-shadow: 4px 0 24px rgba(0, 0, 0, 0.08);
    transform: translateX(-105%);
    transition: transform 0.2s ease;

    &.is-open {
      transform: translateX(0);
    }
  }

  .embed-session-item__delete {
    opacity: 1;
  }
}
</style>
