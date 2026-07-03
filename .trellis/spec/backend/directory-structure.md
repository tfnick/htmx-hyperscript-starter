# Directory Structure

> 后台目录职责和分层边界。

---

## Overview

当前后台以 `api/` 为主体，按照 HTTP、业务用例、数据模型、基础设施和外部 provider 分层。新增代码时先放入现有层级；只有当现有层级无法表达清楚职责时，才新增目录。

更完整的导入方向、防腐层、复用入口和全栈字段同步规则见 [Architecture Guidelines](./architecture-guidelines.md)。本文件只负责目录职责的快速定位。

## Directory Layout

```text
api/
  routes/       Echo handlers, request DTOs, response DTOs, route-level mapping
  usecase/      business commands, queries, Co outputs, orchestration
  models/       persistence models and SQL access
  db/           named database manager, embedded migrations, transactions
  framework/    shared infrastructure by capability
  providers/    concrete integrations for external services
```

`index.go` 或其他入口文件只负责初始化依赖、注册中间件和挂载路由。业务规则不要放在入口文件里。

## Layer Ownership

`api/routes/`：

- 只处理 HTTP 边界：参数读取、body bind、鉴权中间件、分页 query、请求/响应 DTO。
- 调用 `api/usecase`，不要直接访问 `api/db`、`api/models` 或 `api/providers`。
- 内部 API 使用 `api/framework/http/response` 的 envelope helper。
- `open_api_*` 路由可以按 open-api 的 envelope 单独处理，但仍应复用 open-api 错误工具。

`api/usecase/`：

- 负责业务规则、跨模型编排、事务边界、领域事件和 provider 端口调用。
- 接收 `api/framework/usecase.Context`，以 `SurfaceInternalAPI`、`SurfaceOpenAPI`、`SurfaceSystem` 区分调用来源。
- 对外返回 usecase 层的 `Co` 结构或明确的 command/query 结果。
- 不直接依赖 Echo、HTTP response、具体 provider 或 `api/db`。

`api/models/`：

- 负责持久化模型、SQL 查询和数据变更。
- 可以依赖 `api/db` 获取 engine 或事务上下文。
- 不依赖 `routes`、`usecase` 或 `framework/http`。
- 不返回给前端的 response DTO；route 必须显式转换。

`api/framework/`：

- 按能力拆分共享基础设施，例如 `http/response`、`http/middleware`、`http/context`、`usecase`、`logging`、`events`、`queue`、`realtime`、`authz`、`cache`。
- 通用 helper、response envelope、guard test 不应放在 `routes`、`usecase` 或 `models` 根目录。
- framework 能力要保持小而清晰，避免吸收业务规则。

`api/providers/`：

- 放置第三方服务的具体实现，例如支付、OAuth、OSS、LLM、embedding、外部通知等。
- provider 应实现 `api/usecase/integrations/*` 中定义的端口。
- provider 不应依赖 HTTP route、数据库模型或 framework HTTP 层。

## Import Direction

默认导入方向是：

```text
routes -> usecase -> models -> db
              |
              +-> usecase integration ports

providers -> usecase integration ports
framework -> shared capability only
```

架构守护测试会阻止常见反向依赖：

- `models` 不得导入 `routes`、`usecase`、`framework/http`。
- `usecase` 不得导入 `routes`、`providers`、`framework/http` 或直接导入 `api/db`。
- `routes` 不得导入 `api/db`、`api/models`、`api/providers`。
- `providers` 不得导入 `api/db`、`api/models`、`api/routes`、`framework/http`。

如确实需要跨层能力，先把能力沉淀到 `api/framework/<capability>` 或 usecase port，再由具体层依赖。

## DTO Boundary

内部 route 不能把 `models.*` 结构直接作为 JSON payload 返回。route 层应该定义 response DTO，并通过转换函数从 usecase 的 `Co` 或结果结构生成。

推荐形态：

```go
type orderResponse struct {
    ID     string `json:"id"`
    Status string `json:"status"`
}

func toOrderResponse(co usecase.OrderCo) orderResponse {
    return orderResponse{ID: co.ID, Status: co.Status}
}
```

这样可以避免数据库字段、内部状态或未来迁移细节泄漏到前端契约。

## Route Pattern

route 文件通常包含：

- 请求 DTO。
- 响应 DTO。
- `to*Response` 映射函数。
- Echo handler。
- 必要的 route-level 鉴权或分页读取。

handler 内部优先使用 `fwcontext.InternalUsecaseContext(c)` 或 `fwcontext.OpenAPIUsecaseContext(c)` 构造 usecase context，再调用 usecase。错误统一交给 `httpresponse.InternalUsecaseError` 或 `httpresponse.OpenAPIUsecaseError`。

## Usecase Pattern

usecase 文件通常包含：

- command/query 输入结构。
- `Co` 输出结构。
- 业务函数或 service 函数。
- 事务边界与领域事件发布。

需要事务时使用 `fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error { ... })`。需要事务提交后执行副作用时使用 `fwusecase.RegisterAfterCommit`，不要在提交前发送不可回滚的外部通知。

## Model Pattern

model 文件负责 SQL 读写、数据行结构和必要的转换。模型层可以处理数据库错误归一化，但不应该生成 HTTP 状态码、HTTP response body 或 Echo error。

命名应接近业务表和数据含义，例如 `User`、`Order`、`DomainEvent`、`Notification`、`DictionaryType`。复杂查询返回专门的 model/query result，再由 usecase 转成业务输出。

## Provider Pattern

新增外部集成时先在 `api/usecase/integrations/<domain>/ports.go` 定义端口，再在 `api/providers/<domain>/` 提供具体实现。usecase 依赖端口注册表或接口，避免把第三方 SDK 类型扩散到业务层。

provider 内部可以使用第三方 SDK、HTTP client、签名校验和 provider-specific payload；这些类型不应出现在 route、model、HTML template、JS 或持久化业务表中。需要保存 provider 信息时，先归一化成业务字段或脱敏 snapshot。

## Common Mistakes

- 在 route 里直接调用 `models.*` 或 `db.*`。
- 在 usecase 里导入 Echo、HTTP response 或具体 provider。
- 为了复用方便把 `helpers.go`、`responses.go` 放进核心业务目录。
- 让 provider 读取数据库或拼 HTTP response。
- 把 model 结构原样返回给前端。
