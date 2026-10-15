# OpenAPI Thrift TODO

> 仅保留未完成任务；本轮实现和验证见 [严格 JSON](docs/releases/unreleased/2026-10-13-strict-json-directional-dtos.md) 与 [类型化 IR](docs/releases/unreleased/2026-10-15-runtime-typed-contract-ir.md)。

- [ ] 发布前独立处理 backend 完整验证发现的工具链/依赖安全阻塞，并重新验收；当前只交付本地候选。
- [ ] 真实消费者按方向类型重新生成并迁移引用；尚未授权启动。

## 合并与清理（2026-10-15）

- [ ] Go 1.26.9 / 依赖安全复验。
- [ ] 合并已批准改动，保留主工作树 WIP。
- [ ] 审计并清理已吸收分支、worktree 与远程分支。
