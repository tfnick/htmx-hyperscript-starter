# Repo API Capabilities

## Existing Backend Capabilities

- 认证注册：`api/routes/auth.go` 和 `api/usecase/auth.go` 已提供注册、登录、刷新 token、退出、当前用户、密码重置。
- 社交登录：`api/usecase/auth_oauth.go` 已支持 Google/GitHub OAuth adapter、state、callback、结果兑换和用户创建/绑定。
- 用户上下文：`api/framework/http/middleware/auth.go` 与 `api/framework/http/context/usecase_context.go` 已能把当前用户映射到 usecase context。
- 通知/实时：`api/framework/realtime`、`api/usecase/notification.go`、`api/routes/notifications.go`、`api/routes/points.go` 提供用户级实时消息通道和通知能力。
- 积分：`api/usecase/points.go`、`api/models/points.go`、`api/routes/points.go` 提供用户积分余额和实时推送基础。
- 页面浏览：`api/models/page_view.go` 与 page view middleware 已存在，可复用到帖子浏览数或站点统计。
- 数据库/迁移：`api/db` 已支持 app/shared 数据库、SQLite/Postgres、embed migrations、事务和 after-commit hook。

## Current Forum Drift

- `public/index.html` 和 `public/components/forum/*` 仍是旧 htmx forum demo 的 UI。
- `index.go` 仍引用旧 `api/forum` 和 template resolver 入口。
- 当前工作区状态里 `api/forum` 已被删除，说明后续实现应基于新的 `routes -> usecase -> models` 后台架构，而不是恢复旧内存 store。

## Reuse Decisions

- 不重新实现登录、注册、OAuth。
- 发帖、回帖、互动等需要登录的动作直接复用现有 JWT 用户上下文。
- 回复通知、被提及通知或帖子互动通知优先复用现有 notification/realtime 能力。
- 鸡腿/点赞等社区积分激励如进入 MVP，应基于现有 points 能力扩展，不另起积分体系。
