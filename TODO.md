# OpenAPI Thrift TODO

> Active-only tracker. Completed notes belong in `docs/releases/unreleased/` or stable docs.

## Active Work

已批准 1+2 的 converter 交付已完成：`0.3.0-rc.1` 候选、固定核心字节、Go/WASM 本地验证与实现提交的远端 CI 通过；backend 固定消费 clean commit `88ee9ce`。产物和 Actions 证据见 [候选归档](docs/releases/unreleased/2026-09-10-workspace-go-candidate.md)。未发布 npm/Go 版本或迁移真实消费者。

Go 单核心迁移 G1–G5 已完成并归档至 [核心验收](docs/releases/unreleased/2026-09-10-runtime-go-core.md) 与 [backend 无 Node 整链验收](docs/releases/unreleased/2026-09-10-workspace-backend-go-integration.md)。当前源码未发布，真实消费者未迁移。

## 同版本多入口发行（已批准）

执行边界与隔离目录见 [brief](working-delta/distribution-20260910.md)。

- [ ] D1 版本与原生产物：由 package.json 冻结版本；Go CLI 版本/提交可核对；六个平台可执行文件、LICENSE、摘要与校验和。
- [ ] D2 安装验收：隔离 npm tarball 调用、独立 Go consumer、原生 CLI 正常/失败保留输出；验证发布产物，不依赖工作树 dist。
- [ ] D3 流水线：固定 action/工具，六平台运行验收，同提交 npm/Go/CLI；npm OIDC/provenance，RC 使用 next 分发标签。
- [ ] D4 文档与交付：Go/npm/CLI 安装和升级说明、实际 CI 及发布后回读证据；核对 npm 可信发布配置。本机旧 npm 凭证 401，GitHub 无发布 Secret，不自动降低发布鉴证。
