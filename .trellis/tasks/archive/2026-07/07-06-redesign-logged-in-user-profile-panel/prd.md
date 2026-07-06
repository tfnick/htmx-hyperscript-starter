# Redesign Logged-In User Profile Panel

## Goal

将首页登录后的用户属性区域改造成参考截图风格：头像、用户名、快捷图标、浅黄色统计卡片和醒目的发帖按钮，让登录用户信息区域更紧凑、更像论坛个人面板。

## Requirements

* 登录后用户面板展示头像、用户名和一排图标操作。
* 统计区域采用浅黄色卡片，两列展示：等级、鸡腿、星辰、通知、主题帖、评论数、粉丝、收藏。
* “鸡腿”使用现有 `/api/user/points` 的 `balance` 填充；其它统计没有现成接口时先使用 `0` 占位。
* 继续保留当前发帖入口和退出登录能力。
* 图标使用本地内联 SVG，不引入新的前端依赖、网络图标库或构建链。
* 未登录面板保持可用，不在本任务中大改。

## Acceptance Criteria

* [ ] 登录后区域视觉结构与截图一致：顶部头像用户名、图标行、统计卡片、绿色发帖按钮。
* [ ] 退出登录仍可点击并清理登录态。
* [ ] 发帖按钮仍会按当前分类更新 href。
* [ ] `extensions.js` 对缺失 DOM 保持 null-safe。
* [ ] `node --check public/extensions.js` 通过。

## Definition of Done

* 完成 `public/index.html`、`public/styles.css`、必要的 `public/extensions.js` 更新。
* 运行前端 JS 语法检查。
* 若未运行浏览器验证，在最终说明中明确。

## Technical Approach

在 `logged-in-panel` 内重构用户面板 HTML，新增可复用的 `icon` 内联 SVG class 和 `profile-*` 样式。将原退出按钮改成图标按钮但保留 `#logout-button`，发帖入口保留 `#create-post-link`，从而复用现有 JS 事件绑定和链接更新逻辑。

在 `renderAuthState` 里继续填充用户名，并新增 `loadUserPoints()`，登录时调用 `/api/user/points` 更新 `#profile-points-value`；失败时保持默认 `0`，避免阻断用户面板显示。

## Out of Scope

* 不新增主题帖、评论数、粉丝、收藏等后端统计接口。
* 不改未登录面板的整体文案与布局。
* 不引入第三方图标包或前端构建工具。

## Technical Notes

* 参考截图：`D:/腾讯电脑管家截图文件/局部截取_20260706_143801.png`。
* 相关文件：`public/index.html`、`public/styles.css`、`public/extensions.js`。
* 现有接口：`GET /api/user/points` 返回 `{ user_id, balance }`。
