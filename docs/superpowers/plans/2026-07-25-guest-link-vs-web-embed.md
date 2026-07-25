# Guest Link vs Web Embed Split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split publish channels into independent **免登录窗口** (`GuestLinkChannel`, short link `/w/:slug`, multi-session, one per agent) and **网页嵌入** (`EmbedChannel`, iframe/widget, single-fresh session).

**Architecture:** New `guest_link_channels` table + admin API. Embed keeps CRUD but drops `web_slug`. Public chat reuses `/api/v1/embed/:channel_id/*` by teaching `LookupEnabledChannel` to resolve GuestLink IDs into an in-memory `EmbedChannel`-shaped runtime view (shared session token / agent-chat / rate limit). Frontend adds `AgentGuestLinkPanel`, renames web card to embed, and gates `EmbedPage` with `sessionMode: multi | single_fresh`. No data migration (not launched).

**Tech Stack:** Go (Gin, GORM, dig), golang-migrate (PG + SQLite), Vue 3 + TDesign, existing embed public SPA (`embed-main.ts`).

**Spec:** `docs/superpowers/specs/2026-07-25-guest-link-vs-web-embed-design.md`

---

## File map

| File | Responsibility |
|------|----------------|
| `migrations/versioned/000082_guest_link_channels.{up,down}.sql` | Create `guest_link_channels` |
| `migrations/versioned/000083_drop_embed_web_slug.{up,down}.sql` | Drop `web_slug` from `embed_channels` |
| `migrations/sqlite/000010_guest_link_channels.{up,down}.sql` | SQLite create |
| `migrations/sqlite/000011_drop_embed_web_slug.{up,down}.sql` | SQLite drop slug |
| `internal/types/guest_link_channel.go` | `GuestLinkChannel` model + request/response helpers |
| `internal/types/interfaces/guest_link_channel.go` | Repo + service interfaces |
| `internal/application/repository/guest_link_channel.go` | CRUD + GetBySlug + GetByAgent |
| `internal/application/service/guest_link_channel.go` | Create (unique agent), update, delete, allocate slug, `AsEmbedChannel()` |
| `internal/application/service/guest_link_channel_test.go` | Unique-agent + slug tests |
| `internal/application/service/embed_session.go` | Dual lookup in `LookupEnabledChannel`; remove `LookupByWebSlug` from embed path |
| `internal/handler/guest_link_channel.go` | Admin CRUD + move `BootstrapWebLink` here |
| `internal/handler/embed_channel.go` | Stop exposing `web_slug`; delete embed bootstrap |
| `internal/router/router.go` | Register guest-link admin + bootstrap |
| `internal/container/container.go` | dig Provide GuestLink repo/service/handler |
| `frontend/src/api/guest-link/index.ts` | Admin API + types |
| `frontend/src/components/AgentGuestLinkPanel.vue` | Single short-link card UI |
| `frontend/src/components/AgentPublishChannels.vue` | `guest` + `embed` cards |
| `frontend/src/components/AgentEmbedChannelPanel.vue` | Embed-only (no slug) |
| `frontend/src/composables/useEmbedBridge.ts` | `sessionMode` |
| `frontend/src/views/embed/EmbedPage.vue` | Hide sidebar in `single_fresh` |
| `frontend/src/i18n/locales/{zh-CN,en-US}.ts` | Rename copy |

**Runtime bridge (read this before coding):**

```
/w/:slug → BootstrapWebLink(GuestLink)
  → IssueSessionToken(guestLink.ID)
  → client calls /api/v1/embed/{guestLink.ID}/sessions|agent-chat
  → EmbedAuth → LookupEnabledChannel(id)
       1) embed_channels by id
       2) else guest_link_channels by id → AsEmbedChannel()
  → existing handlers keep using *types.EmbedChannel
```

`AsEmbedChannel()` fills AgentID/TenantID/rate limits/appearance; sets `AllowedOrigins` empty and relies on same-host for `/w` bootstrap; `PublishToken` carries the guest link's server-only `session_secret`, which keys the session-handle HMAC (guest links are never resolvable through the publish-token lookup).

Out of scope: webhook on GuestLink, data migration, dual-read of old embed slugs.

---

### Task 1: Migrations — guest_link_channels + drop embed web_slug

