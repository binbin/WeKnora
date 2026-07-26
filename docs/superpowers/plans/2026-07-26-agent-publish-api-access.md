# Agent Publish API Access Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add independent agent-bound publish API keys and a Bearer-only OpenAI-compatible `POST /api/v1/chat/completions` for the publish-channel API experience.

**Architecture:** New `agent_publish_api_keys` table (not `tenant_api_keys`). Admin CRUD under `/agents/:id/publish-api-keys`. Public OpenAPI route uses dedicated `PublishAPIKeyAuth` (Bearer `wkpub_…` only) and maps OpenAI `messages`/`stream` onto existing `SessionService` + `AgentQA`. Sessions are owned via `user_id = api_publish_key:<tenantID>:<keyID>`.

**Tech Stack:** Go (Gin, GORM, dig), golang-migrate (PG + SQLite), Vue 3 + TDesign, existing SSE/event bus for Agent QA.

**Spec:** `docs/superpowers/specs/2026-07-26-agent-publish-api-access-design.md`

---

## File map

| File | Responsibility |
|------|----------------|
| `migrations/versioned/000087_agent_publish_api_keys.{up,down}.sql` | PG table |
| `migrations/sqlite/000015_agent_publish_api_keys.{up,down}.sql` | SQLite table |
| `internal/types/agent_publish_api_key.go` | Model + context helpers |
| `internal/types/principal.go` | `SessionOwnerAPIPublishKeyPrefix` + session owner resolution |
| `internal/types/interfaces/agent_publish_api_key.go` | Repo + service interfaces |
| `internal/application/repository/agent_publish_api_key.go` | CRUD / hash lookup / revoke |
| `internal/application/service/agent_publish_api_key.go` | Create/auth/list/revoke |
| `internal/application/service/agent_publish_api_key_test.go` | Hash/auth/expiry tests |
| `internal/handler/agent_publish_api_key.go` | Admin CRUD handlers |
| `internal/middleware/publish_api_key_auth.go` | Bearer `wkpub_` auth for OpenAPI |
| `internal/handler/openapi_chat.go` | `POST /chat/completions` adapter |
| `internal/handler/openapi_chat_test.go` | Protocol + auth tests |
| `internal/handler/openapi_chat_messages.go` | Extract last user message |
| `internal/middleware/auth.go` | Add `/api/v1/chat/completions` to `noAuthAPI` |
| `internal/router/router.go` | Register admin + OpenAPI routes; wire DI |
| `internal/container/container.go` | Provide repo/service/handler |
| `frontend/src/api/agent-publish-api-key/index.ts` | Admin client |
| `frontend/src/components/AgentPublishChannels.vue` | Replace tenant-key usage |
| `frontend/src/i18n/locales/{zh-CN,en-US,ko-KR,ru-RU}.ts` | Copy for API channel |
| `docs/api/openapi-chat.md` | Usage doc |
| `docs/api/README.md` | Index entry |

**Runtime flow:**

```
Admin (JWT):
  POST /api/v1/agents/:id/publish-api-keys
    → create wkpub_ token (plaintext once)

Caller:
  Authorization: Bearer wkpub_…
  POST /api/v1/chat/completions
    → noAuthAPI bypass → PublishAPIKeyAuth
    → resolve/create Session (user_id=api_publish_key:tid:kid)
    → SessionService.AgentQA
    → OpenAI JSON or SSE chunks
```

Out of scope (P0): PATCH rename, quota UI, `apiKey-agentId` concat, KB routes for publish keys, changing tenant `X-API-Key` auth.

---

### Task 1: Migrations — `agent_publish_api_keys`

**Files:**
- Create: `migrations/versioned/000087_agent_publish_api_keys.up.sql`
- Create: `migrations/versioned/000087_agent_publish_api_keys.down.sql`
- Create: `migrations/sqlite/000015_agent_publish_api_keys.up.sql`
- Create: `migrations/sqlite/000015_agent_publish_api_keys.down.sql`

- [ ] **Step 1: Write PG up migration**

