---
type: fix
scope: ci
audience: developer
summary: 用明确版本和原构建产物验证 npm 发行，提供无发布权限的独立恢复流程
breaking: false
demo_ready: false
tests:
  - go test -race ./cmd/release
  - go vet ./cmd/release
  - actionlint .github/workflows/openapi-thrift-release.yml .github/workflows/openapi-thrift-release-verify.yml
artifacts:
  - cmd/release/registry.go
  - cmd/release/registry_test.go
  - cmd/release/main.go
  - .github/workflows/openapi-thrift-release.yml
  - .github/workflows/openapi-thrift-release-verify.yml
  - docs/distribution.md
  - CLAUDE.md
  - TODO.md
---

## What changed

稳定版 `v0.3.0` 的 [run 34507267145](https://github.com/Gk0Wk/openapi-thrift/actions/runs/34507267145) 已完成全部 13 项构建/检查；OIDC exchange 返回 201，npm 发布 PUT 返回 200，GitHub 正式版和八份资产均创建成功。发布后约三秒的 `npm pack @sttot/openapi-thrift@0.3.0` 回读报 ETARGET，导致原 run 仍为 failure。随后明确版本已能读取，npm latest 和签名/provenance 正常。

回读改为 Go 标准库 helper，通过明确版本的 metadata endpoint 和 tarball 地址校验原始 SHA-512 与精确字节，不依赖可变 package-version index。404/408/429/5xx 的只读等待最多 12 次且有两分钟总超时；身份、认证、响应大小、格式和包体不符立即失败。文件只在验证成功后以不可覆盖方式创建。

新的 `openapi-thrift-release-verify.yml` 是自动发行的后置 job，也是可指定原 tag/run 的手工只读恢复入口。它只有 contents/actions read，绑定 tag 与原工作流源提交，下载原构建和 GitHub 资产做校验，再实际安装 npm 和远端 Go consumer/CLI。原发布步骤不再承担回读，恢复时没有重新 publish 的通道。

## Why it matters

原流程把发布成功与版本索引立即可见当作同一个状态。读取精确发行身份并允许短暂传播延迟后，可以验证实际发布物；不再通过重复发布解决回读失败。原始日志没有 HTTP 级回读信息，不能进一步断言这次 ETARGET 来自 npm 本机缓存还是 registry/CDN 的可见性延迟。

确定性测试覆盖立即可见、metadata/包体传播、503、401、错误版本、摘要、下载源、损坏包体、预算耗尽、取消和超限响应；测试不访问真实外部资源。race/vet 和两个 workflow 的 actionlint 已通过。

## Demo posture / limitations

这是发行验证修复，不改变已发布的 0.3.0 包体、tag、转换核心或 API。真实发布物回读、修复提交的远端 CI 与独立恢复 workflow 仍在本轮执行。原发行 run 的失败事实必须保留；成功恢复不能被写成原 run 全绿或另一个版本已经发布。
