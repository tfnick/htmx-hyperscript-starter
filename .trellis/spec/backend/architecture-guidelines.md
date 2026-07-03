# Architecture Guidelines

> 后台架构边界、解耦、防腐层、复用入口和全栈协作契约。

---

## Overview

当前后台代码已经形成比较明确的分层：

- `api/routes/` 承接 Echo/HTTP 边界，定义 request/response DTO，并调用 usecase。
- `api/usecase/` 承接业务规则、事务编排、领域事件、权限判断和面向 route 的 `Co` 输出。
- `api/models/` 承接 SQL、持久化结构和查询结果。
- `api/framework/` 承接跨业务基础设施能力，例如 response、middleware、pagination、context、transaction、events、queue、logging、authz、cache、realtime。
- `api/usecase/integrations/*` 定义业务层可理解的端口，`api/providers/*` 实现第三方适配。

后续新增能力时，优先让代码贴合这些真实边界；如果一个需求让边界变模糊，先把缺失能力放到合适的 framework capability 或 usecase integration port，再让业务层依赖它。

## Scope / Trigger

修改以下内容时必须阅读本规范：

- 新增或调整 route/usecase/model/provider/framework 之间的导入关系。
- 新增外部服务、第三方 SDK、webhook、OAuth、支付、OSS、LLM、embedding 或通知适配。
- 新增跨层字段、列表、分页、权限、可见性、事务、副作用或领域事件。
- 修改 `api/framework/archguard/*_test.go` 覆盖的任何边界。
- 为了复用新增 helper、registry、response、middleware、queue、cache 或 realtime 能力。

## Layer Contracts

| Layer | Owns | Must Not Own |
| --- | --- | --- |
| `routes` | Echo 绑定、请求 DTO、响应 DTO、HTTP 状态码、response envelope、route-level auth | SQL、事务、provider SDK、业务规则、model JSON |
| `usecase` | command/query、业务校验、权限、事务、领域事件、`Co` 输出、integration port 调用 | Echo context、HTTP response、具体 provider、直接 `api/db` |
| `models` | 持久化结构、SQL、查询结果、数据库错误归一化 | HTTP DTO、usecase 规则、Echo、response envelope |
| `framework` | 可复用基础设施能力和架构守护 | 具体业务流程、产品规则、页面展示逻辑 |
| `providers` | 第三方 payload、SDK、签名、重试分类、provider response normalize | 数据库访问、HTTP route、业务决策、前端 DTO |

默认调用方向保持：

```text
routes -> usecase -> models -> db
              |
              +-> usecase/integrations ports

providers -> usecase/integrations ports
framework -> shared capability only
```

## Anti-Corruption Contracts

第三方服务只能通过 usecase integration port 进入业务层：

- 端口定义放在 `api/usecase/integrations/<domain>/ports.go`。
- 具体实现放在 `api/providers/<domain>/<provider>/`。
- provider 可以理解第三方 payload、SDK、header、签名、状态码和重试分类。
- usecase 只接收端口层的 `ProviderConfig`、request、result、normalized webhook 等内部语义。
- 第三方原始 payload 不应扩散到 route、model、template 或前端；需要持久化时保存经过脱敏和业务归一化的 snapshot。

HTTP 只能停留在 route/framework HTTP 层：

- usecase 不导入 Echo，不读取 `echo.Context`，不调用 `httpresponse`。
- route 使用 `fwcontext.InternalUsecaseContext(c)` 或 `fwcontext.OpenAPIUsecaseContext(c)` 构造 usecase context。
- route 错误统一通过 `httpresponse.InternalUsecaseError` 或 `httpresponse.OpenAPIUsecaseError` 映射。

数据库只能停留在 model/db 入口：

- route 不导入 `api/db` 或 `api/models`。
- usecase 不直接导入 `api/db`；事务使用 `fwusecase.WithAppTx`。
- model 根据传入 `context.Context` 获取 engine 或 active transaction。

实时、事件和队列也需要防腐：

- 业务实时推送走 notification 边界，不在任意业务文件直接导入 `api/framework/realtime`。
- raw `goqite` 只允许在 `api/framework/queue`。
- 已退役 EventBus 和 cookie session auth 不得重新引入。

## Reuse Contracts

新增能力前先搜索并优先复用这些入口：

| Need | Reuse |
| --- | --- |
| 内部 API 成功/失败 envelope | `api/framework/http/response` |
| 分页 query 读取 | `api/framework/http/request.PageQuery` |
| 分页默认值、最大值和结果 | `api/framework/usecase.NormalizePageQuery`、`NewPageResult` |
| usecase 错误分类 | `fwusecase.E`、`CodeOf`、`MessageOf`、`LogErrorOf` |
| HTTP -> usecase context | `api/framework/http/context` |
| 事务和 after-commit | `fwusecase.WithAppTx`、`RegisterAfterCommit` |
| not found 归一化 | `api/framework/data/modelerror.ErrNotFound` |
| 日志 | `api/framework/logging.For(component)` |
| 权限 | `api/framework/authz` 与 HTTP auth middleware |
| provider 错误 | `api/framework/integrations/providererror` |

