/**
 * 'multi': /w/:slug web-link chats — sidebar with localStorage-backed session
 * history, restored on refresh.
 * 'single_fresh': /embed/:channelId iframe/widget embeds — no sidebar, no
 * session history; every load starts a brand-new session and nothing is
 * persisted to the shared multi-session localStorage list.
 */
export type EmbedSessionMode = 'multi' | 'single_fresh'

/**
 * Resolve the effective session mode: an explicit override always wins,
 * otherwise web-link routes default to 'multi' and everything else (bare
 * iframe/widget embeds) defaults to 'single_fresh'.
 */
export function resolveSessionMode(
  isWebLinkRoute: boolean,
  explicit?: EmbedSessionMode,
): EmbedSessionMode {
  return explicit ?? (isWebLinkRoute ? 'multi' : 'single_fresh')
}

/** Only 'multi' sessions persist chat history to the shared localStorage list. */
export function shouldPersistMultiSession(sessionMode: EmbedSessionMode): boolean {
  return sessionMode === 'multi'
}
