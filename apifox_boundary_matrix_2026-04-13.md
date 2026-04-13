# APIFox 边界矩阵（2026-04-13）

这份矩阵基于 APIFox 项目 `DramaWork OpenAPI Thrift Boundary Lab` 的真实导出文件 [apifox-boundary-lab.export.openapi.json](D:/Developer/Project/DramaWork/standard/openapi-thrift/tests/fixtures/apifox-boundary-lab.export.openapi.json)。

## 判定规则

- `APIFox 可导出` 只表示当前 UI 路径下，最终 `OpenAPI export` 中出现了对应结构。
- `converter 可支持` 只表示 `openapi-thrift` 当前可以稳定投影到 Thrift。
- `纳入 profile` 表示允许进入主线；`禁止项` 表示必须在 APIFox 侧改写，或升级统一 binder / codegen 后再谈。

## 矩阵

| case | APIFox 实际导出 | converter 现状 | profile 结论 |
| --- | --- | --- | --- |
| `requestBody $ref` | 可导出 `#/components/requestBodies/*` 或 schema `$ref` | 已支持 `components.requestBodies` 本地引用 | 纳入 profile |
| `header/cookie` 参数 | 可稳定导出标准 parameter schema | 已支持 | 纳入 profile |
| `204` / 空 success response | 已验证可稳定导出无 content 的 `204` | 已支持投影为 `EmptyResponse` | 纳入 profile |
| 非 JSON success response | 可导出单一非 JSON content-type | 已支持投影为 `api.raw_body=""` 包装响应 | 纳入 profile；仅允许单一主 content-type，且 schema 必须是标量或省略 |
| `pattern` | 可导出标准 `pattern` | 不自动投影；支持显式自定义 validator 接管 | 可留在 profile，但必须显式人工接管 |
| `multipleOf` | 可导出标准 `multipleOf` | 不自动投影；支持显式自定义 validator 接管 | 可留在 profile，但必须显式人工接管 |
| `additionalProperties: false` | 可导出标准 `additionalProperties: false` | 不支持，且不允许字段 validator 逃生口 | 禁止项；需要统一 strict binder/decoder 后再谈 |
| `oneOf` | 当前已验证 APIFox 可导出 `oneOf` | 不支持 | 禁止项 |
| `oneOf + discriminator` | 当前可导出，但仍属于 `oneOf` 族 | 不支持 | 禁止项 |
| 多个 `2xx` success response | 当前实验项目未稳定落到多个 `2xx`；导出仍可能被收敛成单 `200` | 不支持多个 `2xx` | 禁止项 |
| 并行多 `content-type` | 标准 OpenAPI 可表达；但当前 APIFox 导入/导出实验会把它收敛成单一 `application/json` | 不支持请求或响应并行多主 content-type；当前 APIFox 工作流本身也不稳定保留该语义 | 禁止项 |
| `parameter $ref` | 当前 UI 路径未稳定导出参数引用 | 不支持 | 禁止项 |
| object query / `deepObject` | 当前 UI 路径不可稳定表达；导出仍回到简单 query 字段 | 不支持 | 禁止项 |
| 非默认 query array 序列化 | 当前 UI 路径未暴露 `style/explode`，导出回到默认数组 | 仅支持 `form + explode=true` | 禁止项 |
| `multipart` 普通字段 | 可导出标准 `multipart/form-data` object | 已支持 | 纳入 profile |
| `multipart` 文件字段 | APIFox 可导出 `string + format=binary` | 明确不支持 | 禁止项；Hertz 官方文档说明 IDL 场景不支持文件绑定 |

## 成功子集 fixture

已从真实导出裁出一份“当前支持能力子集”：

- OpenAPI fixture: [apifox-boundary-lab.supported.openapi.json](D:/Developer/Project/DramaWork/standard/openapi-thrift/tests/fixtures/apifox-boundary-lab.supported.openapi.json)
- 期望 Thrift 输出: [apifox-boundary-lab.supported.thrift](D:/Developer/Project/DramaWork/standard/openapi-thrift/tests/fixtures/apifox-boundary-lab.supported.thrift)

这份子集目前覆盖：

- form-urlencoded
- multipart 普通字段
- 默认 query array
- path/query/header/cookie
- top-level scalar `raw_body`
- `nullable`
- schema / requestBody 引用的已支持形态

## Followup fixture

已额外固化一份 followup APIFox 真实导出：

- [apifox-boundary-lab.followup.export.openapi.json](D:/Developer/Project/DramaWork/standard/openapi-thrift/tests/fixtures/apifox-boundary-lab.followup.export.openapi.json)
- [apifox-boundary-lab.header-cookie.export.openapi.json](D:/Developer/Project/DramaWork/standard/openapi-thrift/tests/fixtures/apifox-boundary-lab.header-cookie.export.openapi.json)

它们确认了三件事：

- `204` / 空 success response 当前可稳定保留在 APIFox 导出里
- 并行多 `content-type` 在当前 APIFox 导入/导出工作流中会被压成只剩 `application/json`
- `header` / `cookie` 参数当前可稳定保留为标准 parameter，header 名大小写按导出结果原样保留

## 当前策略

- 对“APIFox 可导出但 Hertz/Thrift 主线无法稳定承接”的能力，默认保持 fail-fast。
- 对“字段 validator 可以统一接管”的能力，只允许通过显式 vendor extension 接管，不做 silent downgrade。
- 对“binder/decoder 级约束”，不得伪装成字段 validator 支持。
