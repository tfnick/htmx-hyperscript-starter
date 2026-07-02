# Database Guidelines

> 数据库连接、迁移、事务和模型访问规则。

---

## Overview

当前后台已经有完整数据库层，入口位于 `api/db/`。代码通过 `DBManager` 管理命名数据库，默认包含 `app` 与 `shared` 两个库；底层使用 `github.com/tfnick/sqlx`，支持 SQLite 与 Postgres。

默认迁移文件通过 Go `embed` 打包进可执行文件，运行时由 `AutoMigrate` 按数据库名和 driver 选择对应目录执行。

## Runtime Configuration

运行时数据库配置来自显式输入、环境变量和默认值的优先级合并：

| Database | Driver Env | DSN Env | Default |
| --- | --- | --- | --- |
| `app` | `APP_DB_DRIVER` | `APP_DB_DSN` | `sqlite`, `data/app.db` |
| `shared` | `SHARED_DB_DRIVER` | `SHARED_DB_DSN` | `sqlite`, `data/shared.db` |

支持的 driver：

- `sqlite`
- `postgres`

新增配置入口时应复用 `db.LoadRuntimeDatabases` 和 `db.NewDatabaseSpec`，不要在业务代码里手写环境变量优先级。

## Database Manager

`DBManager` 负责：

- 打开命名数据库：`OpenSpec`。
- 获取 `*sqlx.DB`：`GetDB`。
- 获取 `*sqlx.Engine`：`GetEngine`。
- 执行迁移：`AutoMigrate`。
- 重新打开连接：`Reopen`。
- 关闭所有连接：`Close`。

SQLite 初始化时会设置外键、WAL、同步模式、缓存和临时存储 pragma；Postgres 默认设置连接池上限。不要在业务层重复设置这些连接参数。

## Migration Layout

迁移目录按 driver 和数据库名组织：

```text
api/db/migrations/
  sqlite/
    app/
    shared/
  postgres/
    app/
    shared/
```

迁移规则：

- 只有 `app` 和 `shared` 是当前受支持的迁移数据库名。
- 迁移文件按文件名排序执行。
- 已执行迁移记录在 `schema_migrations` 表。
- SQLite 迁移是当前主要实现；Postgres 迁移必须至少具备 `001_schema.sql`，否则会被视为未准备好。
- `api/db/migration_convert.go` 可以从 SQLite 草拟 Postgres 迁移，但输出必须人工复核约束、触发器、向量、时间、blob 和 conflict 语义。

新增表或索引时必须提交迁移文件，不要只改 model 代码。

## Transaction Rules

当前事务只支持 `app` 数据库：

- DB 层入口：`db.WithTx(ctx, "app", fn)`。
- usecase 层入口：`fwusecase.WithAppTx(ctx, fn)`。
- 检查事务：`fwusecase.InAppTx(ctx)`。
- 注册提交后回调：`fwusecase.RegisterAfterCommit(ctx, fn)`。

嵌套事务会复用已有事务上下文。`RegisterAfterCommit` 只能在 active app transaction 中调用；如果事务失败，after-commit hook 不会执行。

业务代码优先使用 `fwusecase.WithAppTx`，这样 usecase context、request context 和事务上下文能一起传递。

## Model Access

SQL 访问应集中在 `api/models/`。model 层根据传入的 `context.Context` 从 `api/db` 获取 engine 或 active transaction；route 与 usecase 不直接导入 `api/db`。

推荐边界：

- route 读取 HTTP 输入并调用 usecase。
- usecase 决定是否开启事务、调用一个或多个 model 函数。
- model 函数执行 SQL 并返回 model/result。
- route 将 usecase 输出映射为 response DTO。

## Domain Events And Queue

领域事件使用 `api/framework/events` 和 `api/framework/queue`，并通过数据库表持久化事件和 delivery 状态。不要重新引入原始 EventBus，也不要把 goqite 依赖散落到业务代码里；raw goqite 只能留在 `api/framework/queue`。

项目自有表和 goqite 自有表要分开迁移。`007_add_goqite.sql` 只承载 goqite 所需表，不应混入 `scheduled_tasks`、`domain_events` 等项目表。

## Testing Requirements

涉及数据库的改动至少考虑：

- `api/db/config_test.go` 风格的配置优先级测试。
- `api/db/tx_test.go` 风格的事务、嵌套事务和 after-commit 测试。
- model 层针对新增查询、分页、状态转换和约束错误的测试。
- 迁移文件的可执行性测试，尤其是新增表和索引。

## Common Mistakes

- 在 route 或 usecase 中直接导入 `api/db`。
- 对 `shared` 数据库开启事务。
- 注册 after-commit hook 时没有 active app transaction。
- 新增 model 字段但忘记迁移。
- 把 Postgres 草稿迁移当作已验证生产迁移。
- 在业务代码里直接操作 goqite 表或 raw goqite package。
