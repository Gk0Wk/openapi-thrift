# 严格 JSON 与方向独立 DTO

2026-10-13 批准显式严格 raw JSON、请求响应 DTO 分开；2026-10-15 批准 Go 编译器唯一规则源、类型化 IR、Hertz 模板直接生成严格 transport、可恢复输出和 clean commit 快照。

## 已落实边界

- `x-ispark-raw-json: true` 只能标记具名组件。使用闭合对象、显式 nullable、精确整数和受限 discriminator 联合；未知语义拒绝，不能自动退回宽松 JSON。
- 请求和成功响应共同可达的组件递归生成 `<Name>Request` / `<Name>Response`，单方向组件保持原名；名称冲突拒绝。JSON 字段名不变。生成类型名称变化属于 breaking change，消费者需重新生成并调整引用。
- required 响应 nullable/map 保留 key。raw envelope 使用准确 JSON tag，参数与 body 独立绑定，响应在写出前校验。
- `CompileRawJSONContract` 输出 version 1 类型化 IR，包含 schema、操作、参数来源和唯一成功状态码。解析器 `Value` 不进入公共 IR；TS/WASM 与 backend 共用 Go 核心。
- backend 用 Hertz 模板生成 raw handler，用 thriftgo 定义类型选项生成 raw 类型；codec/runtime 发射器只读生成的 Go 类型元数据，不再改写 raw handler。
- raw codec/runtime 整批准备、格式校验，再替换文件；捕获失败恢复原字节，中断保留恢复日志并阻止重试。不是整个 Hz 工程的原子事务；同一输出目录必须单写者。

## 验收与限制

converter：Go/race/vet、44 项 WASM 测试、lint/typecheck/pack；backend：真实 Hz 生成、HTTP 正负用例、重复 update、drift、文件替换故障注入。backend 固定 converter clean commit 与文件树哈希，不能另写 schema 规则。

普通 HTTP 的既有 AST 后处理尚未迁移；真实消费者、发布、部署、Hertz Core decorator 扩围均不在本轮。依赖安全扫描失败必须保留为发布阻塞，不得用功能测试通过替代安全准入。
