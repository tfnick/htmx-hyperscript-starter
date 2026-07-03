# Architecture Review Thinking Guide

> **Purpose**: 在实现或审查功能前，从架构、解耦、防腐、复用和全栈效率角度做一轮快速自检。

---

## When To Use

遇到这些情况时先读本 guide：

- 功能跨越 backend、frontend、database、provider、framework 中的多个层。
- 新增第三方集成、webhook、OAuth、支付、OSS、LLM、embedding、通知或队列能力。
- 新增字段需要从 migration 一路流到模板或 JS。
- 准备新增 helper、registry、middleware、response、pagination、transaction 或缓存能力。
- 发现代码“能跑但放在哪都别扭”，说明边界可能没想清楚。

## Architecture Questions

- 这段逻辑属于 HTTP 边界、业务规则、SQL、基础设施、provider 适配还是页面展示？
- 是否保持 `routes -> usecase -> models`，没有反向导入？
- 是否有现成的 `api/framework/<capability>` 可以承载通用能力？
- 如果要新增 framework 能力，它是否足够通用，且不包含具体业务规则？
- 是否需要新增或调整 archguard，而不是靠人工记忆维持边界？

## Decoupling Questions

- usecase 是否只依赖 command/query、`Co`、framework usecase context 和 integration port？
- route 是否只做绑定、鉴权接入、DTO 映射和 response envelope？
- model 是否只做 SQL 与持久化结果，不泄漏 HTTP 或前端展示语义？
- 前端是否只依赖公开 API DTO 和模板契约，不读取数据库/model/provider 细节？
- 入口文件是否只做装配，未承载业务规则或页面数据查询？

## Anti-Corruption Questions

- 第三方 SDK、payload、header、签名、状态码和 provider error 是否只停留在 provider 包？
- `api/usecase/integrations/*/ports.go` 是否表达的是业务内部语义，而不是照搬第三方字段？
- webhook 是否被 normalize 成业务事件后再进入 usecase/model？
- 保存 provider 数据时是否已脱敏，且只保存业务需要的字段或 safe snapshot？
- 用户可见错误是否是安全业务 message，而不是 provider 原始错误文本？

## Reuse Questions

- API response 是否复用 `httpresponse`，分页是否复用 `fwrequest.PageQuery` 和 `fwusecase.NormalizePageQuery`？
- 错误是否复用 `fwusecase.E`、标准错误码和 response mapper？
- 事务和 after-commit 是否复用 `fwusecase.WithAppTx` 与 `RegisterAfterCommit`？
- 前端请求、toast、URL、分类、模板片段是否复用 `apiFetch`、`showToast`、`postPath`、`public/components/**`？
- 是否正在新增第二套 logging、auth、pagination、transaction、response、provider registry 或 JS state helper？

## Full-Stack Efficiency Questions

- 新字段是否已经列出 migration、model、usecase、route DTO、template/JS、test 的同步清单？
- 列表页是否一次拿到摘要字段，避免前端或 route 层 N+1？
- 私有/公开、登录/匿名、分页/排序、错误/空状态是否在前后端语义一致？
- SEO 页面是否有稳定 URL 和服务端 HTML 基础结构，而不是完全依赖 JS 后填？
- 外部 HTML 模板覆盖路径是否仍与内置 `public/` 相对路径一致？

## Review Outcome

审查结束后，把结论放回正确位置：

- 具体代码契约写到 backend/frontend layer spec。
- 思考提醒写到 guides。
- 一次性取舍或尚未确认事项写到当前任务 PRD。
- 如果发现架构守护缺失，新增单独任务补 archguard，不要只在文档里提醒。

## Common Smells

- “先在 route 里写完，之后再抽 usecase”。
- “provider 返回什么前端就显示什么”。
- “列表字段不够，前端循环请求详情补一下”。
- “为了一个页面新建一套 fetch/toast/loading helper”。
- “改了模板路径，但没想外部覆盖目录是否兼容”。
- “测试只覆盖 happy path，没有覆盖未登录、无权限、not found、空列表和错误 envelope”。
