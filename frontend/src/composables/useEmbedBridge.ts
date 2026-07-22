import { computed, onMounted, onUnmounted, ref, type Ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  bootstrapWebLink,
  createEmbedSession,
  exchangeEmbedSession,
  getEmbedConfig,
  getEmbedMessageList,
  getOrCreateEmbedVisitorId,
  isEmbedSessionToken,
  onEmbedHostContext,
  onEmbedHostLocale,
  onEmbedHostToken,
  parseEmbedTokenFromLocation,
  postEmbedBootstrapRequest,
  postEmbedReady,
  readEmbedStoredSessionState,
  upsertEmbedStoredSession,
  writeEmbedStoredSessionState,
  type EmbedChannelPublicConfig,
  type EmbedStoredSessionEntry,
  type EmbedStoredSessionState,
} from '@/api/embed'
import { applyEmbedLocale, readEmbedLocaleFromUrl, syncEmbedLocaleFromUrl } from '@/i18n/embed'

async function isStoredSessionValid(
  channelId: string,
  apiToken: string,
  session: EmbedStoredSessionEntry,
): Promise<boolean> {
  try {
    await getEmbedMessageList(channelId, apiToken, session.id, 1, undefined, session.sig)
    return true
  } catch {
    return false
  }
}

function sortSessions(
  sessions: EmbedStoredSessionEntry[],
): EmbedStoredSessionEntry[] {
  return [...sessions].sort((left, right) => right.updatedAt - left.updatedAt)
}

