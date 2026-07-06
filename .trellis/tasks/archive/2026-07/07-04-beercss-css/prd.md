# 引入 BeerCSS 并替换论坛 CSS

## Goal

在现有 Go/Echo + `public/` 静态 HTML/CSS/JS 论坛中引入 BeerCSS 作为基础 CSS 框架，逐步替换现有手写控件样式，同时保留当前 NodeSeek 风格的 PC 三栏论坛布局与移动端自适应体验。

## Reasonableness Analysis

这个任务是可行的，但不适合做“整站视觉重做”。当前论坛已经有较完整的定制布局和样式体系：顶部导航、左侧板块栏、中间 feed、右侧 sidebar、帖子行、分页、详情、登录/发帖页都依赖 `public/styles.css` 与 `public/extensions.js` 动态生成的 class。BeerCSS 更适合作为表单、按钮、分页、卡片、基础排版的框架层；PC 端布局和 NodeSeek 风格应继续由克制的 `styles.css` 控制。

合理实施边界：

- 引入 BeerCSS 后，`styles.css` 继续存在，用于主题变量、布局网格、论坛专属密度、帖子列表、sidebar、移动端断点和少量覆盖。
- BeerCSS 不应强行接管 `page-shell` 三栏布局、`topbar` 信息密度、右侧赞助卡片风格或帖子列表结构。
- 替换重点放在 button/input/select/textarea/card/notice/pagination 等通用控件，减少重复基础样式。
- 保留无需构建的项目形态，不引入 React/Vue/Tailwind/Vite/Node 构建链。

主要风险：

- BeerCSS 的 Material Design 3 默认视觉可能比当前论坛更“应用化”，如果直接套用 layout class，PC 风格会明显偏离。
- BeerCSS 静态资源包含 CSS、JS、字体/图标等依赖，若使用 CDN 会引入外部网络依赖；若本地嵌入则要新增静态路由或放入现有 `/assets`。
- 现有 `extensions.js` 会动态生成帖子行、分页、回复、空状态等 HTML，CSS class 迁移时必须同步这些字符串模板。

## What I Already Know

- 用户要求创建任务并先分析合理性、实施计划、验收标准。
- 用户要求 PC/移动端继续自适应。
- 用户允许引入 BeerCSS 后继续克制地自定义 `style.css`/`styles.css`。
- 用户要求 PC 页面布局与当前风格保持一致，不能大改。
- 当前项目没有 `package.json`，前端资源直接位于 `public/` 并通过 Go `//go:embed public` 打进 exe。
- 当前页面通过 `/styles.css` 和 `/extensions.js` 加载样式与交互，`index.go` 显式注册静态资源路由。
- 当前论坛主要页面是 `public/index.html`、`public/post.html`、`public/login.html`、`public/register.html`、`public/new-post.html`。
- 当前论坛样式核心在 `public/styles.css`，动态 HTML 样式 class 也散落在 `public/extensions.js`。
- README 仍描述 Pico.css，但实际页面没有加载 Pico.css；本任务应顺手校正文档和旧框架表述。
- BeerCSS 官方定位为 Material Design 3 CSS 框架，并提供 CDN 与本地 CDN 文件使用方式。

## Research References

