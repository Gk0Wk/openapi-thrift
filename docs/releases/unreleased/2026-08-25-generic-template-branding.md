---
type: docs
scope: docs
audience: developer
summary: 移除 OpenAPI Thrift 文档和测试样例中的具体产品品牌。
breaking: false
demo_ready: false
tests:
  - Branding reference scan
  - pnpm run lint
  - pnpm run typecheck
  - pnpm run test
  - pnpm run pack:check
artifacts:
  - README.md
  - AGENTS.md
  - src/
  - tests/
---

## What changed

默认 namespace、CLI 示例和 APIFox fixture 标题统一改为 ISpark 通用命名；`x-dramawork-*` 公共兼容扩展保持不变。

## Why it matters

工具仓可以作为 ISpark 通用 OpenAPI 投影入口复用，而不会把具体产品名称带入新契约样例。

## Demo posture / limitations

默认 namespace 和测试 fixture 名称发生变化；调用方显式传入 namespace 的行为不受影响，兼容扩展未改名。
