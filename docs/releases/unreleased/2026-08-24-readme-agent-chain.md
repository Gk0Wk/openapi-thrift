---
type: docs
scope: docs
audience: developer
summary: 补充 OpenAPI Thrift 在三仓生成链中的位置和 Agent 边界。
breaking: false
demo_ready: false
tests:
  - Docs and AGENTS local-link check
  - pnpm run lint
  - pnpm run typecheck
  - pnpm run test
  - pnpm run pack:check
artifacts:
  - README.md
  - AGENTS.md
  - docs/releases/unreleased/2026-08-24-readme-agent-chain.md
---

## What changed

README 新增三仓协作位置和 validate/thrift 后续验证边界；新增 `AGENTS.md`，规定 profile、converter、fixture 和下游影响的审查顺序。

## Why it matters

OpenAPI profile 变更不再被误认为单仓工具改动，agent 会明确评估后端 Thrift、前端 client 和 fixture 的联动影响。

## Demo posture / limitations

本轮只调整文档与 agent 治理，不改变 converter、CLI 输出或 profile 支持范围。
