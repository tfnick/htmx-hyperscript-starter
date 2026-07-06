# Narrow User Profile Panel Width

## Goal

继续收窄首页登录用户属性面板所在的右侧区域，让帖子列表获得更多横向空间，同时保持用户属性面板在桌面和移动端都不溢出。

## Requirements

* 桌面布局中降低 `right-sidebar` 所在列的宽度上限和默认宽度。
* 用户属性面板内部保持两列属性展示，不出现文字或图标挤出容器。
* 1120px 以下仍沿用单列/折叠布局，不引入新的断点或交互。

## Acceptance Criteria

* [x] 首页桌面布局中帖子列表区域比当前实现更宽。
* [x] 登录用户属性面板在收窄后仍能完整展示等级、鸡腿、星辰、通知、主题帖、评论数、粉丝、收藏等入口。
* [x] CSS 改动局限在现有首页布局和 profile panel 样式。

## Definition of Done

* 前端静态检查通过。
* 相关 Go 测试通过或说明未运行原因。
* 不修改无关 HTML/JS 行为。

## Technical Approach

调整 `public/styles.css` 中 `.feed-panel` 的第二列 `clamp()` 宽度，并适度压缩 profile panel 内部间距与头像/图标尺寸，让右侧栏能在更窄宽度下保持稳定。

## Out of Scope

* 不重新设计用户属性面板内容。
* 不改登录态、积分接口或发帖入口逻辑。
* 不调整后端 API。

## Technical Notes

* 相关文件：`public/styles.css`。
* 当前右侧栏宽度为 `clamp(380px, 31vw, 420px)`，明显偏宽。
