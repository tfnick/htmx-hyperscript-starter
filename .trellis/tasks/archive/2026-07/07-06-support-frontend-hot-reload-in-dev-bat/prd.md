# Support Frontend Hot Reload In Dev Bat

## Goal

让 Windows 开发入口 `dev.bat` 启动后，修改 `public/` 下的 HTML、CSS、JS 和 assets 能通过已有 reload 机制刷新浏览器，并在刷新后读取磁盘上的最新前端资源。

## Requirements

* `dev.bat` 使用 dev 模式启动，并传入本地 `public` 作为 HTML 模板覆盖目录。
* dev 启动时支持本地 `public` 静态资源覆盖，让 `/styles.css`、`/extensions.js` 和 `/assets/*` 可以从磁盘读取。
* 保持默认生产行为不变：未配置外部静态资源目录时仍使用 embedded `publicFS`。
* 不改变 `--template-path` 的 HTML-only 语义，避免破坏已有外部模板覆盖契约。

## Acceptance Criteria

* [ ] `dev.bat` 启动命令包含 dev 热更新所需的模板路径和静态资源路径。
* [ ] 外部静态资源目录存在同名 CSS/JS 时，路由优先返回磁盘文件。
* [ ] 外部静态资源目录缺少同名 CSS/JS 时，路由回退 embedded 文件。
* [ ] 未配置外部静态资源目录时，默认 embedded 静态资源行为不变。
* [ ] 相关 Go 测试通过。

## Definition of Done

* 测试新增或更新，覆盖外部静态资源覆盖与 fallback。
* 至少运行 `go test .`。
* 不引入新的前端构建链或依赖。

## Technical Approach

新增独立 flag `--static-path`，表示外部静态资源根目录，目录结构与 embedded `public/` 相同。`dev.bat` 使用 `--dev --template-path public --static-path public` 启动。

`/styles.css` 和 `/extensions.js` 使用固定文件名的 helper：外部目录优先，缺失时 fallback 到 embedded FS；其他读取错误直接返回错误。`/assets` 在配置了外部 `assets` 目录时挂载磁盘目录，否则继续挂载 embedded assets。

## Decision (ADR-lite)

Context: 当前 `--template-path` 只覆盖 HTML，导致 dev reload 后 CSS/JS/assets 仍来自 embed，前端资源修改不能实时反映。

Decision: 新增 `--static-path` 而不是扩大 `--template-path` 语义。

Consequences: flag 语义更清晰，生产默认行为保持不变；未来若要正式支持生产静态资源覆盖，可沿用并补充文档。

## Out of Scope

* 不引入 Vite、Webpack 或 Node 前端构建链。
* 不修改浏览器 reload websocket 实现。
* 不调整前端页面样式或业务逻辑。

## Technical Notes

* 相关文件：`dev.bat`、`index.go`、`index_test.go`。
* 规范参考：`.trellis/spec/frontend/template-runtime-guidelines.md`、`.trellis/spec/frontend/performance-guidelines.md`。
* 现状：dev 模式已经 watch `public/`，缺口在静态资源响应仍读取 embedded FS。
