# Spec Enrichment For Architecture SEO Performance

## Goal

从整体架构、高效开发、SEO 友好和高性能等角度完善 `.trellis/spec/`，让后续论坛功能开发更快、更一致、更少返工。重点把“默认将前端 HTML 模板打包到 exe 中，同时生产部署时允许通过外部目录优先覆盖 HTML 模板”的能力沉淀为可执行的项目契约，方便用户定制 HTML 响应内容而不需要重新构建程序。

## What I Already Know

* 用户明确要求创建一个新任务，而不是直接零散修改。
* 目标是丰富完善 spec，主要产物应位于 `.trellis/spec/`。
* 当前 spec 标题必须使用英文，正文尽量使用中文。
* 当前项目技术栈是 Go + Echo + htmx + hyperscript + Pico.css / 原生 CSS，不应因为写 spec 引入新前端框架或构建链。
* 当前 `index.go` 已使用 `//go:embed public` 默认打包 `public/` 到 exe。
* 当前 `-template-path` 支持配置外部 HTML 模板目录；外部目录中的同名模板优先于 exe 内置模板。
* 当前 `loadTemplate()` 会清洗模板名，拒绝绝对路径、`..` 和非 `.html` 模板名。
* 当前 `index_test.go` 已覆盖内置模板 fallback、外部模板优先、非法模板名拒绝，以及主要页面路由渲染。
* README 已描述构建 exe、内置模板和 `--template-path` 覆盖方式，但 `.trellis/spec/` 中还没有专门的 frontend/template/runtime deployment 规范。
* 当前 `.trellis/spec/backend/` 已包含分层、数据库、错误、日志、质量、论坛可见性等后端规范。
* 当前 spec 对 SEO、高性能、HTML 模板扩展、页面结构、缓存策略、SSR/渐进增强边界等缺少系统性约束。
* 用户已确认本任务不新增 platform spec 层，只保留 `.trellis/spec/frontend/` 一个新增 spec 层级。

## Assumptions

* 本任务优先更新 Trellis spec，不做业务功能实现。
* 如发现 README 与真实代码/spec 明显不一致，可在本任务中小幅修正文档，但不作为主要目标。
* SEO 友好默认指面向搜索引擎可抓取的服务端 HTML、语义化结构、标题/描述/canonical/分页可发现性、状态码正确，而不是依赖客户端渲染补全关键内容。
* 高性能默认先约束轻量页面、少依赖、合理缓存、避免重复查询和避免无边界列表，而不是引入复杂 CDN/SSR 框架。

## Requirements

* 新增或完善 spec，使后续开发者能明确知道项目的整体架构边界。
* 新增 `.trellis/spec/frontend/`，沉淀 HTML 页面、模板打包与外部覆盖、SEO、性能、渐进增强和前端开发效率契约。
* 新增或完善 spec，明确论坛页面开发应优先服务端返回可索引 HTML，并用 htmx/hyperscript 做渐进增强。
* 新增或完善 spec，明确 HTML 模板默认内置于 exe；生产部署可配置外部模板目录；外部模板优先，缺失时回退内置模板。
* 新增或完善 spec，明确外部模板覆盖只能通过清洗后的相对 `.html` 路径访问，禁止路径穿越。
* 新增或完善 spec，明确新增页面/模板/组件时需要同步路由、模板解析测试、SEO 元信息、空状态和错误状态。
* 新增或完善 spec，明确性能规则：避免为单个页面引入大依赖；列表必须分页或限制；静态资源和组件响应需要考虑缓存语义；数据库访问要避免 N+1 和无索引查询。
* 新增或完善 spec，明确高效开发规则：优先复用现有 `renderTemplate/loadTemplate/apiFetch/showToast/postPath` 等项目模式；新增能力前先搜索现有模式；为跨层字段同步 DTO、Co、model、migration 和测试。
* 更新 spec index，让相关规范能被 `trellis-before-dev` 和后续任务快速发现。
* PRD/任务上下文需要说明这是 spec 文档任务，验收以文本完整性和可执行性为主。

## Acceptance Criteria

