# WeChat Official Account Cloud Relay (P0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship P0 of WeChat Official Account (`wechat_oa`): Cloud third-party pre-auth QR bind, Cloud→instance HMAC message relay, text Q&A via existing IM pipeline; split publish-channel cards so 公众号 / 企业微信 / 个人微信 no longer mix.

**Architecture:** New `internal/im/wechat_oa` adapter (`mode=cloud_relay`) talks to TreeRAG Cloud OA HTTP APIs (client interface + real HTTP impl; tests use fake Cloud). Binding completes via Cloud HMAC callback that creates `im_channels`. Fan messages hit existing `POST /api/v1/im/callback/:channel_id` after HMAC verify. Replies call Cloud customer-service send. Cloud repo implements WeChat Open Platform; this repo owns instance contract + UI.

**Tech Stack:** Go (Gin, GORM, dig), golang-migrate (PG + SQLite), Vue 3 + TDesign, existing IM Service / Agent QA.

**Spec:** `docs/superpowers/specs/2026-07-26-wechat-oa-cloud-relay-design.md`

**Out of scope (P1+):** images, ASR, welcome events, menu clicks, TTS, ops metrics. Stub media types with a fixed “暂不支持” text reply only if Cloud accidentally relays them (optional guard).

---

## File map

| File | Responsibility |
|------|----------------|
| `migrations/versioned/000086_wechat_oa_preauth.{up,down}.sql` | Pre-auth session table (PG) |
| `migrations/sqlite/000014_wechat_oa_preauth.{up,down}.sql` | Pre-auth session table (SQLite) |
| `internal/im/adapter.go` | Add `PlatformWeChatOA` |
| `internal/im/types.go` | `computeBotIdentity` for `wechat_oa`; allow `mode=cloud_relay` |
| `internal/types/knowledge.go` | `ChannelWechatOA` constant |
| `internal/im/service.go` | Map platform→channel; create/delete hooks for Cloud unbind |
| `internal/im/wechat_oa/hmac.go` | Cloud↔instance HMAC sign/verify |
| `internal/im/wechat_oa/cloud_client.go` | Cloud OA HTTP client interface + impl |
| `internal/im/wechat_oa/relay.go` | RelayEvent JSON schema |
| `internal/im/wechat_oa/adapter.go` | Adapter: VerifyCallback, ParseCallback, SendReply |
| `internal/im/wechat_oa/factory.go` | Factory (no long-poll goroutine) |
| `internal/im/wechat_oa/preauth.go` | Preauth model helpers |
| `internal/application/repository/wechat_oa_preauth.go` | CRUD for preauth rows |
| `internal/handler/wechat_oa.go` | PreAuth, Status, BindingComplete |
| `internal/handler/im.go` | `validIMPlatforms` + create-channel defaults for `wechat_oa` |
| `internal/handler/im_platform_test.go` | Expect `wechat_oa` in platform list |
| `internal/container/container.go` | Register factory + wire client |
| `internal/router/router.go` | Routes for preauth / status / binding complete |
| `.env.example` | Document `WECHAT_OA_CALLBACK_BASE_URL` (+ `APP_EXTERNAL_URL` fallback) |
| `frontend/src/api/agent/index.ts` | Types + wechat_oa API helpers |
| `frontend/src/components/AgentPublishChannels.vue` | Split cards: wechat_oa / wecom / wechat |
| `frontend/src/components/IMChannelPanel.vue` | wechat_oa bind UI (or thin WeChatOABindPanel) |
| `frontend/src/i18n/locales/{zh-CN,en-US,ko-KR,ru-RU}.ts` | Copy split |

**Runtime (P0):**

```
Bind:
  UI → POST /agents/:id/wechat-oa/preauth
    → Cloud CreatePreAuth(instance_base_url, state)
    → save wechat_oa_preauths (wait)
    → return qrcode_url + preauth_id
  Admin scans → WeChat → Cloud
  Cloud → POST /api/v1/im/wechat_oa/binding/complete (HMAC)
    → create im_channels(platform=wechat_oa)
    → mark preauth bound
  UI polls GET /wechat-oa/preauth/:id → bound + channel summary

Message:
  Fan → Cloud → POST /api/v1/im/callback/:channel_id (HMAC, RelayEvent body)
    → wechat_oa.Adapter → HandleMessage → Agent QA
    → SendReply → Cloud SendCustomerMessage(text)
```

