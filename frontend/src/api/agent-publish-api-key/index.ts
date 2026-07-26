import { get, post, del } from '@/utils/request'

/**
 * Agent-bound publish API key (masked in list/create data).
 * Create responses also expose `plaintext` once at the top level.
 */
export interface AgentPublishAPIKey {
  id: number
  name: string
  api_key: string
  created_at?: string
  last_used_at?: string | null
  expires_at?: string | null
}

export interface CreateAgentPublishAPIKeyPayload {
  name: string
  expires_at?: string | null
}

export interface CreateAgentPublishAPIKeyResponse {
  success: boolean
  data: AgentPublishAPIKey
  plaintext: string
}

/** Lists non-revoked publish API keys for an agent (masked). */
export async function listAgentPublishAPIKeys(agentId: string) {
  return get<{ success: boolean; data: AgentPublishAPIKey[] }>(
    `/api/v1/agents/${agentId}/publish-api-keys`,
  )
}

/**
 * Mints a publish API key. The plaintext token is returned once in
 * `plaintext` and is never available again from list/get.
 */
export async function createAgentPublishAPIKey(
  agentId: string,
  payload: CreateAgentPublishAPIKeyPayload,
) {
  return post<CreateAgentPublishAPIKeyResponse>(
    `/api/v1/agents/${agentId}/publish-api-keys`,
    payload,
  )
}

/** Revokes a publish API key for the given agent. */
export async function deleteAgentPublishAPIKey(
  agentId: string,
  keyId: number,
) {
  return del<{ success: boolean }>(
    `/api/v1/agents/${agentId}/publish-api-keys/${keyId}`,
  )
}