不要因为局部需求新增第二套 response、pagination、transaction、logging、auth、provider registry 或 error helper。

## Full-Stack Change Flow

新增一个会穿过前后端的字段或能力时，按下面顺序检查，不要只改当前报错的文件：

1. Migration: 是否需要新增表、列、索引、约束或 seed。
2. Model: 是否新增 db struct 字段、query result、SQL select/insert/update。
3. Usecase: 是否新增 command/query、业务校验、权限、事务、`Co` 输出。
4. Route: 是否新增 request/response DTO、mapper、HTTP error mapping、pagination。
5. Frontend: 是否新增模板字段、DOM 节点、`extensions.js` 状态、API 调用和空/错状态。
6. Tests: 是否覆盖 model/usecase/route/frontend/template 的关键断言。
7. Spec: 是否需要把新的稳定契约沉淀回 backend/frontend/guides。

如果其中任一步暂时不做，必须在任务 PRD 写明原因和风险，避免后续误以为已经完整支持。

## Archguard Mapping

`api/framework/archguard` 是架构契约，不是普通单元测试。修改相关代码时要确认这些守护仍然成立：

- 核心层导入方向保持 `routes -> usecase -> models`。
- `routes` 不直接导入 `db`、`models`、`providers`。
- `usecase` 不直接导入 `db`、`routes`、`providers`、`framework/http`。
- `models` 不导入 `routes`、`usecase`、`framework/http`。
- `providers` 不导入 `db`、`models`、`routes`、`framework/http`。
- 内部 route 使用 response envelope helper，不直接 `c.JSON` 输出业务 payload。
- 内部 route 不直接返回 `models.*`。
- raw `goqite`、raw realtime、retired EventBus 和 cookie session auth 不得绕过指定边界。

需要调整 archguard 时，先写明“为什么当前边界不再成立”，再更新本 spec 和对应测试；不要只为让测试通过而放宽规则。

## Good / Base / Bad Cases

- Good: 新增支付 provider 时，先扩展 `api/usecase/integrations/payment/ports.go`，再在 `api/providers/payment/<provider>/` 适配第三方 payload。
- Good: 新增论坛列表字段时，model SQL 一次返回摘要字段，usecase 转成 `Co`，route 转成 response DTO，前端直接渲染，不追加 N+1 请求。
- Good: 发回复后发送通知时，在 app transaction 内注册 `RegisterAfterCommit`，提交成功后再发不可回滚副作用。
- Base: 新增一个只影响 route 的轻量 query 参数，仍使用 `fwrequest.PageQuery` 或现有 request helper 读取。
- Bad: 在 usecase 中导入 provider SDK 并解析第三方 webhook 原始 JSON。
- Bad: route 直接调用 `models.ListForumThreads`，然后把 model struct 放进 `httpresponse.OK`。
- Bad: 为一个业务文件新增 `helpers.go` 放通用 response 或 pagination 逻辑。

## Tests / Checks

- 改动导入边界、DTO、response envelope、queue/realtime/event/auth 入口时，运行 `go test ./api/framework/archguard` 或等价架构守护测试。
- 改动 route DTO 或 response envelope 时，补 route 测试断言 JSON shape。
- 改动 usecase 权限、事务或 after-commit 时，补 usecase 测试覆盖成功和失败路径。
- 改动 provider 时，用 fake client/stub 覆盖 request normalize、错误分类、签名或 retryable 语义。
- 改动跨层字段时，至少覆盖 route/usecase/model 中最能防止漂移的一层测试，并记录未覆盖层的原因。

## Wrong vs Correct

### Wrong

```go
import "github.com/tfnick/go-svelte-starter/api/providers/payment/creem"

func CreateCheckout(ctx fwusecase.Context, cmd CheckoutCmd) error {
    adapter := creem.NewAdapter(nil)
    _, err := adapter.CreatePayment(ctx.Std(), cfg, req)
    return err
}
```

usecase 直接绑定具体 provider，第三方 SDK 和 provider 错误会扩散到业务层。

### Correct

```go
adapter, ok := registeredPaymentAdapter(cfg.AdapterKey)
if !ok {
    return fwusecase.E(fwusecase.CodeValidation, "payment provider is not configured", nil)
}
result, err := adapter.CreatePayment(ctx.Std(), cfg, req)
```

usecase 依赖端口和 registry，具体 provider 只在装配或 provider 包内出现。

### Wrong

```go
thread, _ := models.GetForumThreadDetail(ctx, id)
return httpresponse.OK(c, thread)
```

route 直接暴露 model，数据库字段和内部结构会变成前端契约。

### Correct

```go
thread, err := usecase.GetForumThreadDetail(ctx, qry)
if err != nil {
    return httpresponse.InternalUsecaseError(c, err)
}
return httpresponse.OK(c, ToForumThreadDetailResponse(thread))
```

route 只暴露显式 response DTO，业务输出和前端契约之间有可维护的转换层。
