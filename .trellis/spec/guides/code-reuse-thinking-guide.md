# Code Reuse Thinking Guide

> **Purpose**: 在创建新代码、新模板、新样式或新测试前，先确认项目里是否已有可复用模式。

---

## The Problem

重复实现通常不是一次性成本，而是未来一致性 bug 的来源：

- 同一个错误修复只改到一处，另一处继续坏。
- 同一个字段、路径、样式或错误语义在不同文件里慢慢分叉。
- 新人或 AI 后续不知道哪个实现才是标准。

本项目尤其要注意 Go route/usecase/model、`public/*.html`、`public/components/**`、`public/extensions.js`、toast/loading 模式、模板路径和测试 fixture 的重复。

## Search First

写新东西前先用 `rg` 搜索，不要只凭印象判断不存在。

```bash
rg "renderTemplate|loadTemplate|cleanTemplateName"
rg "apiFetch|showToast|postPath"
rg "thread|category|visibility"
rg "Cache-Control|no-store|template-path"
```

搜索目标可以是函数名、字段名、路径片段、CSS class、错误文案、API endpoint 或测试名。

## Questions Before Writing

| Question | If Yes |
| --- | --- |
| 有没有类似 helper、route、template、component 或测试？ | 复用或扩展它，并保持入口一致 |
| 是否只是同一模式换了字段？ | 优先抽出共享函数、共享模板片段或统一测试 helper |
| 是否在改常量、路径、flag、endpoint、CSS class？ | 搜全仓库，确认所有引用一起更新 |
| 是否会让外部模板覆盖路径变化？ | 同步检查 frontend template runtime spec 和兼容策略 |
| 是否让 API 字段跨过前后端？ | 同步阅读 cross-layer guide，避免字段漏改 |

## Common Duplication Patterns

### Pattern 1: Page Logic Copy-Paste

Bad: 每个页面各写一套 fetch、错误处理、toast、loading 状态。

Good: 优先复用 `apiFetch`、`showToast`、`postPath` 和既有页面初始化模式；必要时抽出小 helper。

### Pattern 2: Template Fragment Drift

Bad: 列表页、分类页、首页各自复制一份主题条目 HTML，字段和可见性慢慢不一致。

Good: 把可复用片段放到 `public/components/**`，并让 `/api/components/*` 只渲染经过清洗的 `.html` 模板。

### Pattern 3: Repeated Constants

Bad: 在多个文件里手写同一个 page size、visibility 字符串、模板路径或缓存头。

Good: 找到最接近的所有引用；如果规则稳定且跨文件复用，放到合适的常量或集中入口。

### Pattern 4: Asymmetric File Lists

Bad: 一个机制递归复制目录，另一个机制手写文件列表；新增目录后只有一边更新。

Good: 如果无法共用同一个发现机制，就增加测试或脚本检查两边输出是否一致。

## When To Abstract

适合抽象：

- 同一逻辑已经出现 3 次或明确会继续出现。
- 逻辑带有安全、权限、路径清洗、缓存或错误语义。
- 多个页面需要同一类空状态、错误状态、toast 或分页行为。

暂时不要抽象：

- 只有一次使用，且未来扩展不明确。
- 抽象后的参数比原逻辑更难理解。
- 只是视觉相似，但语义、权限或数据来源不同。

## Checks After Batch Changes

批量修改后至少做这几件事：

- [ ] 用 `rg` 搜旧名称、旧路径、旧字段，确认没有漏网引用。
- [ ] 检查测试名、fixture、模板名和外部覆盖路径是否跟着改。
- [ ] 如果新增共享 helper，确认它没有偷偷承载业务规则。
- [ ] 如果新增 CSS class，确认没有和现有 class 语义重复。
- [ ] 如果移动模板或组件，确认内置模板和外部模板覆盖路径都说得通。

## Wrong vs Correct

### Wrong

```js
async function loginFetch(url, body) {
  const response = await fetch(url, { method: "POST", body: JSON.stringify(body) });
  if (!response.ok) alert("failed");
  return response.json();
}
```

这里重新发明了请求和错误处理，且用 `alert` 绕开现有 toast 体验。

### Correct

```js
const result = await apiFetch("/api/auth/login", {
  method: "POST",
  body: JSON.stringify(payload),
});
showToast("登录成功", "success");
```

复用现有请求和 toast 入口，错误语义也更容易保持一致。

## Commit Checklist

- [ ] 已搜索相似实现。
- [ ] 没有复制应共享的业务逻辑、模板片段或样式模式。
- [ ] 共享常量、路径、endpoint、模板名只有一个可信入口。
- [ ] 批量修改后已搜索旧值。
- [ ] 必要时已更新对应 layer spec，而不是只把规则写进 guide。
