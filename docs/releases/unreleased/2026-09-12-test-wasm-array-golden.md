---
type: test
scope: ci
audience: developer
summary: 同步数组递归投影在 WASM 支持子集中的期望结果
breaking: false
demo_ready: false
tests: pnpm test; pnpm lint; pnpm typecheck; pnpm pack:check
artifacts: tests/fixtures/apifox-boundary-lab.supported.thrift
---

What changed

支持子集 golden 中两项字符串数组和一项对象数组补充 owner 核心已生成的 `dive` 和元素约束。修复前 WASM 测试 42/43 通过，唯一失败是这三处旧期望。

Why it matters

Go 和 WASM 使用同一核心，发布前必须同时验证两个入口。此次仅同步已确认行为的 fixture，没有增加另一套转换规则。

Demo posture / limitations

本地构建和打包验收不代表 0.3.1 已发布；版本仍为 0.3.0，新的 npm、Go tag 和六平台发行需要独立 release workflow 与回读证据。
