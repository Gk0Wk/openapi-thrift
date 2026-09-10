# OpenAPI Thrift TODO

> Active-only tracker. Completed notes belong in `docs/releases/unreleased/` or stable docs.

## npm 发行回执异常

- [ ] 核对 npm 在版本已入库且 provenance 正确时仍返回 E401 的原因。[本轮证据与恢复结果](docs/releases/unreleased/2026-09-11-ci-rc-publication.md) 已归档；RC 三个发行入口和安装回读已完成，原 Actions publish job 保留失败事实。错误来源尚不能定位到 npm 客户端或 registry，未用 token/provenance 降级或无条件重试掩盖问题。后续发行需继续检查回执与 registry 实际状态。
