---
type: fix
scope: ci
audience: developer
summary: 准备 OpenAPI Thrift 0.3.0 稳定发行并阻止 npm 自动重发发布请求覆盖原始错误
breaking: true
demo_ready: false
tests:
  - npm run test:release
  - go test -race ./...
  - go vet ./...
  - pnpm lint
  - pnpm typecheck
  - pnpm test
  - pnpm pack:check
  - actionlint .github/workflows/openapi-thrift-release.yml
  - git diff --check
artifacts:
  - .github/workflows/openapi-thrift-release.yml
  - cmd/release/npm_test.go
  - package.json
  - docs/distribution.md
  - docs/releases/v0.3.0.md
  - README.md
  - CLAUDE.md
  - TODO.md
---

## What changed

Go 核心及统一发行候选经 [PR #1](https://github.com/Gk0Wk/openapi-thrift/pull/1) 合入 main，提交 `1f112a375adb45915e6588c1fccb5db6cde95d30`。本轮版本设为 `0.3.0`，安装文档和发布说明同步；稳定版本同时选择 npm latest 和 GitHub latest，RC 不改变稳定通道。转换核心、backend 来源快照与 JS/WASM API 不变。

发布步骤保留现有固定 npm 11.9.0、OIDC 和 provenance，新增 `NPM_CONFIG_FETCH_RETRIES=0` 与 HTTP 日志。发布失败仍返回失败；没有 token 回退、认证策略放宽、E401 成功兜底或覆盖已有版本。

## Why it matters

RC 的原 run `34497892720` 在签名后约 72 秒返回 E401，但 registry 随后包含正确包体和来源证明。npm 11.9.0 的 `libnpmpublish` 通过 `npm-registry-fetch` 发送 PUT；`make-fetch-happen` 对部分 5xx/网络失败默认重试 PUT。使用真实客户端和 loopback 模拟 registry 的确定性测试复现：先接受包后响应 503，默认重试发出第二次 PUT，后续 401 覆盖原始错误；禁用重试后只有一次 PUT 并保留 E503。另验证首次 201 成功和首次 401 真实失败。测试不访问真实发布服务、不继承真实凭证，Go 核心使用者无需安装 Node。

依据：[固定版本 publish 源码](https://github.com/npm/cli/blob/v11.9.0/workspaces/libnpmpublish/lib/publish.js)、[npm fetch-retries 配置](https://docs.npmjs.com/cli/v11/using-npm/config#fetch-retries)、[官方可信发布](https://docs.npmjs.com/trusted-publishers/)。上游变更检查没有找到可证明修复该 RC 回执的 npm 升级，因此未用未经验证的工具升级替代原因判断。

## Demo posture / limitations

本地四种发布模拟、Go race/vet、lint/typecheck、43 项 WASM 测试、12 文件 npm 包门禁及 actionlint 已通过。完整候选 CI、稳定 tag 发行与远端安装回读仍待本轮执行，不能从版本文件宣称已发布。

历史 RC 没有请求级日志，无法确认其第一次响应或 npm registry 内部原因；本次修复的是已复现的重发/错误覆盖风险。它不保证外部服务永不失败；部分发行仍遵循原产物回读与恢复规则，失败 run 必须保留。稳定包相对 0.2.x 的 breaking 边界、OS 未签名和真实消费者/生产未迁移的限制仍有效。