```sql
-- Migration: 000087_agent_publish_api_keys
DO $$ BEGIN RAISE NOTICE '[Migration 000087] Creating agent_publish_api_keys'; END $$;

CREATE TABLE IF NOT EXISTS agent_publish_api_keys (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    agent_id VARCHAR(36) NOT NULL,
    name VARCHAR(128) NOT NULL,
    key_prefix VARCHAR(32) NOT NULL,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    api_key TEXT NOT NULL DEFAULT '',
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_by VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agent_publish_api_keys_tenant_agent
    ON agent_publish_api_keys(tenant_id, agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_publish_api_keys_revoked_at
    ON agent_publish_api_keys(revoked_at);

DO $$ BEGIN RAISE NOTICE '[Migration 000087] agent_publish_api_keys ready'; END $$;
```

- [ ] **Step 2: Write PG down migration**

```sql
DROP INDEX IF EXISTS idx_agent_publish_api_keys_revoked_at;
DROP INDEX IF EXISTS idx_agent_publish_api_keys_tenant_agent;
DROP TABLE IF EXISTS agent_publish_api_keys;
```

- [ ] **Step 3: Write SQLite up/down**

Mirror the same columns; use `INTEGER PRIMARY KEY AUTOINCREMENT` for `id`, `TEXT` for timestamps if that matches recent sqlite migrations (`000014_wechat_oa_preauth.up.sql` style).

- [ ] **Step 4: Commit**

```bash
git add migrations/versioned/000087_agent_publish_api_keys.* \
  migrations/sqlite/000015_agent_publish_api_keys.*
git commit -m "feat(db): add agent_publish_api_keys table"
```

---

### Task 2: Types + principal session owner

**Files:**
- Create: `internal/types/agent_publish_api_key.go`
- Modify: `internal/types/principal.go`
- Create: `internal/types/interfaces/agent_publish_api_key.go`

- [ ] **Step 1: Add model + context**

```go
// internal/types/agent_publish_api_key.go
package types

import (
	"context"
	"time"

	"gorm.io/gorm"
)

const AgentPublishAPIKeyPrefix = "wkpub_"

type AgentPublishAPIKey struct {
	ID         uint64     `json:"id" gorm:"primaryKey;autoIncrement"`
	TenantID   uint64     `json:"tenant_id" gorm:"index;not null"`
	AgentID    string     `json:"agent_id" gorm:"type:varchar(36);not null;index"`
	Name       string     `json:"name" gorm:"type:varchar(128);not null"`
	KeyPrefix  string     `json:"key_prefix" gorm:"type:varchar(32);not null"`
	KeyHash    string     `json:"-" gorm:"type:varchar(64);not null;uniqueIndex"`
	APIKey     string     `json:"-" gorm:"column:api_key;type:text;not null;default:''"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty" gorm:"index"`
	CreatedBy  string     `json:"created_by" gorm:"type:varchar(64);not null;default:''"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (AgentPublishAPIKey) TableName() string { return "agent_publish_api_keys" }

type AgentPublishAPIKeyContext struct {
	KeyID    uint64
	TenantID uint64
	AgentID  string
}

type agentPublishAPIKeyContextKey struct{}

func WithAgentPublishAPIKeyContext(
	ctx context.Context, value AgentPublishAPIKeyContext,
) context.Context {
	return context.WithValue(ctx, agentPublishAPIKeyContextKey{}, value)
}

func AgentPublishAPIKeyContextFromContext(
	ctx context.Context,
) (AgentPublishAPIKeyContext, bool) {
	value, ok := ctx.Value(agentPublishAPIKeyContextKey{}).(AgentPublishAPIKeyContext)
	return value, ok
}

// MaskedKey returns wkpub_****abcd style display value from KeyPrefix.
func (key *AgentPublishAPIKey) MaskedKey() string {
	if key == nil || key.KeyPrefix == "" {
		return "—"
	}
	return key.KeyPrefix + "****"
}
```

