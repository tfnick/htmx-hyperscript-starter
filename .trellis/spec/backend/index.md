# Backend Development Guidelines

> 当前后台开发规范入口。标题使用英文，正文尽量使用中文。

---

## Overview

当前 `api/` 已经不是早期的轻量 demo 结构，而是一个按业务分层组织的 Go 后台：

- HTTP 层使用 Echo，路由代码位于 `api/routes/`。
- 业务编排位于 `api/usecase/`，通过 command/query 和 `Co` 输出结构表达用例边界。
- 数据访问和持久化模型位于 `api/models/`。
- 数据库连接、迁移和事务位于 `api/db/`。
- 跨层基础设施位于 `api/framework/`，例如 HTTP response、middleware、usecase context、logging、events、queue、realtime、authz、cache。
- 第三方实现适配位于 `api/providers/`，业务层通过 `api/usecase/integrations/*` 的端口对接。

后续 backend 开发必须先遵守这些已有边界，再考虑新增抽象。

## Pre-Development Checklist

开始写 backend 代码前：

- 阅读 [Architecture Guidelines](./architecture-guidelines.md)，如果改动跨越 route/usecase/model/framework/provider，或涉及防腐层、复用入口、全栈字段同步。
- 阅读 [Directory Structure](./directory-structure.md)，确认目标代码应该落在哪一层。
- 阅读 [Database Guidelines](./database-guidelines.md)，如果涉及 SQL、迁移、事务或 after-commit hook。
- 阅读 [Error Handling](./error-handling.md)，如果新增 route、usecase error、鉴权或 open-api 行为。
- 阅读 [Forum Guidelines](./forum-guidelines.md)，如果修改论坛主题、分类、回复、列表、详情或可见性行为。
- 阅读 [Logging Guidelines](./logging-guidelines.md)，如果新增请求日志、后台任务日志或外部集成日志。
- 阅读 [Quality Guidelines](./quality-guidelines.md)，确认测试、架构守护和响应 envelope 要求。
- 如果改动 `index.go` 的页面路由、`public/` HTML/CSS/JS、`/api/components/*` 或 `--template-path` 模板覆盖行为，同时阅读 [Frontend Development Guidelines](../frontend/index.md)。

## Core Contracts

- 内部请求路径的主要流向是 `routes -> usecase -> models`。
- `routes` 负责 Echo 绑定、鉴权中间件接入、请求/响应 DTO、HTTP 状态码和 response envelope。
- `usecase` 负责业务规则、事务边界、跨模型编排、领域事件发布和面向 route 的 `Co` 输出。
- `models` 负责表结构对应的数据类型和 SQL 访问，不返回 HTTP DTO。
- `framework` 负责可复用基础设施；不要把通用 helper、response 或架构守护文件散落到核心业务目录。
- `providers` 负责外部服务适配；业务层依赖端口，不直接依赖具体 provider。
- 外部系统通过 `api/usecase/integrations/*` 端口进入业务层；第三方 payload、SDK、签名和 provider 错误必须被 provider 层归一化。
- 新增跨层字段时按 migration/model/usecase/route/frontend/test 的顺序核对，避免只修当前层导致后续漂移。

## Quality Check

完成 backend 改动前：

- 对变更的 Go 文件运行 `gofmt`。
- 优先运行 `go test ./...`；如果当前分支存在模块路径或入口装配漂移导致无法运行，必须记录实际阻塞错误。
- 当入口、路由注册、嵌入资源、数据库初始化或构建参数变更时运行 `go build`。
- 新增 SQL 或表结构时补充迁移和对应测试。
- 新增 route 时确认 response envelope、错误映射、鉴权、分页和 DTO 边界。
- 新增跨层能力时确认 `api/framework/archguard` 中的边界测试仍然成立。

## Guidelines Index

| Guide | Purpose | Status |
| --- | --- | --- |
| [Architecture Guidelines](./architecture-guidelines.md) | 架构边界、解耦、防腐层、framework 复用和全栈字段同步 | Filled |
| [Directory Structure](./directory-structure.md) | 后台目录职责、分层边界、DTO 和 provider 归属 | Filled |
| [Database Guidelines](./database-guidelines.md) | DB manager、迁移、事务、模型访问和测试规则 | Filled |
| [Error Handling](./error-handling.md) | usecase 错误码、内部 API/open-api 响应 envelope 和 HTTP 映射 | Filled |
| [Forum Guidelines](./forum-guidelines.md) | 论坛主题、分类、回复、列表、详情和可见性契约 | Filled |
| [Logging Guidelines](./logging-guidelines.md) | zerolog 初始化、请求日志字段、组件日志和脱敏要求 | Filled |
| [Quality Guidelines](./quality-guidelines.md) | gofmt/test/build、架构守护、测试范围和 forbidden patterns | Filled |

## Related Spec Layers

- [Frontend Development Guidelines](../frontend/index.md)：适用于 `public/` 页面模板、htmx/hyperscript 增强、SEO、性能、exe 内置模板和 `--template-path` 外部覆盖。

## Language Rule

`.trellis/spec/` 中的 Markdown heading 必须使用英文；正文优先使用中文。这样可以保持目录索引稳定，同时让团队阅读具体约定时更顺畅。

## Known Drift

当前工作区中的 `api/` 代码大量使用 `github.com/tfnick/go-svelte-starter/...` 导入路径，而仓库根部 `go.mod` 仍可能保留旧模块路径。遇到这类构建漂移时，先用单独任务修复入口和模块路径，不要在普通业务任务里顺手改动整套工程装配。
