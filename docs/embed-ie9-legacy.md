# IE8+ 免登录 / 网页嵌入（Legacy 壳）

## 调研结论（为何自研）

没有可直接对接 WeKnora `Authorization: Embed` + bootstrap/exchange + agent-chat SSE 的成熟开源壳：

| 候选 | 为何不直接用 |
|------|----------------|
| SMF jQuery ChatBox 等论坛聊天 | 轮询自有 PHP API，与本产品协议无关 |
| Chateedze 等客服 Widget | PHP/MySQL 独立后端，无法复用 embed session |
| jquery-sse / EventSource polyfill | 只解决传输，不解决 UI/鉴权/多会话 |

因此采用：**Vue3 现代路径不动** + **jQuery 1.12 原生壳**（`legacy-embed` / `legacy-guest`）复用现有公开 API。

## 浏览器策略

| 客户端 | 行为 |
|--------|------|
| Chrome / Edge / Firefox | `/w/*`、`/embed/*` → Vue3 `embed.html` |
| **IE8–11** `/w/*` | → `legacy-guest.html`（禁 iframe） |
| **IE8–11** `/embed/*` | → `legacy-embed.html`（可被 widget iframe） |
| IE7− | `legacy-unsupported.html` |

### IE8 相对 IE9 的体验差异（已做）

- **对话主路径**：IE8 默认走 `?stream=0` JSON 聚合（等完整回答），不依赖 SSE 渐进读
- IE9+：优先 XHR 渐进 SSE，失败再降级 JSON
- UI：对齐现网结构（顶栏 / 侧栏 / 欢迎气泡 / 建议问题卡片 / 圆角输入框 / 发送钮 / AI 免责声明）；IE8 无圆角/阴影时外观略扁，交互一致
- 上传 / 完整 MCP：仍隐藏或提示用现代浏览器

## 关键文件

- [`frontend/embed.html`](../frontend/embed.html) sniff（`< 8` 才提示）
- [`frontend/public/legacy-embed.html`](../frontend/public/legacy-embed.html) / [`legacy-guest.html`](../frontend/public/legacy-guest.html)
- [`frontend/public/legacy-embed/`](../frontend/public/legacy-embed/)（jQuery 1.12.4、json2、api/app/markdown/style）
- 后端：`stream=0` / `Accept: application/json` 聚合出口

## 验收

- [ ] IE8：`/w/{slug}` 可开会话、发问、看到完整回答（非流式）
- [ ] IE9：同上，尽量有流式增量
- [ ] IE7：仅不兼容页
- [ ] Chrome：仍为 Vue3，行为不变
- [ ] Widget 在 IE8 宿主可弹出，iframe 内为 legacy 壳
