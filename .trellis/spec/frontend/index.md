# Frontend Development Guidelines

> 前端、HTML 模板、页面 SEO、模板运行时和性能规范入口。标题使用英文，正文尽量使用中文。

---

## Overview

当前项目的前端不是单独的 SPA 工程，而是由 Go/Echo 入口服务 `public/` 下的 HTML、CSS、JavaScript 和组件片段：

- 默认模板和静态资源通过 `//go:embed public` 打包进 exe。
- 页面路由在 `index.go` 的 `registerFrontendRoutes` 中挂载。
- HTML 模板通过 `renderTemplate` -> `loadTemplate` 渲染。
- `--template-path` 可配置外部 HTML 模板目录；外部同名模板优先，缺失时回退 exe 内置模板。
- 交互优先使用 htmx、hyperscript 和项目已有的 `public/extensions.js` 模式做渐进增强。
- 论坛业务数据来自 `/api/forum/*` 等后端 API；新增跨层字段时要同步 route DTO、usecase Co、model、migration、template/JS 和测试。

本层规范的目标不是引入新的前端框架，而是让后续论坛页面在当前技术栈下更容易开发、更容易 SEO、更容易被用户通过外部模板定制，并保持性能可控。

## Pre-Development Checklist

开始修改 `public/`、页面路由、HTML 模板、组件片段、SEO 输出、前端 JS/CSS 或模板加载行为前：

- 阅读 [Page Guidelines](./page-guidelines.md)，如果新增或修改页面、组件、登录态视图、空状态、错误状态、SEO 元信息或 htmx/hyperscript 交互。
- 阅读 [Template Runtime Guidelines](./template-runtime-guidelines.md)，如果修改 `index.go` 中模板路由、`renderTemplate`、`loadTemplate`、`cleanTemplateName`、`--template-path` 或外部模板覆盖行为。
- 阅读 [Performance Guidelines](./performance-guidelines.md)，如果新增列表、详情、搜索、分类、静态资源、图片、批量数据请求或新的前端依赖。
- 涉及论坛主题、分类、回复或可见性时，同时阅读 [Backend Forum Guidelines](../backend/forum-guidelines.md)。
- 涉及 API 字段、分页、错误响应或鉴权时，同时阅读 [Backend Development Guidelines](../backend/index.md)。
- 对跨层字段和状态变更，阅读 [Cross-Layer Thinking Guide](../guides/cross-layer-thinking-guide.md)。
- 新增可复用 helper、样式模式或前端工具前，阅读 [Code Reuse Thinking Guide](../guides/code-reuse-thinking-guide.md)，并先搜索现有实现。
- 涉及架构、解耦、防腐、provider、framework 复用或端到端字段同步时，阅读 [Architecture Review Thinking Guide](../guides/architecture-review-thinking-guide.md)。

## Core Contracts

- 不为了单个页面引入 React/Vue/Svelte、Tailwind、daisyUI、额外 Node 构建链或第二套路由系统，除非 PRD 明确决策并给出迁移范围。
- 新增用户可访问页面时，默认放在 `public/*.html`，并通过 `index.go` 明确注册路由。
- 新增可复用 HTML 片段时，默认放在 `public/components/**`，并确保 `/api/components/*` 只能渲染清洗后的 `.html` 模板。
- 面向搜索引擎收录的论坛页面应优先返回有语义、有标题、有主要内容或可发现链接的服务端 HTML；客户端 JS 只能增强体验，不能成为唯一的关键内容来源。
- 登录、注册、发帖、私有主题等不适合公开索引的页面，应避免伪装成公开内容页；需要时使用 `noindex` 或清晰的未登录状态。
- 外部模板覆盖是稳定产品能力：新增模板、重命名模板或移动组件时，必须考虑外部用户覆盖路径的兼容性和迁移说明。
- 所有前端改动必须保持 JS null-safe：同一个 `extensions.js` 会运行在不同页面，不能假设某个 DOM 节点一定存在。
- 前端只依赖 route 暴露的 JSON response DTO 和 HTML 模板契约，不直接假设数据库字段、model 结构或 provider payload。
- 新增端到端字段时，前端变更必须和 backend 的 `Co -> response DTO -> JSON -> template/JS` 路径对齐；不要为缺失字段临时追加详情请求或读取未公开字段。

## Quality Check

完成 frontend/spec 或页面相关改动前：

- Markdown/spec 改动：检查所有 heading 使用英文，正文没有占位符、过期路径或与当前代码相冲突的命令。
- HTML 改动：确认路由能渲染，页面含必要的 `<title>`、主要 landmark、空状态和错误状态。
- JS 改动：至少运行 `node --check public/extensions.js`。
- 模板运行时或页面路由改动：运行相关 `go test`，至少覆盖 `index_test.go` 中模板加载、外部覆盖和路由渲染行为。
- 业务 API 或数据库字段联动改动：按照 backend 规范运行对应 route/usecase/model 测试。
- 纯 spec 文档改动可不运行 Go 测试，但必须记录文本检查方式。

## Guidelines Index

| Guide | Purpose | Status |
| --- | --- | --- |
| [Page Guidelines](./page-guidelines.md) | HTML 页面结构、SEO、语义化、状态、可访问性和渐进增强 | Filled |
| [Template Runtime Guidelines](./template-runtime-guidelines.md) | exe 内置模板、`--template-path` 外部覆盖、路径安全和测试契约 | Filled |
| [Performance Guidelines](./performance-guidelines.md) | 轻量前端、列表/详情性能、缓存语义、依赖控制和 N+1 防护 | Filled |

## Language Rule

`.trellis/spec/` 中的 Markdown heading 必须使用英文；正文优先使用中文。新增 frontend spec 也遵守这一规则。
