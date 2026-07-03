# NodeSeek-Style Forum Basics

## Goal

基于 NodeSeek Daily 板块的信息架构，分步骤实现一个可落地的基础论坛：用户可以浏览分类帖子、查看详情、登录后发帖和回帖，并逐步加入搜索、排序、通知和基础互动。登录注册与社交登录复用现有后台 API，不在本任务重复建设。

## What I Already Know

- 用户指定参考页面：`https://www.nodeseek.com/categories/daily`。
- 用户明确说明登录注册和社交集成后台 API 已存在。
- 当前后台已有 auth、OAuth、current user、JWT middleware、notification、realtime、points、page view、db migration 等能力。
- 当前旧 forum demo 的后台包已被删除或处于漂移状态，后续实现应使用新的 `routes -> usecase -> models` 架构。
- NodeSeek Daily 可作为 category/thread 列表参考：分页、搜索、按新评论/新帖子排序、信息密集列表是核心体验。

## Research References

- [`research/nodeseek-daily-observations.md`](research/nodeseek-daily-observations.md) - 记录 NodeSeek Daily 页面访问限制、可确认特征和产品启发。
- [`research/repo-api-capabilities.md`](research/repo-api-capabilities.md) - 记录本地现有 API 能力和论坛实现应复用的后台基础设施。

## Requirements

- 论坛内容公开可读；创建帖子、回复、互动等写操作要求登录。
- 默认提供 `daily` 分类，并保留多分类扩展模型。
- 帖子列表支持分页、排序和搜索：
  - `latest_reply`
  - `latest_post`
  - 后续可扩展 `hot`
- 帖子详情展示主帖、作者、发布时间、浏览数、回复数、最近回复、回复列表。
- 登录用户可以创建帖子、发布回复。
- 作者可以编辑/删除自己的帖子或回复；管理员可以管理所有内容。
- 回复主帖作者时产生通知，优先复用现有 notification/realtime。
- 浏览数可以复用或参考现有 page view 能力，至少保证不会因列表渲染重复计数。
- UI 使用当前项目已有 htmx/hyperscript/Pico/embedded template 思路，不引入新的前端构建栈。
- 后台遵守 `.trellis/spec/backend/` 中的分层、错误、数据库、日志和质量约定。

## Implementation Plan

### Step 1: Forum Core Backend

- 新增论坛迁移：
  - `forum_categories`
  - `forum_threads`
  - `forum_posts`
  - 必要索引：分类、作者、最近活动、发布时间、删除状态。
- 新增 `api/models`、`api/usecase`、`api/routes` 论坛模块。
- API 覆盖分类列表、帖子列表、帖子详情、创建帖子、回复帖子。
- 写操作复用现有登录态和 usecase context。

### Step 2: Read-First Forum UI

- 替换旧 demo 首页为论坛首页。
- 提供 Daily 分类导航、帖子列表、分页、排序切换和搜索框。
- 提供帖子详情页或详情区域，展示主帖与回复。
- 保持信息密集、可扫描的论坛布局，不做营销落地页。

### Step 3: Authenticated Posting

- 接入现有 auth 状态，未登录用户看到登录引导但仍可阅读。
- 已登录用户可以发帖、回帖。
- 表单错误用现有 response envelope 或 htmx-friendly fragment 显示。
- 作者操作支持编辑/删除自己的内容；管理员具备全量管理入口。

### Step 4: Interaction And Notification

- 回复帖子时通知主帖作者。
- 可选加入轻量互动：点赞/鸡腿/收藏，优先复用 points 或 notification 基础设施。
- 增加浏览数统计和最近活动更新。

### Step 5: Moderation And Polish

- 增加置顶、锁帖、隐藏/删除、基础举报或管理员审核能力。
- 补充空状态、加载态、移动端布局和可访问性细节。
- 优化搜索、分页和 hot 排序性能。

## Acceptance Criteria

- [ ] 数据库迁移可创建论坛分类、帖子、回复数据表，并包含必要索引。
- [ ] `GET /api/forum/categories` 返回分类列表，默认包含 `daily`。
- [ ] `GET /api/forum/categories/:slug/threads` 支持分页、搜索和 `latest_reply/latest_post` 排序。
- [ ] `GET /api/forum/threads/:id` 返回主帖、作者摘要、回复列表、回复数、浏览数和时间字段。
- [ ] 登录用户可以通过 API 创建帖子和回复；未登录写操作返回 `401`。
- [ ] 帖子作者可以编辑/删除自己的内容；管理员可以管理所有内容。
- [ ] 回复成功后能触发主帖作者通知，且不会通知回复者自己。
- [ ] 前端首页能浏览 Daily 列表、打开帖子详情、登录后发帖回帖。
- [ ] UI 不依赖 Node/前端构建链，默认模板仍可随 exe 打包。
- [ ] 后端新增代码通过 gofmt，并覆盖 model/usecase/route 的核心测试。

## Definition Of Done

- 新增或更新迁移、models、usecase、routes、templates 和测试。
- `go test ./...` 能在当前模块路径修复后通过；如果实现时模块路径仍漂移，交付说明要记录阻塞。
- 新增 API 遵守内部 response envelope 和 usecase error mapping。
- 新增 UI 在桌面和移动端基本可用，没有主要内容重叠。
- 不重做登录注册、OAuth、积分或通知基础能力，只做论坛侧接入。

## Out Of Scope

- 重写现有 auth/register/login/OAuth。
- 引入新的前端框架或构建链。
- 完整复刻 NodeSeek 的所有视觉细节和全部社区规则。
- 高级内容审核、反垃圾系统、全文搜索引擎、私信系统。
- 支付、会员、商品、订单等非论坛基础功能。

## Technical Approach

推荐采用“持久化论坛核心 + htmx 渐进增强”的路线：

- 后台新建 forum 模块，遵守当前 `routes -> usecase -> models` 分层。
- 数据库使用 app DB 迁移，写操作在 usecase 中开启事务。
- 登录态通过现有 JWT middleware 获取当前用户。
- 通知通过现有 notification/realtime 扩展，不直接在 forum 里实现第二套实时通道。
- UI 先完成可读和可写闭环，再增加互动和管理能力。

## Decision ADR Lite

Context: 旧 forum demo 已不符合当前后台架构，且用户明确要求复用现有登录注册和社交集成。

Decision: 新论坛以当前后台分层和数据库能力重建，不恢复旧 `api/forum` 内存 store；论坛任务按 Step 1 到 Step 5 分阶段落地。

Consequences: 初期实现会比恢复旧 demo 多一些迁移和用例层工作，但后续能自然接入认证、通知、积分、浏览统计和管理后台。

## Open Questions

- MVP 是否必须包含点赞/鸡腿/收藏，还是先只做发帖、回帖、搜索、排序和通知？
- UI 是否需要严格复刻 NodeSeek 的视觉密度，还是仅复用其信息架构和交互优先级？

## Technical Notes

- NodeSeek 页面直接访问在当前环境不可读，PRD 只采纳可确认的信息架构特征；实现 UI 前最好补充人工截图或浏览器确认。
- 当前 `.trellis/spec/backend/` 已更新为新后台架构约定，但这些 spec 更新尚未提交。
- 当前工作区存在大量 `api/` 未提交改动，本任务只创建需求与计划，不修改代码。