**Files:**
- Create: `migrations/versioned/000082_guest_link_channels.up.sql`
- Create: `migrations/versioned/000082_guest_link_channels.down.sql`
- Create: `migrations/versioned/000083_drop_embed_web_slug.up.sql`
- Create: `migrations/versioned/000083_drop_embed_web_slug.down.sql`
- Create: `migrations/sqlite/000010_guest_link_channels.up.sql`
- Create: `migrations/sqlite/000010_guest_link_channels.down.sql`
- Create: `migrations/sqlite/000011_drop_embed_web_slug.up.sql`
- Create: `migrations/sqlite/000011_drop_embed_web_slug.down.sql`

- [ ] **Step 1: Write PG `000082` up**

```sql
-- Migration: 000082_guest_link_channels
DO $$ BEGIN RAISE NOTICE '[Migration 000082] Creating guest_link_channels'; END $$;

CREATE TABLE IF NOT EXISTS guest_link_channels (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id BIGINT NOT NULL,
    agent_id VARCHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT true,
    web_slug VARCHAR(16) NOT NULL DEFAULT '',
    welcome_message TEXT NOT NULL DEFAULT '',
    rate_limit_per_minute INTEGER NOT NULL DEFAULT 30,
    rate_limit_per_day INTEGER NOT NULL DEFAULT 10000,
    primary_color VARCHAR(32) NOT NULL DEFAULT '',
    page_title VARCHAR(255) NOT NULL DEFAULT '',
    header_title_mode VARCHAR(32) NOT NULL DEFAULT 'channel',
    show_suggested_questions BOOLEAN NOT NULL DEFAULT true,
    allow_web_search BOOLEAN NOT NULL DEFAULT false,
    allow_file_upload BOOLEAN NOT NULL DEFAULT false,
    default_locale VARCHAR(16) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_guest_link_channels_tenant
    ON guest_link_channels (tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_guest_link_channels_agent_unique
    ON guest_link_channels (tenant_id, agent_id)
    WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_guest_link_channels_web_slug
    ON guest_link_channels (web_slug)
    WHERE web_slug <> '' AND deleted_at IS NULL;

COMMENT ON TABLE guest_link_channels IS
    'Per-agent login-free short-link chat window (/w/:slug); at most one per agent';

DO $$ BEGIN RAISE NOTICE '[Migration 000082] done'; END $$;
```

- [ ] **Step 2: Write PG `000082` down + `000083` up/down**

`000082` down: `DROP TABLE IF EXISTS guest_link_channels;`

`000083` up:

```sql
DROP INDEX IF EXISTS idx_embed_channels_web_slug;
ALTER TABLE embed_channels DROP COLUMN IF EXISTS web_slug;
```

`000083` down: re-add `web_slug VARCHAR(16) NOT NULL DEFAULT ''` + unique index (empty is fine; not launched).

- [ ] **Step 3: Mirror SQLite `000010` / `000011`**

Match columns; use SQLite partial unique indexes like `000007_embed_channel_web_slug.up.sql`.

- [ ] **Step 4: Commit**

```bash
git add migrations/versioned/000082_* migrations/versioned/000083_* \
  migrations/sqlite/000010_* migrations/sqlite/000011_*
git commit -m "migrate: add guest_link_channels and drop embed web_slug"
```

---

### Task 2: GuestLink types + repository

**Files:**
- Create: `internal/types/guest_link_channel.go`
- Create: `internal/types/interfaces/guest_link_channel.go`
- Create: `internal/application/repository/guest_link_channel.go`

- [ ] **Step 1: Add model**