**Cloud HTTP contract (instance client):**

| Method | Path (under TreeRAGCloudBaseURL) | Purpose |
|--------|----------------------------------|---------|
| POST | `/api/v1/oa/preauth` | Body: `{instance_base_url,tenant_id,agent_id,state}` → `{preauth_id,qrcode_url,expires_at,callback_secret}` |
| GET | `/api/v1/oa/preauth/{id}` | Sync status if Cloud→instance callback missed |
| POST | `/api/v1/oa/bindings/{authorizer_appid}/unbind` | Disable Cloud binding |
| POST | `/api/v1/oa/message/send` | Body: `{authorizer_appid,touser,msgtype,text:{content}}` |

Instance→Cloud uses existing tenant TreeRAGCloud AppID/AppSecret `Sign()` headers.  
Cloud→instance uses per-binding `instance_callback_secret` HMAC (below).

**HMAC (Cloud→instance):**

```
Header X-WeKnora-OA-Timestamp: unix seconds
Header X-WeKnora-OA-Signature: hex(HMAC-SHA256(secret, timestamp + "\n" + rawBody))
Reject if |now-ts| > 300s or signature mismatch.
```

---

### Task 1: Migrations — wechat_oa_preauths

**Files:**
- Create: `migrations/versioned/000086_wechat_oa_preauth.up.sql`
- Create: `migrations/versioned/000086_wechat_oa_preauth.down.sql`
- Create: `migrations/sqlite/000014_wechat_oa_preauth.up.sql`
- Create: `migrations/sqlite/000014_wechat_oa_preauth.down.sql`

- [ ] **Step 1: Write PG up migration**

```sql
-- Migration: 000086_wechat_oa_preauth
DO $$ BEGIN RAISE NOTICE '[Migration 000086] Creating wechat_oa_preauths'; END $$;

CREATE TABLE IF NOT EXISTS wechat_oa_preauths (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id BIGINT NOT NULL,
    agent_id VARCHAR(36) NOT NULL,
    cloud_preauth_id VARCHAR(128) NOT NULL DEFAULT '',
    state VARCHAR(64) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'wait',
    qrcode_url TEXT NOT NULL DEFAULT '',
    callback_secret VARCHAR(128) NOT NULL DEFAULT '',
    channel_id VARCHAR(36) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_wechat_oa_preauths_tenant_agent
    ON wechat_oa_preauths (tenant_id, agent_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_wechat_oa_preauths_state
    ON wechat_oa_preauths (state);

COMMENT ON TABLE wechat_oa_preauths IS 'Pending WeChat OA Cloud pre-auth sessions for QR binding';
COMMENT ON COLUMN wechat_oa_preauths.status IS 'wait|scaned|bound|expired|cancelled|failed';
COMMENT ON COLUMN wechat_oa_preauths.callback_secret IS 'HMAC secret from Cloud CreatePreAuth; verifies BindingComplete until channel exists';

DO $$ BEGIN RAISE NOTICE '[Migration 000086] Done'; END $$;
```

- [ ] **Step 2: Write PG down**

```sql
DROP TABLE IF EXISTS wechat_oa_preauths;
```

- [ ] **Step 3: Write SQLite up/down** (same columns; use `TEXT` for timestamps if that matches sibling migrations — copy style from `000012_guest_link_session_secret.up.sql`).

- [ ] **Step 4: Commit**

```bash
git add migrations/versioned/000086_wechat_oa_preauth.*.sql \
        migrations/sqlite/000014_wechat_oa_preauth.*.sql
git commit -m "migrate: add wechat_oa_preauths for OA QR binding"
```

---

### Task 2: Platform constants + bot_identity + channel mapping

**Files:**
- Modify: `internal/im/adapter.go`
- Modify: `internal/im/types.go` (`computeBotIdentity`, `BeforeCreate` mode default)
- Modify: `internal/types/knowledge.go`
- Modify: `internal/im/service.go` (`imPlatformToChannel`)
- Test: `internal/im/types_test.go`, `internal/im/channel_mapping_test.go`
- Modify: `internal/handler/im.go` (`validIMPlatforms`)
- Modify: `internal/handler/im_platform_test.go`