* [x] `.trellis/spec/frontend/` 中存在面向前端 HTML 模板、模板打包/外部覆盖、SEO、性能和渐进增强的可执行规范，标题为英文，正文以中文为主。
* [x] 规范明确记录：默认模板打包进 exe，`--template-path` 外部目录优先覆盖，缺失外部模板回退内置模板。
* [x] 规范明确记录：模板名必须清洗并限制为相对 `.html` 路径，禁止路径穿越。
* [x] 规范明确记录：新增页面需要考虑 title/description/canonical/语义结构/状态码/分页可发现性等 SEO 要点。
* [x] 规范明确记录：新增列表、详情、搜索、分类等论坛页面需要考虑分页、索引、缓存、N+1、静态资源体积等性能要点。
* [x] backend 或新增 spec index 已链接新增规范，后续任务能从 spec index 找到它们。
* [x] 文本检查通过：没有占位符、没有明显过期事实、没有与当前代码相冲突的路径或命令。
* [x] 如只修改文档/spec，可不运行 Go 测试；但需要在交付说明中说明实际检查方式。

## Definition Of Done

* 任务 PRD 已确认并启动。
* 新增/更新的 spec 足以指导后续功能开发，不只是原则口号。
* 每个新增 spec 至少包含：Scope/Trigger、Contracts、Good/Base/Bad Cases、Tests/Checks、Wrong vs Correct 或 Common Mistakes。
* spec index 链接完整。
* 完成后提交并按 Trellis 流程归档。

## Technical Approach

新增一个独立 frontend spec 层，而不是把内容塞进 backend index，也不新增 platform 层：

* `.trellis/spec/frontend/index.md`：前端/页面规范入口，链接 HTML 模板、SEO、性能、渐进增强和模板部署覆盖规范。
* `.trellis/spec/frontend/page-guidelines.md`：HTML 页面结构、SEO、语义化、可访问性、空状态/错误状态、htmx/hyperscript 渐进增强。
* `.trellis/spec/frontend/template-runtime-guidelines.md`：exe 内置模板、`--template-path` 外部模板优先级、路径安全、入口职责、部署检查和相关测试。
* `.trellis/spec/frontend/performance-guidelines.md`：页面体积、静态资源、列表分页、缓存语义、避免客户端渲染关键内容等性能规则。
* 更新必要的现有 spec index，使 backend 与 frontend 之间的职责边界清晰。

## Decision (ADR-lite)

**Context**: 当前实现已经支持 exe 内置模板与外部模板覆盖，但该能力主要存在于代码、测试和 README 中，尚未成为 Trellis spec 的显式开发契约。后续论坛功能会持续新增页面、组件、列表和详情，如果没有统一 spec，容易出现 SEO 不可索引、模板覆盖不一致、性能退化或跨层字段遗漏。

**Decision**: 只新增 `.trellis/spec/frontend/`。frontend 同时约束 HTML 模板、exe 内置模板、外部模板覆盖、SEO、性能和渐进增强；不新增 platform spec 层，避免当前阶段 spec 层级过多。规范必须可执行，包含具体文件路径、函数/flag 名称、错误边界、测试点和反例。

**Consequences**: 后续开发前置阅读成本略增，但会显著减少重复解释和跨层返工。外部模板覆盖能力会被视为稳定产品能力，而不是临时实现细节。

## Open Questions

* None.

## Out Of Scope

* 不在本任务中重写模板解析代码。
* 不在本任务中引入 Tailwind、daisyUI、React/Vue/Svelte 或新的前端构建链。
* 不在本任务中实现 sitemap、RSS、静态页面生成、CDN 配置或真实缓存中间件。
* 不在本任务中修复已有 `go test ./...` 的非相关失败。
* 不在本任务中重构 README 的所有过期内容，除非它直接影响本任务新增 spec 的准确性。

## Technical Notes

* Task directory: `.trellis/tasks/07-03-improve-specs-architecture-dev-seo-performance`
* Current template entry: `index.go`
* Embedded template root: `public/`
* Template flag: `--template-path`
* Template loader functions: `renderTemplate`, `loadTemplate`, `cleanTemplateName`
* Existing tests: `index_test.go`
* Existing backend spec index: `.trellis/spec/backend/index.md`
* Related archived PRD: `.trellis/tasks/archive/2026-07/07-01-forum-project-transformation/prd.md`
