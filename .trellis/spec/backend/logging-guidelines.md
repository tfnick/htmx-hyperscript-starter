# Logging Guidelines

> 后台结构化日志、请求日志和脱敏规则。

---

## Overview

当前日志能力由 `api/framework/logging` 提供，底层使用 `zerolog`。`logging.Init(isDevelopment)` 会初始化 `logs/app.log`，同时写入 stdout 和日志文件；开发模式使用 debug level，非开发模式使用 info level。

业务代码通过 `logging.For(component)` 获取带 `component` 字段和 timestamp 的 logger。不要在可复用 package 中随意使用 `fmt.Println`。

## Logger Lifecycle

入口初始化时应调用：

```go
if err := logging.Init(isDevelopment); err != nil {
    return err
}
defer logging.Close()
```

`logging.Close()` 会关闭当前日志文件并把 writer 恢复到 stdout。测试中如果需要重置日志输出，应在测试结束后关闭或恢复状态，避免影响其他测试。

## Component Names

组件名应短、稳定、便于过滤：

- `http`：请求日志和 HTTP handler 错误。
- `db`：数据库连接、迁移和重连。
- `events`：领域事件注册、发布、校验和 delivery。
- 其他后台任务或 provider 可以使用业务域名，例如 `scheduler`、`payment`、`llm`、`external-notification`。

不要把动态值放进 component，例如用户 ID、订单 ID 或 provider 实例名。

## Request Logging

HTTP 请求日志由 `api/framework/http/middleware.RequestLogger(surface)` 生成。默认字段包括：

- `surface`
- `request_id`
- `method`
- `route`
- `path`
- `status`
- `duration`

当 surface 为 `open-api` 且已解析 consumer 时，额外记录：

- `partner_id`
- `account_id`
- `environment`

请求处理返回 error 或状态码为 `5xx` 时使用 error level；其他完成请求使用 info level。

## Error Logging

内部 API handler 错误通过 `httpresponse.InternalServerError` 或 `NotFoundError` 记录，包含 route、path、method、surface、request_id 和安全的 client_error。

usecase 包装错误时，`fwusecase.LogErrorOf` 会递归取出 root cause。记录日志时使用 root cause；返回客户端时使用 usecase message。

## What To Log

- 服务启动、数据库连接和迁移结果。
- 请求完成摘要和 request_id。
- 后台任务、队列任务、领域事件 delivery 的状态变化。
- 第三方 provider 调用失败的安全摘要。
- 被拒绝的重复订阅、非法事件或非法业务状态。

## What Not To Log

- JWT、open-api key、OAuth token、密码、reset token、secret、cookie。
- 请求 body、表单正文、聊天内容、知识库正文、文件内容。
- 支付 webhook 原始 payload，除非经过脱敏和明确任务要求。
- 完整第三方凭证、连接串或带密码的 DSN。
- 大体量 event payload 或 notification body。

## Development Logging

开发模式可以开启 debug level，但调试输出仍应走 zerolog，并放在合适 component 下。临时日志在任务结束前清理；确实需要保留时补充本 spec 或对应 package 说明。

## Common Mistakes

- 在 `models`、`usecase` 或 provider 中留下 `fmt.Printf`。
- 重复记录同一个错误，导致 route、usecase、provider 三层都打 error。
- 记录 Authorization header 或 API key。
- 把 request path 当成唯一追踪手段，忘记传递 request_id。
- 在高频循环中输出 info 日志，造成生产噪音。
