---
type: fix
scope: ci
audience: developer
summary: 发布 OpenAPI Thrift 0.3.0 稳定版并阻止 npm 自动重发发布请求覆盖原始错误
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
  - go run ./cmd/release verify-npm --tag v0.3.0 --archive .tmp/tag-34507267145/release-npm/sttot-openapi-thrift-0.3.0.tgz --output .tmp/registry-v0.3.0
  - npm run install:check -- .tmp/registry-v0.3.0/sttot-openapi-thrift-0.3.0.tgz --keep
  - npm audit signatures --prefix .tmp/npm-stable-signatures --registry=https://registry.npmjs.org
  - go run ./cmd/release smoke-go --module-version v0.3.0
  - go run ./cmd/release smoke --archive .tmp/github-v0.3.0/openapi-thrift_0.3.0_windows_amd64.zip
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

稳定发行经 [PR #2](https://github.com/Gk0Wk/openapi-thrift/pull/2) 合入 main，源提交为 `504062b01ad5478e665b8a27eb58a09f84e3eacb`。`v0.3.0` annotated tag 对象为 `92fb137347b8a6040c50680971c02886bc6f17ba`，指向该提交。[npm 0.3.0](https://www.npmjs.com/package/@sttot/openapi-thrift/v/0.3.0) 已公开，latest 指向稳定版、next 保留 RC；[GitHub 正式版](https://github.com/Gk0Wk/openapi-thrift/releases/tag/v0.3.0) 于 `2026-09-10T17:19:31Z` 发布八份资产并成为 latest release。

## Why it matters

RC 的原 run `34497892720` 在签名后约 72 秒返回 E401，但 registry 随后包含正确包体和来源证明。npm 11.9.0 的 `libnpmpublish` 通过 `npm-registry-fetch` 发送 PUT；`make-fetch-happen` 对部分 5xx/网络失败默认重试 PUT。使用真实客户端和 loopback 模拟 registry 的确定性测试复现：先接受包后响应 503，默认重试发出第二次 PUT，后续 401 覆盖原始错误；禁用重试后只有一次 PUT 并保留 E503。另验证首次 201 成功和首次 401 真实失败。测试不访问真实发布服务、不继承真实凭证，Go 核心使用者无需安装 Node。

依据：[固定版本 publish 源码](https://github.com/npm/cli/blob/v11.9.0/workspaces/libnpmpublish/lib/publish.js)、[npm fetch-retries 配置](https://docs.npmjs.com/cli/v11/using-npm/config#fetch-retries)、[官方可信发布](https://docs.npmjs.com/trusted-publishers/)。上游变更检查没有找到可证明修复该 RC 回执的 npm 升级，因此未用未经验证的工具升级替代原因判断。

## Demo posture / limitations

本地四种发布模拟、Go race/vet、lint/typecheck、43 项 WASM 测试、12 文件 npm 包门禁及 actionlint 已通过。候选 [34506597044](https://github.com/Gk0Wk/openapi-thrift/actions/runs/34506597044)、最终 main [34506930535](https://github.com/Gk0Wk/openapi-thrift/actions/runs/34506930535) 的完整 13 项检查均通过。

[标签 run 34507267145](https://github.com/Gk0Wk/openapi-thrift/actions/runs/34507267145) 的 13 项检查通过，OIDC exchange 返回 201，npm PUT 返回 200，GitHub Release 创建成功。发布后约三秒的版本索引回读报 ETARGET，因此该原 run 仍为 failure。后续独立回读验证了实际 registry tarball 与原标签构建的精确字节、npm registry signature 和 provenance、远端 Go API/CLI、GitHub 下载 Windows CLI 在空 PATH 下运行，以及全部七份包的 GitHub digest。回读流程修复与远端恢复另见 [记录](2026-09-11-ci-release-readback.md)，不重跑 publish、不移动标签。

npm tarball SHA-256 为 `301ae53f7fa6eed9610e801c63a8ef2605817f257034209f19de62858ab7c85c`；Go checksum 为 `h1:QujcxJAsz9xF4pH74TIHHFJNmUHum0gGmKyF9yhWJOQ=`，GoModSum 为 `h1:MM16VRv7ExMfDS7kwQxLgF9M9R4sZe6w9NJoejCxmQ4=`。npm provenance 记录相同源提交、tag、workflow 和 run `34507267145`，GitHub `SHA256SUMS` 文件的 SHA-256 为 `2807207b309fbb7a7f8fbf916f6f195ebac5dbed738090a022b8c486cb57794b`。

已发布 WASM SHA-256 为 `ac08badf38511e925284691925c49b8e0a1da81722b8754481d541df3f1ca432`，与此前 Edge QA 验收字节完全一致；安装测试覆盖正常、非法输入和错误后恢复。backend 的八个核心文件快照同样与稳定版逐文件一致。

历史 RC 没有请求级日志，无法确认其第一次响应或 npm registry 内部原因；本次修复的是已复现的重发/错误覆盖风险。它不保证外部服务永不失败；部分发行仍遵循原产物回读与恢复规则，失败 run 必须保留。稳定包相对 0.2.x 的 breaking 边界、OS 未签名和真实消费者/生产未迁移的限制仍有效。
