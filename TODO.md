# OpenAPI Thrift TODO

> Active-only tracker. Completed notes belong in `docs/releases/unreleased/` or stable docs.

## 同版本多入口发行（已批准）

执行边界与隔离目录见 [brief](working-delta/distribution-20260910.md)。

- [x] D1 版本与原生产物：由 package.json 冻结版本；Go CLI 版本/提交可核对；六个平台可执行文件、LICENSE、摘要与校验和。
- [x] D2 安装验收：隔离 npm tarball 调用、独立 Go consumer、原生 CLI 正常/失败保留输出；验证发布产物，不依赖工作树 dist。
- [x] D3 流水线：固定 action/工具，六平台运行验收，同提交 npm/Go/CLI；npm OIDC/provenance，RC 使用 next 分发标签。[实现提交 CI](https://github.com/Gk0Wk/openapi-thrift/actions/runs/34491390131) 的 13 项构建/检查全部通过。
- [ ] D4 文档与交付：Go/npm/CLI 安装和升级说明已完成；待 npm owner 完成保存 OIDC 绑定时的安全验证，再推送版本标签、发布并核对 registry 与 Go 远端安装结果。
