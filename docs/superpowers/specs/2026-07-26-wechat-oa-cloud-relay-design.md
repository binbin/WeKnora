# 微信公众号扫码绑定（Cloud 中转）设计

日期：2026-07-26  
状态：P0 已实现（实例侧）；Cloud 第三方平台能力依赖 Cloud 仓库

## 背景与决策

发布渠道「微信公众号接入」文案指向公众号，实际实现却是：

- `platform=wechat`：腾讯 iLink 个人微信扫码长轮询
- 发布入口还与 `wecom`（企业微信）混在同一卡片，且「创建机器人」默认 wecom

产品目标：由**公众号管理员扫码授权**，让 TreeRAG 接管该公众号的粉丝聊天互动；每个智能体可绑定不同公众号；Cloud 托管租户与自托管实例均可用。

**结论：采用「TreeRAG Cloud 统一第三方平台 + 实例 Webhook 中转」**。

| 决策 | 选择 |
|------|------|
| 微信侧能力 | 开放平台 · 第三方平台授权（非 iLink、非经典手填开发者模式主路径） |
| 凭证归属 | Cloud 持有 `component_*` 与 authorizer token；实例不存长期微信 token |
| 部署 | Cloud 与自托管都要可用；自托管经 Cloud 扫码与消息中转 |
| 平台共存 | 新增 `wechat_oa`；保留 iLink `wechat`；`wecom` 独立入口 |
| 能力目标 | 接近完整客服（文本/图片/语音/素材/事件），按 P0→P3 分期交付 |

否决方案：Cloud 全托管推理（自托管知识库对不齐）；授权后消息直推实例并下发 token（安全与路由复杂）。

## 产品边界

### 微信公众号（`wechat_oa`，新）

- 发布渠道主操作：绑定公众号（扫码）、列表、启用/停用、重新授权、解绑。
- 1 智能体可绑多个公众号渠道；1 个 `authorizer_appid` **全局唯一**对应 1 个渠道。
- 粉丝聊天经 Cloud 中转到实例 IM 管道，由绑定智能体作答。

### 个人微信（`wechat`，保留）

- 继续 iLink QR + long-poll；产品文案改为「个人微信」，不再叫公众号。

### 企业微信（`wecom`，独立）

- 从「微信公众号」卡片拆出。
- 发布渠道新增独立类型卡片「企业微信」（与飞书/钉钉同级），创建流只含 `wecom`；IM 集成页保留完整平台列表。

### 明确不做（本期）

- 模板消息营销群发
- 一公众号绑定多个智能体
- 实例直持 `authorizer_access_token`
- 用公众号入口继续承载 wecom / iLink

## 架构总览

```
前端发布渠道 (wechat_oa)
  → 实例 API (/api/v1/.../wechat_oa/...)
    → Cloud OA API (预授权 / 发消息 / 素材)
      → 微信开放平台

粉丝消息
  → 微信推送 Cloud 第三方 URL
  → Cloud 规范化 + 签名
      → 实例 /api/v1/im/callback/{channel_id}
        → wechat_oa.Adapter → IM Service / Agent QA
          → SendReply → Cloud → 微信客服消息 API
```

## 数据模型

### 实例：`im_channels`（沿用表）

- `platform = wechat_oa`
- `mode = cloud_relay`（新模式）
- `output_mode = full`（无真正流式；长文客服消息分片）
- `bot_identity = wechat_oa:{authorizer_appid}`（唯一索引防重复绑定）

`credentials`（公开资料 + 中转句柄，**不含**微信长期密钥）：

```json
{
  "authorizer_appid": "wx...",
  "nick_name": "某某公众号",
  "principal_name": "主体名",
  "head_img": "https://...",
  "service_type": 2,
  "verify_type": 0,
  "cloud_binding_id": "bnd_...",
  "instance_callback_secret": "..."
}
```

扫码成功后再落库（避免未授权草稿脏数据）。可选渠道级字段（欢迎语等）可放 `credentials` 或后续独立 JSON 配置列；P0 欢迎语可用智能体开场文案。

### Cloud（概念表）

- `oa_component_state`：`component_verify_ticket`、`component_access_token` 及过期时间。
- `oa_bindings`：`authorizer_appid` → `tenant_id` / `agent_id` / `channel_id` / `instance_base_url` / 回调 HMAC 密钥 / authorizer token 密文 / 状态（active|disabled）。

## 扫码绑定流

1. 用户在发布渠道点击「绑定公众号」。
2. 实例校验：已配置 TreeRAG Cloud 凭证 + 可达回调基址（`APP_EXTERNAL_URL` 或 `WECHAT_OA_CALLBACK_BASE_URL`）；否则禁用并提示。
3. 实例 `POST` Cloud 预授权（租户、智能体、实例回调基址、一次性 `state`，TTL 约 30 分钟）。
4. Cloud 调微信 `create_preauthcode`，返回授权二维码/链接给前端。
5. 公众号管理员扫码确认权限集（消息、客服、素材等）。
6. 微信回调 Cloud；Cloud 换 token，写 `oa_bindings`。
7. Cloud HMAC 回调实例 `POST .../im/wechat_oa/binding/complete`；实例创建 `im_channels`。
8. 前端轮询绑定状态至 `bound`，展示头像/名称。

| 规则 | 行为 |
|------|------|
| 重复 `authorizer_appid` | 拒绝，提示先解绑 |
| 重新授权 | 同渠道刷新权限/资料，不换 appid |
| 停用 | `enabled=false`：保留 binding，Cloud 不再中转（或回「服务未启用」） |
| 解绑 | 删除实例渠道 → Cloud 将 binding 标为 disabled，并尽量调微信解除授权 |
| 预授权过期 / 用户取消 | 前端可刷新二维码重试 |
| Cloud→实例回调失败 | Cloud 重试；前端提供「同步状态」 |
| Cloud 托管租户 | `instance_base_url` 指向该租户所在 Cloud 应用入口（内网或同集群可达） |

