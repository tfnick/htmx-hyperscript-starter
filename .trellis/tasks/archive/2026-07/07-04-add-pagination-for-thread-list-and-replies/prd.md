# Add Pagination for Thread List and Replies

## Goal

为论坛的两个主要浏览页面补齐分页能力：帖子列表页支持稳定的列表分页，帖子详情页的回复区支持独立分页。分页需要复用现有后端分页契约和前端 URL / 状态管理模式，让用户可以在列表与详情回复之间稳定浏览、刷新和分享当前页。

## What I Already Know

* 用户明确要创建新任务。
* 需要处理两个页面：帖子列表、帖子详情的回复。
* 本任务先不考虑在列表中插入推广内容的能力。
* 当前帖子列表 API 已有 `page` / `page_size` 查询参数，返回 `pagination` 元数据。
* 当前首页前端已有 `forumState.page`、`forumState.pageSize`、`prev-page`、`next-page`、`page-summary` 和 `buildPageSummary()`。
* 当前帖子详情 URL 已有 `/post-{id}-{page}` 形式，并且前端能从路径解析 `selectedPostPage`。
* 当前 usecase `GetForumThreadDetail` 已有 `PostPage` / `PostSize` 字段，并对回复调用 `ListForumPosts` 的 `LIMIT/OFFSET`。
* 当前 route `GetForumThread` 没有读取请求分页参数并传给 `GetForumThreadDetail`。
* 当前 `ForumThreadDetailResponse` 只返回 `posts`，没有返回回复分页元数据。
* 当前前端 `loadThread()` 会根据 URL 页码改变路径，但请求 `/api/forum/threads/:id` 时没有带 `page` / `page_size`，详情页也没有回复分页控件。

## Assumptions

* 帖子列表分页应继续使用现有 `page` / `page_size` 查询参数和 `PaginationResponse`。
* 回复分页也使用同一套 `page` / `page_size` 参数，避免新增 `post_page` / `post_size` 这类第二套 API 语义。
* `/post-{id}-{page}` 中的 page 表示回复页码，而不是帖子列表页码。
* 发布新回复后，默认跳转或刷新到能看到最新回复的回复页；具体行为可在实现时基于返回的 `reply_count` 和 `page_size` 计算。
* 不改变数据库结构，不改变帖子和回复的排序语义。

## Open Questions

* None.

## Requirements

* 帖子列表页分页能力要保持并补齐验收：请求必须传 `page` / `page_size`，响应使用 `pagination` 元数据驱动上一页、下一页和当前页摘要。
* 帖子列表页的列表上方和下方都必须出现分页操作区，两处分页状态和可用性保持同步。
* 帖子列表切换分类、搜索、排序时分页重置到第 1 页。
* 帖子详情回复区必须支持独立分页：详情 API 读取 `page` / `page_size`，只返回当前回复页。
* 帖子详情响应必须返回回复分页元数据，供前端判断上一页、下一页、总页数和当前页。
* 详情页 URL `/post-{threadID}-{page}` 必须与当前回复页保持同步，刷新或直接打开该 URL 应加载对应回复页。
* 回复分页控件采用页码按钮交互，页码按钮应显示当前页、可跳转页，并在页数较多时避免把全部页码铺满移动端。
* 回复分页操作不应影响帖子列表当前页状态。
* 进入帖子详情时默认打开回复第 1 页。
* 回复列表为空、只有一页、多页、非法页码和最后一页都要有可理解的 UI 状态。
* 列表和回复分页都要保持后端可见性与权限语义，不因为分页绕过私有帖过滤。
* 本任务不设计、不预留列表中插入推广内容的分页位能力。

## Acceptance Criteria

