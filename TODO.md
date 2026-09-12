# OpenAPI Thrift TODO

> Active-only tracker. Completed notes belong in `docs/releases/unreleased/` or stable docs.

当前进行中的发行任务：为数组 schema 生成递归的 `dive` 与元素约束，确保 primitive/object/nested array 的运行时校验与 OpenAPI 输入一致。完成条件是 Go corpus、正向数组用例、race 和 vet 全部通过；下游 backend snapshot 需在独立同步步骤更新。
