# Spec Review For Architecture Decoupling Reuse

## Goal

结合当前完整代码和已有 `.trellis/spec/**`，从整体架构、解耦、防腐层、代码复用和全栈开发效率角度审查所有规范，并对规范做适当修改，让后续论坛功能开发更容易按现有架构落地，减少跨层漂移、重复实现和临时耦合。

## What I Already Know

* 用户明确要求创建一个新任务，而不是继续复用已完成的 spec enrichment 任务。
* 本任务目标是审查并修改规范，不是实现新的论坛业务功能。
* 现有 spec 层包括 `.trellis/spec/backend/`、`.trellis/spec/frontend/` 和 `.trellis/spec/guides/`。
* spec 标题需要使用英文，正文尽量使用中文。
* 当前项目是 Go + Echo + htmx/hyperscript + Pico.css/原生 CSS；不应因为规范审查引入新技术栈。
* 代码已经存在 `api/routes -> api/usecase -> api/models` 的业务分层。
* `api/framework/` 已包含 HTTP response、middleware、usecase context、logging、events、queue、realtime、authz、cache、archguard 等基础设施。
* `api/providers/` 已承载外部服务具体适配，`api/usecase/integrations/*` 已承载业务可依赖的端口定义。
* 前端以 `public/` HTML 模板、组件片段、`extensions.js` 和内置资源为主，默认打包到 exe，并允许 `--template-path` 外部 HTML 模板覆盖。
* 当前工作区有两个既有临时日志删除项，不属于本任务范围。

## Assumptions

* 本任务以 spec/guides 文档变更为主，不修改业务代码，除非审查发现规范无法与当前代码对齐且必须通过很小的文档辅助修正表达。
* “结合完整代码”意味着需要先盘点主要代码目录、架构守护测试、关键入口、前端模板和已有 provider/port 关系，再决定规范缺口。
* 规范需要更偏向可执行约束：明确职责边界、禁止的耦合方式、复用入口、跨层字段同步和测试检查点。
* 不新增 platform 规范层，继续保持 backend、frontend、guides 三类规范入口。

## Requirements

* 审查 `.trellis/spec/backend/**`、`.trellis/spec/frontend/**` 和 `.trellis/spec/guides/**` 的现有内容。
* 结合当前代码结构，检查规范是否准确反映 routes/usecase/models/framework/providers/front-end templates 的真实边界。
* 强化架构约束：明确业务层、基础设施层、provider 适配层、HTTP 层、模板层之间的职责。
* 强化解耦与防腐约束：业务层依赖端口和 framework 合同，不直接绑定具体 provider、Echo、HTTP DTO、外部 payload 或前端 DOM 细节。
* 强化复用约束：优先复用 framework helper、usecase context、pagination、response envelope、toast/API helper、模板 loader、组件片段和既有测试 helper。
* 强化全栈开发效率：新增字段/页面/API 时有明确的同步清单，避免 route/usecase/model/migration/template/JS/test 漏改。
* 规范修改保持标题英文，正文尽量中文。
* 新增或调整 spec 后，同步更新相关 index，保证后续 Trellis 上下文能发现。

## Acceptance Criteria

* [x] 任务 PRD 已创建并记录目标、约束、验收标准和技术笔记。
* [x] implement/check 上下文引用本次审查需要读取的 spec 文件。
* [x] 已盘点完整代码结构，并把发现映射到规范修改点。
* [x] backend spec 明确架构边界、端口/防腐、framework 复用、route/usecase/model/provider 禁止耦合点。
* [x] frontend spec 明确 HTML 模板、JS、CSS、组件片段与后端 API 的边界和复用入口。
* [x] guides spec 明确审查前思考清单，帮助后续任务从架构、解耦、防腐、复用和全栈效率角度自检。
* [x] 所有新增/修改 spec 的 Markdown heading 为英文。
* [x] 文本检查通过：没有占位符、明显过期路径、platform 规范层路径或与当前代码冲突的命令。
* [x] 如仅修改 spec 文档，可不运行 Go 测试，但必须记录实际检查方式。

## Definition Of Done

* 新任务已创建、启动，并包含 PRD 与上下文文件。
* 所有规范修改均能追溯到当前代码结构或已有 spec 缺口。
* 修改范围聚焦 `.trellis/spec/**` 和本任务目录，不触碰无关日志删除项。
* 完成后给出检查结果和剩余风险。

## Technical Approach

1. 先用 `rg --files` 和关键文件阅读建立代码结构地图。
2. 阅读所有 spec 文件，找出与当前代码不一致或不够可执行的部分。
3. 优先修改现有 spec；只有现有文件无法承载时才新增 guide/spec 文件。
4. 规范内容保持“短但可执行”：边界、入口、反例、检查点、测试建议。
5. 运行 heading、占位符、平台路径和 Markdown diff 检查。

## Decision (ADR-lite)

**Context**: 当前项目代码已经具备较清晰的 backend 分层、provider 适配、framework 基础设施和内置 HTML 前端，但规范仍需要进一步把这些真实边界转化为后续任务可执行的约束。否则论坛功能继续扩展时，容易出现 route 直接承载业务、usecase 绑定 provider、前端重复 fetch/toast、模板覆盖路径漂移等问题。

**Decision**: 继续在现有 backend/frontend/guides spec 中增强约束，不新增 platform spec 层，不引入新技术栈。本任务以规范审查和文档修改为主。

**Consequences**: 后续开发前阅读规范的成本会略有增加，但能减少跨层返工、重复实现和临时耦合；规范需要保持与代码演进同步，避免再次变成口号式文档。

## Out Of Scope

* 不实现新的论坛业务功能。
* 不重构 Go 代码、HTML 模板、JS 或 CSS。
* 不新增 Tailwind、daisyUI、React/Vue/Svelte 或新的构建链。
* 不处理当前工作区已有的 `tmp-codex-server.*.log` 删除项。
* 不新增 platform 规范层。

## Technical Notes

* Task directory: `.trellis/tasks/07-03-review-specs-architecture-decoupling-reuse`
* Spec roots: `.trellis/spec/backend/`、`.trellis/spec/frontend/`、`.trellis/spec/guides/`
* Key backend folders: `api/routes/`、`api/usecase/`、`api/models/`、`api/framework/`、`api/providers/`、`api/db/`
* Key frontend files: `index.go`、`public/*.html`、`public/components/**`、`public/extensions.js`、`public/styles.css`
* Existing architecture guard tests: `api/framework/archguard/*_test.go`

## Verification Notes

* Ran `git diff --check -- .trellis/spec .trellis/tasks/07-03-review-specs-architecture-decoupling-reuse`; passed with CRLF warnings only.
* Checked `.trellis/spec/**/*.md` headings for Chinese characters; passed.
* Searched for placeholder phrases and platform spec paths; no matches after PRD wording cleanup.
* Ran `python ./.trellis/scripts/task.py validate .trellis/tasks/07-03-review-specs-architecture-decoupling-reuse`; implement/check context files passed.
* Checked relative Markdown links under `.trellis/spec`; passed.
* Did not run Go tests because this task only updates Trellis spec/task documents.
