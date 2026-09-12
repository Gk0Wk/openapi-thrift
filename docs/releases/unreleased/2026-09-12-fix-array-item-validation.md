---
type: fix
scope: runtime
audience: developer
summary: 为数组元素生成递归的 validator dive 与元素约束
breaking: false
demo_ready: false
tests: go test -race ./...; go vet ./...
artifacts: projector.go, projector_test.go, tests/fixtures/go-core-reference.json
---

What changed

数组字段现在会递归投影元素的 `minLength`、`maxLength`、enum、format、手写校验以及嵌套数组/对象的 `dive`。对象元素继续依赖生成的嵌套 struct 校验，避免在父字段中拼接无法执行的完整 `go.tag`。

Why it matters

此前转换器只保留数组自身的约束，导致 primitive 数组元素、嵌套数组和带必填字段的 object 元素在 HTTP binder 中被放行。修复后，OpenAPI 的元素边界可以传递到模板生成层，避免把契约校验缺口留给业务代码。

Demo posture / limitations

这次不代表 backend snapshot、完整服务或五个 SDK 已发布；下游必须完成固定快照同步并重新运行生成器与 binding smoke。既有 fixture 中受影响的期望输出已明确更新。
