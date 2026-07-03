# Quality Guidelines

> 后台质量门槛、架构守护和测试要求。

---

## Overview

后台改动应优先保持现有 Go/Echo/sqlx/zerolog 分层和测试风格。不要因为单个需求引入新的后端框架、ORM、前端构建链或全局重构。

质量目标是：分层清晰、response 契约稳定、数据库迁移可追踪、错误可映射、日志可排查、测试能覆盖关键行为。

## Required Commands

完成 backend 代码改动前：

- 对改动的 Go 文件运行 `gofmt`。
- 运行 `go test ./...`。
- 当入口、模块路径、路由挂载、嵌入资源、数据库初始化或构建参数变化时运行 `go build`。

如果当前工作区存在已知模块路径或入口装配漂移，导致 `go test ./...` 或 `go build` 不能代表本次改动结果，必须在交付说明中写明实际命令和阻塞错误。

纯文档/spec 改动不要求运行 Go 测试，但应做文本检查，确认没有遗留占位符和过期事实。

## Architecture Guards

`api/framework/archguard` 中的测试是后台架构契约的一部分。新增代码不能绕过这些边界：

- 核心层导入方向必须保持 `routes -> usecase -> models`。
- `routes` 不直接导入 `db`、`models`、`providers`。
- `usecase` 不直接导入 `db`、`routes`、`providers`、`framework/http`。
- `models` 不导入 `routes`、`usecase`、`framework/http`。
- `providers` 不导入 `db`、`models`、`routes`、`framework/http`。
- 内部 route 使用 response envelope helper，不直接 `c.JSON` 输出业务 payload。
- 内部 route 不直接返回 `models.*`。
- raw `goqite` 依赖只允许在 `api/framework/queue`。
- 已退役的 cookie session auth 符号不得重新出现。
- 业务实时推送必须走 notification 边界，不能随处直接导入 realtime。

## Testing Scope

按风险选择测试层级：

- route：使用 httptest/Echo context 验证绑定、状态码、response envelope、鉴权和分页。
- usecase：验证业务规则、事务边界、状态流转、领域事件和 provider port 调用。
- model/db：验证 SQL 查询、迁移、约束、事务复用和 after-commit hook。
- framework：验证 middleware、request context、error mapping、logging、cache、queue、realtime 等基础能力。
- provider：优先用 fake client 或本地 stub，不依赖真实外部服务。

新增公共能力时必须有 focused test；只改简单 DTO 映射时可以用现有 route/usecase 测试覆盖。

## Response And DTO Rules

- 内部 API 必须使用 `httpresponse` envelope。
- open-api 错误必须使用 open-api error envelope。
- route 层定义请求/响应 DTO，不直接暴露 `models.*`。
- usecase 输出面向业务语义，route 再转换为前端契约。
- 分页参数使用 `api/framework/http/request` 和 `api/framework/usecase` 的分页约定，默认 page 为 `1`，默认 page size 为 `10`，最大 page size 为 `50`。

## Database Rules

- 新增表、索引或约束必须提交迁移。
- 事务优先从 usecase 层开启。
- only `app` database supports transactions in current implementation。
- after-commit hook 只能注册在 active app transaction 中。
- Postgres 迁移草稿必须人工复核后才能视为可交付。

## Security Checklist

- 认证使用 JWT Bearer 或 open-api key 中间件，不恢复 cookie session auth。
- 权限使用 `authz` role/permission 能力，不在 route 中散落字符串判断。
- 不记录 token、密码、secret、API key、请求正文或支付原始 payload。
- 对 webhook 等大 payload 使用读取上限，避免无界内存读取。
- route 只向客户端返回安全 message，内部 cause 留给日志。

## Forbidden Patterns

- 在 route 中直接访问数据库或 provider。
- 在 usecase 中导入 Echo 或 HTTP response。
- 在 model 中拼 HTTP response 或读取 Echo context。
- 在业务代码中直接导入 raw EventBus 或 raw goqite。
- 在核心业务目录中新增通用 `helpers.go`、`responses.go` 或 guard test。
- 因局部需求引入第二套 logging、response envelope、pagination 或 transaction helper。
- 为了快速接入第三方服务，让 usecase 直接依赖具体 provider、SDK、第三方 webhook payload 或 provider error type。

## Review Checklist

提交 backend 改动前逐项确认：

- 分层导入是否符合 archguard。
- 外部服务是否通过 `api/usecase/integrations/*` 端口进入业务层，而不是直接绑定 provider。
- 是否复用了已有 framework helper，避免新增第二套 response、pagination、transaction、logging、auth 或 provider registry。
- route 是否有清晰请求 DTO、响应 DTO 和错误映射。
- usecase 是否返回标准错误码。
- model 是否承担 SQL，而不是 HTTP 或业务展示。
- 新 SQL 是否有迁移和测试。
- 日志是否包含 request_id 或足够定位字段，同时没有敏感信息。
- 测试是否覆盖成功、校验失败、未授权/无权限、not found 和冲突场景中至少相关的部分。
