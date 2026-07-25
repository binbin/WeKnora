# 免登录窗口与网页嵌入拆分设计

日期：2026-07-25  
状态：待实现评审

## 背景与决策

发布渠道里现有的「免登录窗口」实际是 **网页嵌入**（iframe / Widget / secure + 多会话）。产品要拆成两个独立渠道：

| 渠道 | 入口 | 数量 | 会话 |
|------|------|------|------|
| **免登录窗口**（新） | 短链 `/w/:slug` | 每智能体最多 1 条 | 多会话（侧栏 + 新建，刷新可恢复） |
| **网页嵌入**（现有改名） | iframe / Widget / secure | 可多条 | 单通 · 无历史侧栏 · 每次打开全新 |

**结论：两套独立数据模型**（`GuestLinkChannel` + `EmbedChannel`），聊天鉴权 / session sig / 限流 / agent-chat 走共享 service，避免两套聊天管道。未上线，**不做**历史数据迁移与双读兼容。

## 产品边界

### 免登录窗口

- 管理端主操作：创建短链、复制链接、打开、设置。
- 已存在时隐藏「创建」；再建返回业务错误。
- 访客页不可被 iframe 嵌（CSP `frame-ancestors 'none'`）。
- 不承载 iframe snippet / widget 位 / `allowed_origins`（短链页自有源）。

### 网页嵌入

- 管理端主操作：渠道列表、部署代码（iframe / widget / secure）、渠道密钥。
- 去掉短链作为产品入口（不再生成/展示面向用户的 `/w/:slug`）。
- 保留 `allowed_origins`、publish token、限流、外观、webhook 等嵌入配置。

## 数据模型

### `guest_link_channels`（新）

最小字段集（可从现有 Embed 配置拷贝语义）：

- `id`, `tenant_id`, `agent_id`（**同一 tenant 下 agent_id 唯一**）
- `web_slug`（全局唯一短码）
- `name`, `enabled`
- 会话相关外观/能力：`welcome_message`, `primary_color`, `page_title`, `header_title_mode`, `show_suggested_questions`, `allow_web_search`, `allow_file_upload`, `default_locale`
- 限流：`rate_limit_per_minute`, `rate_limit_per_day`
- 时间戳 / soft delete

不包含：`publish_token`（短链用 slug bootstrap）、`allowed_origins`、`widget_position`、webhook（首版可不做；若嵌入已有 webhook 需求，GuestLink 首版可省略）。

### `embed_channels`（保留并收窄）

- 继续服务网页嵌入 CRUD 与公开 exchange。
- 去掉产品层短链职责：未上线，直接从模型与 API **删除** `web_slug`（短码只存在于 `guest_link_channels`）。
- 访客策略固定为 `single_fresh`。

### 共享层

- 短期 session token（`ems_`）、session handle（`sig`）、IP/日限流、agent-chat 流式接口：抽成两边可调用的 service。
- GuestLink bootstrap 与 Embed exchange 各自签发 token，下游聊天 API 不感知渠道表差异（或仅带 source 标记便于审计）。

## API 与路由

### 管理（需登录）

**GuestLink**

- `GET/POST /api/v1/agents/:id/guest-links`
  - POST：若该 agent 已有 → `409 guest_link_exists`
- `GET/PUT/DELETE /api/v1/guest-links/:id`
- 响应包含拼好的 `web_url`，便于一键复制

**EmbedChannel**

- 现有 `/api/v1/agents/:id/embed-channels` 等路径保留
- 请求/响应不再暴露短链产品字段

### 公开

| 路径 | 行为 |
|------|------|
| `GET /w/:slug` | SPA；GuestLink 模式；`sessionMode=multi` |
| `POST /api/v1/embed/web/:slug/bootstrap` | **只**查 `guest_link_channels` |
| `/embed/:channelId` + exchange / widget | EmbedChannel；`sessionMode=single_fresh` |
| sessions / agent-chat 等 | 复用现有 embed 公开 API |

错误：GuestLink 禁用 → 与现网一致的 disabled 响应；未知 slug → 404/invalid。

## 管理端 UI

- `AgentPublishChannels`：渠道网格拆出 `guest`（免登录窗口）与 `embed`（网页嵌入）两张卡；原 `web` 文案改为网页嵌入语义。
- 新组件 `AgentGuestLinkPanel`：空态创建 / 唯一卡片（复制、打开、设置）。
- `AgentEmbedChannelPanel`：保留列表与部署抽屉；去掉 Web 直链部署入口；i18n 从「免登录窗口」改为「网页嵌入」。

## 访客体验

`EmbedPage`（或等价入口）增加 `sessionMode`：

| 模式 | 来源 | UI | 存储 |
|------|------|-----|------|
| `multi` | `/w/:slug` | 侧栏 + 新建 | 沿用 localStorage 会话列表 |
| `single_fresh` | `/embed/:id` | 无侧栏、无新建 | 不读写多会话列表；每次加载 `createSession` |

后端可暂保留多会话 API；嵌入端不调用 list/恢复即可。若后续要防滥用，再对 embed 源禁止 list。

## 迁移与兼容

**不做。** 产品未上线：

- 新表从零使用
- bootstrap 只读 GuestLink
- 开发环境脏数据手动清理
- 无双读、无回滚脚本、无 slug 备份列

## 测试范围

- GuestLink：每 agent 仅 1 条（第二次 409）；复制/打开短链；禁用后 bootstrap 失败
- `/w/:slug`：多会话侧栏、新建、刷新可恢复
- `/embed/:id`：无侧栏；刷新/重挂载为新会话
- 管理端：两卡文案与入口正确；嵌入抽屉无短链入口
- 回归：iframe/widget token exchange、限流、`allowed_origins`

## 明确不做（本设计）

- 为 GuestLink / Embed 复制两套完整 agent-chat 实现
- 历史 Embed → GuestLink 自动迁移
- 每智能体多条免登录短链
- 网页嵌入继续提供产品级短链入口

## 实现顺序建议

1. 后端：`GuestLinkChannel` 模型 + 管理 API + bootstrap 改读新表；Embed 去掉 slug 入口
2. 前端：发布双卡 + `AgentGuestLinkPanel`；嵌入面板改名与去短链
3. 访客：`sessionMode` 分流（multi vs single_fresh）
4. i18n / E2E 断言更新
