# Login Success Home Redirect And Failure Toast

## Goal

优化论坛登录页的提交体验：用户在 `/login` 点击登录后，如果认证成功，应自然进入首页；如果认证失败，应在当前页面用 toast 展示失败原因，不使用 modal/dialog 打断用户。

## What I Already Know

* 用户明确要求：登录成功进入首页；登录失败用 toast 提示原因；不要使用 modal 框。
* 当前已经有独立登录模板 `public/login.html`，路由由 `index.go` 的 `/login` 渲染。
* 当前登录表单逻辑位于 `public/extensions.js` 的 `bindAuthEvents()`。
* 当前 `apiFetch()` 会从后端 response envelope 中提取 `error.message` 并抛出 `Error`。
* 当前 `showToast(message, error)` 已存在，错误态通过 `.toast.is-error` 样式显示。
* 当前登录成功后会 `storeAuth(auth)`、重置表单、显示“已登录”、刷新认证状态，但不会跳转首页。
* 当前登录失败时，事件处理器没有 `try/catch`，因此失败原因不会以 toast 形式稳定展示。

## Requirements

* 登录页点击登录并认证成功后，必须保存现有 token/user 信息，然后进入首页 `/`。
* 成功跳转应使用浏览器自然导航方式，避免停留在 `/login` 造成“已登录但仍在登录页”的割裂感。
* 登录失败时必须停留在 `/login`。
* 登录失败时必须调用现有 toast 能力展示后端返回的失败原因。
* 登录失败提示必须使用 toast，不得使用 `alert`、`dialog`、modal 组件或页面内大块错误面板替代。
* 登录失败不应破坏用户继续修改输入并再次提交的能力。
* 注册、退出登录、发帖、回帖、搜索和列表浏览行为不在本任务中重做，但修改不能破坏它们。

## Acceptance Criteria

* [x] 在 `/login` 使用正确邮箱和密码提交后，认证 token 被保存，页面导航到 `/`。
* [x] 在 `/login` 使用错误密码提交后，页面仍停留在 `/login`。
* [x] 登录失败时 `#toast` 显示后端返回的错误原因，并带错误态样式。
* [x] 登录失败流程不出现 modal/dialog/alert。
* [x] 登录失败后用户可以修改表单并再次点击登录。
* [x] `node --check public/extensions.js` 通过。
* [x] 聚焦测试或浏览器冒烟覆盖登录成功跳转和失败 toast。

## Definition Of Done

* 实现范围尽量收敛在现有登录页前端交互，不新增前端框架或 UI 依赖。
* 复用现有 `apiFetch()`、`storeAuth()`、`showToast()`，除非现有函数确实无法满足需求。
* 可用检查通过；如果全量检查受既有问题阻塞，需要记录实际阻塞点。
* 不引入新的 modal/dialog/alert 登录失败体验。

## Technical Approach

优先在 `public/extensions.js` 的登录表单 submit handler 中加入 `try/catch`：

* 成功路径：调用 `/api/auth/login` 成功后继续 `storeAuth(auth)`，再导航到 `/`。
* 失败路径：捕获 `apiFetch()` 抛出的错误，调用 `showToast(error.message, true)`，保持当前页面和表单可继续操作。
* 如有必要，轻量增加提交中状态，防止重复点击；但不把它作为本任务的硬性范围。

## Decision (ADR-lite)

**Context**: 现有登录页已经具备 toast 和认证 API 封装，但登录 submit handler 没有失败捕获，也没有成功后离开登录页。

**Decision**: 保持现有技术栈和页面结构，只在登录交互层修正成功/失败两条路径。

**Consequences**: 改动小、风险低；失败原因继续由后端 envelope 决定。未来如果需要 `redirect` 查询参数，可以在本逻辑基础上扩展，但本任务默认成功进入首页 `/`。

## Out Of Scope

* 重做登录页视觉设计。
* 新增 Tailwind/daisyUI 或其他 UI 框架。
* 重写后端登录接口、JWT、注册或 OAuth 流程。
* 支持任意登录后回跳地址；本任务默认回到首页 `/`。
* 修改 toast 的全局设计语言，除非现有错误态不可用。

## Technical Notes

* 主要代码文件：`public/extensions.js`。
* 关联模板：`public/login.html` 已包含 `#login-form` 和 `#toast`。
* 关联样式：`public/styles.css` 已包含 `.toast` 和 `.toast.is-error`。
* 入口路由：`index.go` 已将 `/login` 渲染为 `login.html`。
* 相关历史任务：`.trellis/tasks/archive/2026-07/07-03-fix-thread-detail-residual-elements/prd.md` 记录了独立 `login.html` 的引入背景。
