import { get, post, put, del } from '@/utils/request'
import { buildWebChannelURL, type EmbedLocaleTag, type HeaderTitleMode } from '@/api/embed'

/**
 * A guest link publishes an agent chat surface reachable via a shareable
 * short link (`/w/:slug`), with no external-site embedding surface (no
 * allowed_origins, widget snippet, or webhook). At most one guest link
 * exists per agent.
 */
export interface GuestLinkChannel {
  id: string
  tenant_id: number
  agent_id: string
  name: string
  enabled: boolean
  web_slug: string
  web_url?: string
  welcome_message: string
  rate_limit_per_minute: number
  rate_limit_per_day: number
  primary_color?: string
  page_title?: string
  header_title_mode?: HeaderTitleMode
  show_suggested_questions?: boolean
  allow_web_search?: boolean
  allow_file_upload?: boolean
  default_locale?: EmbedLocaleTag
  created_at: string
  updated_at: string
}

/** Fetches the tenant's guest link for an agent, or `null` if none exists yet. */
export async function getAgentGuestLink(agentId: string) {
  return get<{ success: boolean; data: GuestLinkChannel | null }>(
    `/api/v1/agents/${agentId}/guest-links`,
  )
}

/** Creates the (at most one) guest link for an agent. 409s with `guest_link_exists` if one already exists. */
export async function createAgentGuestLink(agentId: string, data: Partial<GuestLinkChannel>) {
  return post<{ success: boolean; data: GuestLinkChannel }>(
    `/api/v1/agents/${agentId}/guest-links`,
    data,
  )
}

export async function getGuestLink(id: string) {
  return get<{ success: boolean; data: GuestLinkChannel }>(`/api/v1/guest-links/${id}`)
}

export async function updateGuestLink(id: string, data: Partial<GuestLinkChannel>) {
  return put<{ success: boolean; data: GuestLinkChannel }>(`/api/v1/guest-links/${id}`, data)
}

export async function deleteGuestLink(id: string) {
  return del(`/api/v1/guest-links/${id}`)
}

/**
 * Resolves the direct-open chat URL for a guest link.
 *
 * The frontend builder wins because it honours the configured public embed
 * origin (EMBED_BASE_URL), while the server's `web_url` is built from whatever
 * host the admin request arrived on. The server value is only a fallback for
 * links whose slug is missing from the payload.
 */
export function resolveGuestLinkURL(
  channel: Pick<GuestLinkChannel, 'web_url' | 'web_slug'>,
  opts?: { locale?: string },
): string {
  if (channel.web_slug) return buildWebChannelURL(channel.web_slug, opts)
  return channel.web_url ?? ''
}
