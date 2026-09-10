<div align="center">

# `@sttot/openapi-thrift`

受限 profile 的 OpenAPI 校验与渲染工具，面向中央 OpenAPI YAML 单向生成 APIFox、CloudWeGo `hz` / `kitex` Thrift IDL，以及前端 API client 的工程链路。

[![CI](https://github.com/Gk0Wk/openapi-thrift/actions/workflows/openapi-thrift-ci.yml/badge.svg)](https://github.com/Gk0Wk/openapi-thrift/actions/workflows/openapi-thrift-ci.yml)
[![npm version](https://img.shields.io/npm/v/%40sttot%2Fopenapi-thrift)](https://www.npmjs.com/package/@sttot/openapi-thrift)
[![npm downloads](https://img.shields.io/npm/dm/%40sttot%2Fopenapi-thrift?label=downloads)](https://www.npmjs.com/package/@sttot/openapi-thrift)
[![license](https://img.shields.io/github/license/Gk0Wk/openapi-thrift)](https://github.com/Gk0Wk/openapi-thrift/blob/main/LICENSE)
[![GitHub stars](https://img.shields.io/github/stars/Gk0Wk/openapi-thrift?style=social)](https://github.com/Gk0Wk/openapi-thrift/stargazers)

</div>

本仓对受限 OpenAPI profile 做显式、可审计的校验和渲染。Go 是唯一规则核心；原生 `openapi-thrift` CLI 与浏览器 WASM 共用它。`0.3.0` 是 Go/WASM 迁移的稳定版本，包含相对 `0.2.x` 的初始化接口和 CLI 不兼容变更；升级说明见 [版本说明](docs/releases/v0.3.0.md)。

## 为什么用它

- 只接受已冻结的 OpenAPI Render profile，不做“看起来能转、实际上丢语义”的静默兼容
- 在 APIFox 渲染、Thrift IDL 投影、前端 API 生成前统一校验同一份 OpenAPI
- 对 unsupported 能力默认 fail-fast，避免把非法契约带进 APIFox、`hz` / `kitex` 或前端 client
- Go 核心复用于原生 CLI、CI 和浏览器 WASM，后端无需安装 Node
- 当前内置 Thrift IDL 输出，便于继续接 `hz update`、`kitex`、模板化生成链路

## 它在标准工程链中的位置

这是三仓链路的最前置校验层，不负责生成完整后端或前端应用：

```text
OpenAPI / Apifox export
  -> openapi-thrift validate
  -> openapi-thrift thrift
  -> standard/backend 的 hz/kitex codegen
  -> 真实前端项目的 Orval client/types
```

因此，修改 profile、投影规则或 unsupported 列表前，必须评估后端 Thrift、前端生成和现有 fixture 的影响；不要在后端或前端用兼容代码掩盖这里的契约失败。

## 原生 CLI

发行采用同一版本的 npm/WASM、Go Module 和六平台 CLI；流程与 OIDC 配置见 [分发说明](docs/distribution.md)。按使用场景选择入口并固定完整版本：

```bash
npm install @sttot/openapi-thrift@0.3.0
go get github.com/Gk0Wk/openapi-thrift@v0.3.0
go install github.com/Gk0Wk/openapi-thrift/cmd/openapi-thrift@v0.3.0
openapi-thrift --version
```

也可从 [GitHub Release](https://github.com/Gk0Wk/openapi-thrift/releases/tag/v0.3.0) 下载 Windows/macOS/Linux amd64/arm64 的预编译 CLI，核对 `SHA256SUMS` 后解包；运行它不需要 Go 或 Node。稳定 npm 使用 `latest` 分发标签，实际项目仍固定完整版本。远端可用性以对应版本的 registry、Git tag 和 Release 资产为准。

```bash
go build -trimpath -o .tmp/openapi-thrift ./cmd/openapi-thrift
go run ./cmd/openapi-thrift validate --input ./project.openapi.yaml
go run ./cmd/openapi-thrift thrift --input ./project.openapi.yaml --output ./idl/project.thrift
```

使用本仓 `go.mod` 固定的 Go 1.26.6。原生构建只依赖 Go，YAML 解析库固定为 `go.yaml.in/yaml/v3 v3.0.5`；不调用 JS、Python、`hz` 或 `kitex`。浏览器包维护者另需 Node/pnpm 构建 TS 薄绑定；浏览器运行时不需要 Node。源码开发与版本安装均使用明确版本，不使用 `@latest`。

```bash
pnpm install --frozen-lockfile
pnpm build
```

## 目标

- 只支持 ISpark 已冻结的 OpenAPI profile
- 不做“万能转换器”
- 核心逻辑仅在 Go 中维护，浏览器绑定只负责初始化与结构化调用
- 第一版内置 APIFox/HZ-Thrift profile 校验和 `.thrift` 文本输出，不直接调用 `hz`、`kitex`、APIFox 或前端生成器

## OpenAPI Render profile

`validate` 子命令执行 `apifox-hz-thrift` profile，当前覆盖：

- APIFox 作者侧规则：`requestBody.examples` / `schema.example(s)` / parameter example 的类型一致性，字段级 `x-apifox-mock` 非空表达式，`mockScript` 非空且提示人工复核，JSON request body 必须使用 `application/json` 而不是 APIFox 导出风险较高的 `json` 简写。
- 数据模型引用规则：schema `$ref` 只允许 `#/components/schemas/*`，requestBody `$ref` 只允许 `#/components/requestBodies/*`，response `$ref` 只允许 `#/components/responses/*`，parameter `$ref` 会被判定为 Hz/Thrift 不可投影。
- Auth 规则：`securitySchemes` 只能使用标准 `apiKey/http/oauth2/openIdConnect`，operation/root `security` 必须引用已声明 scheme；公开接口使用 `security: []`。
- Hz/Thrift 可表达范围：复用下方 “当前支持 / 当前不支持” 的受限投影规则，并在校验阶段额外执行一次真实 Thrift 投影探针。

## 当前支持

- `OpenAPI 3.0/3.1 JSON 或 YAML`；保留属性顺序和原有 Thrift 字段编号
- `operationId`；若缺失，可通过 `--idl-dir` 从既有 Thrift 路由索引回填方法名
- `application/json` 主 request/response body，以及 APIFox 导出的 `json` content-type 别名
- 单一非 JSON success response：投影为带 `api.raw_body=""` 的响应包装结构
- `application/x-www-form-urlencoded` 请求体 -> `api.form`
- `multipart/form-data` 的普通字段 -> `api.form`
- `path/query/header/cookie` 参数
- 单一 `204` / 空 success response -> `EmptyResponse`
- query array 参数的默认 `form + explode=true`（重复 key）序列化
- object / array / scalar / 有限 `map<string, T>`
- 顶层 scalar requestBody 会退化为单字段 `api.raw_body=""` 请求结构
- 组件 schema 本地引用：`#/components/schemas/*`
- 组件 requestBody 本地引用：`#/components/requestBodies/*`
- `required` / `default` / `enum` 到 `go.tag validate/default` 的有限投影
- `minimum/maximum` -> `gte/lte`
- `minLength/maxLength` -> `min/max`
- `minItems/maxItems` -> `min/max`
- 白名单 `format` -> `validate` tag：`base64 / date / date-time / email / e164 / hexcolor / hostname / ipv4 / ipv6 / json / jwt / uri / url / ulid / uuid / uuid3 / uuid4 / uuid5`
- 标准标量格式：
  - `integer`: `int32 / int64`
  - `number`: `float / double`
  - `string`: `binary`
- nullable union：`anyOf [T, null]`
- 可组合对象 `allOf` 的最小支持（当前已覆盖 JobSeek 出现的单 `ref` 包装）
- 可通过 vendor extension 显式接管少量不支持的字段校验：
  - `x-ispark-allow-unsupported-validation: true`
  - `x-ispark-validate: "custom_rule"` 或 `string[]`

## 当前不支持

- `oneOf`
- 非 nullable 的 `anyOf`
- 复杂 `allOf` 继承拼装
- `pattern`（当前会显式报错；如确有统一 validator，可用 `x-ispark-validate` 显式接管）
- `multipleOf`（当前会显式报错；如确有统一 validator，可用 `x-ispark-validate` 显式接管）
- `additionalProperties: false`（当前会显式报错；这属于 binder/decoder 级约束，不能通过字段 validator 接管）
- 未列入白名单的 string `format`（当前会显式报错，要求手写 validator 或收紧 schema）
- `GET/DELETE/HEAD/OPTIONS` requestBody
- `multipart/form-data` 文件字段（单文件和多文件当前都会显式报错；APIFox 可导出 `string/binary`，但 Hertz 官方文档说明 IDL 场景不支持文件绑定）
- object query 参数
- `deepObject` query
- `spaceDelimited` / `pipeDelimited` query
- 非默认 `form + explode=true` 的 query array 序列化
- `parameter.content`
- `parameter $ref`
- 多个 `2xx` success response
- 并行多 `content-type`（当前 APIFox 导入/导出工作流也会把它收敛为单一 `application/json`）
- `oneOf + discriminator`
- 非 JSON success response 对应的 object/array schema
- `additionalProperties: true`
- 包含空白字符的 string enum

## 手写校验逃生口

当某个字段使用了当前不支持自动投影的规则，但你已经在 Hertz/validator 侧准备好了手写校验器，可以显式写：

```json
{
  "type": "string",
  "pattern": "^1\\d{10}$",
  "x-ispark-allow-unsupported-validation": true,
  "x-ispark-validate": "cn_mobile"
}
```

或者：

```json
{
  "type": "number",
  "multipleOf": 0.5,
  "x-ispark-allow-unsupported-validation": true,
  "x-ispark-validate": ["half_step", "gte=0"]
}
```

这条能力的含义是“我明确知道这里不是自动生成，而是人工接管”。如果只写允许开关、不提供 `x-ispark-validate`，converter 会直接报错。

### `x-dramawork-*` 兼容迁移

`x-ispark-*` 是当前 canonical 扩展。`x-dramawork-*` 仅保留为已发布文档和服务的兼容读取入口，并会产生弃用 warning。迁移期间允许双写，但同一 schema 上的新旧值必须完全一致；不一致会 fail-fast。新模板、示例和新接口只应写 `x-ispark-*`。后续移除旧字段前会单独发布 breaking change。

注意：

- 这条逃生口只适用于“字段 validator 能感知到的约束”，例如 `pattern`、`multipleOf`、部分未白名单 `format`
- `additionalProperties: false` 不属于这一类；JSON 绑定完成后，多余字段通常已经被丢弃，单靠字段 validator 无法恢复这条语义

## APIFox 边界矩阵

当前 APIFox 真实导出、converter 支持面和 profile 结论见：

- <https://github.com/Gk0Wk/openapi-thrift/blob/main/apifox_boundary_matrix_2026-04-13.md>

当前还额外固化了两份 APIFox 真实导出 followup fixture：

- `apifox-boundary-lab.followup.export.openapi.json`
  - 确认 `204` / 空 success response 会稳定保留
  - 确认并行多 `content-type` 在当前 APIFox 导入/导出工作流里会被收敛成单一 `application/json`
- `apifox-boundary-lab.header-cookie.export.openapi.json`
  - 确认 `header` / `cookie` 参数会稳定保留为标准 parameter
  - 确认 header 名大小写按导出结果原样保留，例如 `X-Request-ID`

## 使用方式

仓内直接运行，命令以本仓目录为当前路径：

```bash
go run ./cmd/openapi-thrift validate --input ./project.openapi.yaml --strict-warnings
```

生成 Thrift，可选现有 IDL 路由命名、namespace 与服务名：

```bash
go run ./cmd/openapi-thrift thrift --input ./project.openapi.yaml \
  --idl-dir ./idl --namespace ispark.project --service-name ProjectService \
  --output ./idl/project.thrift
```

CLI 输入上限 16 MiB，IDL 目录总读取上限 32 MiB，拒绝符号链接路径。转换成功后才替换输出文件；错误不破坏既有 IDL。JSON/YAML 重复键、YAML 别名环、多个文档、非有限数值和超过 128 层的输入会被拒绝；合法递归对象可投影，循环类型别名不可投影。

推荐的实际顺序是先 `validate`，再 `thrift`，最后进入后端 `cloudwego-codegen`。`thrift` 成功只代表当前 profile 可以投影，不代表 Hz/Kitex、业务实现或前端 API client 已经验证完成；后续必须分别运行对应仓库的 drift-check、构建和测试。

以库方式调用：

```ts
import {
  initializeOpenApiThrift,
  assertOpenApiRenderDocument,
  convertOpenApiToThrift,
} from "@sttot/openapi-thrift"

await initializeOpenApiThrift()
assertOpenApiRenderDocument(openApiDocument)

const result = convertOpenApiToThrift(openApiDocument, {
  namespace: "ispark.project",
  serviceName: "ProjectService",
})

console.log(result.thrift)
```

Node 脚本显式读取包内 WASM 资源（无需安装 Go）：

```ts
import { readFile } from "node:fs/promises"
import { initializeOpenApiThrift, convertOpenApiToThrift } from "@sttot/openapi-thrift"

await initializeOpenApiThrift({
  wasm: await readFile(new URL(import.meta.resolve("@sttot/openapi-thrift/openapi-thrift.wasm"))),
})
const result = convertOpenApiToThrift(openApiDocument)
```

浏览器必须先等待初始化，随后保留同步转换/校验 API。默认从 `index.js` 旁加载 `openapi-thrift.wasm`；打包器需保留该资源和固定 Go 配套 `wasm_exec.js`，也可传入 `{ wasmURL }` 或 `{ wasm: bytesOrModule }`。CSP 需允许同源资源与 `wasm-unsafe-eval`，无需 JS `unsafe-eval`。前端 npm 包已移除 Node CLI 和 `openapi-render` 别名；未自动迁移任何真实消费者。部署/worker 用法与包体限制见 [Go/WASM 边界](docs/go-core.md)。

真实浏览器示例：`pnpm build` 后运行 `go run ./cmd/browser-preview`，打开打印的 loopback URL。文档不会上传。较大输入建议在 Web Worker 内初始化和调用，避免占用页面线程。

## 开发命令

```bash
go test -race ./...
go vet ./...
pnpm typecheck
pnpm lint
pnpm test
pnpm pack:check
```

实际安装验收：`go run ./cmd/release smoke-go`；提交后 `go run ./cmd/release build` 构建并解包验证本机 CLI；`npm run install:check -- <package.tgz>` 只从 npm 压缩包安装、调用 JS/WASM。六平台矩阵与发布后远端安装由发行 workflow 执行。
