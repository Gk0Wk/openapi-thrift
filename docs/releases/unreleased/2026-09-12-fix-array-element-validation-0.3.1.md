---
type: fix
scope: runtime
audience: developer
summary: 修复数组元素递归校验并同步 WASM 与 backend 固定快照
breaking: false
demo_ready: false
tests:
  - "go test -race ./..."
  - "go vet ./..."
  - "pnpm test"
  - "pnpm lint"
  - "pnpm typecheck"
  - "pnpm pack:check"
artifacts:
  - internal/projector/projector.go
  - tests/fixtures/apifox-boundary-lab.supported.thrift
  - tests/fixtures/go-core-reference.json
  - dist/openapi-thrift.wasm
---

## What changed

投影器现在对数组元素递归生成 `dive` 校验标签，并同步 Go 核心、WASM、支持子集 golden 和 backend 固定快照。0.3.0 已发布版本不包含该修复，下一发行候选为 0.3.1。

## Why it matters

数组字段的元素约束会进入生成 Thrift/Go 绑定，避免结构校验通过而非法元素进入服务运行时；native、WASM 与模板生成链共享同一修复事实源。

## Demo posture / limitations

这次不代表 0.3.1 已打 tag、推送、上传 npm 或发布六平台 CLI；发行仍需按 `docs/distribution.md` 执行候选构建、不可变 tag 和产物回读。当前本地候选包版本仍为 0.3.0。
