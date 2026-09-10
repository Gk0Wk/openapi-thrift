---
type: feature
scope: ci
audience: developer
summary: 同一 Go 核心版本生成 npm、Go Module 与六平台原生 CLI，并验证实际安装后的调用
breaking: false
demo_ready: false
tests:
  - go test ./...
  - go test ./internal/cli -run TestVersionDoesNotRequireInputOrExternalTools -count=1
  - go run ./cmd/release smoke-go
  - npm run install:check -- .tmp/release/sttot-openapi-thrift-0.3.0-rc.1.tgz --keep
  - pnpm lint
  - go test -race ./...
  - go vet ./...
  - pnpm typecheck
  - pnpm test
  - pnpm pack:check
  - govulncheck ./...
  - pnpm audit --registry=https://registry.npmjs.org --audit-level high
artifacts:
  - .github/workflows/openapi-thrift-release.yml
  - .gitattributes
  - cmd/release
  - internal/cli/cli.go
  - scripts/check-package-install.mjs
  - scripts/package-consumer.mjs
  - docs/distribution.md
---

## What changed

新增原生归档构建、来源/校验和与实际解包 smoke；Go CLI 提供版本标识，Go Module 用独立消费项目和真实安装命令验证。npm tarball 在隔离目录离线安装并调用已安装 JS/WASM，浏览器预览可指向该安装目录。GitHub 流水线覆盖六个原生平台与 Node 22/24，并在全部 gate 通过后从标签发布同一份 tarball、Go 版本及 CLI。

Windows 新检出复现了 CRLF 导致的既有 Thrift fixture/lint 失败；在 `.gitattributes` 固定 LF 后原测试通过，未更改转换输出或放宽断言。CLI `--version` 先以失败测试复现缺失，再实现并通过该包回归。

## Why it matters

npm 包继续供 JS/TS 消费；Go 开发和预编译 CLI 使用者无需 Node。发行校验从源码测试延伸到真正安装后的 API、文件生成、错误处理、包体与来源一致性。只有 publish job 获得 OIDC 和 GitHub 写权限，不将长期 npm token 暴露给构建与测试。

## Demo posture / limitations

本地 Go race/vet、43 项 WASM 测试、lint/typecheck/pack、govulncheck/npm audit、npm 安装、Go consumer 与 CLI smoke 已通过。Edge 从已安装包加载 JS/WASM，正常 YAML 转换、oneOf 精确拒绝、非法 YAML 反馈及旧结果清空均通过；console warn/error 为空，资源清单只有 loopback example.js/index.js/wasm_exec.js/WASM。actionlint v1.7.12 检查通过。

实现提交 `c8b97e8d3040cf6303818e928a73a2dfe16de148` 的 [远端 CI](https://github.com/Gk0Wk/openapi-thrift/actions/runs/34491390131) 已通过全部 13 项构建/检查：六个平台分别实际解包执行原生 CLI 并编译独立 Go consumer，Node 22/24 安装并调用同一 npm tarball；普通分支的 publish 按设计跳过。

版本标签、npm/Go/六平台 CLI 发行及回读已完成，D1–D4 完整计划、OIDC 绑定、npm 回执异常与 GitHub 恢复证据见 [RC 发行记录](2026-09-11-ci-rc-publication.md)。此条目不代表实际服务迁移、OS 签名/公证或生产准入。
