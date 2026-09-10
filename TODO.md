# OpenAPI Thrift TODO

> Active-only tracker. Completed notes belong in `docs/releases/unreleased/` or stable docs.

## 稳定发行 0.3.0

- [x] Go 核心候选通过 PR #1 合入 main，合并提交 `1f112a375adb45915e6588c1fccb5db6cde95d30`。
- [x] 本地真实 npm 客户端复现默认 PUT 重试覆盖原始 503 为 E401；禁用发布重试时只发一次且保留真实错误。历史 RC 缺少原始 HTTP 日志，不能据此断言其 registry 根因。
- [x] stable 候选及 main CI 通过，`v0.3.0` 与 npm `0.3.0` 已公开，npm PUT 正常返回 200；npm/GitHub latest 均已更新，npm 签名和 provenance 验证通过。
- [ ] 标签 run `34507267145` 的自动回读报 ETARGET：实现明确版本的 Go 回读及只读 reusable/manual workflow，测试、合入 main 后用原 run/tag 完成独立恢复。原失败 run 保留，不重发版本。
