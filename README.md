<div align="center">

# `@sttot/openapi-thrift`

受限 profile 的 `OpenAPI 3.0 JSON -> Thrift IDL` 投影器，面向 APIFox / OpenAPI 到 CloudWeGo `hz` / `kitex` 的工程链路。

[![CI](https://github.com/Gk0Wk/openapi-thrift/actions/workflows/openapi-thrift-ci.yml/badge.svg)](https://github.com/Gk0Wk/openapi-thrift/actions/workflows/openapi-thrift-ci.yml)
[![npm version](https://img.shields.io/npm/v/%40sttot%2Fopenapi-thrift)](https://www.npmjs.com/package/@sttot/openapi-thrift)
[![npm downloads](https://img.shields.io/npm/dm/%40sttot%2Fopenapi-thrift?label=downloads)](https://www.npmjs.com/package/@sttot/openapi-thrift)
[![license](https://img.shields.io/github/license/Gk0Wk/openapi-thrift)](https://github.com/Gk0Wk/openapi-thrift/blob/main/LICENSE)
[![node](https://img.shields.io/badge/node-%3E%3D20-5fa04e?logo=nodedotjs&logoColor=white)](https://www.npmjs.com/package/@sttot/openapi-thrift)
[![GitHub stars](https://img.shields.io/github/stars/Gk0Wk/openapi-thrift?style=social)](https://github.com/Gk0Wk/openapi-thrift/stargazers)

</div>

它不是通用 OpenAPI codegen，也不会默默降级 unsupported schema；定位就是一个对受限 OpenAPI profile 做显式、可审计、fail-fast 的 Thrift 文本投影器。

## 为什么用它

- 只接受已冻结的 OpenAPI profile，不做“看起来能转、实际上丢语义”的静默兼容
- 对 unsupported 能力默认 fail-fast，避免把非法契约带进 `hz` / `kitex`
- 纯 TypeScript 实现，同一套逻辑可在浏览器、Node CLI、CI 中复用
- 默认输出 `.thrift` 文本，便于继续接 `hz update`、`kitex`、模板化生成链路

## 安装

```bash
npm install @sttot/openapi-thrift
```

或：

```bash
pnpm add @sttot/openapi-thrift
```

## 目标

- 只支持 DramaWork 已冻结的 OpenAPI profile
- 不做“万能转换器”
- 核心逻辑保持纯 TypeScript，便于浏览器和 CLI 复用
- 第一版只输出 `.thrift` 文本，不直接调用 `hz` 或 `kitex`

## 当前支持

- `OpenAPI 3.0 JSON`
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
  - `x-dramawork-allow-unsupported-validation: true`
  - `x-dramawork-validate: "custom_rule"` 或 `string[]`

## 当前不支持

- `oneOf`
- 非 nullable 的 `anyOf`
- 复杂 `allOf` 继承拼装
- `pattern`（当前会显式报错；如确有统一 validator，可用 `x-dramawork-validate` 显式接管）
- `multipleOf`（当前会显式报错；如确有统一 validator，可用 `x-dramawork-validate` 显式接管）
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
  "x-dramawork-allow-unsupported-validation": true,
  "x-dramawork-validate": "cn_mobile"
}
```

或者：

```json
{
  "type": "number",
  "multipleOf": 0.5,
  "x-dramawork-allow-unsupported-validation": true,
  "x-dramawork-validate": ["half_step", "gte=0"]
}
```

这条能力的含义是“我明确知道这里不是自动生成，而是人工接管”。如果只写允许开关、不提供 `x-dramawork-validate`，converter 会直接报错。

注意：

- 这条逃生口只适用于“字段 validator 能感知到的约束”，例如 `pattern`、`multipleOf`、部分未白名单 `format`
- `additionalProperties: false` 不属于这一类；JSON 绑定完成后，多余字段通常已经被丢弃，单靠字段 validator 无法恢复这条语义

## APIFox 边界矩阵

当前 APIFox 真实导出、converter 支持面和 profile 结论见：

- <https://github.com/sttot/openapi-thrift/blob/main/apifox_boundary_matrix_2026-04-13.md>

当前还额外固化了两份 APIFox 真实导出 followup fixture：

- `apifox-boundary-lab.followup.export.openapi.json`
  - 确认 `204` / 空 success response 会稳定保留
  - 确认并行多 `content-type` 在当前 APIFox 导入/导出工作流里会被收敛成单一 `application/json`
- `apifox-boundary-lab.header-cookie.export.openapi.json`
  - 确认 `header` / `cookie` 参数会稳定保留为标准 parameter
  - 确认 header 名大小写按导出结果原样保留，例如 `X-Request-ID`

## 使用方式

仓内开发先构建：

```bash
pnpm --dir standard/openapi-thrift build
```

如果从 npm 安装，建议直接用 `npx`：

```bash
npx @sttot/openapi-thrift --input ./project.openapi.json --output ./idl/project.thrift
```

再运行 CLI：

```bash
node standard/openapi-thrift/dist/cli.js \
  --input ./project.openapi.json \
  --namespace dramawork.project \
  --service-name ProjectService
```

带既有 IDL 路由索引一起运行：

```bash
node standard/openapi-thrift/dist/cli.js \
  --input ./project.openapi.json \
  --idl-dir D:/Developer/JobSeek/tutoring-demand/idl \
  --output ./idl/project.thrift
```

输出到文件：

```bash
node standard/openapi-thrift/dist/cli.js \
  --input ./project.openapi.json \
  --output ./idl/project.thrift
```

以库方式调用：

```ts
import { convertOpenApiToThrift } from "@sttot/openapi-thrift"

const result = convertOpenApiToThrift(openApiDocument, {
  namespace: "dramawork.project",
  serviceName: "ProjectService",
})

console.log(result.thrift)
```

## 开发命令

```bash
pnpm --dir standard/openapi-thrift typecheck
pnpm --dir standard/openapi-thrift lint
pnpm --dir standard/openapi-thrift build
pnpm --dir standard/openapi-thrift test
```
