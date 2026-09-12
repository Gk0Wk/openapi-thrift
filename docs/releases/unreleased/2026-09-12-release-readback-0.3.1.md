---
type: docs
scope: ci
audience: developer
summary: 记录 0.3.1 稳定发行与原始产物回读
breaking: false
demo_ready: true
tests:
  - "GitHub Actions run 34691064113: completed success"
  - "npm view @sttot/openapi-thrift@0.3.1"
  - "GitHub Release API v0.3.1: 6 native archives + npm tarball + SHA256SUMS"
artifacts:
  - https://github.com/Gk0Wk/openapi-thrift/releases/tag/v0.3.1
  - https://github.com/Gk0Wk/openapi-thrift/actions/runs/34691064113
  - https://registry.npmjs.org/@sttot/openapi-thrift/-/openapi-thrift-0.3.1.tgz
---

## What changed

`v0.3.1` 已从提交 `2d4902c3042e6798820d0c11633898caddf64aa4` 推送并完成 GitHub Actions 发布。工作流通过六平台构建、npm consumer、npm OIDC provenance、GitHub Release 和原始产物回读。

## Why it matters

npm 版本 `0.3.1`、稳定 dist-tag、Git tag、GitHub Release 资产和原始构建产物已由同一工作流绑定并校验，数组元素递归校验修复现在可被 npm、Go module 和 CLI 消费者使用。

## Demo posture / limitations

这次不代表 NewNanManager 后端已发布或生产服务已切换；本次发行只覆盖 `openapi-thrift` 工具链及其六平台 CLI/npm 包。
