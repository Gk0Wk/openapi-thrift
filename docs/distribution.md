# npm、Go Module 与原生 CLI 发行

同一源码提交的 Go 核心提供三种入口：`@sttot/openapi-thrift` npm 包包含 JS/TS 接口、预编译 WASM 与同版本 `wasm_exec.js`；Go Module 为 `github.com/Gk0Wk/openapi-thrift`；原生 CLI 归档覆盖 Windows/macOS/Linux 的 amd64/arm64。npm 消费者不需要 Go 编译器，原生归档消费者不需要 Go 或 Node。

## 版本与来源

`package.json.version` 是发行版本输入；Git tag 必须为对应的 `v<version>`，Go module 使用该 tag。CLI 归档内的 `release.json` 记录版本、提交、Go 版本、平台和可执行文件 SHA-256；`--version` 输出版本及来源提交，`go install` 构建使用 Go module 的版本信息。

发行源码必须干净；所有平台按 `.gitattributes` 保持 LF，共享核心的八个文件继续保持精确字节。原生归档有逐文件 `.sha256`，GitHub Release 附带包含六个 CLI 归档及 npm tarball 的 `SHA256SUMS`。Go tag 与 npm 版本均不可重用或覆盖；修复发行内容必须新建版本。

## 准备与验收

1. 更新 `package.json` 的明确版本并添加 `docs/releases/v<version>.md`；使用 `go.mod` 固定的 Go、`.nvmrc` 的 Node 和固定 pnpm。预发布版本的 npm dist-tag 固定为 `next`，稳定版本为 `latest`。
2. 运行 `go test -race ./...`、`go vet ./...`、`pnpm lint`、`pnpm typecheck`、`pnpm test`、`pnpm pack:check`。新 CLI 版本标识、版本冲突、归档不可覆盖、路径/链接/重复项和损坏二进制均有回归测试。
3. 提交后运行 `go run ./cmd/release build`；归档从零解包后在清空 PATH 的环境执行版本、严格校验、转换及失败保留旧输出。运行 `go run ./cmd/release smoke-go` 验证独立 module 与 `go install` 本地源码。这里使用 replace 连接候选源码，不代表远端 tag 已发布。
4. `npm pack --ignore-scripts --pack-destination .tmp/release` 后，通过 `npm run install:check -- .tmp/release/sttot-openapi-thrift-<version>.tgz` 验证真实安装；只安装 tarball、离线、禁用 lifecycle hooks、PATH 仅有 Node。`--keep` 可保留本轮新建 consumer 供浏览器检查。
5. 浏览器验收使用 `go run ./cmd/browser-preview --package-root .tmp/npm-consumer-<id>/node_modules/@sttot/openapi-thrift`；运行的是已安装包的 JS/WASM。输入正常 YAML、unsupported schema 和非法 YAML，检查结果、错误、console 与 network。

`openapi-thrift-release.yml` 在 main、`release/distribution-*` 分支及手工触发时只构建验证；标签触发时，全部 gate 通过后发布。六个平台均使用 GitHub 原生 runner，实际运行各自解包后的二进制；Node 22/24 使用同一份 npm tarball 做独立消费测试。Windows/macOS 归档当前没有操作系统签名/公证，SHA-256 不能替代 OS 签名。

## npm 可信发布配置

本包采用 npm 官方 OIDC 可信发布，不依赖 `NPM_TOKEN`。由 npm 包 owner 在 `@sttot/openapi-thrift` 设置中添加 GitHub Actions publisher：

| 字段 | 值 |
| --- | --- |
| Organization or user | `Gk0Wk` |
| Repository | `openapi-thrift` |
| Workflow filename | `openapi-thrift-release.yml` |
| Environment name | 留空（workflow 未使用 environment） |
| Allowed actions | 允许 `npm publish` |

只在 publish job 开启 `id-token: write` 和 `contents: write`；安装依赖和测试 job 没有发布权限。npm CLI 固定为 `11.9.0`，发布直接消费已验收 tarball，开启 provenance；不在发布时重建或回退为无 provenance/token 发布。当前本机旧 npm 凭证返回 401，不能作为发布可用性证据。

官方依据：[npm 可信发布](https://docs.npmjs.com/trusted-publishers/)、[Go Module 发布](https://go.dev/doc/modules/publishing)、[GitHub 原生 runner](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)。

## 发布与回读

确认候选 CI 与 npm 绑定后，创建并推送匹配版本的 Git tag。流水线将检查 tag 指向当前提交，发布同一份 npm tarball、CLI 归档和版本说明；随后从 registry 重新下载 tarball，逐字节比较并安装运行，再通过真实远端版本执行 Go consumer 与 `go install`。

Git tag、npm registry 与 GitHub Release 之间不存在跨系统事务。若任一步失败，立即核对 tag SHA、npm `dist.integrity`、Actions 原产物和 GitHub assets；不要移动已公开 tag、覆盖资产、取消发布旧版本或为重试更换包体。npm 已成功而后续步骤失败时，不重跑 publish；用原 run 的已验证资产恢复缺失的 GitHub Release/回读步骤，并记录实际状态。

普通源码 push 不代表包已经发布。发行是否成功以对应 tag 的完整 Actions 结果、npm 版本及 GitHub Release 资产为准；此流程不迁移 backend 快照或任何真实服务。
