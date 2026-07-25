/**
 * Pure localStorage helpers for embed chat session history.
 *
 * Deliberately dependency-free (only the `localStorage` global) so this
 * module — and the `multi` vs `single_fresh` persistence contract it
 * implements — can be unit-tested directly, without pulling in axios/i18n
 * through the rest of `@/api/embed`.
 */

/** localStorage key prefix for persisted embed chat sessions (per channel). */
export const EMBED_CHAT_SESSION_STORAGE_PREFIX = 'weknora-embed-session:'

export function embedChatSessionStorageKey(channelId: string): string {
  return `${EMBED_CHAT_SESSION_STORAGE_PREFIX}${channelId}`
}

/** One persisted embed chat conversation (needs sig to reload messages). */
export interface EmbedStoredSessionEntry {
  id: string
  sig: string
  agentId?: string
  title?: string
  updatedAt: number
}

/** Multi-conversation state persisted per channel in localStorage. */
export interface EmbedStoredSessionState {
  currentId: string
  sessions: EmbedStoredSessionEntry[]
}

const EMBED_SESSION_HISTORY_LIMIT = 40

function isSessionEntry(value: unknown): value is EmbedStoredSessionEntry {
  if (!value || typeof value !== 'object') return false
  const entry = value as EmbedStoredSessionEntry
  return typeof entry.id === 'string'
    && entry.id.length > 0
    && typeof entry.sig === 'string'
}

/** Parse legacy single-session or multi-session localStorage payloads. */
export function parseEmbedStoredSessionState(
  raw: string | null,
): EmbedStoredSessionState | null {
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as unknown
    if (!parsed || typeof parsed !== 'object') return null

    // Legacy: { id, sig, agentId? }
    if (isSessionEntry(parsed) && !('sessions' in (parsed as object))) {
      const legacy = parsed as EmbedStoredSessionEntry
      const agentId = typeof legacy.agentId === 'string' ? legacy.agentId.trim() : ''
      const entry: EmbedStoredSessionEntry = {
        id: legacy.id,
        sig: legacy.sig,
        updatedAt: Date.now(),
        ...(agentId ? { agentId } : {}),
      }
      return { currentId: entry.id, sessions: [entry] }
    }

    const state = parsed as EmbedStoredSessionState
    if (!Array.isArray(state.sessions)) return null
    const sessions = state.sessions
      .filter(isSessionEntry)
      .map((entry) => {
        const agentId = typeof entry.agentId === 'string' ? entry.agentId.trim() : ''
        const title = typeof entry.title === 'string' ? entry.title.trim() : ''
        const updatedAt = typeof entry.updatedAt === 'number' && entry.updatedAt > 0
          ? entry.updatedAt
          : Date.now()
        return {
          id: entry.id,
          sig: entry.sig,
          updatedAt,
          ...(agentId ? { agentId } : {}),
          ...(title ? { title } : {}),
        } satisfies EmbedStoredSessionEntry
      })
    if (sessions.length === 0) return null
    const currentId = typeof state.currentId === 'string' && sessions.some((item) => item.id === state.currentId)
      ? state.currentId
      : sessions[0].id
    return { currentId, sessions }
  } catch {
    return null
  }
}

export function readEmbedStoredSessionState(
  channelId: string,
): EmbedStoredSessionState | null {
  if (typeof localStorage === 'undefined' || !channelId) return null
  try {
    return parseEmbedStoredSessionState(
      localStorage.getItem(embedChatSessionStorageKey(channelId)),
    )
  } catch {
    return null
  }
}

export function writeEmbedStoredSessionState(
  channelId: string,
  state: EmbedStoredSessionState | null,
): void {
  if (typeof localStorage === 'undefined' || !channelId) return
  try {
    const key = embedChatSessionStorageKey(channelId)
    if (!state || state.sessions.length === 0) {
      localStorage.removeItem(key)
      return
    }
    const sessions = [...state.sessions]
      .sort((left, right) => right.updatedAt - left.updatedAt)
      .slice(0, EMBED_SESSION_HISTORY_LIMIT)
    const currentId = sessions.some((item) => item.id === state.currentId)
      ? state.currentId
      : sessions[0].id
    localStorage.setItem(key, JSON.stringify({ currentId, sessions }))
  } catch {
    // localStorage may be unavailable (private mode).
  }
}

/** Upsert one session and mark it current. */
export function upsertEmbedStoredSession(
  channelId: string,
  entry: EmbedStoredSessionEntry,
  opts?: { makeCurrent?: boolean },
): EmbedStoredSessionState {
  const makeCurrent = opts?.makeCurrent !== false
  const existing = readEmbedStoredSessionState(channelId)
  const sessions = existing?.sessions ? [...existing.sessions] : []
  const index = sessions.findIndex((item) => item.id === entry.id)
  if (index >= 0) {
    sessions[index] = {
      ...sessions[index],
      ...entry,
      updatedAt: entry.updatedAt || Date.now(),
    }
  } else {
    sessions.unshift({ ...entry, updatedAt: entry.updatedAt || Date.now() })
  }
  const next: EmbedStoredSessionState = {
    currentId: makeCurrent
      ? entry.id
      : (existing?.currentId && sessions.some((item) => item.id === existing.currentId)
        ? existing.currentId
        : entry.id),
    sessions,
  }
  writeEmbedStoredSessionState(channelId, next)
  return next
}

/** Drop a persisted embed chat session so the next load starts fresh. */
export function clearEmbedStoredChatSession(channelId: string): void {
  writeEmbedStoredSessionState(channelId, null)
}

/** Clear stored chats created under a different agent binding. */
export function clearEmbedStoredChatSessionIfAgentMismatch(
  channelId: string,
  agentId: string,
): void {
  if (!channelId || !agentId) return
  const state = readEmbedStoredSessionState(channelId)
  if (!state) return
  const sessions = state.sessions.filter(
    (entry) => !entry.agentId || entry.agentId === agentId,
  )
  if (sessions.length === state.sessions.length) return
  if (sessions.length === 0) {
    clearEmbedStoredChatSession(channelId)
    return
  }
  const currentId = sessions.some((entry) => entry.id === state.currentId)
    ? state.currentId
    : sessions[0].id
  writeEmbedStoredSessionState(channelId, { currentId, sessions })
}
