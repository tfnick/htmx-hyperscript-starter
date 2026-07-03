# Review Forum Code Standards And Refactor Opportunities

## Goal

检查论坛及相关新功能是否符合当前 Trellis backend/frontend 代码规范，识别代码结构、跨层契约、测试覆盖、模板运行时和性能方面的偏差，并判断是否需要做重构或拆分后续修复任务。

## What I Already Know

- 用户希望创建一个新任务，检查“论坛等新功能”是否符合代码规范，以及是否需要重构。
- 现有代码库已经包含论坛相关实现：`api/models/forum.go`、`api/usecase/forum.go`、`api/routes/forum.go`、论坛迁移、论坛测试和 `public/components/forum/*`。
- 已有进行中的任务 `07-02-nodeseek-style-forum-basics` 记录了论坛基础功能的原始 PRD。
- 当前 git 工作区在创建本任务前已有两个日志文件删除：`tmp-codex-server.err.log`、`tmp-codex-server.out.log`；本任务不应回滚这些既有变更。

## Requirements

- 审查论坛相关 backend 分层是否遵守 `routes -> usecase -> models` 边界。
- 审查论坛 API 的 response envelope、错误映射、鉴权、分页、DTO 边界和私有主题可见性契约。
- 审查论坛数据库迁移、SQL 查询、事务边界和索引是否符合 backend database 规范。
- 审查 frontend 模板、页面路由、htmx/hyperscript、SEO/语义结构、外部模板覆盖和 JS null-safe 约定。
- 审查测试覆盖是否覆盖 model/usecase/route/template 的关键行为，尤其是私有主题、列表过滤、详情访问、创建和回复。
- 运行项目质量检查，并把失败项按“必须修复 / 建议重构 / 可后续处理”分类。
- 如发现小范围、低风险且明确违反规范的问题，可以在用户确认范围后修复；较大重构应先形成建议和拆分计划。

## Acceptance Criteria

- [x] 新任务目录和 PRD 已创建。
- [x] 适用的 backend/frontend spec 已读取并作为审查依据。
- [x] 论坛相关实现文件和测试已审查。
- [x] 至少运行 `go test ./...`；如失败，记录具体失败原因和是否由论坛相关代码引起。
- [x] 如涉及 JS，运行 `node --check public/extensions.js` 或记录无需运行的原因。
- [x] 输出一份审查结论：合规项、问题项、重构建议、优先级和建议下一步。
- [x] 如做代码修改，修改后重新运行相关检查，并更新任务文档。

## Definition Of Done

- 审查结论可追溯到具体文件、规范或测试结果。
- 不回滚用户已有变更。
- 对需要重构的事项给出明确范围和优先级，而不是泛泛建议。
- 如果发现需要沉淀的新规范，标记是否需要后续更新 `.trellis/spec/`。

## Out Of Scope

- 不重新设计论坛产品需求。
- 不大规模重写论坛功能，除非用户明确同意把本任务从审查扩展为实现重构。
- 不修复与论坛及近期新功能无关的历史问题。

## Technical Notes

- Applicable specs include `.trellis/spec/backend/index.md`, `.trellis/spec/backend/forum-guidelines.md`, `.trellis/spec/frontend/index.md`, and related quality/database/error/template/performance guides.
- Current task directory: `.trellis/tasks/07-03-review-forum-code-standards-refactor`.
- Initial complexity: moderate, because the review spans backend, frontend, database, tests, and cross-layer contracts.

## Review Findings

### What Looks Compliant

- Forum backend follows the main `routes -> usecase -> models` split: `api/routes/forum.go` defines HTTP DTO/mappers and calls usecase; `api/usecase/forum.go` owns validation, authorization, transactions and notification after-commit; `api/models/forum.go` owns SQL and persistence structs.
- Forum routes use internal response helpers (`httpresponse.OK`, `Created`, `InternalUsecaseError`) and do not directly return model structs.
- Forum visibility contract is mostly aligned with the spec: `visibility` is present in request/response/DB, public list queries filter `visibility = 'public'`, optional auth is enabled for detail reads, and unauthorized private reads map to `not_found`.
- View count is incremented after the private-thread visibility check, matching the explicit bad/good case in `.trellis/spec/backend/forum-guidelines.md`.
- Focused forum checks passed:
  - `go test ./api/usecase -run Forum -count=1`
  - `go test ./api/routes -run Forum -count=1`
  - `go test . -run 'Test(Post|Login|Register|NewPost|LoadTemplate)' -count=1`
  - `go test ./api/models -count=1`
  - `node --check public/extensions.js`
  - `gofmt -l api/models/forum.go api/usecase/forum.go api/routes/forum.go api/usecase/forum_test.go api/routes/forum_test.go index.go index_test.go`

