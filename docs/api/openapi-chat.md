# OpenAPI 兼容对话（发布渠道 API Key）

[返回目录](./README.md)

面向第三方 SDK / 脚本的 OpenAI 风格对话接口。密钥绑定单个智能体，仅可调用本页所述路由。

| 方法 | 路径 | 描述 |
| ---- | ---- | ---- |
| POST | `/chat/completions` | OpenAI 兼容聊天补全（非流式 / SSE 流式） |

## 创建密钥

在 Web 控制台打开目标智能体 → **发布渠道** → **API**，创建发布渠道 API Key。

- 明文仅在创建成功时展示一次，请立即复制保存
- 密钥前缀为 `wkpub_`，与空间级 `X-API-Key` 不是同一套体系

## BaseURL 与 Endpoint

| 概念 | 值 | 说明 |
|------|-----|------|
| BaseURL（API 根地址） | `{origin}/api/v1` | 例如 `https://your-host/api/v1` |
| Endpoint | `/chat/completions` | 相对 BaseURL 拼接 |

完整路径：

```text
POST {origin}/api/v1/chat/completions
```

根地址不是接口本身，使用 OpenAI SDK 时将 `base_url`（或等价配置）设为
`{origin}/api/v1`，请求路径使用 `/chat/completions`。

## 鉴权

仅接受发布渠道密钥：

```http
Authorization: Bearer wkpub_...
```

**空间 API Key（`X-API-Key`）不能调用本路由。** 本接口走独立的
`PublishAPIKeyAuth`，不会把空间密钥当作发布渠道密钥校验。

## 请求字段

| 字段 | 必填 | 说明 |
|------|------|------|
| `messages` | 是 | OpenAI 风格消息列表；本轮 query 取**最后一条** `user` |
| `stream` | 否 | `true` 时返回 SSE；默认非流式 JSON |
| `model` | 否 | 可忽略或仅回显；实际模型以绑定智能体配置为准 |
| `session_id` | 否 | 续聊主键；省略则新建会话，并在响应中返回 |
| `chat_id` | 否 | `session_id` 的别名；二者都传时以 `session_id` 为准 |
| `agent_id` | 否 | 密钥已绑定智能体；若传入且与绑定不一致 → `403` |

## 非流式示例

```bash
curl -X POST 'http://localhost:8080/api/v1/chat/completions' \
  -H 'Authorization: Bearer wkpub_YOUR_KEY' \
  -H 'Content-Type: application/json' \
  -d '{
    "messages": [
      {"role": "user", "content": "你好，请介绍一下你自己"}
    ]
  }'
```

响应示例：

```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "model": "My Agent",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 0,
    "completion_tokens": 0,
    "total_tokens": 0
  },
  "session_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
}
```

`session_id` 为 WeKnora 扩展字段，用于续聊。

## 流式示例

```bash
curl -N -X POST 'http://localhost:8080/api/v1/chat/completions' \
  -H 'Authorization: Bearer wkpub_YOUR_KEY' \
  -H 'Content-Type: application/json' \
  -d '{
    "stream": true,
    "session_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    "messages": [
      {"role": "user", "content": "继续上一个话题"}
    ]
  }'
```

响应为 `text/event-stream`，形状贴近 OpenAI `chat.completion.chunk`：

```text
data: {"id":"chatcmpl-...","object":"chat.completion.chunk","model":"...","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}],"session_id":"..."}

data: {"id":"chatcmpl-...","object":"chat.completion.chunk","model":"...","choices":[{"index":0,"delta":{"content":"你好"},"finish_reason":null}],"session_id":"..."}

data: {"id":"chatcmpl-...","object":"chat.completion.chunk","model":"...","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"session_id":"..."}

data: [DONE]
```

每个 chunk 都带扩展字段 `session_id`。

## session_id / chat_id

- 不传：服务端创建新会话，并在响应（或每个 SSE chunk）中返回 `session_id`
- 传入已有 ID：校验归属同一空间、同一发布密钥、同一智能体后复用
- `chat_id` 与 `session_id` 等价；优先使用 `session_id`
- 会话历史以服务端为准；本轮只取 `messages` 中最后一条 user 内容

## 错误形态

与 OpenAI 风格对齐：

```json
{
  "error": {
    "message": "...",
    "type": "invalid_request_error",
    "code": "..."
  }
}
```

| 场景 | HTTP | type | code |
|------|------|------|------|
| 缺少 / 无效 Bearer、密钥不存在 | 401 | `authentication_error` | `unauthorized` |
| 智能体不可用 / body `agent_id` 不匹配 | 403 | `permission_error` | `agent_unavailable` |
| 会话不属于该密钥 / 智能体 | 403 | `permission_error` | `session_forbidden` |
| `messages` 为空或无 user 轮 | 400 | `invalid_request_error` | `invalid_request` |
| 内部失败 | 500 | `server_error` | `server_error` |

流式请求在已写出 SSE 头之后若失败，可能先推送一条 JSON `error` 事件，再以
`data: [DONE]` 结束。

## 与空间 API 的区别

| | 发布渠道 API Key | 空间 API Key |
|--|------------------|--------------|
| 创建位置 | 智能体 → 发布渠道 → API | 账户 / 空间集成设置 |
| 请求头 | `Authorization: Bearer wkpub_…` | `X-API-Key: …` |
| 能力范围 | 仅本智能体的 `/chat/completions` | 知识库、会话等管理与集成 API |
| 互用 | 不可用空间 key 调本路由 | 不可用 `wkpub_` 调空间路由 |
