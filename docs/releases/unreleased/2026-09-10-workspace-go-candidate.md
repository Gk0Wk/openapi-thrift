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

实现提交为 `adb52b9a64113be0a824e0fdf7c26c06f18456df`，浏览器示例补齐后冻结为 `88ee9ceaa89f42a34c117938c6c8e0bf129c6461`。该干净提交的 [Actions 34475907414](https://github.com/Gk0Wk/openapi-thrift/actions/runs/34475907414) 中 verify/security/package-analysis 三个 job 全部成功；backend 的来源锁固定此提交与 `daeeedb07107591123751f60ef0e05c285a3ab5dc2f58b360594c0475997b42e` 核心树哈希。后续文档提交不改变这个实现身份。

实际本地 tarball 位于 `.tmp/delivery-candidate-20260910/sttot-openapi-thrift-0.3.0-rc.1.tgz`，SHA-256 `368c29965f95d6dd9c18780629a1faeb3559489fe35174a6ff3a6937a3a7cd96`。WASM SHA-256 `ac08badf38511e925284691925c49b8e0a1da81722b8754481d541df3f1ca432`；配套 `wasm_exec.js` 为 `0c949f4996f9a89698e4b5c586de32249c3b69b7baadb64d220073cc04acba14`。这些是候选载荷，未上传 npm。

这次不代表 npm 发布、发行标签、main 合入、真实浏览器迁移或生产准入。远端 CI 与最终候选提交对应关系在交付验收条目记录；上述命令是本轮验证清单，只有实际结果可作为通过证据。历史本地核心/浏览器结果见同日核心验收条目。