```go
// internal/types/guest_link_channel.go
package types

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type GuestLinkChannel struct {
	ID                     string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID               uint64         `json:"tenant_id" gorm:"not null;index"`
	AgentID                string         `json:"agent_id" gorm:"type:varchar(36);not null"`
	Name                   string         `json:"name" gorm:"type:varchar(255);not null;default:''"`
	Enabled                bool           `json:"enabled" gorm:"not null;default:true"`
	WebSlug                string         `json:"web_slug" gorm:"type:varchar(16);not null;default:''"`
	WelcomeMessage         string         `json:"welcome_message" gorm:"type:text;not null;default:''"`
	RateLimitPerMinute     int            `json:"rate_limit_per_minute" gorm:"not null;default:30"`
	RateLimitPerDay        int            `json:"rate_limit_per_day" gorm:"not null;default:10000"`
	PrimaryColor           string         `json:"primary_color" gorm:"type:varchar(32);not null;default:''"`
	PageTitle              string         `json:"page_title" gorm:"type:varchar(255);not null;default:''"`
	HeaderTitleMode        string         `json:"header_title_mode" gorm:"type:varchar(32);not null;default:'channel'"`
	ShowSuggestedQuestions bool           `json:"show_suggested_questions" gorm:"not null;default:true"`
	AllowWebSearch         bool           `json:"allow_web_search" gorm:"not null;default:false"`
	AllowFileUpload        bool           `json:"allow_file_upload" gorm:"not null;default:false"`
	DefaultLocale          string         `json:"default_locale" gorm:"type:varchar(16);not null;default:''"`
	CreatedAt              time.Time      `json:"created_at"`
	UpdatedAt              time.Time      `json:"updated_at"`
	DeletedAt              gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (GuestLinkChannel) TableName() string { return "guest_link_channels" }

func (ch *GuestLinkChannel) BeforeCreate(tx *gorm.DB) error {
	if ch.ID == "" {
		ch.ID = uuid.New().String()
	}
	if ch.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if IsBuiltinAgentID(ch.AgentID) {
		return fmt.Errorf("built-in agents cannot be used for guest links")
	}
	if ch.RateLimitPerMinute <= 0 {
		ch.RateLimitPerMinute = 30
	}
	if ch.RateLimitPerDay <= 0 {
		ch.RateLimitPerDay = DefaultEmbedRateLimitPerDay
	}
	if ch.HeaderTitleMode == "" {
		ch.HeaderTitleMode = DefaultEmbedHeaderTitleMode
	}
	return nil
}

// AsEmbedChannel maps a guest link into the runtime shape used by embed handlers.
func (ch *GuestLinkChannel) AsEmbedChannel() *EmbedChannel {
	return &EmbedChannel{
		ID:                     ch.ID,
		TenantID:               ch.TenantID,
		AgentID:                ch.AgentID,
		Name:                   ch.Name,
		Enabled:                ch.Enabled,
		WelcomeMessage:         ch.WelcomeMessage,
		RateLimitPerMinute:     ch.RateLimitPerMinute,
		RateLimitPerDay:        ch.RateLimitPerDay,
		PrimaryColor:           ch.PrimaryColor,
		PageTitle:              ch.PageTitle,
		HeaderTitleMode:        ch.HeaderTitleMode,
		ShowSuggestedQuestions: ch.ShowSuggestedQuestions,
		AllowWebSearch:         ch.AllowWebSearch,
		AllowFileUpload:        ch.AllowFileUpload,
		DefaultLocale:          ch.DefaultLocale,
		CreatedAt:              ch.CreatedAt,
		UpdatedAt:              ch.UpdatedAt,
	}
}
```

- [ ] **Step 2: Add interfaces**

```go
// internal/types/interfaces/guest_link_channel.go
package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

type GuestLinkChannelRepository interface {
	Create(ctx context.Context, ch *types.GuestLinkChannel) error
	GetByID(ctx context.Context, id string) (*types.GuestLinkChannel, error)
	GetByWebSlug(ctx context.Context, slug string) (*types.GuestLinkChannel, error)
	GetByAgent(ctx context.Context, tenantID uint64, agentID string) (*types.GuestLinkChannel, error)
	Update(ctx context.Context, ch *types.GuestLinkChannel) error
	Delete(ctx context.Context, tenantID uint64, id string) error
}

type GuestLinkChannelService interface {
	GetByAgent(ctx context.Context, tenantID uint64, agentID string) (*types.GuestLinkChannel, error)
	Create(ctx context.Context, tenantID uint64, agentID string, req *types.GuestLinkChannel) (*types.GuestLinkChannel, error)
	Update(ctx context.Context, tenantID uint64, id string, req *types.GuestLinkChannel, enabled *bool) (*types.GuestLinkChannel, error)
	Delete(ctx context.Context, tenantID uint64, id string) error
	LookupByWebSlug(ctx context.Context, slug string) (*types.GuestLinkChannel, error)
	LookupEnabled(ctx context.Context, id string) (*types.GuestLinkChannel, error)
}
```

- [ ] **Step 3: Implement repository** mirroring `internal/application/repository/embed_channel.go` (`NewGuestLinkChannelRepository`, soft-delete aware queries).

- [ ] **Step 4: Commit**

```bash
git add internal/types/guest_link_channel.go \
  internal/types/interfaces/guest_link_channel.go \
  internal/application/repository/guest_link_channel.go
git commit -m "feat(guest-link): add model and repository"
```

---

### Task 3: GuestLink service + unique-agent tests (TDD)

