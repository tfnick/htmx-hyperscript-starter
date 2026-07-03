# Template Runtime Guidelines

> exe 内置模板、外部模板覆盖、路径安全、入口职责和测试契约。

---

## Overview

当前模板运行时由 `index.go` 直接提供：

- `//go:embed public` 把 `public/` 打包进 exe。
- `echo.MustSubFS(embeddedPublic, "public")` 暴露内置模板和静态资源根目录。
- `--template-path` 指定外部 HTML 模板目录。
- `renderTemplate` 调用 `loadTemplate` 读取模板并用 `html/template` 渲染。
- `loadTemplate` 先调用 `cleanTemplateName`，再按“外部目录优先，内置模板兜底”的顺序读取。
- `/api/components/*` 通过 `components/<name>.html` 渲染组件，并设置 `Cache-Control: no-store`。

外部模板覆盖是正式产品能力，不是开发调试技巧。后续新增模板必须保持这个能力可预测、可测试、安全。

## Scope / Trigger

修改以下内容时必须阅读本规范：

- `index.go` 中 `embeddedPublic`、`registerFrontendRoutes`、`registerComponentRoutes`
- `renderTemplate`、`loadTemplate`、`cleanTemplateName`
- `--template-path` flag
- `public/*.html`
- `public/components/**/*.html`
- `index_test.go` 中模板读取、外部覆盖和路由渲染测试

## Runtime Contracts

- 默认情况下，所有必须运行的 HTML 模板应位于 `public/` 下，并随 `go build` 打包进 exe。
- 用户不配置 `--template-path` 时，程序必须只依赖 exe 内置模板即可启动并渲染页面。
- 用户配置 `--template-path <dir>` 时，`<dir>` 是外部模板根目录，模板名仍然相对 `public/` 根目录。
- 外部目录中存在同名 HTML 模板时，必须优先使用外部模板。
- 外部目录中不存在同名 HTML 模板时，必须回退到 exe 内置模板。
- 外部模板读取遇到非 `not exist` 错误时，应返回错误而不是静默回退，避免权限、损坏或部署错误被隐藏。
- 当前 `--template-path` 只覆盖 HTML 模板；`/styles.css`、`/extensions.js` 和 `/assets` 仍来自内置 `publicFS`，除非后续任务明确设计并测试静态资源覆盖。
- 新增页面模板后，必须同时确认内置读取和路由渲染；新增组件模板后，必须确认 `/api/components/*` 路径能访问。

## Path Safety Contracts

- 所有模板名必须先经过 `cleanTemplateName` 或等价安全函数处理。
- 模板名必须是相对路径，不能为空，不能以 `/` 开头。
- 模板名必须以 `.html` 结尾。
- 反斜杠需要先转换为 `/`，避免 Windows 路径绕过。
- `path.Clean(name)` 后结果必须与原始清洗后名称一致；包含 `..`、`../`、`.` 或路径归一化差异时必须拒绝。
- 禁止把用户输入、URL path、query string 或表单字段直接传给 `filepath.Join(externalRoot, ...)`。
- 动态组件路径只能拼成 `components/<component>.html` 后再交给 `renderTemplate/loadTemplate`；不能直接读取任意磁盘路径。
- 非法模板名应返回 400 语义；内置模板缺失应返回 404 语义。

## Entry Boundary Contracts

- `index.go` 可以负责 flag、嵌入资源、路由挂载、模板读取和数据库初始化装配。
- `index.go` 不应承载论坛业务规则、权限规则、数据库查询或 response DTO 组装。
- 页面数据如果需要后端业务规则，应通过 `api/routes` -> `api/usecase` -> `api/models` 或已有 API 获取，不要在模板运行时绕过 backend 分层。
- 模板 loader 应保持小而确定；不要把主题系统、用户权限、缓存策略和业务查询都塞进 `loadTemplate`。
- 新增模板运行时能力前，先搜索现有 `renderTemplate/loadTemplate/cleanTemplateName` 测试，优先扩展现有入口。

## Deployment Contracts

- 生产部署时，二进制文件应能在没有外部模板目录的情况下正常服务默认 UI。
- 需要定制 HTML 响应时，运维只放置要覆盖的 `.html` 文件，不必复制整个 `public/`。
- 外部模板路径结构必须与内置模板相同，例如 `templates/components/forum/thread-list.html` 覆盖 `public/components/forum/thread-list.html`。
- 删除或重命名内置模板前，必须评估外部覆盖路径兼容性；破坏性路径变更需要任务 PRD 明确记录。
- dev 模式下，如果配置了 `--template-path`，reload watch paths 应包含外部模板目录，保证定制模板修改能触发刷新。

## Good / Base / Bad Cases

- Good: 新增 `public/search.html` 后，在 `registerFrontendRoutes` 注册 `/search`，并在测试中验证内置模板和外部 `search.html` 覆盖。
- Good: 组件 URL `/api/components/forum/thread-list` 最终只解析 `components/forum/thread-list.html`，且仍然经过 `cleanTemplateName`。
- Base: 外部目录只提供 `index.html`，其他页面仍使用内置模板。
- Bad: 直接把 `/api/components/../../secrets` 拼到磁盘路径读取。
- Bad: 外部模板缺失时返回 404，导致用户必须复制整套模板才能覆盖一个文件。
- Bad: 新增模板放在 `templates/` 或其他运行时目录，但没有被 `//go:embed public` 打包。

## Tests / Checks

- `loadTemplate` 必须覆盖：内置 fallback、外部模板优先、外部缺失回退、非法模板名拒绝、内置缺失返回 not found。
- `cleanTemplateName` 必须覆盖：空字符串、绝对路径、`..`、归一化差异、反斜杠、非 `.html` 后缀。
- 新增完整页面路由时，补充路由渲染测试。
- 新增动态组件路径时，补充路径清洗和 not found 行为测试。
- 改动 `--template-path` 或嵌入根目录时，运行 `go build`，确认 exe 可构建。
- 文档交付时，说明 `--template-path` 的根目录语义和覆盖示例。

## Wrong vs Correct

### Wrong

```go
component := c.QueryParam("name")
body, err := os.ReadFile(filepath.Join(templatePath, component))
```

该写法直接信任用户输入，绕过 `.html` 限制和路径穿越防护。

### Correct

```go
component := strings.TrimPrefix(c.Request().URL.Path, "/api/components/")
return renderTemplate(publicFS, externalRoot, "components/"+component+".html", nil)(c)
```

模板名仍然会进入 `loadTemplate` 和 `cleanTemplateName`，外部覆盖与内置回退行为保持一致。

## Common Mistakes

- 以为 `--template-path` 会覆盖 CSS/JS/assets，但当前只覆盖 HTML 模板。
- 新增页面模板后忘记更新 `index_test.go` 的测试文件系统。
- 组件路径改名后没有考虑生产用户已有外部模板覆盖。
- 为外部模板覆盖复制整套 `public/`，导致未来内置模板更新无法自然回退。
- 把业务权限判断放进模板 loader，而不是 route/usecase 或前端 API 状态。

