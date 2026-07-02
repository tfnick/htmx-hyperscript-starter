# Update Backend Specs From API

## Background

当前 `api/` 目录已经演进为较完整的后台代码结构，包括 HTTP routes、usecase、models、db、framework、providers、事件队列、实时通知、鉴权、开放接口等模块；而 `.trellis/spec/backend/` 仍停留在旧的 forum demo、模板 resolver 和内存存储描述上。

这会导致后续开发前读取 spec 时得到错误约定，尤其是目录分层、数据库事务、错误响应、日志和测试边界。

## Goal

基于当前工作区中的 `api/` 后台代码更新 `.trellis/spec/backend/`，让后端规范反映真实代码结构，并可作为后续开发前置指南。

## Requirements

- 所有 spec 文档标题和 Markdown heading 使用英文。
- spec 正文尽量使用中文，方便团队快速阅读。
- 更新范围限定在 `.trellis/spec/backend/` 和本任务目录。
- 以当前文件系统中的 `api/` 作为事实来源，不修改、不回滚 `api/` 现有改动。
- 移除旧的 `api/forum`、`api/templates`、无数据库等过期约定。
- 覆盖当前后台的核心约定：
  - `routes -> usecase -> models` 分层边界。
  - `api/framework` 中 HTTP、usecase、logging、events、queue、realtime、authz 等共享能力归属。
  - `api/db` 数据库管理、迁移、事务和 after-commit hook。
  - 内部 API 与 open-api 的响应 envelope 和错误映射。
  - zerolog 日志初始化、请求日志字段和敏感信息限制。
  - 架构守护测试、DTO 边界、go test/build/gofmt 质量门槛。

## Non-Goals

- 不修复 `go.mod`、`index.go` 与当前 `api/` 之间可能存在的模块路径或入口装配不一致。
- 不修改后台业务代码、迁移 SQL 或测试。
- 不新增新的技术栈或重构代码。

## Acceptance Criteria

- `.trellis/spec/backend/*.md` 不再引用已删除的 forum/template resolver 作为当前事实。
- 每个 backend spec 文件的所有 heading 均为英文。
- 文档正文以中文描述，并引用当前 `api/` 目录中真实存在的模块和约定。
- `implement.jsonl` 与 `check.jsonl` 使用真实 spec 文件，不保留模板示例行。
- 可通过文本检查确认 backend spec 无明显占位内容。

## Technical Notes

当前工作区中 `api/` 存在大量未提交改动，且 `go.mod` 仍显示旧模块路径 `github.com/zachatrocity/htmx-hyperscript-starter`，而新后台代码导入路径为 `github.com/tfnick/go-svelte-starter/...`。本任务只同步 spec，不试图修复构建入口或模块路径。
