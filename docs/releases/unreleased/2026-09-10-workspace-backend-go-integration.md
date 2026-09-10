---
type: refactor
scope: workspace
audience: developer
summary: 后端工具消费唯一 Go 核心并通过完整无 Node 生成链验收
breaking: true
demo_ready: false
tests:
  - go -C ../backend/tooling run ./cmd/backend-check verify
  - govulncheck ./...
artifacts:
  - docs/go-core.md
  - README.md
  - CLAUDE.md
  - ../backend/tooling/third_party/openapi-thrift/source.json
---

## What changed

已批准 Go 核心迁移 G1–G5 整族完成：Go 解析、profile、projector、renderer、原生 CLI 和浏览器 WASM 见 [核心归档](2026-09-10-runtime-go-core.md)。backend 现直接导入该核心，不调用 Node/Python，不复制第二份规则；旧 TS 核心/CLI 与 backend 旧 JS 工具快照退出。当前源码仍未发布。

backend 快照包含 go.mod/go.sum、LICENSE、value/profile/projector/thrift/bridge 共 8 文件，来源基线 `d501adc` 有未提交改造；完整树 SHA-256 为 `daeeedb07107591123751f60ef0e05c285a3ab5dc2f58b360594c0475997b42e`。维护同步先核验旧副本身份和完整哈希，拒绝覆盖本地不明改动。

backend 在确认 PATH 无 node/python/python3/pnpm 后通过完整 verify，包括真实 JSON/YAML、5 类 HTTP 响应、原生 CLI、init/update、只读 drift 正负例、手写层保护、补丁和 Git checkout 字节矩阵。验收日志为其 `.tmp/go-verify-no-node-fixed-20260910.log`，SHA-256 `ac3480f786a5b4e388e323cf587100e9a530a5c94162c2195f313f2e07d2cdd5`。导入本核心的 tooling 源码 govulncheck 未发现漏洞。

## Why it matters

后端开发无需 Node，浏览器保留相同投影与严格拒绝语义。规则改动仍先落在 owner 仓，再同步受校验的 backend 快照。

## Demo posture / limitations

这次不代表发布 npm/Go 版本、远端 CI、前端消费者迁移或全浏览器兼容。npm 0.2.0 仍是旧实现；WASM 增加异步初始化和资源分发要求。大型输入、包体与浏览器限制见 [Go/WASM](../../go-core.md)。backend 的性能/存储实验与生产准入独立记录，不由 converter 验收代替。