- [BeerCSS website](https://www.beercss.com/) - BeerCSS 是 Material Design 3 风格 CSS 框架。
- [BeerCSS README](https://github.com/beercss/beercss/blob/main/README.md) - 官方示例支持 CDN 引入，也支持下载 CDN 文件放到项目本地目录。
- [jsDelivr beercss package](https://cdn.jsdelivr.net/npm/beercss/) - 可用版本列表显示 `beercss@4.0.23`。

## Assumptions (Temporary)

- 默认采用“渐进式替换”：先引入 BeerCSS，再将现有基础控件样式迁移到 BeerCSS 语义/组件，最后保留少量自定义 CSS 维持论坛布局和品牌风格。
- 默认不改变论坛信息架构、页面路由、API、分页语义、登录发帖流程。
- 默认不会新增 Node 构建链；如需本地化 BeerCSS 资源，优先把固定版本资源放入 `public/assets/vendor/beercss/` 或类似路径，并由现有 embedded static 机制提供。
- 默认 `styles.css` 是现有实际文件名；用户提到的 `style.css` 理解为继续允许项目自定义样式文件。

## Open Questions

- BeerCSS 资源希望用哪种方式引入：本地嵌入到 Go exe（推荐，离线稳定、可控）还是 CDN（改动少，但依赖外网）？

## Requirements (Evolving)

- 引入 BeerCSS，作为论坛前端基础 CSS 框架。
- 保留 `styles.css` 作为克制的自定义覆盖层。
- PC 端维持当前整体布局风格：sticky 顶栏、左板块栏、中间帖子 feed、右侧功能/赞助栏，不做大改版。
- 移动端继续自适应，保留当前断点下的一列布局、横向板块入口和可读的帖子列表。
- 替换或精简现有基础控件样式，但不牺牲帖子列表的信息密度。
- 同步更新动态生成 HTML 的 class，使列表、分页、回复、toast、表单状态样式一致。
- 更新 README 或相关文案，移除过期 Pico.css 描述并说明 BeerCSS。

## Acceptance Criteria (Evolving)

- [ ] 所有论坛页面正常加载 BeerCSS，并且 `styles.css` 在 BeerCSS 之后加载以保留项目覆盖能力。
- [ ] 首页 `/`、分类页 `/categories/:slug`、详情页 `/post-*`、登录页 `/login`、注册页 `/register`、发帖页 `/new-post` 在桌面宽度下仍保持当前信息架构和视觉密度。
- [ ] 移动端宽度下无横向溢出，顶部导航、板块入口、帖子行、详情、回复分页、登录/发帖表单均可用。
- [ ] 帖子列表分页和回复分页的 hover、disabled、current 状态清晰可辨。
- [ ] 表单控件、按钮、空状态、notice、toast 在 BeerCSS 引入后视觉一致，没有重叠、错位或文字溢出。
- [ ] `extensions.js` 中动态生成的样式 class 与模板/CSS 保持一致。
- [ ] README 不再声称当前项目使用 Pico.css，并说明 BeerCSS 引入方式。
- [ ] `go test ./...` 通过。
- [ ] `node --check public/extensions.js` 通过。
- [ ] 浏览器人工或自动截图检查覆盖至少桌面与移动端两个视口。

## Technical Approach

推荐采用“BeerCSS 基础层 + 论坛布局覆盖层”的方式：

1. 固定 BeerCSS 版本与资源引入方式。
2. 在所有 HTML 页面的 `<head>` 中按顺序加载 BeerCSS，再加载 `/styles.css`。
3. 将 `styles.css` 拆减为主题变量、布局、论坛专属列表密度、sidebar、响应式断点和必要覆盖。
4. 对静态模板和 `extensions.js` 动态模板做 class 映射，优先迁移按钮、输入框、select、textarea、卡片/notice/pagination。
5. 保持 `page-shell`、`topbar`、`category-rail`、`feed-panel`、`right-sidebar` 等布局 class，避免 PC 大改版。
6. 校正 README 中 Pico.css 相关描述。
7. 通过 Go 测试、JS 语法检查和桌面/移动端页面检查验收。

## Implementation Plan

- PR1: 资源引入与文档校正。固定 BeerCSS 资源来源，更新 HTML head 加载顺序，更新 README 旧框架描述。
- PR2: 基础控件迁移。迁移按钮、输入、select、textarea、空状态、notice、toast、分页等基础控件样式，删除可由 BeerCSS 承担的重复 CSS。
- PR3: 论坛布局收敛与响应式检查。保留 PC 三栏风格，调整移动端断点，修复动态模板 class 与视觉状态，做截图和测试验收。

## Out of Scope

- 不重做论坛信息架构。
- 不把论坛改成完整 Material Design app 风格。
- 不引入 SPA 框架或 Node/Vite 构建链。
- 不改后端 API、数据库、分页规则、登录发帖业务逻辑。
- 不新增主题切换、暗色模式或动态取色，除非后续单独提出。

## Definition of Done

- 需求和资源引入方式已确认。
- 代码实现完成并保持 PC/移动端布局符合验收标准。
- `go test ./...` 与 `node --check public/extensions.js` 通过。
- README/任务记录更新。
- 如实现过程中形成可复用规范，按 Trellis 规则更新对应 spec。

## Technical Notes

- 受影响文件预计包括：`public/styles.css`、`public/index.html`、`public/post.html`、`public/login.html`、`public/register.html`、`public/new-post.html`、`public/extensions.js`、`README.md`，以及必要时的 `index.go` 静态资源路由。
- 前端规范要求继续使用 `public/` 下的服务端 HTML/静态资源，交互优先 htmx/hyperscript/现有 `extensions.js` 渐进增强。
- 本任务为中等复杂度：CSS 影响面较广，但不涉及后端数据模型和 API 语义。