export function useEmbedBridge(
  channelId: Ref<string>,
  opts?: { webSlug?: Ref<string> },
) {
  const { locale: activeLocale, t } = useI18n()
  const route = useRoute()
  const webSlug = opts?.webSlug

  const token = ref('')
  const config = ref<EmbedChannelPublicConfig | null>(null)
  const sessionId = ref('')
  const sessionSig = ref('')
  const visitorId = ref('')
  const loadError = ref('')
  const awaitingToken = ref(false)
  const bootstrapping = ref(false)
  const hostContext = ref<Record<string, unknown>>({})
  const sessionHistory = ref<EmbedStoredSessionEntry[]>([])

  let removeHostListener: (() => void) | null = null
  let removeLocaleListener: (() => void) | null = null
  let removeTokenListener: (() => void) | null = null
  let bootstrapped = false
  let hostLocalePinned = false
  if (typeof window !== 'undefined') {
    hostLocalePinned = Boolean(readEmbedLocaleFromUrl())
    if (hostLocalePinned) {
      syncEmbedLocaleFromUrl(activeLocale)
    }
  }

  const syncHistoryFromStorage = (channel: string) => {
    const state = readEmbedStoredSessionState(channel)
    sessionHistory.value = state ? sortSessions(state.sessions) : []
  }

  const applyActiveSession = (
    channel: string,
    entry: EmbedStoredSessionEntry,
    opts?: { touch?: boolean },
  ) => {
    const next: EmbedStoredSessionEntry = {
      ...entry,
      updatedAt: opts?.touch === false ? entry.updatedAt : Date.now(),
    }
    const state = upsertEmbedStoredSession(channel, next, { makeCurrent: true })
    sessionId.value = next.id
    sessionSig.value = next.sig
    sessionHistory.value = sortSessions(state.sessions)
  }

  const persistState = (channel: string, state: EmbedStoredSessionState | null) => {
    writeEmbedStoredSessionState(channel, state)
    sessionHistory.value = state ? sortSessions(state.sessions) : []
  }

  const finishBootstrapWithSession = async (
    id: string,
    apiToken: string,
    publicConfig: EmbedChannelPublicConfig,
  ) => {
    config.value = publicConfig
    if (publicConfig.default_locale && !hostLocalePinned) {
      applyEmbedLocale(publicConfig.default_locale, activeLocale)
    }

    const configAgentId = String(publicConfig.agent_id || '').trim()
    const storedState = readEmbedStoredSessionState(id)
    let resolved: EmbedStoredSessionEntry | null = null

    if (storedState) {
      const candidates = sortSessions(storedState.sessions).filter((entry) => {
        if (!entry.agentId || !configAgentId) return true
        return entry.agentId === configAgentId
      })
      const preferred = candidates.find((entry) => entry.id === storedState.currentId)
        || candidates[0]
      if (preferred && (await isStoredSessionValid(id, apiToken, preferred))) {
        resolved = {
          ...preferred,
          agentId: configAgentId || preferred.agentId,
        }
        // Keep sibling chats locally; validate lazily when the user switches.
        const siblings = candidates
          .filter((entry) => entry.id !== preferred.id)
          .map((entry) => ({
            ...entry,
            agentId: configAgentId || entry.agentId,
          }))
        persistState(id, {
          currentId: resolved.id,
          sessions: [resolved, ...siblings],
        })
      } else if (candidates.length > 0) {
        // Preferred session is gone — drop only that entry and try next.
        const rest = candidates.filter((entry) => entry.id !== preferred?.id)
        for (const entry of rest) {
          if (await isStoredSessionValid(id, apiToken, entry)) {
            resolved = {
              ...entry,
              agentId: configAgentId || entry.agentId,
            }
            persistState(id, {
              currentId: resolved.id,
              sessions: [
                resolved,
                ...rest
                  .filter((item) => item.id !== entry.id)
                  .map((item) => ({
                    ...item,
                    agentId: configAgentId || item.agentId,
                  })),
              ],
            })
            break
          }
        }
        if (!resolved) {
          persistState(id, null)
        }
      }
    }

    if (!resolved) {
      const sessionRes = await createEmbedSession(id, apiToken)
      const newId = sessionRes?.data?.id || ''
      if (newId) {
        resolved = {
          id: newId,
          sig: sessionRes?.data?.sig || '',
          agentId: configAgentId,
          updatedAt: Date.now(),
        }
      }
    }

    if (!resolved) {
      loadError.value = t('embedPublish.sessionFailed')
      return
    }

    applyActiveSession(id, resolved, { touch: false })
    token.value = apiToken
    postEmbedReady(id)
  }

  const bootstrapFromWebSlug = async (slug: string) => {
    if (!slug || bootstrapped) return
    bootstrapped = true
    awaitingToken.value = false
    bootstrapping.value = true
    try {
      const res = await bootstrapWebLink(slug)
      const payload = res?.data
      if (!payload?.channel_id || !payload?.session_token || !payload?.config) {
        loadError.value = t('embedPublish.invalidChannel')
        return
      }
      channelId.value = payload.channel_id
      visitorId.value = getOrCreateEmbedVisitorId(payload.channel_id)
      await finishBootstrapWithSession(
        payload.channel_id,
        payload.session_token,
        payload.config,
      )
    } catch (error: unknown) {
      bootstrapped = false
      const msg = String((error as { message?: string })?.message || '')
      if (msg.includes('disabled')) {
        loadError.value = t('embedPublish.channelDisabled')
      } else if (msg.includes('not found')) {
        loadError.value = t('embedPublish.invalidChannel')
      } else {
        loadError.value = msg || t('embedPublish.loadError')
      }
    } finally {
      bootstrapping.value = false
    }
  }

  const bootstrap = async (embedToken: string) => {
    const id = channelId.value
    if (!id || !embedToken || bootstrapped) return
    bootstrapped = true
    awaitingToken.value = false
    bootstrapping.value = true
    token.value = embedToken
    visitorId.value = getOrCreateEmbedVisitorId(id)

    try {
      let apiToken = embedToken
      if (!isEmbedSessionToken(embedToken)) {
        try {
          const exchangeRes = await exchangeEmbedSession(id, embedToken)
          if (exchangeRes?.data?.session_token) {
            apiToken = exchangeRes.data.session_token
          } else if (!import.meta.env.DEV) {
            throw new Error('embed session exchange returned no token')
          }
        } catch (exchangeErr) {
          if (!import.meta.env.DEV) {
            throw exchangeErr
          }
        }
      }

      const res = await getEmbedConfig(id, apiToken)
      if (!res?.success || !res.data) {
        loadError.value = t('embedPublish.invalidChannel')
        return
      }
      await finishBootstrapWithSession(id, apiToken, res.data)
    } catch (e: unknown) {
      bootstrapped = false
      const msg = String((e as { message?: string })?.message || '')
      if (msg.includes('disabled')) {
        loadError.value = t('embedPublish.channelDisabled')
      } else if (msg.includes('failed to create session')) {
        loadError.value = t('embedPublish.sessionFailed')
      } else {
        loadError.value = msg || t('embedPublish.loadError')
      }
    } finally {
      bootstrapping.value = false
    }
  }

  const startNewSession = async () => {
    const id = channelId.value
    const apiToken = token.value
    if (!id || !apiToken) return
    try {
      const sessionRes = await createEmbedSession(id, apiToken)
      const newId = sessionRes?.data?.id || ''
      if (!newId) return
      const agentId = String(config.value?.agent_id || '').trim()
      applyActiveSession(id, {
        id: newId,
        sig: sessionRes?.data?.sig || '',
        agentId,
        updatedAt: Date.now(),
      })
    } catch {
      // Non-fatal: keep the current session if creating a new one fails.
    }
  }

  const updateSessionTitle = (
    targetId: string,
    title: string,
    opts?: { onlyIfEmpty?: boolean },
  ) => {
    const id = channelId.value
    const trimmed = title.trim()
    if (!id || !targetId || !trimmed) return
    const entry = sessionHistory.value.find((item) => item.id === targetId)
    if (!entry) return
    if (opts?.onlyIfEmpty && entry.title?.trim()) return
    const next = upsertEmbedStoredSession(
      id,
      { ...entry, title: trimmed, updatedAt: Date.now() },
      { makeCurrent: targetId === sessionId.value },
    )
    sessionHistory.value = sortSessions(next.sessions)
  }

  const removeSession = async (targetId: string) => {
    const id = channelId.value
    if (!id || !targetId) return
    const remaining = sessionHistory.value.filter((item) => item.id !== targetId)
    if (remaining.length === 0) {
      persistState(id, null)
      await startNewSession()
      return
    }
    const nextCurrent = targetId === sessionId.value
      ? remaining[0]
      : remaining.find((item) => item.id === sessionId.value) || remaining[0]
    persistState(id, {
      currentId: nextCurrent.id,
      sessions: remaining,
    })
    if (targetId === sessionId.value) {
      applyActiveSession(id, nextCurrent, { touch: false })
    }
  }

  const switchSession = async (targetId: string) => {
    const id = channelId.value
    const apiToken = token.value
    if (!id || !apiToken || !targetId || targetId === sessionId.value) return
    const entry = sessionHistory.value.find((item) => item.id === targetId)
    if (!entry) return
    if (!(await isStoredSessionValid(id, apiToken, entry))) {
      await removeSession(targetId)
      return
    }
    applyActiveSession(id, entry)
  }

  const start = async () => {
    removeHostListener = onEmbedHostContext((payload) => {
      hostContext.value = { ...hostContext.value, ...payload }
    })

    removeLocaleListener = onEmbedHostLocale((locale) => {
      hostLocalePinned = true
      applyEmbedLocale(locale, activeLocale)
    })

    removeTokenListener = onEmbedHostToken((providedToken, providedChannelId) => {
      if (providedChannelId && providedChannelId !== channelId.value) return
      bootstrap(providedToken)
    })

    const slug = String(webSlug?.value || '').trim()
    if (slug) {
      await bootstrapFromWebSlug(slug)
      return
    }

    if (!channelId.value) {
      loadError.value = t('embedPublish.missingChannel')
      return
    }

    syncHistoryFromStorage(channelId.value)

    const initialToken = String(route.query.token || '') || parseEmbedTokenFromLocation()
    if (initialToken) {
      await bootstrap(initialToken)
      return
    }

    if (window.parent !== window) {
      awaitingToken.value = true
      postEmbedBootstrapRequest(channelId.value)
      return
    }

    loadError.value = t('embedPublish.missingChannel')
  }

  onMounted(() => {
    start()
  })

  onUnmounted(() => {
    removeHostListener?.()
    removeLocaleListener?.()
    removeTokenListener?.()
  })

  const touchCurrentSession = () => {
    const id = channelId.value
    const entry = sessionHistory.value.find((item) => item.id === sessionId.value)
    if (!id || !entry) return
    applyActiveSession(id, entry)
  }

  const sessions = computed(() => sessionHistory.value)

  return {
    token,
    config,
    sessionId,
    sessionSig,
    visitorId,
    loadError,
    awaitingToken,
    bootstrapping,
    hostContext,
    sessions,
    startNewSession,
    switchSession,
    updateSessionTitle,
    removeSession,
    touchCurrentSession,
  }
}
