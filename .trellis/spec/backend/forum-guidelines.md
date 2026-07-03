# Forum Guidelines

> 论坛主题、回复、分类和可见性相关的后端契约。

---

## Scenario: Thread Visibility

### 1. Scope / Trigger

- Trigger: 修改论坛发帖、列表、详情、回复或数据库字段时，必须保持公开/私有主题的跨层契约一致。
- 涉及层级：`api/routes/forum.go` -> `api/usecase/forum.go` -> `api/models/forum.go` -> `forum_threads` migration。

### 2. Signatures

- API: `POST /api/forum/threads`
- Request fields: `category_slug`, `title`, `body`, `visibility`
- Usecase: `CreateForumThreadCmd{CategorySlug, Title, Body, Visibility}`
- DB: `forum_threads.visibility TEXT NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'private'))`
- Response DTO: thread summary/detail 必须输出 `visibility`

### 3. Contracts

- `visibility` 可为空；为空时等价于 `public`。
- `visibility=public`：主题进入公共聚合列表和分类列表，匿名用户可读详情。
- `visibility=private`：主题不进入公共聚合列表和分类列表；详情仅作者本人和管理员可读。
- 论坛 GET 详情路由需要支持 optional auth，这样带 token 的作者/管理员能读取私有主题，同时匿名访问公开主题仍然可用。
- 前端创建主题时可以传 JSON 或 form 字段；route 层要同时兼容 `c.Bind` 和 `c.FormValue` 的现有模式。

### 4. Validation & Error Matrix

| Condition | Error |
| --- | --- |
| `visibility` 不是空、`public` 或 `private` | `validation` |
| 匿名或普通非作者访问私有主题详情 | `not_found` |
| 匿名或普通非作者回复私有主题 | `not_found` |
| 主题不存在或已删除 | `not_found` |
| 创建主题未登录 | `unauthorized` |

私有主题对无权用户使用 `not_found`，不要返回 `forbidden`，避免泄露主题存在性。

### 5. Good/Base/Bad Cases

- Good: 作者创建私有主题后可直接进入详情页，详情 API 返回 `visibility:"private"`。
- Base: 未传 `visibility` 创建主题，后端保存为 `public`。
- Bad: 私有主题出现在 `GET /api/forum/threads` 或 `GET /api/forum/categories/:slug/threads`。
- Bad: 详情查询先增加浏览量再做可见性检查。

### 6. Tests Required

- usecase test: 默认创建公开主题；私有主题不在列表中；匿名/其他普通用户读详情和回复返回 `CodeNotFound`；作者和管理员可读。
- route test: 创建私有主题时 request/response 带 `visibility`；匿名详情返回 404，作者详情返回 200。
- frontend/template test: `/new-post` 路由渲染成功，列表页不再含旧 `#thread-form`。
- smoke test: 登录用户从 `/categories/:slug` 进入 `/new-post?category=:slug` 时默认选中对应分类。

### 7. Wrong vs Correct

#### Wrong

```go
thread, _ := models.GetForumThreadDetail(ctx.Std(), id)
models.IncrementForumThreadViewCount(ctx.Std(), id)
if thread.Visibility == "private" && thread.AuthorID != ctx.Actor.UserID {
    return fwusecase.E(fwusecase.CodeForbidden, "forbidden", nil)
}
```

#### Correct

```go
thread, _ := models.GetForumThreadDetail(ctx.Std(), id)
if !canViewForumThread(ctx, thread.AuthorID, thread.Visibility) {
    return fwusecase.E(fwusecase.CodeNotFound, "thread not found", nil)
}
models.IncrementForumThreadViewCount(ctx.Std(), id)
```

