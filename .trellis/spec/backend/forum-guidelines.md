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

## Scenario: Thread List And Reply Pagination

### 1. Scope / Trigger

- Trigger: 修改论坛主题列表、主题详情、回复列表、分页参数、分页响应或对应前端渲染时，必须保持列表分页和详情回复分页使用同一套分页契约。
- 涉及层级：`api/routes/forum.go` -> `api/usecase/forum.go` -> `api/models/forum.go` -> `public/extensions.js`。

### 2. Signatures

- API: `GET /api/forum/threads?page=<n>&page_size=<n>`
- API: `GET /api/forum/categories/:slug/threads?page=<n>&page_size=<n>`
- API: `GET /api/forum/threads/:id?page=<n>&page_size=<n>`
- Usecase: `ForumThreadsQry{Page, PageSize}`
- Usecase: `ForumThreadDetailQry{ID, CountView, PostPage, PostSize}`
- Response DTO: list response includes `pagination`; detail response includes `posts` and reply `pagination`.

### 3. Contracts

- 列表分页和详情回复分页都使用 query string 字段 `page` 与 `page_size`，不要为回复分页新增第二套 `post_page` / `post_size` API 参数。
- Route 层必须通过 `fwrequest.PageQuery(c)` 读取分页参数，Usecase 层必须通过 `fwusecase.NormalizePageQuery` 校验默认值和上限。
- 主题列表的 `pagination.total_items` 来自主题总数；主题详情的 `pagination.total_items` 来自该主题下已发布回复总数。
- 主题详情的 `/post-{id}-{page}` 前端 URL 中的 page 表示回复页码，不表示主题列表页码。
- 详情页返回的 `posts` 只能包含当前回复页，不要为了前端分页一次返回全部回复。
- 私有主题仍然先执行可见性检查；无权用户不得通过分页参数探测回复数量。

### 4. Validation & Error Matrix

| Condition | Error |
| --- | --- |
| `page <= 0` | `validation` |
| `page_size <= 0` | `validation` |
| `page_size > MaxPageSize` | `validation` |
| 主题不存在、已删除或无权访问 | `not_found` |
| 回复页超过总页数 | 返回空 `posts`，保留规范化后的 `pagination` |

### 5. Good/Base/Bad Cases

- Good: `GET /api/forum/threads/:id?page=2&page_size=10` 返回第 2 页回复，并返回 `pagination.page=2`、`page_size=10`、`total_items=reply_count`。
- Base: 未传分页参数时，详情页回复使用默认 `page=1` 和默认 `page_size`。
- Bad: 详情接口忽略 query 参数，导致 `/post-{id}-2` 仍然展示第 1 页回复。
- Bad: 详情接口一次返回全部回复，再让前端在浏览器里切片。

### 6. Tests Required

- usecase test: 创建超过默认页大小的回复，断言 `GetForumThreadDetail(PostPage, PostSize)` 返回稳定排序的目标页回复和正确 `PostPagination`。
- route test: 请求 `GET /api/forum/threads/:id?page=2&page_size=2`，断言 response 中 `posts` 与 `pagination` 对应第 2 页。
- validation test: 非法 `page` / `page_size` 通过现有分页校验返回 validation 语义。
- visibility test: 私有主题详情分页仍对匿名和无权用户返回 not found。

### 7. Wrong vs Correct

#### Wrong

```go
thread, _ := usecase.GetForumThreadDetail(ctx, usecase.ForumThreadDetailQry{
    ID: c.Param("id"),
})
```

#### Correct

```go
page := fwrequest.PageQuery(c)
thread, _ := usecase.GetForumThreadDetail(ctx, usecase.ForumThreadDetailQry{
    ID:       c.Param("id"),
    PostPage: page.Page,
    PostSize: page.PageSize,
})
```

