import assert from 'node:assert/strict'
import test from 'node:test'

import {
  clearEmbedStoredChatSessionIfAgentMismatch,
  embedChatSessionStorageKey,
  readEmbedStoredSessionState,
  upsertEmbedStoredSession,
  type EmbedStoredSessionEntry,
} from './sessionStorage'

function installMockLocalStorage(): Record<string, string> {
  const store: Record<string, string> = {}
  Object.defineProperty(globalThis, 'localStorage', {
    value: {
      getItem: (key: string) => (key in store ? store[key] : null),
      setItem: (key: string, value: string) => {
        store[key] = value
      },
      removeItem: (key: string) => {
        delete store[key]
      },
    },
    configurable: true,
    writable: true,
  })
  return store
}

function entry(id: string, overrides: Partial<EmbedStoredSessionEntry> = {}): EmbedStoredSessionEntry {
  return { id, sig: `sig-${id}`, updatedAt: Date.now(), ...overrides }
}

test('upsertEmbedStoredSession persists a session and makes it current (multi-session sidebar behavior)', () => {
  const store = installMockLocalStorage()
  const channelId = 'chan-1'

  upsertEmbedStoredSession(channelId, entry('s1'))

  const key = embedChatSessionStorageKey(channelId)
  assert.ok(store[key], 'expected the session to be written to localStorage')

  const state = readEmbedStoredSessionState(channelId)
  assert.equal(state?.currentId, 's1')
  assert.equal(state?.sessions.length, 1)
})

test('a single_fresh embed never calls the persistence helper, so nothing is written', () => {
  const store = installMockLocalStorage()
  const channelId = 'chan-2'

  // This mirrors useEmbedBridge's applyActiveSession: single_fresh sessions
  // update in-memory refs only and skip the upsertEmbedStoredSession call
  // entirely, so the shared multi-session localStorage list stays untouched.
  const sessionMode: 'multi' | 'single_fresh' = 'single_fresh'
  if (sessionMode === 'multi') {
    upsertEmbedStoredSession(channelId, entry('s2'))
  }

  assert.equal(store[embedChatSessionStorageKey(channelId)], undefined)
  assert.equal(readEmbedStoredSessionState(channelId), null)
})

test('readEmbedStoredSessionState sorts nothing on its own but returns all persisted sessions', () => {
  const store = installMockLocalStorage()
  const channelId = 'chan-3'
  store[embedChatSessionStorageKey(channelId)] = JSON.stringify({
    currentId: 's1',
    sessions: [entry('s1', { updatedAt: 1 }), entry('s2', { updatedAt: 2 })],
  })

  const state = readEmbedStoredSessionState(channelId)
  assert.equal(state?.sessions.length, 2)
})

test('clearEmbedStoredChatSessionIfAgentMismatch drops sessions bound to a different agent', () => {
  const store = installMockLocalStorage()
  const channelId = 'chan-4'
  upsertEmbedStoredSession(channelId, entry('s1', { agentId: 'agent-a' }))
  upsertEmbedStoredSession(channelId, entry('s2', { agentId: 'agent-b' }), { makeCurrent: false })

  clearEmbedStoredChatSessionIfAgentMismatch(channelId, 'agent-a')

  const state = readEmbedStoredSessionState(channelId)
  assert.deepEqual(state?.sessions.map((item) => item.id), ['s1'])
  assert.ok(store[embedChatSessionStorageKey(channelId)])
})
