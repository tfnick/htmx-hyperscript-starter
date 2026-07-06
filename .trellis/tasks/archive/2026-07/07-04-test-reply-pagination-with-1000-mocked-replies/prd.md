# Test Reply Pagination With 1000 Mocked Replies

## Goal

针对帖子详情页 `http://localhost:3000/post-019f2359-1a56-7185-8d99-dde927452fe1-1` 做分页组件压力展示测试：在该帖子下 mock 1000 条回复，然后打开页面检查回复分页组件是否按预期显示、收缩、右对齐并可用。

## What I Already Know

* 用户明确要求创建一个测试任务。
* 目标帖子 URL 是 `http://localhost:3000/post-019f2359-1a56-7185-8d99-dde927452fe1-1`。
* 该 URL 的最后一段 `-1` 表示回复第 1 页。
* 当前回复分页组件由 `public/extensions.js` 的 `renderReplyPagination()` 渲染。
* 当前回复分页页码范围由 `replyPageItems(current, total)` 控制，理论上大量页数时会显示首尾页、当前页附近页和省略号。
* 当前回复分页样式由 `public/styles.css` 的 `.reply-pagination`、`.reply-page-button`、`.reply-page-gap` 控制。
* 项目可以通过 `go run .` 启动到 `http://localhost:3000`。

## Open Questions

* None.

## Requirements

* 为指定帖子构造 1000 条回复数据，确保详情 API 能返回多页回复分页元数据。
* mock 方式采用写入本地开发数据库，测试后清理 mock 回复。
* 打开 `http://localhost:3000/post-019f2359-1a56-7185-8d99-dde927452fe1-1` 检查第 1 页分页组件。
* 检查中间页和最后一页分页组件显示，例如第 50 页、最后一页。
* 分页组件应显示首尾页、当前页附近页和省略号，不应铺满全部页码。
* 分页组件应保持右对齐，且不横向溢出。
* 页码按钮、上一页、下一页、当前页高亮、禁用态应符合预期。
* 测试数据必须可清理，不应污染后续开发环境。

## Acceptance Criteria

* [ ] 指定帖子下能看到基于 1000 条回复形成的多页分页。
* [ ] 第 1 页分页显示包含当前页、高亮、下一页可用、上一页禁用。
* [ ] 中间页分页显示包含省略号和当前页附近页码。
* [ ] 最后一页分页显示下一页禁用、当前页高亮。
* [ ] 分页组件右对齐且移动端/桌面端无横向溢出。
* [ ] 测试结束后 mock 数据能被清理或不会进入提交。
* [ ] 记录测试方式、观察结果和截图/结论。

## Definition of Done

* 测试任务记录 mock 方法、测试 URL、观察页码和结果。
* 如发现 bug，另起或继续任务修复；如符合预期，记录通过。
* 不提交临时 mock 数据到仓库。

## Out of Scope

* 不新增生产 seed 数据。
* 不修改分页业务逻辑，除非测试发现明确 bug。
* 不实现推广内容插入能力。
* 不压测后端性能，只验证分页组件显示是否合理。

## Technical Approach

使用可回滚的本地测试数据方式：如果本地数据库中存在目标帖子，则通过脚本或 SQL 在该帖子下插入 1000 条发布状态回复，测试后删除这些 mock 回复。若本地数据库中目标帖子不存在，先记录阻塞原因，不改用浏览器/API 临时拦截，除非用户另行确认。

## Decision (ADR-lite)

**Context**: 这次测试既要验证前端分页组件，也希望尽量接近真实接口返回的分页元数据。

**Decision**: 采用写入本地开发数据库的 mock 方式，插入 1000 条可识别、可清理的发布状态回复。

**Consequences**: 测试更接近真实链路，但必须确保 mock 数据带有明确前缀或标记，并在测试后清理，避免污染开发环境。

浏览器验证建议使用桌面宽度和移动宽度各检查一次，并至少访问第 1 页、中间页、最后一页。

## Technical Notes

* Target URL: `http://localhost:3000/post-019f2359-1a56-7185-8d99-dde927452fe1-1`
* Target thread ID: `019f2359-1a56-7185-8d99-dde927452fe1`
* Likely frontend files:
  * `public/extensions.js`
  * `public/styles.css`
* Relevant backend/model files if using DB mock:
  * `api/models/forum.go`
  * `api/db/migrations/sqlite/app/035_add_forum.sql`
* Existing test helpers:
  * `api/routes/forum_test.go`
  * `api/usecase/forum_test.go`