Follow existing GORM encryption hooks pattern from `TenantAPIKey` if `api_key` column uses AES — reuse the same hook registration approach used for tenant keys (check `BeforeSave`/`AfterFind` on tenant key). If hooks are type-specific, either share helper or store plaintext hash-only (prefer encrypt like tenant keys when `SYSTEM_AES_KEY` set).

- [ ] **Step 2: Extend principal**

In `internal/types/principal.go`:

```go
const SessionOwnerAPIPublishKeyPrefix = "api_publish_key:"

const PrincipalAPIPublish = "api_publish_key"
```

Update `SessionOwnerIDFromContext` to handle `PrincipalAPIPublish` (or `AgentPublishAPIKeyContextFromContext`):

```go
if pub, ok := AgentPublishAPIKeyContextFromContext(ctx); ok && pub.KeyID > 0 {
	return fmt.Sprintf("%s%d:%d", SessionOwnerAPIPublishKeyPrefix, pub.TenantID, pub.KeyID)
}
```

Add this **before** the generic userID fallback.

- [ ] **Step 3: Interfaces**

```go
package interfaces

type AgentPublishAPIKeyCreateRequest struct {
	TenantID  uint64
	AgentID   string
	Name      string
	CreatedBy string
	ExpiresAt *time.Time
}

type AgentPublishAPIKeyCreateResult struct {
	APIKey *types.AgentPublishAPIKey
	Token  string // plaintext, once
}

type AgentPublishAPIKeyRepository interface {
	Create(ctx context.Context, key *types.AgentPublishAPIKey) error
	GetByHash(ctx context.Context, hash string) (*types.AgentPublishAPIKey, error)
	ListByAgent(ctx context.Context, tenantID uint64, agentID string) ([]*types.AgentPublishAPIKey, error)
	Revoke(ctx context.Context, tenantID uint64, agentID string, keyID uint64) error
	UpdateLastUsed(ctx context.Context, keyID uint64, at time.Time) error
}

type AgentPublishAPIKeyService interface {
	Create(ctx context.Context, req AgentPublishAPIKeyCreateRequest) (*AgentPublishAPIKeyCreateResult, error)
	Authenticate(ctx context.Context, token string) (*types.AgentPublishAPIKey, error)
	ListByAgent(ctx context.Context, tenantID uint64, agentID string) ([]*types.AgentPublishAPIKey, error)
	Revoke(ctx context.Context, tenantID uint64, agentID string, keyID uint64) error
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/types/agent_publish_api_key.go \
  internal/types/principal.go \
  internal/types/interfaces/agent_publish_api_key.go
git commit -m "feat: add agent publish API key types"
```

---

### Task 3: Repository + service (TDD)

**Files:**
- Create: `internal/application/repository/agent_publish_api_key.go`
- Create: `internal/application/service/agent_publish_api_key.go`
- Create: `internal/application/service/agent_publish_api_key_test.go`

- [ ] **Step 1: Write failing service tests**

```go
func TestAuthenticateRejectsRevokedAndExpired(t *testing.T) {
	// Use sqlite in-memory or fake repo; assert:
	// - valid wkpub_ token authenticates
	// - revoked → error
	// - expired → error
	// - empty / wrong prefix → error
}

func TestCreateTokenHasPrefixAndHashLookup(t *testing.T) {
	result, err := svc.Create(ctx, interfaces.AgentPublishAPIKeyCreateRequest{
		TenantID: 1, AgentID: "agent-1", Name: "test", CreatedBy: "user-1",
	})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(result.Token, "wkpub_"))
	require.Equal(t, result.APIKey.KeyPrefix, result.Token[:min(12, len(result.Token))])
	got, err := svc.Authenticate(ctx, result.Token)
	require.NoError(t, err)
	require.Equal(t, result.APIKey.ID, got.ID)
}
```

- [ ] **Step 2: Run tests — expect FAIL**

```bash
go test ./internal/application/service/ -run AgentPublish -count=1
```

Expected: package or symbol not found / FAIL.

- [ ] **Step 3: Implement repository**

