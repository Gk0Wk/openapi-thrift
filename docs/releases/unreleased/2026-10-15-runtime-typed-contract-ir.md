---
type: feature
scope: runtime
audience: developer
summary: 严格 JSON 输出类型化契约 IR 并统一 transport 元数据
breaking: true
demo_ready: false
tests:
  - go test -race ./...
  - go vet ./...
  - node --test tests/*.test.mjs
  - corepack pnpm@10.14.0 run lint
  - corepack pnpm@10.14.0 run typecheck
  - corepack pnpm@10.14.0 run pack:check
artifacts:
  - contract_ir.go
  - contract_ir_test.go
  - raw_json.go
  - src/index.ts
  - docs/strict-json-directional-plan.md
---

## What changed

公共 raw JSON IR 使用明确 Go 类型，不暴露 Value 树；operation 统一提供成功状态和参数来源。空闭合对象与显式空 required 集合在序列化后保留语义。与前一条交付共同实现严格 raw JSON 和方向独立 DTO。

## Why it matters

schema 规则和 transport 元数据来自同一个 Go 编译器，backend 无需重新解释 OpenAPI。显式标记、严格检查以及请求/响应分离是已批准的目标。

## Demo posture / limitations

原生 Go/race/vet、44 项 WASM 测试、lint/typecheck/pack 通过。这次不代表 npm/Go 版本已发布、真实消费者已迁移或 backend 安全准入；backend 完整验证仍受原有工具链/依赖漏洞扫描阻塞。IR 限于 raw JSON 操作，普通 HTTP 后处理不在本轮移除范围。
