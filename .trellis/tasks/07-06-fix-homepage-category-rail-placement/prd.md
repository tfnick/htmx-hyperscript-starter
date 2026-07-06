# Fix Homepage Category Rail Placement

## Goal

修复首页 DeepFlood 复刻后的板块区域位置：板块列表应显示在帖子列表区域左侧，并且用户滚动页面时板块区域应保持固定不动，方便随时切换版块。

## Requirements

* 首页桌面布局中，板块列表从右侧信息栏移到帖子列表区域左侧。
* 板块区域需要使用 sticky/fixed 等合适方式，在用户垂直滚动时保持可见。
* 帖子列表仍保持主要内容区域，右侧栏继续显示登录、快捷入口、用户/页脚等信息卡片。
* 保持现有动态板块渲染和切换逻辑可用，`#category-list` 仍由 `public/extensions.js` 渲染。
* 移动端不能出现横向溢出；小屏幕下板块区域可以回到普通流式布局。
* 不改变后端 API、论坛分类 slug 或帖子列表数据流。

## Acceptance Criteria

* [ ] 桌面端首页呈现左侧固定板块栏 + 中间帖子列表 + 右侧信息栏。
* [ ] 页面滚动时左侧板块栏保持固定可见，不跟随帖子列表一起滚出视口。
* [ ] 点击板块仍能切换列表，当前板块高亮仍可用。
* [ ] 右侧栏不再承载“所有版块”卡片。
* [ ] 移动端无横向滚动，板块区域显示合理。
* [ ] 相关 HTML/CSS 变更通过目标测试或浏览器 smoke 验证。

## Technical Notes

* Likely files:
  * `public/index.html`
  * `public/styles.css`
* Existing dynamic anchor:
  * `#category-list`
* Verification:
  * Browser check homepage desktop/mobile.
  * Run targeted Go route/template tests if feasible.

