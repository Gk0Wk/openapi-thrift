---
type: feature
scope: runtime
audience: developer
summary: 将 OpenAPI 手工校验扩展规范化为 x-ispark-* 并保留旧字段兼容读取
breaking: false
demo_ready: false
tests: pnpm test; pnpm run typecheck; pnpm run lint; pnpm run pack:check
artifacts: src/model.ts, src/profile.ts, src/projector.ts, tests/projector.test.mjs, README.md
---

## What changed

`x-ispark-validate` 和 `x-ispark-allow-unsupported-validation` 成为 canonical 扩展。已发布文档中的 `x-dramawork-*` 继续可读并产生弃用 warning；新旧双写必须一致，冲突会 fail-fast。工具版本提升到 `0.2.0`。

## Why it matters

通用 ISpark 模板不再把扩展命名绑定到旧产品，同时保持现有 OpenAPI 文档和服务的迁移窗口，避免静默改变校验语义。

## Demo posture / limitations

backend 模板和 vendored fixture 已同步；npm `0.2.0` 尚未发布，因此当前不能把远端安装作为可用性证明，也不能推送依赖该版本的 backend 收尾提交。
