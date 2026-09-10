# OpenAPI Thrift

受限 APIFox/Hertz/Thrift 契约校验与投影；Go 唯一核心供原生 CLI 与浏览器 WASM 共用，源码候选 `0.3.0-rc.1` 尚未发布。

- `value.go / profile.go / projector.go / thrift.go`：解析、校验、投影与渲染。
- `cmd/openapi-thrift`：原生 CLI；`cmd/build-wasm`：固定 Go 构建；`src/`：浏览器薄绑定与类型。
- `tests/fixtures/`：真实导出及迁移基线；维护边界见 [README](README.md)、[Go/WASM 与 backend 集成](docs/go-core.md)、[矩阵](apifox_boundary_matrix_2026-04-13.md)。
- 原生：`go run ./cmd/openapi-thrift --help`；验证：`go test -race ./... && go vet ./...`。
- 浏览器维护：`pnpm install --frozen-lockfile`，`pnpm lint && pnpm typecheck && pnpm test && pnpm pack:check`；真实预览：`go run ./cmd/browser-preview`。
- 修改公共 profile、Thrift 语义或 unsupported 列表先评估三仓影响；当前迁移授权见 [TODO](TODO.md)。
