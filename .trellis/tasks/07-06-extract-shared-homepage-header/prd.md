# Extract Shared Homepage Header

## Goal

修复首页 header 与登录、注册、发帖、帖子详情页面各自复制导致的展示不一致问题，将首页 header 抽成公共模板片段并被这些页面复用。

## Requirements

* 使用 `public/components/layout/header.html` 作为公共 header 片段。
* 首页、登录页、注册页、发帖页、帖子详情页都通过同一个 header 片段渲染。
* 公共 header 使用当前首页的 DeepFlood 品牌、热门版块容器、搜索框和主题切换按钮。
* 继续支持 `--template-path` 外部模板覆盖，外部覆盖同名 header 片段时页面渲染应优先使用外部版本。

## Acceptance Criteria

* [x] `public/*.html` 页面不再复制完整 `.site-header` markup。
* [x] `/`、`/login`、`/register`、`/new-post`、`/post-*` 均能渲染公共 header。
* [x] header 片段外部覆盖有测试保护。
* [x] 不引入新的前端框架或 JS 依赖。

## Definition of Done

* `go test` 覆盖模板加载和页面路由渲染。
* `node --check public/extensions.js` 通过。
* 如发现可复用模板约定，更新 frontend spec。

## Technical Approach

扩展 `renderTemplate` 的解析流程，让完整页面模板在解析自身前同时解析共享 layout 片段。页面使用 `{{template "layout/header" .}}` 引用公共 header。`loadTemplate` 仍然负责外部模板优先和内置模板 fallback，保持路径安全规则一致。

## Out of Scope

* 不重做页面布局。
* 不调整认证、发帖或帖子详情业务逻辑。
* 不新增动态 header 状态。

## Technical Notes

* 相关文件：`index.go`、`index_test.go`、`public/components/layout/header.html`、`public/*.html`。
* 现有 `public/components/layout/header.html` 内容不是当前站点 header，需要替换。
