# Homepage DeepFlood Full Replication

## Goal

根据附件 `C:\Users\mac\Downloads\idnex.html` 保存的 DeepFlood 首页，对当前项目首页进行完整复刻：不仅复刻视觉布局、配色、密度和响应式表现，也复刻首页可见的站点身份、导航文案、右侧卡片内容、帖子列表表达方式和社区氛围。实现时仍保留当前 Go + htmx/hyperscript 论坛的数据接口、路由和基础交互能力，避免把保存页中的不可维护脚本直接搬进项目。

## What I Already Know

* 用户要求“创建一个首页微调任务，根据附件的首页样式，对目前的首页进行精确的复刻”。
* 用户已确认复刻范围为“复刻全部”。
* 附件路径为 `C:\Users\mac\Downloads\idnex.html`，保存来源标记为 `https://www.deepflood.com/`。
* 附件旁存在 `C:\Users\mac\Downloads\idnex_files\` 资源目录，包含保存下来的 CSS、JS、图标和图片资源。
* 当前首页入口是 `public/index.html`，主样式文件是 `public/styles.css`，动态渲染逻辑主要在 `public/extensions.js`。
* 当前首页已是论坛首页信息架构：顶部导航、左侧板块、帖子列表、分页、右侧登录/快捷入口/赞助卡片。
* 当前页面文案存在部分乱码，首页复刻时必须修正可见文案，否则无法完成视觉验收。
* 当前项目使用 htmx、hyperscript 和轻量原生 JS，不应为单页视觉复刻引入 Vue/Nuxt 或新构建链。

## Requirements

* 首页整体复刻 DeepFlood 保存页的第一屏和主要列表区域，包括背景、主容器宽度、导航高度、侧栏布局、列表密度、卡片阴影、边框、字体尺寸、链接颜色、按钮和标签状态。
* 站点身份改为 DeepFlood 风格：首页标题、品牌名、导航文案、社区定位文案和右侧卡片文案都应贴近附件可见内容。
* 保留当前项目可运行的数据来源和交互：板块切换、排序切换、搜索、登录入口、发帖入口、帖子列表点击和分页仍可用。
* 复刻附件中的帖子列表表达方式：头像/作者、标题、标签、浏览/回复/时间等元信息要在当前动态数据可支持的范围内映射出来。
* 复刻附件中的右侧内容氛围：登录/用户区域、快捷入口、信息/赞助卡片应尽量贴近参考页的视觉和文案节奏。
* 修正首页所有可见乱码文案，中文 UI 必须可读。
* 使用附件资源时必须复制到项目可维护路径或用 CSS/本地资源等价重建，不能让运行时依赖 `C:\Users\mac\Downloads\...`。
* 移动端需要保持可读、无横向溢出，导航、板块、帖子列表和侧栏内容要合理折叠或重排。
* 不改变当前后端业务契约，除非实现中发现现有接口无法支撑首页可见信息并另行记录。

## Acceptance Criteria

* [ ] `http://localhost:3000/` 首页在桌面宽度下与附件 DeepFlood 首页的整体布局、品牌表达、配色、卡片密度、帖子列表层级和右侧内容高度接近。
* [ ] 顶部导航、左侧入口区、中间帖子列表、分页、右侧信息卡片都完成参考风格和参考文案方向调整。
* [ ] 首页可见中文文案不再乱码，站点名和社区定位呈现 DeepFlood 风格。
* [ ] 当前动态数据渲染和交互仍可用，不因复刻破坏帖子列表、分页、搜索、排序或登录/发帖入口。
* [ ] 运行时不依赖 `Downloads` 目录或 DeepFlood 远端业务脚本。
* [ ] 移动端视口下无横向滚动，主要内容和操作仍可读可用。
* [ ] 使用浏览器截图或等效方式对比附件与当前页面，记录主要差异和已接受的非目标差异。
* [ ] 完成 `node --check public/extensions.js`（如修改 JS）和 `go test ./...`，如无法运行需记录原因。

## Definition of Done

* 首页完整复刻实现完成，并通过本地浏览器检查。
* 关键截图或验证结论记录在任务上下文或最终汇报中。
* 现有测试通过，或明确记录无法通过/无法运行的原因。
* 若形成新的可复用前端规范，更新 `.trellis/spec/frontend/`。
* 变更完成后按 Trellis 流程进入 check、spec update、commit 和 finish-work。

## Out of Scope

* 不重建完整 DeepFlood 产品功能、账号体系、真实数据源或第三方追踪脚本。
* 不迁移到 Vue/Nuxt 或其他前端框架。
* 不复制不可维护的远端运行时代码；保存页中的 JS/CSS 只能作为分析参考或静态资源取样。
* 不修改论坛后端分页、发帖、登录、权限等业务逻辑，除非样式复刻暴露明确 bug。
* 不要求复制附件中每一条真实帖子内容；当前应用仍使用自己的接口数据，但展示方式和文案氛围要贴近 DeepFlood。

## Technical Approach

以附件 HTML/CSS 和保存资源作为视觉参考，先提炼 DeepFlood 首页的视觉 token、布局模式和内容层级，再映射到当前首页结构。推荐路径是保留现有数据驱动 DOM 锚点和 htmx/hyperscript 引入方式，主要修改 `public/index.html` 的语义结构、`public/styles.css` 的布局与组件样式，必要时小幅调整 `public/extensions.js` 生成的帖子行 markup，使动态列表能承载参考页中的头像、标题、元信息、标签、统计信息等视觉层级。

实现时允许把参考页中必要的静态图片或图标复制到 `public/assets/` 下并重新命名，但需要保持来源清晰、运行路径稳定、体积可控。若附件资源过多或不适合复制，优先用 CSS、现有 assets 或可维护的轻量本地资源重建视觉效果。

## Decision (ADR-lite)

**Context**: 当前项目首页已经具备论坛首页结构，附件参考页也是社区论坛首页，两者信息架构相近。用户确认复刻范围是“全部”，因此品牌文案和右侧内容也纳入本任务。

**Decision**: 采用“保留当前数据与路由 + 完整复刻首页可见体验”的方式实现。附件用于提炼最终视觉和首页内容方向，项目代码仍以当前模板和动态渲染逻辑为基础。

**Consequences**: 首页会更像 DeepFlood，包括站点身份和可见内容氛围；但不会直接搬入远端应用脚本、账号体系或真实数据源。实现检查需要同时关注视觉相似度、交互可用性和本地可维护性。

## Technical Notes

* Reference HTML: `C:\Users\mac\Downloads\idnex.html`
* Reference assets: `C:\Users\mac\Downloads\idnex_files\`
* Current homepage:
  * `public/index.html`
  * `public/styles.css`
  * `public/extensions.js`
* Relevant current behavior:
  * 首页帖子列表、板块、排序、分页由现有前端脚本和后端接口驱动。
  * 当前 CSS 已有三栏布局、卡片、帖子行、分页、侧栏赞助卡片等基础结构，可作为复刻落点。

