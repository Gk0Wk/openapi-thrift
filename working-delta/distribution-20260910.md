# OpenAPI Thrift 同版本发行 brief

- Outcome：同一源码提交可交付 `@sttot/openapi-thrift` npm/WASM 包、Go Module 和六个 OS/arch 原生 CLI，所有发行物可核对版本、提交与 SHA-256。
- Context：沿用 Go 唯一核心与当前 JS 薄绑定；原生 CLI 已替代 Node CLI，npm 包需要显式异步初始化。起点 `a8422fd`，候选版本 `0.3.0-rc.1`。
- Constraints：保留包名与投影语义；Go 工具使用标准库，版本固定；发布同一份通过验收的 tarball；发布权限限制在发布 job，npm 使用 OIDC/provenance，预发布不覆盖稳定分发标签。
- Non-goals：不改 backend 快照、转换规则、实际消费者，不恢复 npm CLI，不改已有 npm 版本，不操作其他 worktree。
- Assumptions：首批 CLI 目标为 Windows/macOS/Linux 的 amd64/arm64；公开仓的 GitHub 托管 runner 足以执行原生安装 smoke。
- Success criteria：单元测试覆盖版本冲突、发行物校验与 CLI 标识；隔离目录安装 npm tarball 后解析/校验/转换；独立 Go consumer 与 `go install` 可用；六个平台的实际二进制运行 fixture；发布后核对 registry/tag/asset 身份。
- Verification plan：版本/打包工具测试 -> 本地 npm 与 Go 安装 smoke -> Go race/vet、JS lint/typecheck/test/pack -> 六平台 GitHub CI -> 发布后按同一入口回读 smoke。
- Stop rules：版本/标签已被其他提交占用、发布身份不一致、同一问题连续 2–3 次失败、所需权限超出本包发行，或必须由 owner 登录/2FA 时停止该环节并记录。
- Integration notes：owner Codex；隔离 worktree `.worktrees/openapi-thrift-release-20260910`，分支 `release/distribution-20260910`。仅在全部提交保留、canonical 同步且无 WIP 后才具备回收条件，本轮不执行历史清理。

## 发布前已知条件

- GitHub 仓库具备 push/admin 权限，当前没有 Actions secrets。
- 本机 `.local/npm-publish.npmrc` 的 `npm whoami` 返回 401；不读取或打印 token，不用失效凭证进行发布试错。
- 先完成全部可审阅代码、测试与发行产物；npm 可信发布配置若需 owner 交互，在最后提供精确字段。
