# Error Handling

> 后台错误类型、HTTP 映射和响应 envelope。

---

## Overview

错误分类以 `api/framework/usecase` 为核心。usecase 层使用 `fwusecase.E(code, message, cause)` 生成可映射错误，route 层再通过 response helper 转成内部 API 或 open-api 的 HTTP 响应。

低层错误可以作为 `Cause` 保留给日志，但返回给客户端的 message 必须是安全、可理解的业务描述。

## Error Codes

当前标准 usecase 错误码：

| Code | Meaning | Typical HTTP |
| --- | --- | --- |
| `validation` | 参数、状态或业务校验失败 | `400` |
| `unauthorized` | 未登录、token 或 key 缺失/无效 | `401` |
| `forbidden` | 已认证但无权限或账号被禁用 | `403` |
| `not_found` | 资源不存在 | `404` |
| `conflict` | 状态冲突、重复提交、唯一性冲突 | `409` |
| `internal` | 未分类内部错误 | `500` |

不要在业务代码里发明临时字符串错误码；需要新错误类型时先更新 `api/framework/usecase/errors.go` 和本 spec。

## Internal API Envelope

内部 API 使用 `api/framework/http/response`：

成功响应：

```json
{"success": true, "data": {}}
```

失败响应：

```json
{"success": false, "error": {"code": "validation", "message": "invalid input"}}
```

内部 route 不直接调用 `c.JSON` 返回业务 payload，应使用：

- `httpresponse.OK`
- `httpresponse.Created`
- `httpresponse.OKEmpty`
- `httpresponse.OKMessage`
- `httpresponse.BadRequest`
- `httpresponse.Unauthorized`
- `httpresponse.Forbidden`
- `httpresponse.InternalUsecaseError`

架构测试会检查内部 route 的 direct `c.JSON` 使用。

## Open API Envelope

open-api 错误使用 `httpresponse.ErrorResponse(code, message)`，并由 `httpresponse.OpenAPIUsecaseError` 统一映射 usecase 错误。

open-api 鉴权中间件支持：

- `Authorization: Bearer <api-key>`
- `X-API-Key: <api-key>`

缺失或无效 key 返回 `401`，错误 code 为 `unauthorized`。

## Binding And Validation

route 层负责 HTTP 绑定失败的 `400` 映射，例如 body 格式错误、必填 query 缺失或非法分页参数。业务状态校验放在 usecase，使用 `CodeValidation` 或更具体的标准 code。

推荐：

```go
if err := c.Bind(&req); err != nil {
    return httpresponse.BadRequest(c, "invalid request body")
}
```

不要把数据库驱动错误、第三方 SDK 错误或内部 panic 文本直接返回给客户端。

## Authentication And Authorization

内部登录态由 JWT Bearer token 解析，并通过 `api/framework/http/middleware.RequireAuth` 写入当前用户上下文：

- 未提供 token：`401 not logged in`。
- token 无效或过期：`401 unauthorized`。
- 用户不存在：`401`。
- 用户禁用：`403`。

角色和权限使用 `RequireRole`、`RequireAdmin`、`RequirePermission`。业务层需要读取调用者时使用 `fwusecase.Context.Actor`，不要在 usecase 中读取 Echo context。

## Logging Errors

`httpresponse.InternalServerError` 和 `NotFoundError` 会记录结构化日志字段：surface、method、route、path、request_id 和 client_error。`fwusecase.LogErrorOf` 会剥离 usecase 包装，尽量把 root cause 留给日志。

客户端 message 与日志 error 要区分：客户端看到的是安全描述，日志中才保留可排查的 cause。

## Common Mistakes

- 在 usecase 中返回普通 `fmt.Errorf`，导致 route 只能映射成 `500`。
- 在 route 中手写 JSON envelope，绕过 `httpresponse`。
- 把 `Cause.Error()` 直接发给客户端。
- 内部 route 直接返回 `models.*`。
- open-api 和内部 API 混用 response helper。
