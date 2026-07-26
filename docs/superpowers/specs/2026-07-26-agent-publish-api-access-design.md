# 发布渠道 API 访问（独立凭证 + OpenAI 兼容）设计

日期：2026-07-26  
状态：待实现评审  
参考：[FastGPT OpenAPI 介绍](https://doc.fastgpt.io/zh-CN/openapi/intro)、[通过 API 访问应用](https://doc.fastgpt.io/zh-CN/guide/build/publish/openapi)

## 背景与决策

发布渠道已有「API」页签，但当前实现直接复用空间级 `tenant_api_keys`（`listTenantAPIKeys` / `createTenantAPIKey`），文档链到 GitHub 首页，且对话入口仍是内部 `agent-chat` + `X-API-Key`，难以对接 OpenAI SDK / 第三方客户端。

产品目标：对齐 FastGPT「应用 → 发布渠道 → API 访问」体验，并提供 OpenAI 风格 `chat/completions` 调用路径。

| 决策 | 选择 |
|------|------|
| 范围 | 发布渠道体验 + OpenAI 兼容对话（A+B） |
| 实现路径 | **独立发布渠道凭证表**，与空间 API Key 分离 |
| 鉴权 | 发布渠道 OpenAPI **仅** `Authorization: Bearer` |
| 绑定 | 密钥绑定当前智能体；空间 key 仍走「API 集成」页 |
| 协议面 | `POST /api/v1/chat/completions` + OpenAI 常用字段（`messages`/`stream`/`model`） |

否决方案：扩展 `tenant_api_keys.agent_ids` 复用空间密钥（边界混杂）；仅改 UI 不提供真正 `/chat/completions`（SDK 接不上）。

## 产品边界

### 发布渠道 API Key（新）

- 管理入口：智能体编辑 → 发布渠道 → API
- 作用域：**仅当前 agent + 对话**
- 鉴权：`Authorization: Bearer wkpub_...`
- 可调用路由：**仅** OpenAPI 兼容对话层

### 空间 / 平台 API Key（既有，不变）

- 管理入口：API 集成 / 平台密钥
- 继续使用既有 `X-API-Key` 与能力模型
- **不能**调用发布渠道 OpenAPI 路由
- 发布渠道 key **不能**当作空间 `X-API-Key` 使用

### 明确不做（P0）

- `apiKey-agentId` 拼接格式（key 已绑定 agent）
- 额度 / 配额 UI（可预留 `expires_at`）
- 发布渠道 key 访问知识库管理 / 上传等非对话接口
- 把全站鉴权改成只认 Bearer
- 会话列表等额外管理类 OpenAPI

## 架构总览

```
[发布渠道 UI]
    │ CRUD / 明文仅创建时展示
    ▼
[Agent Publish API Key Service]  ← 新模块
    │ 鉴权: Authorization: Bearer wkpub_xxx
    ▼
[OpenAPI Compat Handler]
    POST /api/v1/chat/completions
    │ 映射 messages/stream → 内部 session + agent-chat
    ▼
[现有 Session / Agent QA 流水线]
```

## 数据模型

新表 `agent_publish_api_keys`（versioned + sqlite 双轨 migration）：

| 字段 | 说明 |
|------|------|
| `id` | 主键 |
| `tenant_id` | 所属空间 |
| `agent_id` | 绑定智能体（创建后不可改） |
| `name` | 展示名 |
| `key_prefix` | 可见前缀，便于列表识别 |
| `key_hash` | 哈希，鉴权查找 |
| `api_key_enc` | 可选加密存明文（遵循现有 `SYSTEM_AES_KEY` 惯例）；列表默认脱敏 |
| `last_used_at` | 最近使用 |
| `expires_at` | 可选过期 |
| `revoked_at` | 吊销时间 |
| `created_by` | 创建者用户 ID |
| `created_at` / `updated_at` | 时间戳 |

密钥格式：`wkpub_` + 随机串，避免与空间 key 混淆。

**无数据迁移**：现有发布渠道展示的空间 key 不自动转换；UI 切换后用户需在发布渠道新建。

## OpenAPI 协议与鉴权

### 鉴权

- 请求头：`Authorization: Bearer <wkpub_...>`
- 缺少 / 格式错误 / 不存在 / 已吊销 / 过期 → `401`
- agent 已删或不可用 → `403`
- 发布渠道 key 与空间 key 互不通用

### 核心接口

`POST /api/v1/chat/completions`

| 字段 | 必填 | 行为 |
|------|------|------|
| `messages` | 是 | OpenAI 风格；取末条 `user` 作为本轮 query |
| `stream` | 否 | `true` 时 SSE，事件形状贴近 OpenAI chunk |
| `model` | 否 | 忽略或仅回显；真实模型以绑定 agent 配置为准 |
| `session_id` | 否 | 续聊主键；有则复用，无则新建并在响应返回 |
| `chat_id` | 否 | `session_id` 的别名（FastGPT/习惯兼容）；二者都传时以 `session_id` 为准 |

`agent_id` **不从 body 指定**（key 已绑定）。若第三方传入且与绑定不一致 → `403`。

### 会话策略

- 无 `session_id`：创建绑定该 agent 的 session，`channel=api`，并写入 `publish_api_key_id`（或等价元数据）归属该 key
- 有 `session_id`：校验归属同一 `tenant_id` + `agent_id` + **同一 `publish_api_key_id`**
- **P0：以服务端 session 历史为准，本轮只取最后一条 user message**（避免双源冲突）

### 响应

- 非流式：OpenAI `chat.completion` 核心字段（`id`/`object`/`choices`/`usage`）
- 流式：`chat.completion.chunk` + 结束 `data: [DONE]`
- 扩展字段（文档标明）：`session_id`，便于续聊

### 错误形态

```json
{
  "error": {
    "message": "...",
    "type": "invalid_request_error",
    "code": "..."
  }
}
```

| 场景 | HTTP | code |
|------|------|------|
| 无/坏 Bearer、key 不存在 | 401 | `unauthorized` |
| 已吊销 / 过期 | 401 | `key_revoked` / `key_expired` |
| agent 不可用 | 403 | `agent_unavailable` |
| session 不属于该 key/agent | 403 | `session_forbidden` |
| messages 空 / 无 user 轮 | 400 | `invalid_request_error` |
| 上游/内部失败 | 500 / 502 | `server_error` / `upstream_error` |

日志结构化：记录 `tenant_id`、`agent_id`、`key_id`（不记明文）、`session_id`、错误码。

## 管理 API 与发布渠道 UI

### 管理 API（登录 + 发布渠道管理权限）

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/agents/:agent_id/publish-api-keys` | 列表（脱敏） |
| `POST` | `/api/v1/agents/:agent_id/publish-api-keys` | 创建；响应一次性返回 `plaintext` |
| `DELETE` | `/api/v1/agents/:agent_id/publish-api-keys/:id` | 吊销/删除 |

P0 不包含改名/改过期（`PATCH`）；需要时吊销后重建。创建请求可带可选 `expires_at`。

创建请求：`{ "name": "...", "expires_at": null }`  
创建响应额外字段：`plaintext`（仅本次）。

权限：管理侧与现有 `canManagePublishChannels` 一致；调用侧持有效 `wkpub_` key 即可，不走用户 JWT。

### 发布渠道 UI

替换现有「复用空间 Tenant API Key」逻辑：

1. API 根地址（可复制）+ 旁注「根地址不是接口本身」
2. 鉴权说明：`Authorization: Bearer <key>`
3. curl 示例（非流式 + 流式），路径 `/api/v1/chat/completions`
4. 文档入口指向项目内 OpenAPI 对话文档（不再链 GitHub 首页）
5. 密钥表：名称 / 前缀脱敏 / 创建时间 / 最近使用 / 吊销
6. 新建成功弹窗：明文 + 一键复制 +「关闭后无法再看」

空间级 key 管理保持原样；文案区分：空间 key = 管理/集成，发布渠道 key = 仅本智能体 OpenAPI 对话。

## 文档

- `docs/api/` 增加 OpenAPI 对话兼容说明（鉴权、字段、示例）
- 发布渠道「查看文档」指向该页
- `docs/api/README.md` 增加索引条目

## 测试（P0）

1. **单元**：key 哈希鉴权、过期/吊销、agent 绑定校验、messages→query 映射
2. **Handler**：Bearer 成功/失败、流式与非流式形状、session 续聊隔离
3. **前端 e2e（轻量）**：API 页可见根地址、可创建 key、明文弹窗；不再混入空间 key 列表

## 模块职责（实现时）

| 模块 | 职责 |
|------|------|
| `types` + migration | `AgentPublishAPIKey` 模型与表 |
| repository / service | 创建、列表、鉴权、吊销 |
| middleware / handler | Bearer 鉴权；`/chat/completions` 适配 |
| router | 管理路由挂 agent；兼容路由公开（仅 Bearer） |
| frontend `AgentPublishChannels` | API 渠道改接新 API |
| docs | OpenAPI 使用说明 |

每个模块单一职责；兼容层只做协议映射，不复制 Agent QA 业务逻辑。
