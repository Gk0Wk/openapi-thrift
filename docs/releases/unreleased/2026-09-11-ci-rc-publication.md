---
type: feature
scope: ci
audience: developer
summary: 发布 OpenAPI Thrift 0.3.0-rc.1 的 npm、Go Module 和六平台 CLI，并完成发行产物回读
breaking: true
demo_ready: false
tests:
  - gh run view 34497892720 --repo Gk0Wk/openapi-thrift --json status,conclusion,jobs
  - go run ./cmd/release smoke-go --module-version v0.3.0-rc.1
  - npm run install:check -- .tmp/registry-v0.3.0-rc.1/sttot-openapi-thrift-0.3.0-rc.1.tgz --keep
  - npm audit signatures --prefix .tmp/npm-registry-signatures --registry=https://registry.npmjs.org
  - go run ./cmd/release smoke --archive .tmp/github-v0.3.0-rc.1/openapi-thrift_0.3.0-rc.1_windows_amd64.zip
  - git diff --check
artifacts:
  - package.json
  - .github/workflows/openapi-thrift-release.yml
  - cmd/release
  - internal/cli/cli.go
  - docs/distribution.md
  - docs/releases/v0.3.0-rc.1.md
---

## What changed

同一提交 `ebcaece65c26ebb69bdc9ad4168fc36e781f00a6` 已发布为 `v0.3.0-rc.1`：

- [npm 0.3.0-rc.1](https://www.npmjs.com/package/@sttot/openapi-thrift/v/0.3.0-rc.1) 保留原包名，包含 JS/TS 接口与预编译 Go WASM；`next` 指向本 RC，`latest` 保持 `0.2.0`。
- Go Module 为 `github.com/Gk0Wk/openapi-thrift@v0.3.0-rc.1`；同一版本的 `cmd/openapi-thrift` 可通过 `go install` 安装。
- [GitHub 预发布](https://github.com/Gk0Wk/openapi-thrift/releases/tag/v0.3.0-rc.1) 于 `2026-09-10T16:00:11Z` 发布，包含 Linux/macOS/Windows amd64/arm64 的六份 CLI 归档、原 npm tarball 和 `SHA256SUMS`，共八个资产。未替换 GitHub 的稳定 latest release。

npm 可信发布连接已成功保存并回读：`Gk0Wk/openapi-thrift`、`openapi-thrift-release.yml`、空 environment、允许 `npm publish`。原有包 2FA 策略未更改，无新增长期 npm token。

已完成的 D1–D4 计划归档于此：版本和来源绑定、六平台原生归档、实际安装验收、固定工具的发行流水线、安装升级文档、OIDC/provenance、版本发布与回读全部完成。临时 brief 已移除，长期规则保留在 [分发说明](../../distribution.md)。本轮从 `a8422fd` 开始，owner 为 Codex，发行分支为 `release/distribution-20260910`；隔离目录 `.worktrees/openapi-thrift-release-20260910` 保留供复查，历史目录和其他任务 worktree 不在清理范围。

## Why it matters

JS/TS 使用者继续安装同名 npm 包；Go 工程和预编译 CLI 使用者无需 Node。版本标签、npm provenance、Go module 来源和原生归档内的 CLI 提交标识均指向同一源码；npm 发布和后续 GitHub 恢复都使用已验收产物，没有重新构建或替换包体。

发行包的 npm SHA-256 为 `af37fdf009cef709753943a84bcb8093b8c66e936288341e28cdf4e3b295dd96`。Go module checksum 为 `h1:Hr/BGrpPA+XP2/r3w1Bwticypp4JFVE1d9jxZ5VvPK0=`，其 origin hash 与标签提交一致。六个原生包及 npm tarball 的 GitHub 资产摘要均与原 Actions 产物一致；完整摘要见 Release 的 `SHA256SUMS`。

## Demo posture / limitations

[标签流水线](https://github.com/Gk0Wk/openapi-thrift/actions/runs/34497892720) 的 13 项构建/检查全部通过：Go race/vet、安全扫描、WASM 与包门禁、六平台实际解包运行、独立 Go consumer，以及 Node 22/24 的安装调用。发布后另完成 registry 包安装、Windows 上的远端 Go consumer/CLI、GitHub 下载 CLI 在空 PATH 下运行，以及 npm registry signature 和 provenance attestation 验证。npm 来源证明明确记录该标签、提交、工作流和 run `34497892720`。

浏览器 JS、`wasm_exec.js` 和 WASM 的发布字节与此前 Edge 安装包 QA 使用的字节完全一致；正常转换、oneOf 拒绝及非法 YAML 的 Edge 验收证据沿用 [分发实现记录](2026-09-10-ci-unified-distribution.md)，错误后恢复由已发布 npm 包的安装测试覆盖。本机 Go 镜像首次返回新版本 404，切换本次进程到官方 Go proxy 后真实版本安装通过，未修改全局 Go 配置。

npm 发布命令返回 `E401: Failed to generate Web Auth URLs ... token is invalid`，但随后 registry 已包含该版本，包体与原产物相同，签名和 provenance 均验证通过。因此原流水线的 `publish` job 仍为 failure，不能宣称整条自动发行通过；未重跑 npm publish。已按既定部分发布恢复流程，用原 run 的七份包补齐 GitHub Release 并完成独立回读。该 npm 回执异常的根因尚未确认，后续项保留在 TODO。

这是包含初始化接口变更、移除 npm CLI 的 RC；Windows/macOS 二进制仍未做 OS 签名/公证。此次发行不代表真实消费者迁移、backend 快照更新、main 合并、所有浏览器/打包器验收或生产准入；共享 Go 核心的八个文件未因发行工作而改变。