**Files:**
- Create: `internal/application/service/guest_link_channel.go`
- Create: `internal/application/service/guest_link_channel_test.go`
- Modify: reuse slug helpers from `embed_channel.go` (`generateEmbedWebSlug` pattern) — extract shared `allocatePublicWebSlug(exists func) (string, error)` into `embed_slug.go` **or** copy the 8-retry loop into guest_link service (YAGNI: copy first).

- [ ] **Step 1: Write failing tests**

```go
func TestGuestLinkCreateRejectsSecondForSameAgent(t *testing.T) {
	// arrange repo fake that returns existing on GetByAgent
	// act Create twice
	// assert second error is ErrGuestLinkExists
}

func TestGuestLinkCreateAllocatesSlug(t *testing.T) {
	// assert created.WebSlug != "" and len <= 16
}
```

Define:

```go
var ErrGuestLinkExists = errors.New("guest link already exists for agent")
var ErrGuestLinkDisabled = errors.New("guest link is disabled")
var ErrGuestLinkNotFound = errors.New("guest link not found")
```

- [ ] **Step 2: Run tests — expect FAIL**

```bash
go test ./internal/application/service/ -run TestGuestLink -v
```

Expected: FAIL (undefined or not implemented).

- [ ] **Step 3: Implement service**

`Create`:
1. Reject builtin agent IDs (same as embed).
2. `GetByAgent` — if found → `ErrGuestLinkExists`.
3. Allocate unique `web_slug` via `GetByWebSlug` retries.
4. `repo.Create`.

`LookupByWebSlug`: empty/invalid → not found; disabled → `ErrGuestLinkDisabled`.

- [ ] **Step 4: Run tests — expect PASS**

```bash
go test ./internal/application/service/ -run TestGuestLink -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/application/service/guest_link_channel.go \
  internal/application/service/guest_link_channel_test.go
git commit -m "feat(guest-link): service with one-per-agent constraint"
```

---

### Task 4: Dual lookup + move BootstrapWebLink

**Files:**
- Modify: `internal/application/service/embed_session.go` (`LookupEnabledChannel`)
- Modify: `internal/types/interfaces/embed_channel.go` — inject optional guest lookup **or** change `embedChannelService` constructor to accept `GuestLinkChannelRepository`
- Modify: `internal/handler/embed_channel.go` — remove `BootstrapWebLink`
- Create/Modify: `internal/handler/guest_link_channel.go` — add `BootstrapWebLink`
- Modify: `internal/router/router.go` — wire bootstrap to guest handler
- Modify: fakes in `internal/middleware/embed_auth_test.go`, `internal/handler/embed_*_test.go` if constructor changes
- Test: `internal/application/service/embed_session_test.go` or new `guest_link_bootstrap_test.go`

- [ ] **Step 1: Failing test — LookupEnabledChannel finds guest link**

```go
func TestLookupEnabledChannelFallsBackToGuestLink(t *testing.T) {
	// embed repo miss, guest repo hit → returns AsEmbedChannel() with same ID/AgentID
}
```

- [ ] **Step 2: Implement dual lookup**

In `embedChannelService`, hold `guestLinkRepo interfaces.GuestLinkChannelRepository` (nullable for old tests: if nil, skip).

```go
func (s *embedChannelService) LookupEnabledChannel(ctx context.Context, channelID string) (*types.EmbedChannel, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return nil, ErrEmbedTokenInvalid
	}
	ch, err := s.repo.GetByID(ctx, channelID)
	if err == nil && ch != nil {
		if !ch.Enabled {
			return nil, ErrEmbedChannelDisabled
		}
		return ch, nil
	}
	if s.guestLinkRepo == nil {
		return nil, err
	}
	gl, gerr := s.guestLinkRepo.GetByID(ctx, channelID)
	if gerr != nil || gl == nil {
		return nil, err
	}
	if !gl.Enabled {
		return nil, ErrEmbedChannelDisabled
	}
	return gl.AsEmbedChannel(), nil
}
```

Update `NewEmbedChannelService` signature + `container.go` Provide accordingly.

- [ ] **Step 3: Move bootstrap**

`GuestLinkChannelHandler.BootstrapWebLink`:
1. `guestSvc.LookupByWebSlug(slug)`
2. Origin: **same-host only** (no `allowed_origins` on GuestLink). Reject cross-origin.
3. `embedSvc.IssueSessionToken(ctx, gl.ID)`
4. Config: reuse `embedSvc.PublicConfig(ctx, gl.AsEmbedChannel())`

