# Page Guidelines

> HTML 页面、SEO、语义化、可访问性、状态和渐进增强规范。

---

## Overview

项目页面由 Go/Echo 返回 HTML 模板，再由 htmx、hyperscript 和 `public/extensions.js` 做渐进增强。新增页面时要同时考虑三类使用者：搜索引擎、未启用或加载失败 JavaScript 的用户、以及登录后的交互用户。

## Scope / Trigger

修改以下内容时必须阅读本规范：

- `public/*.html`
- `public/components/**/*.html`
- `public/extensions.js`
- `public/styles.css`
- `index.go` 中页面路由和组件路由
- 任何影响 title、description、canonical、分页链接、登录态展示、错误状态或空状态的改动

## Page Contracts

- 每个完整页面模板必须包含明确的 `<title>`；面向公开索引的页面还应包含 `description` 和 canonical 语义。
- 每个完整页面必须有一个稳定的主内容区域，例如 `<main>`；页面只允许一个清晰的主标题层级。
- 页面结构优先使用语义化 HTML：`header`、`nav`、`main`、`section`、`article`、`aside`、`footer`，不要只用无含义的 `div` 堆出页面。
- 新增入口页必须在 `registerFrontendRoutes` 中注册，并在测试里验证路由能渲染对应模板。
- 新增组件片段必须放在 `public/components/**`，并通过 `/api/components/*` 访问时仍然满足模板路径清洗。
- `extensions.js` 中的初始化函数必须对缺失 DOM 节点直接返回，不能让某个页面缺少节点时影响其他页面。
- 表单成功、失败和校验错误优先使用页面内状态或 toast；不要把普通失败做成 blocking modal。
- 登录态区域、未登录态区域、空列表、加载失败、权限不足和 not found 都要有可理解的 UI 状态。

## SEO Contracts

- 公开论坛首页、分类页、帖子详情页、搜索结果页应优先让服务端 HTML 提供可抓取的标题、描述、主要内容或可发现链接；不要只返回空壳再完全依赖客户端 JS 填充关键内容。
- 分类页和分页列表应有稳定 URL，例如 `/categories/:slug` 和明确的 `page` 参数；分页要能被链接发现，不要只靠按钮状态存在于 JS 内存中。
- 帖子详情页应有稳定 URL，例如当前的 `/post-{id}-{page}` 模式；不要把可分享详情藏在 hash 或 localStorage 状态里。
- 不存在、已删除、私有且无权访问的内容应返回正确状态或清晰错误模板；不要用 200 页面伪装所有错误。
- 登录、注册、发帖、账号中心、私有主题等不应作为公开 SEO 内容；需要公开页面结构时，正文要清楚表达登录/权限状态。
- 页面标题应先表达本页主题，再表达站点名称；不要所有页面共用同一个泛标题。
- 元信息应来自真实业务数据；不要硬编码与页面内容无关的关键词堆砌。

## Progressive Enhancement Contracts

- When a page has multiple create-post entry links that must follow the current forum category, mark each link with `data-create-post-link` and update them through the shared `updateCreatePostLink()` flow instead of hand-writing separate href sync logic.

- htmx/hyperscript 和 `extensions.js` 用于增强交互，不应破坏基础链接、表单或页面阅读。
- 点击可分享内容时优先导航到稳定 URL，再在目标页加载增强数据。
- 需要局部刷新时优先复用 `/api/components/*` 组件片段；组件片段不应夹带整页 `<html>`、`<head>` 或重复全局脚本。
- 前端 API 请求复用现有 `apiFetch`、`showToast`、`postPath` 等模式；新增 helper 前先搜索现有实现。
- 跨页面共享 JS 状态必须有明确默认值和恢复逻辑，不能依赖某个页面先访问过。

### Forum Pagination Contracts

- 帖子列表页如果同时提供上方和下方分页操作区，两处必须读取同一个 `forumState.page` 和同一份 API `pagination` 元数据；不要为上下分页各维护一份状态。
- 上下分页控件应使用共享 class 或 `data-*` action 绑定事件，点击任一处都调用同一套加载逻辑，并在响应后同步所有分页摘要、页码按钮和 disabled 状态。
- 帖子详情页的 `/post-{id}-{page}` 中 page 表示回复页码；`loadThread` 必须把该页码作为 `page` query 参数传给详情 API。
- 回复分页使用页码按钮时，按钮必须能表达当前页，页数很多时应收缩显示，避免移动端横向溢出。
- 发布新回复后，如果产品希望用户立即看到新回复，应根据 `reply_count` 和当前 `page_size` 计算最后一页，再加载对应回复页；不要假设新回复一定在当前页。
- 推广、广告或推荐内容插入列表会改变分页计数语义，必须另起任务设计；普通列表分页任务不要预留隐式占位。

