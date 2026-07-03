# Spec Writing Thinking Guide

> **Purpose**: 在更新 `.trellis/spec/**` 前，判断规则应该写在哪、写到什么深度，以及如何保持后续任务可执行。

---

## The Problem

spec 的价值不在于“看起来完整”，而在于后续开发能按它少走弯路。常见问题是：

- 把原则性口号写进 spec，却没有字段、路径、错误和测试点。
- 把详细实现契约写进 `guides/`，导致真正的 layer spec 读不到。
- 标题语言和正文语言混乱，后续上下文注入时可读性变差。
- 新增功能后只更新 PRD，没有把稳定约定沉淀到 `.trellis/spec/`。

## Language Contract

- Markdown heading 必须使用英文，例如 `## Runtime Contracts`。
- 正文尽量使用中文，解释原因、边界、例外和检查方式。
- 代码、命令、路径、API 字段、flag、表格列名可以保留英文。
- 如果引用已有函数或路径，保持真实名称，例如 `renderTemplate`、`--template-path`、`public/components/**`。
- 不要留下占位短语；不确定的内容写入任务 PRD 的 open question。

## Location Decision

| Learning | Put In |
| --- | --- |
| 具体 API 字段、数据库字段、错误状态、命令参数、模板路径 | 对应 layer spec |
| 前端 HTML、SEO、模板覆盖、性能、渐进增强规则 | `.trellis/spec/frontend/` |
| 后端分层、数据库、错误处理、日志、论坛权限规则 | `.trellis/spec/backend/` |
| “开发前要想什么”的提醒和清单 | `.trellis/spec/guides/` |
| 一次性任务背景、尚未确认的取舍 | 当前任务 `prd.md` |

如果一条规则可以被测试或在代码里定位到文件/函数，优先写入 layer spec，而不是 guide。

## Depth Decision

遇到以下触发点，spec 需要写到 code-spec 深度：

- 新增或修改命令、flag、API endpoint、请求/响应字段。
- 新增或修改数据库 schema、migration、索引或查询约束。
- 改变模板加载、外部覆盖、缓存、安全边界或部署行为。
- 改变跨层可见性、鉴权、分页、错误语义。

code-spec 深度至少包含：

- Scope / Trigger
- Signatures 或相关入口
- Contracts
- Validation / Error behavior
- Good / Base / Bad cases
- Tests / Checks
- Wrong vs Correct 或 Common Mistakes

## Update Flow

1. 先读目标 spec index，确认该 layer 的 pre-development checklist。
2. 搜索现有规则，避免重复写同一条约束。
3. 判断规则落点：layer spec、guide、任务 PRD，三者不要混写。
4. 修改正文时保持 heading 英文，说明文字中文。
5. 新增文件后同步更新对应 index。
6. 文档任务完成前检查 heading、链接、路径、命令和当前代码是否一致。

## Good / Base / Bad Cases

- Good: `--template-path` 外部模板优先级写入 frontend template runtime spec，并包含路径清洗、缺失回退和测试点。
- Good: “跨层字段变更前先画数据流”写入 cross-layer guide，因为它是思考提醒。
- Base: 只修正文档错别字时不新增完整 code-spec 七段，但仍保证标题英文、正文中文。
- Bad: 把 `cleanTemplateName` 的详细路径校验矩阵只写进 guide，导致实现前 checklist 读不到具体契约。
- Bad: 新增 spec 文件但不更新 index，后续 `trellis-before-dev` 无法自然发现。

## Review Checklist

- [ ] 所有 Markdown heading 使用英文。
- [ ] 正文解释尽量使用中文。
- [ ] 具体契约写在 layer spec，思考提醒写在 guide。
- [ ] 新增文件已加入对应 `index.md`。
- [ ] 链接、路径、函数名、flag 名与当前代码一致。
- [ ] 没有占位符、过期事实或和当前技术栈冲突的建议。
- [ ] 文档说明不要求引入未决策的新框架、构建链或运行时依赖。
