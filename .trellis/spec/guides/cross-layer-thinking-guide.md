# Cross-Layer Thinking Guide

> **Purpose**: 在实现跨层功能前，把数据怎么流动、在哪验证、怎么报错先想清楚。

---

## The Problem

很多问题不是某一层写错，而是层与层之间没对齐：

- API 返回字段是 `reply_count`，前端读取的是 `replyCount`。
- 数据库允许空值，模板假设一定有值。
- 私有主题在 usecase 过滤了，但列表 API 或前端缓存绕过了过滤。
- 模板默认内置到 exe，但外部覆盖目录没有同名文件时没有正确回退。

跨层功能要先画清楚数据流，再决定每层只承担自己的职责。

## Map The Flow

先写出最小数据流，不需要画复杂图，能说清楚即可。

```text
Request -> Route -> Usecase -> Model/DB -> DTO -> Template/JS -> User
```

每个箭头都问三件事：

- 数据格式是什么？字段名、类型、空值、时间格式是否明确？
- 谁负责验证？谁负责权限或可见性过滤？
- 出错时向上一层返回什么语义？状态码、错误码、toast 文案是否一致？

## Boundary Checklist

| Boundary | Think About |
| --- | --- |
| Route -> Usecase | 参数清洗、登录态、权限、分页默认值和最大值 |
| Usecase -> Model/DB | 查询条件、索引、事务、可见性过滤、空值处理 |
| Model/DB -> DTO | 字段命名、时间格式、计数字段、敏感字段脱敏 |
| Backend -> Frontend | JSON shape、HTTP status、错误消息、缓存语义 |
| Template -> JS | DOM id/class、data attribute、空状态、重复绑定 |
| Built-in Template -> External Template | 相同相对路径、缺失回退、路径清洗、兼容迁移 |

## Forum-Specific Triggers

遇到这些变化时，默认按跨层功能处理：

- 新增主题、回复、分类、用户展示、通知、搜索、收藏或点赞字段。
- 调整 `visibility`、登录态、作者权限、管理员权限或私有内容访问规则。
- 新增页面需要同时改 HTML 模板、JS、API 和 Go 测试。
- 新增列表、分页、排序、搜索、计数或聚合字段。
- 新增或移动 `public/*.html`、`public/components/**/*.html`，并影响外部模板覆盖路径。

## Contract Questions

实现前先回答：

- API 请求字段是什么？默认值和最大值是什么？
- API 响应字段是什么？前端是否只依赖这些字段就能渲染？
- 空列表、未登录、无权限、资源不存在和服务错误分别怎么展示？
- 私有内容是否在后端就被过滤，而不是交给前端隐藏？
- 模板路径是否仍然能从 exe 内置模板回退，并允许外部目录覆盖？
- 新字段是否同步到 migration、model、usecase、route、template/JS 和测试？

## Common Mistakes

### Mistake 1: Implicit Field Mapping

Bad: 后端随手返回蛇形字段，前端按驼峰字段读取，没有测试覆盖。

Good: 在 route/DTO 层明确 JSON 字段名，前端按实际响应读取，并补响应断言。

### Mistake 2: Scattered Visibility Logic

Bad: 后端返回所有主题，前端用 `if (visibility === "private")` 隐藏。

Good: 可见性过滤在后端 usecase/query 层完成，前端只负责展示已授权数据。

### Mistake 3: Template Path Assumptions

Bad: 移动 `public/components/forum/thread-list.html` 后只改内置模板调用，忘记外部覆盖路径和测试。

Good: 路径变更在 PRD/spec 中说明兼容影响，并补内置读取、外部覆盖和缺失回退测试。

### Mistake 4: UI State Without API Semantics

Bad: 登录失败只在页面里写一个通用“失败”，忽略后端返回的具体原因。

Good: API 返回可展示错误消息，前端用 toast 展示，并保持页面跳转/停留语义明确。

## Tests To Consider

- [ ] Route/usecase 测试覆盖成功、未登录、无权限、not found、非法参数。
- [ ] 分页测试覆盖默认 page、默认 size、最大 size 和空列表。
- [ ] 前端 smoke 测试或模板渲染测试覆盖关键页面。
- [ ] JS 变更至少通过 `node --check public/extensions.js`。
- [ ] 模板运行时变更覆盖内置模板、外部模板优先、外部缺失回退和非法路径拒绝。
- [ ] SEO 相关页面检查 `<title>`、语义结构、canonical、公开/私有页面的索引语义。

## When To Update Specs

如果答案变成了“以后所有类似功能都应该这么做”，就不要只留在任务 PRD 里：

- 具体实现契约写到对应 layer spec，例如 backend 或 frontend。
- 思考清单、预检问题、容易忘的风险点写到 `guides/`。
- 新增 guide 或重大规则时更新 `.trellis/spec/guides/index.md`。
