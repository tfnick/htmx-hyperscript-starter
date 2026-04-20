# htmx-hyperscript-starter
A full stack starter go project using [HTMX](https://htmx.org/reference/) + [Hyperscript](https://hyperscript.org/reference/) + [Pico.css](https://v2.picocss.com/docs/modal) with browser hot reloading and live reloading

## Quick Start
Assuming you have `go` and [gow](https://github.com/mitranim/gow) installed you can simply do the following: 
- Click 'Use this template' button above or [click here](https://github.com/new?template_name=htmx-hyperscript-starter&template_owner=zachatrocity)
- Clone your repo
- Run `./dev.sh`

## Preview
![Boilerplate](preview.png)

## Options
- `--port`: specficy the port to run on

## Motivation
There are quite a hodge podge of starter templates for the HTMX stack however most of them were very opinionated on frontend frameworks and many even leveraged the npm eco system which felt yucky to me.

## Dependencies
- Go
- Gow (for live reload of .go code, browser code will still hot reload without gow)
- [aarol/reload](https://github.com/aarol/reload) for hot reload of the web browser

## Optional Dependencies
- Pico.css - put whatever css framework you would like in the index.html `head`

## Roadmap
- ✅ Add api boilerplate for backend API
- ⬜ Add simple authentication flow

### Repo
Source of truth for myself is on my [sr.ht repo](https://git.sr.ht/~zachr/htmx-hyperscript-starter) but I keep this up to date for the templating in Github


### 📊 htmx vs hyperscript 核心对比

| 维度 | htmx | hyperscript |
|------|------|--------------|
| 核心定位 | 客户端 ↔ 服务器通信库 | 客户端本地脚本语言 |
| 主要职责 | 发起 AJAX / WebSocket / SSE 请求，用服务器返回的 HTML 片段更新页面 | 操作 DOM、处理事件、控制动画、管理本地交互状态 |
| 代码形式 | HTML 属性：`hx-get`、`hx-post`、`hx-target` 等 | HTML 属性：`_`（下划线）内写类自然语言脚本 |
| 语法示例 | `<button hx-get="/api" hx-target="#result">加载</button>` | `<div _="on click add .highlight to me">点我高亮</div>` |
| 依赖关系 | 独立运行，不依赖 hyperscript | 可与 htmx 完美协作，也可单独使用 |
| 擅长解决 | “如何从服务器获取数据并局部刷新页面？” | “如何优雅地实现客户端交互（弹窗、标签页、动画、表单验证）？” |
| 学习曲线 | 低（只需了解几个属性） | 中（类英语语法，但需适应声明式思维） |
| 典型应用 | 无限滚动、实时搜索、表单提交、分页 | 模态框动画、键盘快捷键、拖拽、表单本地验证 |

```html
<button
  hx-get="/api/items"
  hx-target="#list"
  _="on htmx:afterRequest add .loaded to me"
>
  加载
</button>
```
hx-get 是 htmx 去请求服务器，_="..." 里的 _hyperscript 则是在请求完成后纯粹在浏览器里给按钮加一个 CSS 类——服务器对此一无所知。