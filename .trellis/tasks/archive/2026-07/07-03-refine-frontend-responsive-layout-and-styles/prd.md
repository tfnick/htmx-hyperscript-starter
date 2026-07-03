# Refine Frontend Responsive Layout and Styles

## Goal

提升论坛前端的布局与视觉细节，让 PC 与移动端都能自适应展示。整体以 Pico CSS 的语义化 HTML、表单、按钮和内容元素为基础，再通过全局 `public/styles.css` 做 NodeSeek 风格的个性化精修。重点页面是帖子列表、帖子详情，以及它们周边的导航、侧栏、分页、发帖入口和回复区域。

## Requirements

* 布局在桌面端、平板端、手机端都要自适应，不出现横向溢出、文字互相遮挡、按钮或标签挤压变形。
* Pico CSS 使用本地 vendor 方式引入，放在 `public/` 下并随 embedded assets 一起服务。
* 自定义样式集中在 `public/styles.css`，并在 Pico 之后加载，用于主题变量、布局网格、帖子列表、帖子详情、侧栏、表单、toast 和响应式规则。
* 首页帖子列表要更精细：标题、状态徽标、作者、板块、浏览数、回复数、最后活跃时间应层级清楚，hover / active / loading / empty 状态可辨识。
* 帖子详情要更精细：标题区、作者和统计信息、正文、回复列表、回复表单、未登录提示、空状态要有清晰视觉层级。
* 优先使用语义 HTML 元素和 Pico 友好的结构：`main`、`section`、`article`、`header`、`nav`、`form`、`button`、`input`、`select`、`textarea` 等。
* 样式应保留适合论坛的高信息密度与可扫描性，避免过度装饰和营销页风格。
* 修正主要模板和动态渲染中的可见乱码文案，保证 UI 文案可读。

## Acceptance Criteria

* [ ] 桌面端宽屏下首页保持左侧板块、主列表、右侧功能区的清晰布局。
* [ ] 中等宽度下侧栏合理下移或重排，主列表不被挤压。
* [ ] 手机端下导航、板块入口、帖子列表、详情、回复表单均单列可读，页面无横向滚动。
* [ ] 帖子列表长标题、长板块名、长用户名、多个状态徽标不会破坏布局。
* [ ] 帖子详情长正文和多条回复保持良好行高、间距和换行。
* [ ] 发帖页、登录/注册页不因全局样式调整出现明显退化。
* [ ] 页面加载、空列表、错误、未登录提示和 toast 状态样式一致。
* [ ] Pico CSS 的本地引入方式清晰，并且自定义样式不会与 Pico 基础样式互相打架。
* [ ] 运行现有 Go 测试通过。
* [ ] 通过浏览器手动或自动截图检查至少桌面与移动端两个 viewport。

## Definition of Done

* Pure CSS/template work 至少运行现有 Go 测试。
* Browser verification covers desktop and mobile viewport.
* Task notes record final Pico integration decision.
* Spec update considered if this task establishes reusable frontend styling rules.

## Technical Approach

采用渐进式前端整理：保留现有 Go embedded static/template 架构和 htmx/hyperscript 运行方式，先修正文案与 HTML 语义结构，再本地引入 Pico CSS，最后用 `public/styles.css` 覆盖出项目自己的布局、间距、颜色、列表、详情和响应式行为。

帖子列表和详情的动态 HTML 在 `public/extensions.js` 中生成，因此这次任务会同时触碰模板、JS 渲染片段和 CSS。改动时需要先搜索 class 与文本复用，避免只改模板而漏掉动态渲染状态。

## Decision (ADR-lite)

**Context**: 项目 README 已声明使用 Pico.css，但当前默认页面只加载 `/styles.css`，没有单独的 Pico 基础样式。用户希望总体使用 Pico CSS 的语义样式，同时用全局样式做个性化。

**Decision**: 选择本地 vendor Pico CSS。将 Pico CSS 放入 `public/` 下的 vendor 目录，模板先加载 Pico，再加载 `/styles.css`。

**Consequences**: 默认 UI 不依赖外部网络，符合当前 embedded assets 的部署方式；后续升级 Pico 时需要记录来源并替换本地 vendor 文件。

## Out of Scope

* 不新增论坛业务功能，例如编辑、删除、通知、权限、分页 API 改造。
* 不重写后端 usecase、model 或数据库结构。
* 不引入重型前端框架或构建链。
* 不制作营销首页。
* 不做完整品牌系统，只做本项目默认论坛 UI 的精修。

## Technical Notes

* Task directory: `.trellis/tasks/07-03-refine-frontend-responsive-layout-and-styles/`
* Inspected files:
  * `public/styles.css`
  * `public/index.html`
  * `public/post.html`
  * `public/new-post.html`
  * `public/components/forum/thread-list.html`
  * `public/components/forum/thread-detail.html`
  * `public/extensions.js`
  * `README.md`
* Existing static CSS route: `index.go` serves `/styles.css`.
* Existing breakpoint anchors: `1180px`, `820px`, `520px`.
* Selected Pico integration: vendor Pico locally under `public/`, then load `/styles.css` after it.
* Potentially relevant spec indexes:
  * `.trellis/spec/frontend/index.md`
  * `.trellis/spec/backend/index.md`
