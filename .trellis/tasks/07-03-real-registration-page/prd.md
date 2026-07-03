# Real Registration Page

## Goal

新增一个真正独立的注册页面，让注册流程从 `/register` 完成；注册成功后自动保存登录态并自然进入首页 `/`。登录页只负责登录，不再与注册表单同页展示。

## What I Already Know

* 用户明确要求：创建任务，新增真正的注册页；注册完成自动跳转到首页。
* 用户明确禁止：把登录与注册展示在同一个页面。
* 当前 `index.go` 只注册了 `/login` 前端路由，没有 `/register` 前端路由。
* 当前 `public/login.html` 同时包含 `#login-form` 和 `<details class="login-register">` 里的 `#register-form`，不符合“登录与注册不要同页展示”。
* 当前 `public/index.html` 和 `public/post.html` 的右侧登录卡也存在 inline `#register-form`，更像嵌入式注册而不是“真正的注册页”。
* 当前 `public/extensions.js` 已有 `#register-form` submit 逻辑，会调用 `/api/auth/register`、`storeAuth(auth)`，但注册成功后只 toast + refreshAuthState，没有跳首页。
* 当前 `api/routes/auth.go` 已提供 `POST /api/auth/register`，注册成功会返回 token 和 user。
* 当前 toast 能力由 `showToast(message, error)` 提供，失败原因可沿用 `apiFetch()` 抛出的后端 envelope message。

## Requirements

* 新增独立注册模板 `public/register.html`。
* 新增前端路由 `/register`，渲染 `register.html`。
* `/register` 页面只展示注册表单和必要导航，不展示登录表单。
* `/login` 页面只展示登录表单和跳转到注册页的链接，不展示注册表单或可展开注册区域。
* 首页、帖子详情页等注册入口应跳转到 `/register`，不再内嵌注册表单。
* 注册表单提交成功后必须保存 token/user，并导航到首页 `/`。
* 注册失败时停留在 `/register`，用现有 toast 展示后端返回的错误原因。
* 注册提交中应尽量避免重复点击；失败后恢复可继续修改并再次提交。
* 不引入新的前端框架、Tailwind、daisyUI 或 modal/dialog 体验。

## Acceptance Criteria

* [x] `GET /register` 返回注册页 HTML。
* [x] `/register` 页面存在 `#register-form`，不存在 `#login-form`。
* [x] `/login` 页面存在 `#login-form`，不存在 `#register-form`。
* [x] 首页和帖子页的注册入口是指向 `/register` 的链接，不是 inline 注册表单。
* [x] 在 `/register` 使用合法新账号提交后，localStorage 写入登录 token，并跳转到 `/`。
* [x] 在 `/register` 使用已存在邮箱或非法输入提交后，页面仍停留在 `/register`，toast 显示后端错误原因。
* [x] 注册失败流程不出现 modal/dialog/alert。
* [x] `node --check public/extensions.js` 通过。
* [x] 如入口路由或嵌入模板变更，`go test . -count=1` 或等价主包测试通过。

## Definition Of Done

* 注册页与登录页模板职责分离。
* 复用现有 `/api/auth/register`、`storeAuth()`、`apiFetch()`、`showToast()`。
* 可用检查通过；若全量测试受既有问题阻塞，交付说明需记录实际阻塞点。
* 不破坏登录成功跳首页和登录失败 toast 的现有修复。

## Technical Approach

* 在 `index.go` 的 `registerFrontendRoutes()` 中新增 `registerHandler` 并挂载 `/register`。
* 新增 `public/register.html`，沿用当前站点 header 与 `login-shell/login-panel/login-form` 视觉样式，但只放注册表单。
* 修改 `public/login.html`，移除 `<details class="login-register">` 注册区域，保留一个“注册新账号”链接到 `/register`。
* 修改 `public/index.html` 和 `public/post.html` 的右侧卡片，把 inline 注册 form 改成 `/register` 链接。
* 修改 `public/extensions.js` 中 register submit handler，成功后 `window.location.assign("/")`，失败则 toast。
* 如需要，给登录/注册页的小链接补少量 CSS，但不重做页面视觉。

## Decision (ADR-lite)

**Context**: 当前注册表单复用在登录页和侧栏里，虽然能注册，但产品结构上不是独立注册页，也违反“登录与注册不要同页展示”。

**Decision**: 将注册升级为独立 `/register` 页面；登录页只登录，其他页面只提供注册链接。

**Consequences**: 用户路径更清晰，也便于未来扩展注册页字段、OAuth 注册提示、条款确认等功能。短期需要让共享 JS 对登录页和注册页的不同 DOM 保持 null-safe。

## Out Of Scope

* 重写后端注册逻辑、JWT、OAuth 或用户模型。
* 新增邮箱验证码、邀请码、服务条款勾选或 CAPTCHA。
* 支持任意注册后回跳地址；本任务默认注册成功进入首页 `/`。
* 重做全站导航或右侧栏视觉。

## Technical Notes

* 主要文件：`index.go`、`public/register.html`、`public/login.html`、`public/index.html`、`public/post.html`、`public/extensions.js`。
* 当前登录修复任务已让登录成功跳首页、失败 toast；注册页应沿用相同体验。
* 现有 `apiFetch()` 会从失败 envelope 中提取 `error.message` 并抛出 `Error`。
* 现有 `showToast()` 使用 `#toast`，因此 `register.html` 需要包含同样的 toast 节点。