Mirror `tenant_api_key.go` patterns: `GetByHash` with `SkipHooks: true` if encryption hooks interfere with hash lookup; list filters `revoked_at IS NULL` optional (list may show revoked — **P0: list only non-revoked**).

- [ ] **Step 4: Implement service**

```go
func generateAgentPublishAPIKeyToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return types.AgentPublishAPIKeyPrefix +
		base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

func hashAgentPublishAPIKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
```

`Create`: trim name; generate token; `KeyPrefix = token[:12]` (or first 8 after prefix); store hash + token in `APIKey`; return plaintext once.

`Authenticate`: trim; require `strings.HasPrefix(token, types.AgentPublishAPIKeyPrefix)`; hash lookup; reject revoked/expired; async touch `last_used_at` (copy tenant key throttle pattern).

- [ ] **Step 5: Run tests — expect PASS**

```bash
go test ./internal/application/service/ -run AgentPublish -count=1
```

- [ ] **Step 6: Commit**

```bash
git add internal/application/repository/agent_publish_api_key.go \
  internal/application/service/agent_publish_api_key.go \
  internal/application/service/agent_publish_api_key_test.go
git commit -m "feat: agent publish API key service and repository"
```

---

### Task 4: Admin handlers + routes + DI

**Files:**
- Create: `internal/handler/agent_publish_api_key.go`
- Modify: `internal/router/router.go`
- Modify: `internal/container/container.go`

- [ ] **Step 1: Handler**

```go
type AgentPublishAPIKeyHandler struct {
	svc   interfaces.AgentPublishAPIKeyService
	agent interfaces.CustomAgentService // or existing agent getter used by embed handlers
}

// List GET /agents/:id/publish-api-keys
// Create POST — body {name, expires_at?} ; response includes plaintext
// Delete DELETE /agents/:id/publish-api-keys/:key_id
```

Response list item JSON:

```json
{
  "id": 1,
  "name": "ci",
  "api_key": "wkpub_xxxx****",
  "created_at": "...",
  "last_used_at": null,
  "expires_at": null
}
```

Create response:

```json
{
  "success": true,
  "data": { "...masked fields..." },
  "plaintext": "wkpub_..."
}
```

Match existing handler response helpers (`c.JSON` patterns in `guest_link_channel.go` / embed admin).

Enforce: path `agent_id` belongs to current tenant; caller has manage-channels permission via existing RBAC group (`apiKeyManageChannels` + JWT Editor/Admin — same as guest-links).

- [ ] **Step 2: Register routes** (near guest-link registration ~L1469)

```go
func (g *rbacGuards) RegisterAgentPublishAPIKeyRoutes(
	r *gin.RouterGroup, handler *handler.AgentPublishAPIKeyHandler,
) {
	group := g.apiKeyGroup(
		r.Group("/agents/:id/publish-api-keys"),
		apiKeyManageChannels(apiKeyFullAccess()),
	)
	group.GET("", g.Editor(), handler.List)
	group.POST("", g.Editor(), handler.Create)
	group.DELETE("/:key_id", g.Editor(), handler.Delete)
}
```

Use the same role helper guest-links use if not exactly `Editor()` — **copy the guest-link route guards verbatim**.

- [ ] **Step 3: Wire container dig Provide** for repo, service, handler; call register from `SetupRouter`.

- [ ] **Step 4: Smoke compile**

```bash
go test ./internal/handler/ -count=1 -run NONE
go build -o /dev/null ./cmd/server/
```

- [ ] **Step 5: Commit**

```bash
git add internal/handler/agent_publish_api_key.go \
  internal/router/router.go internal/container/container.go
git commit -m "feat: admin API for agent publish API keys"
```

---

### Task 5: PublishAPIKeyAuth middleware + noAuth

**Files:**
- Create: `internal/middleware/publish_api_key_auth.go`
- Create: `internal/middleware/publish_api_key_auth_test.go`
- Modify: `internal/middleware/auth.go` (`noAuthAPI`)

