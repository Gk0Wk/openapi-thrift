# OpenAPI Thrift TODO

> Active-only tracker. Completed notes belong in `docs/releases/unreleased/` or stable docs.

## Active Work

已批准 1+2 的 converter 交付已完成：`0.3.0-rc.1` 候选、固定核心字节、Go/WASM 本地验证与实现提交的远端 CI 通过；backend 固定消费 clean commit `88ee9ce`。产物和 Actions 证据见 [候选归档](docs/releases/unreleased/2026-09-10-workspace-go-candidate.md)。未发布 npm/Go 版本或迁移真实消费者。

Go 单核心迁移 G1–G5 已完成并归档至 [核心验收](docs/releases/unreleased/2026-09-10-runtime-go-core.md) 与 [backend 无 Node 整链验收](docs/releases/unreleased/2026-09-10-workspace-backend-go-integration.md)。当前源码未发布，真实消费者未迁移。

- [ ] Configure `NPM_TOKEN` before enabling GitHub Actions automatic npm publish.