Router: keep `POST /api/v1/embed/web/:slug/bootstrap` but call guest handler.

- [ ] **Step 4: Remove embed `LookupByWebSlug` usage from bootstrap; delete or leave dead method until Task 5.**

- [ ] **Step 5: Run focused tests**

```bash
go test ./internal/application/service/ ./internal/handler/ ./internal/middleware/ -count=1
```

Expected: PASS (update fakes that break on new ctor args).

- [ ] **Step 6: Commit**

```bash
git commit -am "feat(guest-link): bootstrap short links and dual channel lookup"
```

---

### Task 5: Strip web_slug from EmbedChannel + admin GuestLink API

**Files:**
- Modify: `internal/types/embed_channel.go` — remove `WebSlug` field
- Modify: `internal/application/repository/embed_channel.go` — remove `GetByWebSlug`
- Modify: `internal/application/service/embed_channel.go` — remove `allocateWebSlug` / create-time slug
- Modify: `internal/types/interfaces/embed_channel.go` — remove `GetByWebSlug`, `LookupByWebSlug`
- Modify: `internal/handler/embed_channel.go` — remove `web_slug` from `embedChannelResponse`
- Modify: `internal/handler/guest_link_channel.go` — admin handlers
- Modify: `internal/router/router.go` — register admin routes
- Modify: `internal/container/container.go`
- Fix: all compile errors / fakes referencing `WebSlug` on embed

- [ ] **Step 1: Admin routes**

```
GET    /api/v1/agents/:id/guest-links   → GetByAgent (200 + null/empty if none)
POST   /api/v1/agents/:id/guest-links   → Create (409 on ErrGuestLinkExists)
GET    /api/v1/guest-links/:id
PUT    /api/v1/guest-links/:id
DELETE /api/v1/guest-links/:id
```

Response JSON include `web_url` built like frontend `buildWebChannelURL` (handler uses request host or config base URL — mirror any existing public URL helper; if none, return `web_slug` only and let frontend build URL).

409 body: `{"error":"guest_link_exists"}`.

- [ ] **Step 2: Delete embed slug code paths**; fix `AsEmbedChannel` (no WebSlug field).

- [ ] **Step 3: Compile + test**

```bash
go test ./internal/... -count=1
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(guest-link): admin API and remove embed web_slug"
```

---

### Task 6: Frontend API + GuestLink panel + publish cards

**Files:**
- Create: `frontend/src/api/guest-link/index.ts`
- Create: `frontend/src/components/AgentGuestLinkPanel.vue`
- Modify: `frontend/src/components/AgentPublishChannels.vue`
- Modify: `frontend/src/api/embed/index.ts` — remove `web_slug` from `EmbedChannel` type if present
- Modify: `frontend/src/i18n/locales/zh-CN.ts`, `en-US.ts`

- [ ] **Step 1: API client**

```ts
// frontend/src/api/guest-link/index.ts
export interface GuestLinkChannel {
  id: string
  agent_id: string
  name: string
  enabled: boolean
  web_slug: string
  web_url?: string
  // appearance / limits fields as returned by API
}

export function getAgentGuestLink(agentId: string) { /* GET .../guest-links */ }
export function createAgentGuestLink(agentId: string, body: Partial<GuestLinkChannel>) { /* POST */ }
export function updateGuestLink(id: string, body: Partial<GuestLinkChannel>) { /* PUT */ }
export function deleteGuestLink(id: string) { /* DELETE */ }
```

Reuse `buildWebChannelURL` from `api/embed/index.ts` when `web_url` absent.

- [ ] **Step 2: `AgentGuestLinkPanel.vue`**

- Props: `agentId`, `canManage`
- Load `getAgentGuestLink` on mount
- Empty: button「创建短链」→ `createAgentGuestLink`
- Filled: show URL, Copy, Open (`window.open`), Settings drawer (name/welcome/enabled/limits/appearance — lean form; no origins/widget/webhook)
- No second create button when exists; surface 409 as toast

Expose nothing required for parent beyond optional `reload`.

- [ ] **Step 3: Split publish cards in `AgentPublishChannels.vue`**

```ts
type ChannelTypeKey = 'guest' | 'embed' | 'api' | 'feishu' | 'dingtalk' | 'wechat' | 'portal'
```

- `guest` → `AgentGuestLinkPanel`
- `embed` → existing `AgentEmbedChannelPanel` (was `web`)
- Default `selectedType = 'guest'`
- i18n:
  - `types.guest` / `guestDesc` = 免登录窗口 / 短链直开…
  - `types.embed` / `embedDesc` = 网页嵌入 / iframe 与挂件…
  - Remove or repoint old `types.web*`

