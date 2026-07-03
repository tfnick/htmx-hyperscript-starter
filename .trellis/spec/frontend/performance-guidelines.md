# Performance Guidelines

> 轻量前端、列表/详情性能、缓存语义、依赖控制和跨层性能防护。

---

## Overview

当前项目选择 Go + Echo + htmx + hyperscript + Pico.css / 原生 CSS，是为了保持论坛页面轻量、可部署、可定制。高性能不是只靠某个缓存层，而是靠页面结构、API 分页、数据库查询、静态资源体积和渐进增强共同约束。

## Scope / Trigger

修改以下内容时必须阅读本规范：

- 论坛首页、分类页、列表页、搜索页、帖子详情页
- `public/extensions.js` 中批量请求、轮询、事件绑定或 DOM 更新逻辑
- `public/styles.css`、图片、图标、第三方前端依赖
- `/api/forum/*` 列表、详情、搜索、分页、计数或聚合字段
- 任何可能新增 N+1 查询、无界列表、无索引过滤或大体积响应的改动

## Dependency Contracts

- 不为单个页面或单个控件引入大型前端框架、UI 套件或构建链。
- 新增依赖必须写入 PRD 的 Decision，说明为什么 htmx/hyperscript/原生 JS/Pico.css 无法满足、体积影响是什么、如何验证。
- 不默认引入 Tailwind 或 daisyUI；如果未来任务决策引入，必须同时更新模板、构建、缓存和部署 spec。
- 优先复用现有 `apiFetch`、`showToast`、`postPath`、分类渲染和 toast 模式，避免每个页面各写一套 fetch/error/loading 逻辑。
- CSS 应先扩展现有布局和组件类；不要为同一种按钮、面板、列表重复创建语义相近但行为不同的样式系统。

## Data And Query Contracts

- 所有公开列表必须分页或明确限制数量；不得一次性拉取全站所有主题、回复、用户或通知。
- 后端分页默认遵守 backend 规范：page 默认 `1`，page size 默认 `10`，最大 `50`。
- 分类、搜索、可见性、排序字段需要数据库索引或明确的查询规模理由。
- 列表 API 应返回列表页所需摘要字段，例如作者名、分类、回复数、最后回复时间、置顶状态；前端不得为每个列表项再请求一次详情来补字段。
- 详情页只请求当前详情和当前页回复；不要为了渲染详情预取所有分页回复。
- 私有主题不得出现在公共列表或分类列表；性能优化不能绕过可见性过滤。
- 新增跨层字段时，必须同步 migration/model/usecase/route/frontend/test，避免前端为了缺失字段追加额外请求。
- 前端性能问题优先追溯到数据契约：如果页面需要额外循环请求才能展示列表，先补后端列表 DTO 或 usecase 聚合，而不是在 JS 中堆并发请求。

## Rendering Contracts

- 首屏关键结构应由 HTML 模板提供，JS 只补充动态数据、登录态和增强交互。
- 大列表渲染时应最小化 DOM 重写；能替换列表容器就不要重建整个页面。
- 全局事件监听要避免重复绑定；在多页面共享 JS 中使用幂等绑定或明确的绑定标记。
- 加载状态应有稳定高度或骨架，避免内容加载后大幅布局跳动。
- 图片必须有合理尺寸、`alt` 和懒加载策略；不要把大图作为列表默认首屏依赖。
- 不要在滚动、输入和 resize 中直接执行重查询或大 DOM 操作；需要时做节流、防抖或改为用户触发。

## Cache Contracts

- `/api/components/*` 当前设置 `Cache-Control: no-store`，适合登录态和动态片段；不要无意改成可缓存导致用户态串数据。
- 静态资源如果增加强缓存，必须配套版本化文件名、hash 或明确的 cache busting 策略。
- 个性化 API、私有主题、通知、当前用户信息应默认 no-store 或由后端明确控制缓存语义。
- 公开分类、公开列表、公开详情可以考虑短缓存，但必须先确认分页、权限和更新频率。
- 外部模板覆盖开启时，不要假设模板内容与 exe 内置版本相同；缓存策略不能让外部模板修改长期不可见。

## Good / Base / Bad Cases

- Good: 分类列表 API 一次返回当前页主题摘要和分页信息，前端一次渲染列表。
- Good: 搜索页限制 page size，并要求搜索字段有索引或明确的后续索引任务。
- Good: 私有详情页作者可读，但公共列表查询始终带 `visibility='public'` 过滤。
- Base: 一个小交互用原生 JS 和现有 toast 实现，不新增依赖。
- Bad: 首页先拉取所有分类所有主题，再在浏览器里过滤当前分类。
- Bad: 列表每个主题调用一次 `/api/forum/threads/:id` 补作者和回复数。
- Bad: 为一个下拉菜单引入完整 UI 框架和构建链。

## Tests / Checks

- 新增列表或搜索：检查 API 有分页参数、最大 page size、空结果状态和错误状态。
- 新增聚合字段：检查 SQL 是否避免 N+1，必要时补充索引迁移和 model/usecase 测试。
- 新增前端依赖：在 PRD 中记录决策，并检查构建、部署、缓存和许可证影响。
- 修改 `extensions.js`：运行 `node --check public/extensions.js`，并 smoke 测试至少一个包含该脚本但没有目标控件的页面。
- 修改模板体积较大的页面：检查首屏是否有语义结构，图片是否有尺寸和懒加载策略。
- 修改缓存头：检查公开/私有/登录态三类页面或组件不会互相串缓存。
- 新增端到端字段：检查列表 API 是否一次返回渲染所需字段，避免前端 N+1。

## Wrong vs Correct

### Wrong

```js
const threads = await apiFetch("/api/forum/threads?size=10000");
for (const thread of threads.items) {
  thread.detail = await apiFetch(`/api/forum/threads/${thread.id}`);
}
```

该写法同时制造无界列表和 N+1 请求。

### Correct

```js
const threads = await apiFetch(`/api/forum/threads?page=${page}&size=20`);
renderThreadSummaries(threads.items, threads.pagination);
```

列表只请求当前页摘要字段，详情数据留给详情页。

## Common Mistakes

- 认为前端性能只等于压缩 CSS/JS，忽略无界 API 和 N+1 查询。
- 为了 UI 统一过早引入大型组件库，结果破坏 exe 单文件部署和模板可覆盖性。
- 给动态组件加缓存但忘记登录态和私有内容。
- 为了减少一次 API 请求，把权限或可见性判断搬到前端。
- 只在桌面宽屏测试，忽略移动端列表高度、按钮换行和布局跳动。