- [ ] **Step 1: Write failing identity test**

```go
func TestComputeBotIdentity_WeChatOA(t *testing.T) {
	ch := &IMChannel{
		Platform:    "wechat_oa",
		Credentials: []byte(`{"authorizer_appid":"wxabc"}`),
	}
	want := "wechat_oa:wxabc"
	if got := ch.computeBotIdentity(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./internal/im/ -run TestComputeBotIdentity_WeChatOA -count=1`  
Expected: FAIL (empty identity or missing case)

- [ ] **Step 3: Implement**

In `adapter.go` add `PlatformWeChatOA Platform = "wechat_oa"`.

In `types.go` `computeBotIdentity` add:

```go
case "wechat_oa":
	if appID := str("authorizer_appid"); appID != "" {
		return "wechat_oa:" + appID
	}
```

In `BeforeCreate`, if `ch.Platform == "wechat_oa"` set `Mode = "cloud_relay"` and `OutputMode = "full"` when empty.

In `knowledge.go` add `ChannelWechatOA = "wechat_oa"`.

In `imPlatformToChannel` map `"wechat_oa"` → `types.ChannelWechatOA`.

In `handler/im.go` add `"wechat_oa": true` to `validIMPlatforms`.

In create-channel handler, treat `wechat_oa` like wechat for forced mode:

```go
if req.Platform == "wechat" {
	channel.Mode = "longpoll"
	channel.OutputMode = "full"
}
if req.Platform == "wechat_oa" {
	channel.Mode = "cloud_relay"
	channel.OutputMode = "full"
}
```

- [ ] **Step 4: Extend channel mapping test + platform test; run all**

Run: `go test ./internal/im/ ./internal/handler/ -run 'TestComputeBotIdentity_WeChatOA|TestIMPlatformToChannel|TestValidIMPlatforms' -count=1`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/im/adapter.go internal/im/types.go internal/im/types_test.go \
        internal/im/service.go internal/im/channel_mapping_test.go \
        internal/types/knowledge.go internal/handler/im.go \
        internal/handler/im_platform_test.go