- [ ] **Step 1: Add noAuth entry**

```go
"/api/v1/chat/completions": {"POST"},
```

- [ ] **Step 2: Middleware**

```go
func PublishAPIKeyAuth(
	svc interfaces.AgentPublishAPIKeyService,
	tenantService interfaces.TenantService,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			abortOpenAPIUnauthorized(c, "unauthorized", "missing bearer token")
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		key, err := svc.Authenticate(c.Request.Context(), token)
		if err != nil {
			abortOpenAPIUnauthorized(c, "unauthorized", "invalid api key")
			return
		}
		// Load tenant into context (same helpers as attachAPIKeyAuthContext)
		// Set AgentPublishAPIKeyContext
		// Set Principal{Type: PrincipalAPIPublish, ID: fmt.Sprintf("%d", key.ID)}
		c.Next()
	}
}
```

OpenAPI error helper:

```go
func abortOpenAPIError(c *gin.Context, status int, typ, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"error": gin.H{"message": message, "type": typ, "code": code},
	})
}
```

Map revoked/expired to `key_revoked` / `key_expired` if service returns typed errors; otherwise `unauthorized` is acceptable for P0 if typed errors are awkward — prefer typed errors in service.

- [ ] **Step 3: Unit test middleware** with httptest```go
// missing header → 401
// Bearer wkpub_bad → 401
// valid key → Next + context populated
```

- [ ] **Step 4: Commit**

```bash
git add internal/middleware/publish_api_key_auth.go \
  internal/middleware/publish_api_key_auth_test.go \
  internal/middleware/auth.go
git commit -m "feat: Bearer auth for publish API keys"
```

---

### Task 6: OpenAPI message helpers + non-stream completions

**Files:**
- Create: `internal/handler/openapi_chat_messages.go`
- Create: `internal/handler/openapi_chat_messages_test.go`
- Create: `internal/handler/openapi_chat.go`
- Create: `internal/handler/openapi_chat_test.go`
- Modify: `internal/router/router.go`
- Modify: `internal/container/container.go`

- [ ] **Step 1: Message extraction tests**

```go
func TestLastUserMessage(t *testing.T) {
	query, err := lastUserQuery([]openAIChatMessage{
		{Role: "system", Content: "x"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
		{Role: "user", Content: "second"},
	})
	require.NoError(t, err)
	require.Equal(t, "second", query)
}

func TestLastUserMessageMissing(t *testing.T) {
	_, err := lastUserQuery(nil)
	require.Error(t, err)
}
```

Content may be string or array (OpenAI multimodal) — P0: accept string only; if array, concat text parts or 400.

- [ ] **Step 2: Implement `ChatCompletions` handler (non-stream first)**

Request DTO:

```go
type openAIChatCompletionRequest struct {
	Messages  []openAIChatMessage `json:"messages"`
	Stream    bool                `json:"stream"`
	Model     string              `json:"model"`
	SessionID string              `json:"session_id"`
	ChatID    string              `json:"chat_id"`
	AgentID   string              `json:"agent_id"` // if set and != bound → 403
}
```

Logic:

1. Read `AgentPublishAPIKeyContext` (must exist).
2. Resolve `sessionID = req.SessionID`; if empty use `req.ChatID`.
3. If `req.AgentID != "" && req.AgentID != ctx.AgentID` → 403 `agent_unavailable` or dedicated code.
4. Load agent; if missing → 403 `agent_unavailable`.
5. If sessionID empty: `CreateSession` with `TenantID`, `UserID=SessionOwnerIDFromContext`, `LastRequestState.AgentID=ctx.AgentID`.
6. If sessionID set: `GetSession`; verify `TenantID`, `UserID` matches publish-key owner string, and `LastRequestState.AgentID` (or stored agent) matches — if session has no agent state yet, set it; if mismatch → 403 `session_forbidden`.
7. Extract last user query.
8. Create user+assistant messages the same way `AgentQA` handler does (read `executeQA` / message create helpers — **prefer calling shared internal helpers rather than duplicating**). If too coupled, invoke a thin package-level helper in `handler/session` exported for reuse, or call `SessionService` APIs used by `AgentQA`.
9. Non-stream: run `AgentQA` with an event bus that accumulates answer text; return:

```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "model": "<echo or agent name>",
  "choices": [{
    "index": 0,
    "message": {"role": "assistant", "content": "..."},
    "finish_reason": "stop"
  }],
  "usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
  "session_id": "<id>"
}
```

P0: usage zeros OK if not available from events.

- [ ] **Step 3: Register route**

```go
openapi := r.Group("/api/v1")
openapi.POST("/chat/completions",
	middleware.PublishAPIKeyAuth(publishKeySvc, tenantSvc),
	openapiChatHandler.ChatCompletions,
)
```

Register **on engine** (or outside JWT-required confusion): path must match `noAuthAPI` exactly `/api/v1/chat/completions`. Prefer registering beside other public-ish routes; ensure `apiKeyAuthorizer` does not deny JWT-less callers (no TenantAPIKeyScope → pass).

- [ ] **Step 4: Handler tests** with gin engine + fake services (or sqlite):

- 401 without Bearer
- 400 empty messages
- 403 wrong agent_id in body
- 200 non-stream returns `choices[0].message.content` and `session_id`

- [ ] **Step 5: Commit**

```bash
git add internal/handler/openapi_chat*.go internal/router/router.go \
  internal/container/container.go
git commit -m "feat: OpenAI-compatible chat completions (non-stream)"
```

---

### Task 7: Streaming completions

**Files:**
- Modify: `internal/handler/openapi_chat.go`
- Modify: `internal/handler/openapi_chat_test.go`

- [ ] **Step 1: When `stream=true`, set SSE headers**