## 消息收发与客服能力

### 入站

1. 微信推送到 Cloud；Cloud **5 秒内**回 `success`，再异步中转实例。
2. Cloud 按 `authorizer_appid` 查 binding，构造 `RelayEvent`，HMAC 签名后  
   `POST {instance}/api/v1/im/callback/{channel_id}`。
3. `wechat_oa.Adapter` 转为 `im.IncomingMessage`，进入现有 IM 会话映射 / 限流 / Agent QA。
4. 幂等键：微信 `MsgId` 或 Cloud `relay_event_id`，防止重试双答。

### 出站

- 主路径：微信**客服消息 API**（经 Cloud；Cloud 负责 token 刷新）。
- 超长文本按微信限制切片为多条。
- **48 小时会话窗口**：窗外第一期不回或回固定提示，不做模板营销。
- 实例 `SendReply` 只调 Cloud，不直连微信。

### 能力矩阵

| 粉丝输入 | 处理 | 回复 |
|----------|------|------|
| 文本 | Agent QA | 文本（可多条） |
| 图片 | 经 Cloud 拉素材；可选视觉/附件策略 | 文本；可选回图 |
| 语音 | 拉取 → ASR 转写 → 当文本 | 文本；（TTS 语音回复属 P3） |
| 视频/文件 | 元数据 + 暂不支持提示，或降级描述 | 文本 |
| 位置/链接 | 抽成文本上下文 | 文本 |
| 关注 | 渠道/智能体欢迎语 | 文本 |
| 菜单点击等事件 | 映射文本或固定回复 | 文本 |

素材上下载一律 Cloud 代理。

### 错误处理

| 场景 | 行为 |
|------|------|
| 渠道停用 / 已解绑 | Cloud 丢弃或固定「服务未启用」 |
| 实例不可达 | 有限重试 + 死信；可对用户「稍后再试」 |
| 超 48h | 不调客服 API；记日志 |
| Agent 失败 | 客服消息发友好错误文案 |
| 媒体失败 | 当次提示，不阻断渠道 |

## 模块与 API（实例）

新包 `internal/im/wechat_oa/`：`Factory`（`cloud_relay`，无长连接）、`Adapter`（ParseRelay + SendReply）、Cloud client。

建议管理 API（需登录 / 渠道管理能力）：

- `POST /api/v1/agents/:id/wechat-oa/preauth` — 申请扫码
- `GET /api/v1/wechat-oa/preauth/:id` — 轮询状态
- `POST /api/v1/im/wechat_oa/binding/complete` — Cloud 回调完成绑定（HMAC）
- `POST /api/v1/im/callback/:channel_id` — Cloud 消息中转（HMAC；与其它 IM 共用路径）
- 现有 IM 渠道 CRUD / toggle / delete 扩展支持 `wechat_oa`

Cloud 侧（概念）：第三方回调 URL、预授权、发消息、素材代理、binding CRUD；细节在 Cloud 仓库实现，本仓库以 client 契约为准。

## 前端

- `AgentPublishChannels`：
  - 「微信公众号」卡片只滤 `wechat_oa`，主操作「绑定公众号」走扫码（不再默认 wecom）。
  - 新增「企业微信」卡片只滤 `wecom`。
  - 新增「个人微信」卡片只滤 iLink `wechat`（保留发现性；与公众号文案分离）。
- 绑定弹层：二维码、状态、过期刷新。
- 已绑定行：头像、昵称、主体、开关、重新授权、解绑；设置项含欢迎语、语音转写开关等（随分期露出）。
- i18n：公众号 / 个人微信 / 企业微信文案拆分（zh/en/ko/ru）。

## 配置

| 位置 | 项 | 用途 |
|------|-----|------|
| 实例 | 已有 TreeRAG Cloud 凭证 | 调 Cloud OA API |
| 实例 | `PUBLIC_BASE_URL` 或 `WECHAT_OA_CALLBACK_BASE_URL` | Cloud 回实例基址 |
| Cloud | `component_appid` / secret / Token / AESKey | 第三方平台 |
| Cloud | 权限集、重试与死信策略 | 运维 |

## 分期交付

| 阶段 | 范围 | 验收 |
|------|------|------|
| **P0** | 平台拆分；Cloud 预授权扫码绑定；文本问答往返；解绑 | 扫码绑定后粉丝发文字能收到智能体回复 |
| **P1** | 图片入站、素材代理、关注欢迎语、菜单点击 | 图文与基础事件可用 |
| **P2** | 语音 ASR、多段客服消息、48h 提示、失败重试可观测 | 接近完整客服 |
| **P3** | TTS 语音回复、视频友好降级、运营统计 | 体验打磨 |

P0 即可替换发布渠道「公众号」主路径；P1–P2 对齐完整客服目标。

## 测试要点

- 绑定：预授权、回调、重复 appid、解绑、无公网/无 Cloud 凭证时的禁用提示
- 中转：验签、幂等、实例宕机重试
- 消息：文本往返；P1+ 图片/语音/关注
- 回归：`wechat`（iLink）与 `wecom` 行为不变；发布渠道入口不再混用

## 开放依赖（非阻塞设计，实现前需对齐）

- Cloud 仓库需落地第三方平台资质与 OA 中转服务；本仓库先定 client 契约与实例侧 IM/前端。
- ASR/TTS 复用实例已有模型能力或 Cloud 侧能力，P2/P3 选定具体提供方。