## API Boundary Contracts

- 前端只消费 `/api/*` 返回的 response DTO，不直接把数据库字段、Go model 字段或 provider 原始 payload 当成页面契约。
- `extensions.js` 中的状态字段应来自明确的 API response、URL path/query 或页面 DOM，不从 localStorage 随意拼装业务事实。
- API envelope 失败时优先使用 `apiFetch` 解析出的安全 message，再通过 `showToast` 或页面状态展示；不要在每个页面重写 fetch/envelope/error 逻辑。
- 新增列表字段时，优先让列表 API 返回摘要字段；不要在前端对每个列表项追加详情请求来补字段。
- 新增表单字段时，确认 route request DTO、usecase command、model/migration、模板字段名和 JS payload 使用同一个业务语义。
- provider-specific 字段不得直接暴露到模板或 JS；需要展示时，后端先归一化成业务字段。

## Accessibility Contracts

- 可点击命令使用 `button`，导航使用 `a href`；不要用无 href 的链接承担按钮行为。
- 表单字段必须有可访问 label 或等价文本；错误提示应关联到对应字段或表单状态。
- toast 或页面状态更新应可被用户感知，不要只改变颜色表达错误。
- 新增 icon-only 控件必须有 `aria-label` 或可访问名称。
- 弹层、菜单和焦点管理必须可键盘操作；若做不到，应先使用普通页面跳转。

## Good / Base / Bad Cases

- Good: 分类页的 HTML 提供分类名、标题、描述和分页链接，JS 只负责刷新列表和展示登录态操作。
- Good: 发帖页未登录时显示登录/注册入口，登录后显示表单；同一个 JS 在首页、详情页、发帖页都不会因为缺少节点报错。
- Base: 新增静态页面先有可访问 HTML、路由和模板渲染测试，再逐步补 htmx 增强。
- Bad: 新增页面只有 `<div id="app"></div>`，所有标题、正文、分页和错误都由客户端 JS 后填。
- Bad: 页面点击列表项只改 localStorage，不改变 URL，导致刷新、分享和搜索引擎都找不到详情。
- Bad: 所有错误都弹 modal，用户无法继续阅读或复制错误信息。

## Tests / Checks

- 新增页面路由：补充 `index_test.go` 或等价测试，确认路由渲染成功。
- 新增模板文件：确认默认内置模板可读取；如支持外部覆盖，补充同名外部模板优先测试。
- 新增 JS：运行 `node --check public/extensions.js`，并人工或浏览器 smoke 测试关键页面。
- 新增 SEO 公开页：检查 `<title>`、主要 heading、canonical/分页链接、空状态、错误状态。
- 修改跨层字段：同步 API response、前端渲染、空值处理、错误处理和测试。
- 修改 API envelope、字段名或错误语义：检查 `apiFetch` 调用点和页面 toast/状态展示是否仍然一致。
- 修改论坛分页：检查列表上下分页控件同步、详情回复页 URL 刷新恢复、`node --check public/extensions.js`，并配合后端 route/usecase 分页测试。

## Wrong vs Correct

### Wrong

```html
<main id="forum-root"></main>
<script>
  renderEverythingFromApi()
</script>
```

该页面没有可抓取标题、链接或正文，用户和搜索引擎都只能等待 JS 成功。

### Correct

```html
<main>
  <h1>Tech</h1>
  <p>技术讨论与经验分享。</p>
  <nav aria-label="Pagination">
    <a href="/categories/tech?page=2">Next</a>
  </nav>
  <section id="thread-list" aria-live="polite"></section>
</main>
```

基础 HTML 提供页面主题和可发现链接，JS 再增强列表刷新和登录态操作。

## Common Mistakes

- 只改 `public/*.html`，忘记在 `index.go` 注册路由。
- 新增元素后在 `extensions.js` 直接 `document.querySelector(...).addEventListener`，没有空值保护。
- 新增公开页面但没有独立 title，导致所有页面在搜索结果中标题相同。
- 把私有主题、发帖页或账号页面当作公开可索引内容处理。
- 为一次性小交互新增大型依赖或第二套状态管理。
- 页面为了补字段循环请求详情 API，绕开后端列表 DTO 的职责。
- 把第三方 provider 状态码、payload 字段或签名错误原样显示给用户。