git commit -m "feat(im): add wechat_oa platform identity and channel mapping"
```

---

### Task 3: HMAC + RelayEvent + Cloud client

**Files:**
- Create: `internal/im/wechat_oa/hmac.go`
- Create: `internal/im/wechat_oa/hmac_test.go`
- Create: `internal/im/wechat_oa/relay.go`
- Create: `internal/im/wechat_oa/cloud_client.go`
- Create: `internal/im/wechat_oa/cloud_client_test.go`

- [ ] **Step 1: Failing HMAC test**

```go
func TestVerifyHMAC_AcceptsValid(t *testing.T) {
	secret := "sekrit"
	body := []byte(`{"msg_id":"1"}`)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := SignHMAC(secret, ts, body)
	if err := VerifyHMAC(secret, ts, body, sig, time.Now(), 5*time.Minute); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyHMAC_RejectsTampered(t *testing.T) {
	secret := "sekrit"
	body := []byte(`{"msg_id":"1"}`)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := SignHMAC(secret, ts, body)
	if err := VerifyHMAC(secret, ts, []byte(`{"msg_id":"2"}`), sig, time.Now(), 5*time.Minute); err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

Run: `go test ./internal/im/wechat_oa/ -run TestVerifyHMAC -count=1`

- [ ] **Step 3: Implement hmac.go**

```go
package wechat_oa

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

func SignHMAC(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("\n"))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyHMAC(secret, timestamp string, body []byte, signature string, now time.Time, skew time.Duration) error {
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp")
	}
	delta := now.Unix() - ts
	if delta < 0 {
		delta = -delta
	}
	if time.Duration(delta)*time.Second > skew {
		return fmt.Errorf("timestamp skew")
	}
	expected := SignHMAC(secret, timestamp, body)
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return fmt.Errorf("bad signature")
	}
	return nil
}
```

- [ ] **Step 4: Define RelayEvent in relay.go**

```go
type RelayEvent struct {
	RelayEventID string `json:"relay_event_id"`
	MsgID        string `json:"msg_id"`
	AuthorizerAppID string `json:"authorizer_appid"`
	FromUser     string `json:"from_user"` // openid
	MsgType      string `json:"msg_type"`  // text|image|voice|event|...
	Content      string `json:"content"`
	CreateTime   int64  `json:"create_time"`
	Event        string `json:"event,omitempty"`
	EventKey     string `json:"event_key,omitempty"`
}
```

- [ ] **Step 5: CloudClient interface + HTTP impl + fake in tests**

```go
type CloudClient interface {
	CreatePreAuth(ctx context.Context, req PreAuthRequest) (*PreAuthResponse, error)
	GetPreAuth(ctx context.Context, cloudPreAuthID string) (*PreAuthStatus, error)
	Unbind(ctx context.Context, authorizerAppID string) error
	SendText(ctx context.Context, authorizerAppID, toUser, text string) error
}

type PreAuthRequest struct {
	InstanceBaseURL string `json:"instance_base_url"`
	TenantID        uint64 `json:"tenant_id"`
	AgentID         string `json:"agent_id"`
	State           string `json:"state"`
}

type PreAuthResponse struct {
	PreAuthID      string    `json:"preauth_id"`
	QRCodeURL      string    `json:"qrcode_url"`
	ExpiresAt      time.Time `json:"expires_at"`
	CallbackSecret string    `json:"callback_secret"`
}
```

HTTP client: base `provider.TreeRAGCloudBaseURL`, sign with tenant AppID/AppSecret via `modelsutils.Sign`. Constructor receives `baseURL`, `appID`, `appSecret`, `httpClient`.

Fake for tests: `httptest.NewServer` implementing the four paths.

- [ ] **Step 6: Tests PASS + commit**

```bash
go test ./internal/im/wechat_oa/ -count=1
git add internal/im/wechat_oa/
git commit -m "feat(wechat_oa): add HMAC, relay schema, and Cloud client"
```

---

### Task 4: Adapter + Factory

**Files:**
- Create: `internal/im/wechat_oa/adapter.go`
- Create: `internal/im/wechat_oa/adapter_test.go`
- Create: `internal/im/wechat_oa/factory.go`
- Modify: `internal/container/container.go`

- [ ] **Step 1: Failing adapter parse/send tests** using gin test context + fake CloudClient.

```go
func TestAdapter_ParseCallback_Text(t *testing.T) {
	// Build gin context with JSON RelayEvent body and HMAC headers
	// adapter.VerifyCallback + ParseCallback → IncomingMessage
	// Platform=wechat_oa, UserID=openid, Content=text, MessageID=msg_id
}

func TestAdapter_SendReply_CallsCloud(t *testing.T) {
	fake := &fakeCloud{}
	a := NewAdapter("wxapp", "secret", fake)
	err := a.SendReply(context.Background(), &im.IncomingMessage{UserID: "o1"}, &im.ReplyMessage{Content: "hi"})
	if err != nil || fake.sent != "hi" {
		t.Fatalf("err=%v sent=%q", err, fake.sent)
	}
}
```

- [ ] **Step 2: Implement Adapter**

```go
type Adapter struct {
	authorizerAppID string
	callbackSecret  string
	cloud           CloudClient
}

func (a *Adapter) Platform() im.Platform { return im.PlatformWeChatOA }

func (a *Adapter) VerifyCallback(c *gin.Context) error {
	body, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	ts := c.GetHeader("X-WeKnora-OA-Timestamp")
	sig := c.GetHeader("X-WeKnora-OA-Signature")
	return VerifyHMAC(a.callbackSecret, ts, body, sig, time.Now(), 5*time.Minute)
}

func (a *Adapter) ParseCallback(c *gin.Context) (*im.IncomingMessage, error) {
	var ev RelayEvent
	if err := c.ShouldBindJSON(&ev); err != nil {
		return nil, err
	}
	if ev.MsgType != "text" {
		// P0: ignore non-text (return nil → ACK only)
		return nil, nil
	}
	id := ev.MsgID
	if id == "" {
		id = ev.RelayEventID
	}
	return &im.IncomingMessage{
		Platform:    im.PlatformWeChatOA,
		MessageType: im.MessageTypeText,
		UserID:      ev.FromUser,
		Content:     ev.Content,
		MessageID:   id,
		ChatType:    im.ChatTypeDirect,
	}, nil
}

func (a *Adapter) SendReply(ctx context.Context, in *im.IncomingMessage, reply *im.ReplyMessage) error {
	return a.cloud.SendText(ctx, a.authorizerAppID, in.UserID, reply.Content)
}

func (a *Adapter) HandleURLVerification(c *gin.Context) bool { return false }
```

Check existing `ChatType` constants in `adapter.go` / types and use the DM value.

Factory:

```go
func NewFactory(cloudFactory func(ctx context.Context, tenantID uint64) (CloudClient, error)) im.AdapterFactory {
	return func(factoryCtx context.Context, channel *im.IMChannel, _ func(context.Context, *im.IncomingMessage) error) (im.Adapter, context.CancelFunc, error) {
		creds, err := im.ParseCredentials(channel.Credentials)
		// require authorizer_appid, instance_callback_secret
		client, err := cloudFactory(factoryCtx, channel.TenantID)
		adapter := NewAdapter(appID, secret, client)
		return adapter, func() {}, nil // no long-poll cancel work
	}
}
```

Register in container: `imService.RegisterAdapterFactory("wechat_oa", wechat_oa.NewFactory(...))`.

Wire `cloudFactory` to load tenant TreeRAGCloud credentials; if missing, factory returns clear error.

- [ ] **Step 3: Tests PASS + commit**

```bash
go test ./internal/im/wechat_oa/ -count=1
git add internal/im/wechat_oa/ internal/container/container.go
git commit -m "feat(wechat_oa): add cloud_relay adapter and factory"
```

---

### Task 5: Preauth repository + binding handlers

**Files:**
- Create: `internal/types/wechat_oa_preauth.go`
- Create: `internal/application/repository/wechat_oa_preauth.go`
- Create: `internal/handler/wechat_oa.go`
- Create: `internal/handler/wechat_oa_test.go`
- Modify: `internal/handler/im.go` (optional: delete channel → Cloud Unbind)
- Modify: `internal/router/router.go`
- Modify: `internal/container/container.go`

- [ ] **Step 1: Model**

```go
type WeChatOAPreauth struct {
	ID             string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID       uint64    `json:"tenant_id"`
	AgentID        string    `json:"agent_id"`
	CloudPreauthID string    `json:"cloud_preauth_id"`
	State          string    `json:"state"`
	Status         string    `json:"status"`
	QRCodeURL      string    `json:"qrcode_url"`
	CallbackSecret string    `json:"-"` // never expose to browser
	ChannelID      string    `json:"channel_id"`
	ErrorMessage   string    `json:"error_message"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
func (WeChatOAPreauth) TableName() string { return "wechat_oa_preauths" }
```

- [ ] **Step 2: Repo** — `Create`, `GetByID`, `GetByState`, `Update`.

- [ ] **Step 3: Handler PreAuth** `POST /api/v1/agents/:id/wechat-oa/preauth`

Logic:
1. Resolve tenant Cloud credentials; else 400 `weknoracloud_credentials_required`.
2. Resolve instance base URL: `WECHAT_OA_CALLBACK_BASE_URL` else `APP_EXTERNAL_URL`; else 400 `callback_base_url_required`.
3. `state = random 32 hex`.
4. Call Cloud `CreatePreAuth`.
5. Insert preauth row `status=wait`.
6. Return `{preauth_id, qrcode_url, expires_at, status}`.

- [ ] **Step 4: Handler Status** `GET /api/v1/wechat-oa/preauth/:id`  
Tenant-scoped get; if still wait, optionally sync from Cloud `GetPreAuth`.

- [ ] **Step 5: Handler BindingComplete** `POST /api/v1/im/wechat_oa/binding/complete`  
Public (no user JWT). Body:

```json
{
  "state": "...",
  "authorizer_appid": "wx...",
  "nick_name": "...",
  "principal_name": "...",
  "head_img": "...",
  "service_type": 2,
  "verify_type": 0,
  "cloud_binding_id": "bnd_...",
  "instance_callback_secret": "..."
}
```

Flow:
1. Load preauth by `state`; reject expired.
2. Verify HMAC using `preauth.callback_secret` (returned by Cloud CreatePreAuth and stored on the row).
3. Create `im.IMChannel` with credentials JSON; `CreateChannel` (duplicate bot_identity → 409).
4. Update preauth `status=bound`, `channel_id=...`.
5. Return `{channel_id}`.

- [ ] **Step 6: On IM channel delete for wechat_oa**, call Cloud `Unbind(authorizer_appid)` (best-effort log on failure).

- [ ] **Step 7: Router**

```go
// auth'd
POST /agents/:id/wechat-oa/preauth
GET  /wechat-oa/preauth/:id
// public HMAC
POST /im/wechat_oa/binding/complete
// existing
POST /im/callback/:channel_id  // already works once adapter registered
```

Register with same capability gate as other channel manage routes (`ManageChannels`).

- [ ] **Step 8: Handler tests with fake Cloud + sqlite/memory if project pattern allows; else table-driven with mocked repo interfaces.** Prefer httptest for BindingComplete HMAC round-trip.

- [ ] **Step 9: Commit**

```bash
git commit -m "feat(wechat_oa): preauth APIs and Cloud binding callback"
```

---

### Task 6: Dedup for relay MsgId (P0 safety)

**Files:**
- Modify: `internal/im/service.go` (or wechat_oa wrapper) if HandleMessage already dedups by MessageID — verify.
- If no dedup: add short-TTL in-memory/Redis set keyed `wechat_oa:msgid:{id}` in adapter or service before QA.

- [ ] **Step 1: Grep** `MessageID` / dedup in `internal/im/service.go`.

- [ ] **Step 2:** If absent, add `wechat_oa/dedup.go` with process-local sync.Map + 10m TTL; call from Adapter.ParseCallback or start of HandleMessage path for platform wechat_oa only. Test double Parse same MsgID → second returns nil.

- [ ] **Step 3: Commit** `fix(wechat_oa): dedupe relay messages by MsgId`

---

### Task 7: Frontend API + publish channel split

**Files:**
- Modify: `frontend/src/api/agent/index.ts`
- Modify: `frontend/src/components/AgentPublishChannels.vue`
- Modify: `frontend/src/i18n/locales/zh-CN.ts` (and en/ko/ru)

- [ ] **Step 1: Extend IMChannel platform union** with `'wechat_oa'`.

- [ ] **Step 2: Add API helpers**

```ts
export function createWeChatOAPreauth(agentId: string) {
  return post<{ data: { preauth_id: string; qrcode_url: string; expires_at: string; status: string } }>(
    `/api/v1/agents/${agentId}/wechat-oa/preauth`,
  )
}
export function getWeChatOAPreauthStatus(preauthId: string) {
  return get<{ data: { status: string; channel_id?: string; qrcode_url?: string; error_message?: string } }>(
    `/api/v1/wechat-oa/preauth/${preauthId}`,
  )
}
```

- [ ] **Step 3: Split `channelTypes` in AgentPublishChannels.vue**

Replace single `wechat` card with three:

| key | filter | primary create |
|-----|--------|----------------|
| `wechat_oa` | `['wechat_oa']` | open OA bind (not wecom) |
| `wecom` | `'wecom'` | `wecom` |
| `wechat` | `['wechat']` | iLink QR flow |

Update `isImType` accordingly. i18n:

- `types.wechat` → keep as 微信公众号 but point to wechat_oa key rename: use `types.wechat_oa` / `wechat_oaDesc`
- `types.wecom` / `wecomDesc` for 企业微信
- `types.wechat_personal` / desc for 个人微信 (iLink)

- [ ] **Step 4: Manual smoke** — open publish tab, confirm three cards, wecom create still works, personal wechat still shows iLink UI.

- [ ] **Step 5: Commit** `feat(frontend): split WeChat publish channels into OA / WeCom / personal`

---

### Task 8: WeChat OA bind UI

**Files:**
- Modify: `frontend/src/components/IMChannelPanel.vue` **or** Create: `frontend/src/components/WeChatOABindPanel.vue` hosted by publish channels for `wechat_oa`
- Modify: i18n keys `agentEditor.im.wechatOa*`

- [ ] **Step 1:** For `platform === 'wechat_oa'`, replace credential form with:

1. Button「绑定公众号」→ `createWeChatOAPreauth` → show QR image (`qrcode_url`)
2. Poll status every 2s until `bound` | `expired` | `failed`
3. On bound: refresh IM channel list; show nick_name/head_img from channel credentials summary if API returns `credentials_configured` + optional public fields via GetChannel (if credentials redacted, add summary fields `oa_nick_name` later — P0: list refresh is enough)
4. Rebind / unbind use existing delete channel

Disable bind button when API returns 400 codes; show toast with message.

- [ ] **Step 2:** Publish card「绑定公众号」calls `imPanelRef?.openCreate('wechat_oa')` or dedicated bind method that skips name form and starts QR immediately (channel name default = nick_name after bind; before bind use「微信公众号」).

Simplest P0: create channel only on BindingComplete (server-side); UI never POSTs empty wechat_oa channel. Panel in list mode only +「绑定」opens QR dialog without create-until-bound.

- [ ] **Step 3: Commit** `feat(frontend): WeChat OA admin QR bind flow`

---

### Task 9: Config docs + .env.example

**Files:**
- Modify: `.env.example`
- Modify: `docs/IM集成开发文档.md` — add「微信公众号」P0 section (bind via Cloud, requires APP_EXTERNAL_URL)

- [ ] **Step 1: Add to .env.example**

```bash
# Public base URL of this TreeRAG API (no trailing slash). Required for WeChat OA Cloud relay callbacks.
# APP_EXTERNAL_URL=https://weknora.example.com
# Optional override for OA callbacks only:
# WECHAT_OA_CALLBACK_BASE_URL=
```

- [ ] **Step 2: Short doc section** — prerequisites (Cloud credentials, APP_EXTERNAL_URL), bind steps, text-only P0 limits.

- [ ] **Step 3: Commit** `docs: WeChat OA P0 config and IM guide`

---

### Task 10: End-to-end handler test (fake Cloud)

**Files:**
- Create: `internal/handler/wechat_oa_e2e_test.go` (or under `internal/im/wechat_oa/e2e_test.go`)

- [ ] **Step 1:** Spin fake Cloud + gin router with preauth + binding/complete + callback.

Script:
1. PreAuth → get state + secret
2. BindingComplete with HMAC → channel created
3. POST callback RelayEvent text → HandleMessage invoked (mock imService or spy SendReply)
4. Assert fake Cloud received SendText

Keep under ~150 lines; skip if heavy dig container — prefer unit-level service test.

- [ ] **Step 2: Commit** `test(wechat_oa): cover bind and text relay happy path`

---

## Self-review (plan vs spec)

| Spec item | Task |
|-----------|------|
| Platform split wechat_oa / wechat / wecom | T2, T7 |
| Cloud third-party preauth QR | T3, T5, T8 |
| im_channels credentials shape + bot_identity | T2, T5 |
| cloud_relay mode + full output | T2, T4 |
| HMAC Cloud→instance | T3, T4, T5 |
| Text Q&A via IM callback | T4, T6, T10 |
| Unbind → Cloud | T5 |
| APP_EXTERNAL_URL / callback base | T5, T9 |
| P1+ media/ASR/welcome | Explicitly out of scope |
| Cloud repo WeChat component | Contract only (T3); implementation external |

No TBD placeholders. `callback_secret` is in Task 1 schema and CreatePreAuth response. Types `CloudClient`, `RelayEvent`, `PlatformWeChatOA` are defined before use.

---

## Follow-up plans (do not implement in this plan)

- **P1:** image relay + media proxy + subscribe welcome + menu click  
- **P2:** voice ASR, multi-segment CS messages, 48h window UX, dead-letter visibility  
- **P3:** TTS reply, video degrade, ops stats  
- **Cloud repo:** component ticket, WeChat auth callback, binding store, instance forwarder
