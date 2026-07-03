# Thinking Guides

> **Purpose**: 在写代码或更新 spec 前，帮助开发者先想清楚边界、复用、文档落点和风险点。

---

## Guide Boundaries

`guides/` 不是具体实现规范的替代品。它只回答“开始前应该想什么”，不重复每个 layer spec 里的详细契约。

| Type | Location | Content Rule |
| --- | --- | --- |
| Code spec | `.trellis/spec/backend/`、`.trellis/spec/frontend/` | 写清楚签名、字段、错误、测试点和反例 |
| Thinking guide | `.trellis/spec/guides/` | 用短清单提醒先搜索、先画流、先判断落点 |

如果一条内容是在规定“代码必须怎么写”，放到对应 layer spec；如果是在提醒“写之前别忘了想什么”，放到本目录。

## Available Guides

| Guide | Purpose | When To Use |
| --- | --- | --- |
| [Architecture Review Thinking Guide](./architecture-review-thinking-guide.md) | 从架构、解耦、防腐、复用和全栈效率角度做预检 | 功能跨多层、接入 provider、调整 framework 或新增端到端字段前 |
| [Code Reuse Thinking Guide](./code-reuse-thinking-guide.md) | 避免重复实现，先找现有 helper、样式、API 和模板入口 | 新增函数、组件、常量、CSS 模式、脚本工具或批量改类似文件前 |
| [Cross-Layer Thinking Guide](./cross-layer-thinking-guide.md) | 梳理 API、usecase、model、DB、template、JS 之间的数据流 | 功能跨越 3 个以上层，或字段会在前后端之间流动时 |
| [Spec Writing Thinking Guide](./spec-writing-thinking-guide.md) | 判断 spec 更新应该写在哪、写到什么深度、如何保持可执行 | 新增/更新 `.trellis/spec/**` 或任务沉淀项目规则时 |

## Quick Reference

### When To Think About Architecture

- [ ] 代码跨越 route、usecase、model、framework、provider 或 frontend 多层。
- [ ] 需要新增第三方 provider、webhook、OAuth、支付、OSS、LLM、embedding 或通知能力。
- [ ] 准备新增通用 helper、registry、middleware、response、pagination、transaction、cache 或 realtime 能力。
- [ ] 不确定某段逻辑属于业务规则、基础设施、provider 适配还是页面展示。

阅读 [Architecture Review Thinking Guide](./architecture-review-thinking-guide.md)。

### When To Think About Cross-Layer Issues

- [ ] 功能涉及 API、usecase、model、DB、template、JS、CSS 中的 3 层以上。
- [ ] 同一个字段在数据库、DTO、JSON、HTML data attribute 或 JS state 中转换。
- [ ] 后端返回格式和前端展示格式不完全一致。
- [ ] 可见性、登录态、分页、错误语义会穿过多层。
- [ ] 不确定逻辑应该放在 route、usecase、template 还是 JS。

阅读 [Cross-Layer Thinking Guide](./cross-layer-thinking-guide.md)。

### When To Think About Code Reuse

- [ ] 正在写一个和现有函数、模板、样式或测试很像的东西。
- [ ] 同一种变更需要改 3 个以上文件。
- [ ] 正在新增常量、配置、路径、CSS class、API helper 或 toast/loading 逻辑。
- [ ] 正在复制一段代码再改几个字段。
- [ ] 正在改模板路径、组件路径或静态资源路径。

阅读 [Code Reuse Thinking Guide](./code-reuse-thinking-guide.md)。

### When To Think About Spec Updates

- [ ] 任务暴露了新的跨层契约、运行时 flag、模板路径、API 字段或错误语义。
- [ ] 实现中做了一个未来会被复用的设计选择。
- [ ] 修 bug 后发现以后应该提前检查某个条件。
- [ ] 新增或调整 `.trellis/spec/**`，需要保证标题语言、正文语言和落点一致。

阅读 [Spec Writing Thinking Guide](./spec-writing-thinking-guide.md)。

## Pre-Modification Rule

> **Before changing any reusable value, search first.**

改路径、字段、常量、模板名、CSS class、API endpoint、flag、错误码、缓存头之前，先搜索现有用法：

```bash
rg "value_to_change"
```

如果搜索结果跨越多个层，先补一张最小数据流清单，再动代码。

## Language Rule

`.trellis/spec/**` 的 Markdown heading 使用英文，正文尽量使用中文。表格列名、代码、命令、路径、API 字段名可以保留英文；解释性段落优先中文。

## Update Rule

新增 guide 时同步更新本索引。guide 应该保持短、可扫读、能指向具体 spec；不要把完整 API 设计、数据库字段矩阵或模板运行时契约写进 `guides/`。
