---
title: 发布渠道 API 访问
tags: [API参考, OpenAPI, 发布渠道, chat/completions]
aliases: [发布渠道API, OpenAPI对话, wkpub]
source: api/openapi-chat.md
---

# 发布渠道 API 访问

通过智能体 **发布渠道 → API** 创建的 `wkpub_` 密钥，调用 OpenAI 兼容的
`POST /api/v1/chat/completions`。

完整字段、curl 示例与错误码见：

- 仓库文档：[openapi-chat.md](../../api/openapi-chat.md)
- 控制台说明页：`/platform/docs/openapi-chat`

## 要点

| 项 | 说明 |
|----|------|
| 创建入口 | 智能体 → 发布渠道 → API |
| 鉴权 | `Authorization: Bearer wkpub_…` |
| 路径 | `{origin}/api/v1/chat/completions` |
| 绑定 | 密钥绑定单个智能体；空间 `X-API-Key` 不能调用本路由 |

## 相关主题

- [API文档概览](./API文档概览.md) — REST API 总索引
- [openapi-chat.md](../../api/openapi-chat.md) — 完整 OpenAPI 对话说明

---

## 反向链接

- [Home](../Home.md)
- [API文档概览](./API文档概览.md)