```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

- [ ] **Step 2: Map AgentQA answer events to OpenAI chunks**

For each answer delta:

```
data: {"id":"chatcmpl-...","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"..."},"finish_reason":null}]}
```

Final chunk: `finish_reason: "stop"` then `data: [DONE]\n\n`.

Include `session_id` in the first or last chunk as an extension field, or send a non-standard first event — **P0 preference:** put `session_id` on every chunk as top-level extension (documented).

- [ ] **Step 3: Test stream path** with recorder; assert body contains `chat.completion.chunk` and `[DONE]`.

- [ ] **Step 4: Commit**

```bash
git add internal/handler/openapi_chat.go internal/handler/openapi_chat_test.go
git commit -m "feat: stream OpenAI-compatible chat completions"
```

---

### Task 8: Frontend admin client + publish channel UI

**Files:**
- Create: `frontend/src/api/agent-publish-api-key/index.ts`
- Modify: `frontend/src/components/AgentPublishChannels.vue`
- Modify: `frontend/src/i18n/locales/zh-CN.ts`
- Modify: `frontend/src/i18n/locales/en-US.ts`
- Modify: `frontend/src/i18n/locales/ko-KR.ts`
- Modify: `frontend/src/i18n/locales/ru-RU.ts`

- [ ] **Step 1: API client**

```ts
export interface AgentPublishAPIKey {
  id: number
  name: string
  api_key: string
  created_at?: string
  last_used_at?: string | null
  expires_at?: string | null
}

export function listAgentPublishAPIKeys(agentId: string) {
  return get(`/api/v1/agents/${agentId}/publish-api-keys`)
}

export function createAgentPublishAPIKey(
  agentId: string,
  payload: { name: string; expires_at?: string | null },
) {
  return post(`/api/v1/agents/${agentId}/publish-api-keys`, payload)
}

export function deleteAgentPublishAPIKey(agentId: string, keyId: number) {
  return del(`/api/v1/agents/${agentId}/publish-api-keys/${keyId}`)
}
```

Follow existing `frontend/src/api/guest-link` import style for `get/post/del`.

- [ ] **Step 2: Replace tenant key logic in `AgentPublishChannels.vue`**

- Remove `listTenantAPIKeys` / `createTenantAPIKey` imports for this panel.
- `loadApiRows` → `listAgentPublishAPIKeys(props.agentId)`.
- Create dialog: on success show **plaintext modal** (new state) with copy button; warn cannot view again.
- Add revoke action calling `deleteAgentPublishAPIKey`.
- Show API root + hint: root is not the endpoint.
- Show curl sample using `Authorization: Bearer` and `/api/v1/chat/completions`.
- Doc link → `/docs` path used by project for `docs/api/openapi-chat.md` if frontend has doc route; otherwise link relative markdown on GitHub path `docs/api/openapi-chat.md` under the repo (prefer in-app docs if `ApiIntegrationSettings` has a pattern — **copy that pattern**).

- [ ] **Step 3: i18n keys** under `agentEditor.publish.*`:

- `apiRootHint`: 根地址不是接口本身，请拼接 `/chat/completions`
- `apiAuthHint`: 使用 `Authorization: Bearer <key>`
- `apiPlaintextTitle` / `apiPlaintextWarn`
- `apiRevoke` / `apiCurlTitle`
- Update `apiHint` to remove “空间 key + agent_id” wording — keys are bound to this agent.

- [ ] **Step 4: Manual check / e2e if feasible**

If `frontend/e2e/publish-exact-url.spec.ts` can extend cheaply, assert API panel no longer calls tenants api-keys; otherwise skip e2e in P0 and rely on component wiring.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/api/agent-publish-api-key \
  frontend/src/components/AgentPublishChannels.vue \
  frontend/src/i18n/locales/zh-CN.ts \
  frontend/src/i18n/locales/en-US.ts \
  frontend/src/i18n/locales/ko-KR.ts \
  frontend/src/i18n/locales/ru-RU.ts
git commit -m "feat(frontend): publish-channel API keys UI"
```

---

### Task 9: Docs

**Files:**
- Create: `docs/api/openapi-chat.md`
- Modify: `docs/api/README.md`

- [ ] **Step 1: Write usage doc** covering:

- Where to create key (发布渠道 → API)
- BaseURL vs endpoint
- `Authorization: Bearer wkpub_…`
- curl non-stream + stream
- `session_id` / `chat_id`
- Error shape
- Note: space API keys (`X-API-Key`) cannot call this route

- [ ] **Step 2: Index in README**

- [ ] **Step 3: Commit**

```bash
git add docs/api/openapi-chat.md docs/api/README.md
git commit -m "docs: OpenAPI chat completions for publish API keys"
```

---

### Task 10: Spec coverage self-check + regression

- [ ] **Step 1: Run focused tests**

```bash
go test ./internal/application/service/ -run AgentPublish -count=1
go test ./internal/middleware/ -run PublishAPIKey -count=1
go test ./internal/handler/ -run OpenAPI -count=1
```

Expected: PASS.

- [ ] **Step 2: Confirm against spec checklist**

| Spec item | Task |
|-----------|------|
| Independent table | Task 1–3 |
| Admin CRUD + plaintext once | Task 4, 8 |
| Bearer only OpenAPI | Task 5–7 |
| Bound agent | Task 3, 6 |
| messages/stream/model | Task 6–7 |
| session_id isolation by key | Task 6 |
| Docs + UI experience | Task 8–9 |
| No tenant key in publish UI | Task 8 |

- [ ] **Step 3: Final commit only if stray fixes remain**; otherwise done.

---

## Implementation notes for agents

1. **Do not** authenticate `wkpub_` via global `X-API-Key` or JWT Bearer path — JWT parser would reject it; use `noAuthAPI` + `PublishAPIKeyAuth`.
2. **Do not** grant `TenantAPIKeyScope` for publish keys — keeps `APIKeyRouteAuthorizer` from treating them as workspace keys.
3. Prefer reusing `SessionService.AgentQA` + event bus over HTTP self-calls.
4. Keep functions short; split OpenAPI mapping helpers into their own files.
5. Line length ~88–100; meaningful names; structured logs with `tenant_id`/`agent_id`/`key_id` only.