- [ ] **Step 4: Manual smoke** (dev server): open agent publish → see two cards; create guest link; copy URL.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/api/guest-link frontend/src/components/AgentGuestLinkPanel.vue \
  frontend/src/components/AgentPublishChannels.vue frontend/src/i18n/locales/zh-CN.ts \
  frontend/src/i18n/locales/en-US.ts frontend/src/api/embed/index.ts
git commit -m "feat(frontend): guest-link panel and publish channel split"
```

---

### Task 7: Embed panel cleanup + single_fresh session mode

**Files:**
- Modify: `frontend/src/components/AgentEmbedChannelPanel.vue` — ensure no web slug / web link deploy UI
- Modify: `frontend/src/composables/useEmbedBridge.ts`
- Modify: `frontend/src/views/embed/EmbedPage.vue`
- Modify: `frontend/src/embed-main.ts` if route meta needed
- Modify: i18n `embedPublish` / `agentEditor.publish` leftover「免登录」strings for embed context

- [ ] **Step 1: `useEmbedBridge` sessionMode**

```ts
export type EmbedSessionMode = 'multi' | 'single_fresh'

// options: { sessionMode?: EmbedSessionMode }
// default: route.meta.webLink ? 'multi' : 'single_fresh'
```

In `finishBootstrapWithSession`:
- if `single_fresh`: **do not** read/restore `readEmbedStoredSessionState`; always `createEmbedSession`; **do not** `upsertEmbedStoredSession` into a multi list (skip persistence or write ephemeral only).
- if `multi`: keep current behavior.

`startNewSession` / sidebar handlers: no-ops or hidden when `single_fresh`.

- [ ] **Step 2: `EmbedPage.vue`**

```vue
<EmbedSessionSidebar v-if="sessionMode === 'multi'" ... />
<!-- hide header menu + new-chat when single_fresh -->
```

Pass `sessionMode` from route: `webLink` meta → `multi`, else `single_fresh`.

- [ ] **Step 3: Verify manually**

1. `/w/{slug}` — sidebar visible; new chat works; refresh restores.
2. `/embed/{id}` with token — no sidebar; refresh gets new empty session.

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(embed): single_fresh session mode for web embed"
```

---

### Task 8: Regression tests + copy polish

**Files:**
- Modify/add Go handler test for bootstrap 409 / same-host
- Modify E2E if present: `frontend` or `e2e` specs asserting「免登录窗口」text
- Grep for `web_slug` / `types.web` / `Login-free` leftovers

- [ ] **Step 1: Grep cleanup**

```bash
rg -n "web_slug|types\.web|免登录窗口|Login-free|deployStepWeb" \
  frontend/src internal --glob '!**/docs/**'
```

Fix stragglers (embed should say 网页嵌入; guest says 免登录窗口).

- [ ] **Step 2: Go regression**

```bash
go test ./internal/handler/ ./internal/application/service/ ./internal/middleware/ -count=1
```

- [ ] **Step 3: Frontend typecheck if available**

```bash
cd frontend && pnpm exec vue-tsc --noEmit
```

(Skip if project has no vue-tsc script; use `pnpm build` only if cheap.)

- [ ] **Step 4: Final commit**

```bash
git commit -am "chore: polish guest-link vs embed copy and tests"
```

---

## Spec coverage checklist

| Spec requirement | Task |
|------------------|------|
| Two publish cards | 6 |
| GuestLink model + one per agent | 1–3 |
| Short link `/w/:slug` + bootstrap | 4 |
| Embed drops web_slug | 1, 5 |
| Shared chat pipeline | 4 (`AsEmbedChannel` + dual lookup) |
| Guest multi-session | 7 (`multi`) |
| Embed single_fresh | 7 |
| No migration | 1 (create/drop only) |
| No GuestLink webhook | 2 (fields omitted) |
| Admin guest API + 409 | 5 |
| i18n rename | 6, 8 |

---

## Self-review notes

- No TBD placeholders; dual-lookup bridge is explicit so Task 4 does not invent a second chat stack.
- `AsEmbedChannel` never copies slug onto embed; `web_slug` lives only on GuestLink (removed from Embed in Task 5).
- Type names: `GuestLinkChannel`, `ErrGuestLinkExists`, `sessionMode: 'multi' | 'single_fresh'` used consistently.
