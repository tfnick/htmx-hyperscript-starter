# Fix Homepage Post List Left Right Layout

## Goal

调整首页论坛主体布局：将用户信息、发帖入口、快捷功能、用户数目和新用户区块作为一个整体移动到帖子列表区域内部，使帖子列表区域形成左右布局。

## Requirements

- 保留当前左侧 `category-rail` 版块导航。
- 将现有 `right-sidebar` 移入 `feed-panel`，作为帖子列表区域的右栏。
- 将帖子列表工具栏、标题、列表和分页包在左侧内容栏中。
- 桌面端保持帖子内容和功能区左右并排。
- 窄屏端保持单列堆叠，避免内容挤压或横向溢出。
- 不新增前端依赖，不调整 API 或 JS 数据流。

## Acceptance Criteria

- [ ] 首页 DOM 中用户信息、快捷功能和用户数目区块位于 `feed-panel` 内。
- [ ] 桌面宽度下 `feed-panel` 呈左右布局。
- [ ] 1120px 以下布局退化为单列，功能区不再强行占右栏。
- [ ] 帖子列表分页、分类列表挂载点、登录态挂载点的 id 保持不变。
- [ ] 现有 HTML/CSS 基本检查通过。

## Definition of Done

- 前端规范已阅读。
- 变更范围限制在首页 HTML/CSS。
- 执行可用的静态/测试检查，并记录结果。
- 不回滚已有未提交改动。

## Technical Approach

在 `public/index.html` 中为帖子列表主体新增 `.feed-main` 包裹层，并把现有 `.right-sidebar` 移入 `.feed-panel`。在 `public/styles.css` 中把 `.page-shell` 调整为左侧分类 rail + 右侧 feed 两列，同时把 `.feed-panel` 设置为内部 grid。

## Decision (ADR-lite)

Context: 右侧功能区当前是页面级第三列，但产品期望它属于帖子列表区域。

Decision: 保留已有 `right-sidebar` 语义和内部区块，只移动其 DOM 层级，并增加 `.feed-main` / `feed-panel` 内部布局样式。

Consequences: 改动小，不影响 JS id 绑定；桌面端总宽度稍收敛，窄屏端需要显式把内部 grid 改为单列。

## Out of Scope

- 不重新设计视觉风格。
- 不调整分类 rail 位置。
- 不改登录、发帖、分页或分类加载逻辑。
- 不修改后端路由/API。

## Technical Notes

- Relevant files: `public/index.html`, `public/styles.css`.
- Existing uncommitted task already moved `category-rail` out of sidebar; this task builds on that state.