* [ ] 首页帖子列表点击上一页/下一页会请求正确的 `page` / `page_size`，并按 API 的 `pagination.has_previous` / `pagination.has_next` 禁用按钮。
* [ ] 首页帖子列表上方和下方都显示分页操作区，任一处操作都会更新同一页数据，并同步另一处分页状态。
* [ ] 分类、搜索、排序变更后，帖子列表回到第 1 页。
* [ ] `/api/forum/threads/:id?page=2&page_size=10` 返回第 2 页回复和对应 `pagination` 元数据。
* [ ] `/post-{id}-2` 直接打开或刷新后，详情页加载第 2 页回复。
* [ ] 回复分页控件能在详情页通过页码按钮跳转指定回复页，并同步浏览器 URL。
* [ ] 回复页没有上一页或下一页时，对应按钮禁用。
* [ ] 发布新回复后，用户能看到更新后的回复列表，并且分页状态合理。
* [ ] 后端 usecase / route 测试覆盖列表分页和回复分页的关键路径。
* [ ] `node --check public/extensions.js` 通过。
* [ ] `go test ./...` 或相关后端测试通过。

## Definition of Done

* Tests added/updated for backend pagination behavior.
* Frontend JS remains null-safe across index, post, new-post, login, and register pages.
* Existing route/static/template behavior remains compatible.
* No promotional insertion capability is implemented in this task.
* Spec update considered if a new reusable forum pagination contract emerges.

## Technical Approach

后端复用现有分页基础设施：route 层用 `fwrequest.PageQuery(c)` 读取 `page` / `page_size`，usecase 层继续用 `fwusecase.NormalizePageQuery` 和 `fwusecase.NewPageResult`。列表页基本沿用现有 `ForumThreadsCo.Pagination`，详情页需要在 `ForumThreadDetailCo` 或等价响应结构中加入回复分页结果。

前端复用现有 `forumState.selectedPostPage`、`postRouteFromPath()`、`postPath()` 和 `buildPageSummary()`。帖子列表分页控件需要从单个顶部 pager 扩展为上下两个 pager，使用共享 class / data attribute 绑定事件和同步状态，避免复制两套互相漂移的逻辑。`loadThread()` 应将当前回复页带到详情 API 请求中，`renderThreadDetail()` 应渲染回复页码按钮并绑定点击事件，点击后调用 `loadThread(thread.id, { postPage })`，同时保持 URL 同步。

## Decision (ADR-lite)

**Context**: 项目已经存在通用分页请求和响应结构，帖子列表也已经部分使用。回复分页目前后端 usecase 有基础，但 route、response 和前端控件未串起来。

**Decision**: 使用同一套 `page` / `page_size` 查询参数和 `pagination` 响应字段为帖子列表与详情回复分页建模；详情页 URL 的尾部页码代表回复页；回复分页 UI 使用页码按钮。

**Consequences**: 实现保持一致，前端可以复用现有分页摘要和状态逻辑。页码按钮比简单上一页/下一页更清晰，但需要处理页数较多时的收缩展示和移动端换行。未来如果要在列表中插入推广内容，需要另起任务重新评估列表分页计数、位置占位和曝光统计，本任务不处理。

## Out of Scope

* 不实现帖子列表中插入推广内容、广告内容、推荐内容或占位内容。
* 不改变帖子排序、回复排序、可见性规则或权限规则。
* 不新增无限滚动。
* 不新增每页数量选择器、输入框跳页或游标分页。
* 不改数据库结构。
* 不做前端整体样式重构。

## Technical Notes

* Task directory: `.trellis/tasks/07-04-add-pagination-for-thread-list-and-replies/`
* Likely files:
  * `api/usecase/forum.go`
  * `api/routes/forum.go`
  * `api/routes/forum_test.go`
  * `api/usecase/forum_test.go`
  * `public/extensions.js`
  * `public/index.html`
  * `public/post.html`
  * `public/styles.css`
* Existing backend pagination helpers:
  * `api/framework/http/request/pagination.go`
  * `api/framework/usecase/pagination.go`
  * `api/routes/order.go` `ToPaginationResponse`
* Existing frontend pagination helpers:
  * `forumState.page`
  * `forumState.selectedPostPage`
  * `postRouteFromPath()`
  * `postPath()`
  * `buildPageSummary()`
