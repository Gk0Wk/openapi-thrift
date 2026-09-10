---
type: refactor
scope: runtime
audience: developer
summary: OpenAPI 规则迁入单一 Go 核心，提供原生 CLI 与浏览器 WASM
breaking: true
demo_ready: false
tests:
  - go test -race ./...
  - go vet ./...
  - go test -run ^$ -fuzz ^FuzzDocumentBoundary$ -fuzztime 20s -parallel 4 .
  - govulncheck ./...
  - pnpm lint
  - pnpm typecheck
  - pnpm test
  - pnpm pack:check
artifacts:
  - value.go
  - profile.go
  - projector.go
  - thrift.go
  - bridge.go
  - internal/cli
  - cmd/openapi-thrift
  - cmd/openapi-thrift-wasm
  - src/index.ts
  - tests/fixtures/go-core-reference.json
  - docs/go-core.md
---

## What changed

已批准 G1–G4 完成本地验收。Go 是唯一 profile/projector/renderer 规则核心；原生 CLI 接受 JSON/YAML，浏览器 WASM 复用同一实现。保留旧源码 `d501adc` 的结果、错误与源码摘要，并用完整 Go 结果比较与原有 42 项测试验证；新增初始化失败、重试、实例复用和错误后继续调用，WASM 共 43 项通过。

严格解析保留字段顺序，拒绝重复键、循环别名、非有限数值、多个文档和过深输入。合法递归对象可投影。原生 CLI 在无 Node PATH 下执行校验、真实 fixture 转换和输出替换；失败时原文件不变。Go race/vet 通过，20 秒 fuzz 执行 293290 次通过，govulncheck v1.7.0 报告未发现漏洞。

删除被替代的 TS profile/projector/route-index/Node CLI 源码、兼容 CLI 别名，以及本轮使用完的两个参考数据捕获脚本。源文件仍可从 Git `d501adc` 恢复；行为 corpus 和原始导出 fixture 保留。构建仅清理明确归编译器所有的退役输出，不递归删除整个 dist。浏览器薄绑定/类型、测试和 npm 包检查仍使用其原生 JS/TS 工具；它们不是后端运行前提。

真实 Edge 页面通过默认 YAML 转换、oneOf 拒绝、语法错误反馈和旧输出清空，console error/warn 为空。资源观测只包含本地 example/index/wasm_exec/WASM。lint/typecheck/pack 白名单通过；包体约 1.45 MB 压缩、5.30 MB 解包。

## Why it matters

后端工具能够调用 Go 转换器，浏览器继续使用相同的校验/投影规则；不再维护两套实现。字段编号、unsupported、手工 validator、响应包装和路由命名由同一回归基线保护。

## Demo posture / limitations

这次不代表后端整条生成链已无 Node、真实消费者迁移或发布。G5 的 backend 快照与真实 init/update/drift/补丁验收仍在进行。npm `0.2.0` 已发布版本仍为旧实现；没有发布、推送或自动更新任何消费者。

浏览器增加异步初始化和 WASM 资源分发要求；Go 标准运行时使包体大于旧 TS 包。CSP、Worker、资源和固定 Go 配对见 [Go/WASM](../../go-core.md)。没有承诺大型文档性能、全部浏览器或全部 bundler。旧 unsupported 契约仍保持；原始 APIFox 投影 fixture 的作者侧错误没有被悄悄改为有效。
