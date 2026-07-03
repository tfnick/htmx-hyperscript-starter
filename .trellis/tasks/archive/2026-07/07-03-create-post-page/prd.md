# Create Post Page

## Goal

在登录后的“欢迎回来”区域增加一个明确的“发帖”入口。用户点击后进入专门的发帖页面，在该页面填写标题、正文，选择发布板块，并选择帖子可见性：公开或私有，默认公开。发帖成功后自然进入新帖子详情页。

## What I Already Know

* 用户明确要求创建新任务，而不是立即内联修改。
* 入口位置是登录后的 `#logged-in-panel` 区域，也就是当前展示“欢迎回来”和退出登录按钮的位置。
* 当前首页已有内嵌 `#thread-form` 发帖表单，位于 `public/index.html` 的 feed 上方；它提交到 `POST /api/forum/threads`。
* 当前帖子详情页 `public/post.html` 也有同样的登录态欢迎区域，但没有发帖入口。
* 当前前端会通过 `GET /api/forum/categories` 加载板块分类。
* 当前创建主题 API 请求体只支持 `category_slug`、`title`、`body`。
* 当前 `forum_threads` 表没有公开/私有字段；只有 `status`、`is_pinned`、`is_locked` 等字段。
* 当前前端路由在 `index.go` 注册了 `/`、`/categories/:slug`、`/post-*`、`/login`、`/register`，尚无专门的发帖页路由。
* 当前项目没有引入 Tailwind/daisyUI，页面使用 `public/styles.css` 和原生 JS。
* 用户补充要求：每个板块（分类）下的帖子列表页面上部不再需要发帖功能，需要移除。
* 用户补充要求：如果用户从某个板块下点击发帖进入发帖页面，发帖页默认选中该板块。
* 用户已确认：私有帖子仅作者本人和管理员可见，且不出现在公共列表和分类列表中。

## Assumptions

* 发帖页面需要要求登录；未登录用户访问时可提示需要登录，并提供登录入口。
* 发帖成功后跳转到 `postPath(thread.id, 1)` 指向的新帖子详情页。
* 发帖页可以通过 query string 承接来源板块，例如 `/new-post?category=tech`；如果参数匹配已启用分类，则默认选中该分类。

## Requirements

* 在登录态“欢迎回来”区域增加“发帖”按钮或链接。
* 点击“发帖”后进入专门的发帖页面，例如 `/new-post` 或 `/posts/new`。
* 首页和每个分类帖子列表页上部移除现有内嵌发帖表单，不再在列表内容区直接发帖。
* 从分类页点击“发帖”入口时，链接需要携带当前分类上下文，发帖页面默认选中该分类。
* 从聚合首页或帖子详情页点击“发帖”入口时，发帖页面仍要求用户选择板块；可默认选中 `daily` 或第一个启用分类，但控件必须可见且可修改。
* 新增发帖页面模板，包含标题、正文、板块选择、公开/私有选择。
* 板块选择应使用现有 `GET /api/forum/categories` 数据，默认可选 `daily` 或当前分类上下文。
* 公开/私有选择默认公开。
* 发帖提交应复用现有认证 token 和 `apiFetch()` 错误处理模式。
* 发帖成功后跳转到新帖子详情页。
* 发帖失败时留在发帖页，并通过 toast 展示后端返回原因。
* 后端创建主题 API 需要接收并保存可见性字段。
* 列表和详情读取需要尊重私有可见性，避免把私有帖子暴露给无权限用户。
* 私有帖子仅作者本人和管理员可见；公共列表和分类列表只展示公开帖子。
* 无权限用户直接访问私有帖子详情时，应按不可见处理，优先返回 not found 语义以避免泄露帖子存在性。

## Acceptance Criteria

