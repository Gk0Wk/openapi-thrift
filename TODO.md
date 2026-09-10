# OpenAPI Thrift TODO

> Active-only tracker. Completed notes belong in `docs/releases/unreleased/` or stable docs.

## 稳定发行 0.3.0

- [x] Go 核心候选通过 PR #1 合入 main，合并提交 `1f112a375adb45915e6588c1fccb5db6cde95d30`。
- [x] 本地真实 npm 客户端复现默认 PUT 重试覆盖原始 503 为 E401；禁用发布重试时只发一次且保留真实错误。历史 RC 缺少原始 HTTP 日志，不能据此断言其 registry 根因。
- [ ] 完成 stable 候选本地和 CI 验证，合入 main，发布 `v0.3.0` / npm `0.3.0`，核对完整自动发行与真实安装回读。