### Must Fix / High Priority

- `api/usecase/forum.go` has an update-reply correctness bug: `UpdateForumPost` updates the row, then calls `GetForumThreadDetail` with default pagination and searches only the first page of replies. If the updated reply is not on that page, the update can persist but the API returns `not_found`. This needs a targeted refactor and regression test.
- Full `go test ./...` fails today, so the repo is not globally compliant yet. Failures observed:
  - `api/framework/archguard`: `api/routes/experiment_sql.go` imports `api/db`; `api/routes/support_console.go` imports `api/models`.
  - `api/usecase`: KB embedding/support-answer tests fail with an unexpected embedding provider and an async panic. These look unrelated to forum, but they block the overall quality gate.

### Refactor Recommended

- Public forum pages are mostly client-rendered. `public/index.html` has an empty `#thread-list`, and `public/post.html` has a loading shell for `#thread-detail`. This conflicts with the frontend SEO/progressive-enhancement contract for public forum pages, which expects server-returned HTML to include crawlable titles, primary content or discoverable links.
- `public/components/forum/thread-list.html` and `public/components/forum/thread-detail.html` appear to be stale demo fragments:
  - They expect old template data shapes such as `.Threads`, `.Thread.Replies`, `.Thread.CreatedAt.Format`.
  - `thread-detail.html` posts to `/api/forum/threads/{{.Thread.ID}}/replies`, while the current route is `/api/forum/threads/:id/posts`.
  - They are reachable through `/api/components/*` but are not covered by component route tests.
- Forum model behavior is mostly covered indirectly through usecase/route tests, but there is no focused `api/models/forum_test.go` for SQL filtering, search, ordering, visibility and activity rebuild behavior.
- Minor cleanup candidates: `ForumThreadPostsCo`, `forumPayloadJSON`, and the `composer-panel` JS/CSS path appear unused after the current forum UI moved to dedicated list/detail/new-post flows.

### Refactor Decision

Yes, some refactoring is warranted, but it should be scoped:

1. Fix the `UpdateForumPost` pagination bug first and add a regression test.
2. Clean up or realign stale `public/components/forum/*` fragments, then add component-route/template tests if the components remain part of the supported surface.
3. Add focused model tests for forum SQL and visibility behavior.
4. Treat server-rendered/crawlable forum list/detail pages as a larger frontend refactor; it is important for SEO/progressive enhancement, but it is bigger than a small cleanup.
5. Handle non-forum archguard failures in a separate cleanup slice unless this task is explicitly expanded to “all new backend features”.

## Open Questions

- None. 用户已确认按建议实施本任务内的小范围修复，较大的 SEO/渐进增强改造保留为后续任务。

## Implementation Update

### Summary

- Fixed `UpdateForumPost` so an updated reply is reloaded directly by post id instead of searching only the default first detail page.
- Added `models.GetForumPostListItemByID` using the existing `forumPostListSelectSQL()` author join, then mapped it through the usecase `ForumPostCo` boundary.
- Added a usecase regression test that creates one more reply than the default detail page size, updates the reply beyond page 1, and asserts the updated reply is returned.
- Added model-level forum tests for public/private thread list filtering and thread activity rebuild after deleting the latest reply.
- Replaced stale `public/components/forum/thread-list.html` and `public/components/forum/thread-detail.html` demo fragments with safe static fallback fragments that do not depend on `.Threads` / `.Thread` data and do not post to the removed `/replies` endpoint.
- Added component-route coverage for `/api/components/forum/thread-list` and `/api/components/forum/thread-detail`.

### Verification

- PASS: `gofmt -w api/models/forum.go api/models/forum_test.go api/usecase/forum.go api/usecase/forum_test.go index_test.go`
- PASS: `go test ./api/usecase -run Forum -count=1`
- PASS: `go test ./api/models -run Forum -count=1`
- PASS: `go test . -run 'Test.*(Component|NewPost|LoadTemplate|PostRoute)' -count=1`
- PASS: `node --check public/extensions.js`

### Notes

- Full `go test ./...` was not run in this implementation slice per dispatch instruction; the earlier review already recorded unrelated global blockers in archguard and KB tests.
- The larger SEO/progressive-enhancement refactor for public forum pages remains a follow-up.

## Check Update

- PASS: `gofmt -l api/models/forum.go api/models/forum_test.go api/usecase/forum.go api/usecase/forum_test.go index_test.go`
- PASS: `go test ./api/usecase -run Forum -count=1`
- PASS: `go test ./api/models -run Forum -count=1`
- PASS: `go test . -run 'Test.*(Component|NewPost|LoadTemplate|PostRoute)' -count=1`
- PASS: `node --check public/extensions.js`
