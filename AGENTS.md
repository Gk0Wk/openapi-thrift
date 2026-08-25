# OpenAPI Thrift Agent Guide

## 角色与事实源

本仓是 ISpark 三仓生成链的契约校验和投影层，不是通用 OpenAPI codegen，也不直接生成完整 Hertz、Kitex 或前端应用。

```text
OpenAPI / Apifox export
  -> validate (profile gate)
  -> thrift (derived IDL)
  -> standard/backend (Hz/Kitex)
  -> real frontend project (Orval client/types)
```

OpenAPI 是输入事实源，Thrift 是派生物。任何 profile 或投影变化都必须考虑后端生成、前端生成和 fixture 的联动影响。

## 必须先读

1. 本仓 [README.md](README.md)。
2. [apifox_boundary_matrix_2026-04-13.md](apifox_boundary_matrix_2026-04-13.md)。
3. [后端 codegen 链路](../backend/docs/codegen-flow.md) 和 [前端 API codegen 链路](../frontend/docs/api-codegen.md)。

## 修改与验证

- 新增支持能力：同时补 model/profile/projector、正向 fixture、负向 fixture、边界矩阵和 README。
- 不支持的 schema 必须 fail-fast；不要在下游通过宽松类型、字符串拼接或默认值猜测恢复语义。
- 保持 CLI `openapi-thrift` 为 canonical 名称，`openapi-render` 只作为兼容别名。
- 使用固定版本，禁止文档和脚本引用 `latest`。
- `x-ispark-*` 是当前 canonical vendor extension；`x-dramawork-*` 仅作兼容读取并产生弃用 warning。双写时值必须一致，冲突必须 fail-fast。

```bash
pnpm install --frozen-lockfile
pnpm run lint
pnpm run typecheck
pnpm run test
pnpm run pack:check
```

修改公共 profile、Thrift 输出、vendor extension 或 unsupported 列表前必须停下，列出下游影响并先确认契约边界；不能只修一个 converter 测试就宣称三仓链路完成。
