---
type: chore
scope: workspace
audience: developer
summary: 固定 Go 与 WASM 源码候选身份并纳入远端 Go 安全扫描
breaking: true
demo_ready: false
tests:
  - go test -race ./...
  - go vet ./...
  - govulncheck ./...
  - pnpm lint
  - pnpm typecheck
  - pnpm test
  - pnpm pack:check
artifacts:
  - package.json
  - .gitattributes
  - .github/workflows/openapi-thrift-ci.yml
  - docs/go-core.md
---

## What changed

已批准的 Go 单核心、原生 CLI 与浏览器 WASM 改造进入 `0.3.0-rc.1` 源码候选。backend 消费的 8 个核心文件采用精确字节 Git 属性，避免跨平台 checkout 换行转换改变来源身份。CI 的 security job 以固定 Go 和 govulncheck 同时检查核心与原生 CLI，保留现有 npm 依赖扫描和包载荷检查；workflow 权限仅 contents:read。

## Why it matters

候选包与已发布的旧 `0.2.0` 明确区分。backend 在本仓验证并提交后同步 clean commit、文件哈希和树哈希；浏览器构建继续使用同一个 Go module 与配套 wasm_exec.js，不建立第二套规则。

## Demo posture / limitations

本地 Go race/vet、lint/typecheck、43 项 WASM/生命周期测试与包载荷检查通过；候选压缩包 1,452,267 bytes、解包 5,296,806 bytes。govulncheck v1.7.0 在 Go 1.26.6 下扫描核心与 CLI，finding 为 0，漏洞库时间 `2026-09-09T17:56:34Z`；npm 依赖扫描无已知漏洞。8 个共享核心文件与前轮验收哈希完全一致。原始日志在本仓 ignored `.tmp/delivery-*-20260910.*`。

这次不代表 npm 发布、发行标签、main 合入、真实浏览器迁移或生产准入。远端 CI 与最终候选提交对应关系在交付验收条目记录；上述命令是本轮验证清单，只有实际结果可作为通过证据。历史本地核心/浏览器结果见同日核心验收条目。
