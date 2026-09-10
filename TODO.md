# OpenAPI Thrift TODO

> Active-only tracker. Completed notes belong in `docs/releases/unreleased/` or stable docs.

## Active Work

- [ ] 已批准 1+2：固定 Go 核心/CLI/WASM 候选身份，完成本地 Go/包/安全验证、独立交付分支提交与远端 CI，backend 消费该 clean commit。未授权 npm 发布或真实消费者迁移。

Go 单核心迁移 G1–G5 已完成并归档至 [核心验收](docs/releases/unreleased/2026-09-10-runtime-go-core.md) 与 [backend 无 Node 整链验收](docs/releases/unreleased/2026-09-10-workspace-backend-go-integration.md)。当前源码未发布，真实消费者未迁移。

- [ ] Run GitHub Actions CI at least once and observe whether `pnpm audit` has environment-specific noise.
- [ ] Configure `NPM_TOKEN` before enabling GitHub Actions automatic npm publish.