* [x] 登录后“欢迎回来”区域展示可点击的“发帖”入口。
* [x] 点击入口可进入专门发帖页面。
* [x] 首页和分类帖子列表页上部不再展示 `#thread-form` 或同等内嵌发帖表单。
* [x] 从 `/categories/:slug` 页面点击发帖后，发帖页默认选中对应 `slug` 的板块。
* [x] 从聚合首页或帖子详情页进入发帖页时，板块选择控件可见且可修改。
* [x] 发帖页面存在标题、正文、板块选择和公开/私有选择控件。
* [x] 可见性默认值为公开。
* [x] 发帖页面的板块选择来自现有分类接口或与其保持一致。
* [x] 公开帖子发布成功后进入新帖子详情页，并可在公共列表中看到。
* [x] 私有帖子发布成功后进入新帖子详情页；匿名用户和其他普通用户不能在公共列表中看到它，也不能直接访问详情。
* [x] 私有帖子作者本人可以访问详情；管理员如已有权限上下文，应可访问。
* [x] 发帖失败时页面不跳走，toast 展示错误原因。
* [x] 未登录用户访问发帖页时不会出现可提交表单，页面应提示登录或跳转登录页。
* [x] `node --check public/extensions.js` 通过。
* [x] `go test . -count=1` 或覆盖相关包的等价测试通过。

## Definition Of Done

* 新发帖页与首页/帖子详情页的登录态入口串联完成。
* 前端、后端 API、usecase、model、migration 对公开/私有字段保持一致。
* 私有帖子的列表与详情权限边界有测试覆盖。
* 现有公开帖子流程不回退。
* 不引入新的前端框架或 UI 依赖。

## Technical Approach

* 在 `index.go` 注册一个新的前端路由，例如 `/new-post`，渲染新增模板 `public/new-post.html`。
* 在 `public/index.html` 和 `public/post.html` 的 `#logged-in-panel` 内增加发帖入口。
* 移除 `public/index.html` 中列表区域顶部的 `#composer-panel/#thread-form`，并清理不再使用的锚点入口。
* 在分类页场景下，`#logged-in-panel` 的发帖入口应生成 `/new-post?category=<current-slug>`；聚合首页可生成 `/new-post`。
* 在 `public/extensions.js` 新增发帖页初始化逻辑：加载分类、绑定 `#new-thread-form`、提交后跳详情页。
* 发帖页初始化分类下拉框时读取 `category` query 参数；如果该 slug 存在于分类接口返回值中，则将其设为默认选中值。
* 复用现有 `apiFetch()`、`storeAuth()`、`showToast()`、`postPath()` 和分类加载逻辑，避免复制一套 API 客户端。
* 后端新增可见性字段，优先考虑字段名 `visibility`，取值 `public/private`；数据库可保存为 `visibility TEXT NOT NULL DEFAULT 'public'` 或等价结构。
* `CreateForumThreadRequest`、`CreateForumThreadCmd`、`models.ForumThread`、insert/select SQL 和 response 结构同步可见性。
* 列表查询只返回公开帖子，不把私有帖子混入公共聚合列表或分类列表。
* 详情查询需要在 usecase 层做可见性检查：公开帖子可公开访问；私有帖子仅作者本人和管理员可访问；其他用户返回不可见错误。

## Decision (ADR-lite)

**Context**: 当前发帖能力已经存在，但入口嵌在首页内容流里，无法选择板块，也没有公开/私有语义。用户希望登录后从欢迎区域进入专门发帖页。

**Decision**: 将发帖作为独立页面能力建设；前端入口放在登录态卡片中，后端扩展主题可见性字段，默认公开。

**Consequences**: 用户路径更清晰，未来也能扩展草稿、标签、附件、预览等能力；短期需要补数据库迁移和权限过滤，避免私有内容泄露。

**Follow-up Decision**: 分类列表页不再保留顶部内嵌发帖表单；分类上下文通过发帖入口链接传递到专门发帖页，并在发帖页默认选中来源分类。

**Visibility Decision**: 私有帖子仅作者本人和管理员可见；私有帖子不进入公共聚合列表或分类列表；无权限直接访问时按不可见处理。

## Open Questions

* None.

## Out Of Scope

* 富文本编辑器、Markdown 预览、附件上传、图片上传。
* 草稿箱、定时发布、审核流。
* 修改或删除帖子的专门页面。
* 复杂的板块权限体系，例如某些板块只允许特定角色发帖。

## Technical Notes

* 相关前端文件：`public/index.html`、`public/post.html`、`public/extensions.js`、`public/styles.css`、新增发帖模板。
* 相关入口文件：`index.go`、`index_test.go`。
* 相关后端文件：`api/routes/forum.go`、`api/usecase/forum.go`、`api/models/forum.go`、forum migration、forum tests。
* 当前现有创建主题 API：`POST /api/forum/threads`。
* 当前分类 API：`GET /api/forum/categories`。
