# openapi-thrift TODO

最后更新: 2026-04-13

## 当前状态

- [x] 已从 `web/packages/openapi-thrift` 迁移到 `standard/openapi-thrift`
- [x] 已补成独立 Node.js 项目，不再依赖前端 workspace 的 `tsconfig` 与脚本入口
- [x] 当前仍维持“受限 OpenAPI profile -> Thrift 文本投影器”定位，不扩成通用 codegen
- [x] 已补 `components.requestBodies` 本地引用支持
- [x] 已补非 JSON success response -> `api.raw_body` 响应包装结构投影
- [x] 已完成一轮 APIFox 边界实测对齐：当前 UI 路径下 `deepObject/object query`、非默认 query array 序列化、`parameter $ref`、多文件 multipart 仍不可稳定表达
- [x] 已将当前 APIFox 边界实验项目真实导出固化为测试 fixture
- [x] 已从真实导出裁出仅含支持能力的成功子集 fixture，并固化期望 Thrift 输出
- [x] 已确认 `multipart` 文件字段虽然可被 APIFox 导出，但 Hertz 官方文档说明 IDL 场景不支持文件绑定，因此 converter 继续保持 fail-fast
- [x] 已明确校验策略：`pattern` / `multipleOf` 只允许通过显式自定义 validator 接管；`additionalProperties:false` 归类为 binder/decoder 级约束，不允许走字段 validator 逃生口
- [x] 已补 APIFox 边界矩阵文档，正式区分“APIFox 可导出”“converter 可支持”“profile 是否允许”
- [x] 已补 `header/cookie`、`204 empty response`、并行多 `content-type`、`oneOf + discriminator` 的边界回归测试
- [x] 已补 APIFox followup 真实导出 fixture，确认 `204` 可稳定导出，而并行多 `content-type` 会在当前工作流中被 APIFox 收敛掉
- [x] 已补 APIFox `header/cookie` 专项真实导出 fixture，确认两类参数都会稳定保留为标准 parameter，且 header 名大小写按导出结果原样保留
- [x] 已补 npm 发布前最小收口：切到 `@sttot/openapi-thrift`、补 MIT `LICENSE`、限制 tarball 只发最小文件集，并新增根工作区发布 workflow
- [x] 已把仓库元数据对齐到 `Gk0Wk/openapi-thrift`，并将 workflow 改成以 CI / 安全审计 / 包体分析为主
- [x] 已重写 README 顶部展示区：补 badge、`npx @sttot/openapi-thrift` 推荐用法和更适合 GitHub 首页的结构

## 后续可继续

- [x] 继续按覆盖矩阵补 fixtures 与边界回归
- [x] 已决定发布为公开 npm 包 `@sttot/openapi-thrift`
- [x] 已补独立 CI workflow
- [ ] 待实际确认 GitHub 仓库 `Gk0Wk/openapi-thrift` 已创建并与 npm metadata 一致
- [ ] 待首次执行 GitHub Actions CI 并观察 `pnpm audit` 在 GitHub 环境下的噪声水平
- [ ] 待实际配置 `NPM_TOKEN` 并完成首次 npm 发布
