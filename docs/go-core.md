# Go 核心与浏览器边界

事实源在本仓的 Go `Value → profile/projector → ThriftDocument → renderer`。原生 CLI 在 `internal/cli` 处理本地文件；WASM 通过 `bridge.go` 的结构化 JSON 消息调用同一核心。浏览器只保留加载、类型和错误类，不下载或执行 Node，不保存第二份校验/转换规则。`standard/backend` 消费可核验的源码快照，不能自行改写投影语义。

## 输入与规则保留

JSON/YAML 解析保留字段顺序，数字对象键仍按 ECMAScript 规则排序，以保持已发布转换器的字段编号。数值与旧 JS 一样采用有限 float64。输入最多 16 MiB、128 层、100 万节点；拒绝重复键、别名环、多个文档与非 JSON YAML tag，支持有界 alias/merge 和显式覆盖。校验遍历最多 10 万节点、4096 个普通 issue，超出后增加明确错误；结构体引用可递归，循环别名失败。

稳定 YAML v3.0.5 来自 [官方 go-yaml](https://github.com/yaml/go-yaml)，固定在 go.mod/go.sum；当前 v4 为预发布版本，本次不为语言迁移引入另一项实验。依赖不得随构建浮动。

`tests/fixtures/go-core-reference.json` 固化旧源码 `d501adc74b9083f1790e9fe126dfa950d0d2cf75` 的调用参数、结果、错误和源码摘要；它只作回归事实，不是另一份算法。原有 42 项测试改为调用 Go WASM，原生回归另外比较完整投影对象、Thrift 文本、定位、错误码和顺序。既有 `apifox-boundary-lab.supported.openapi.json` 是投影正例，含作者侧 example 类型问题，不能以文件名代替 profile 验收。

## 浏览器接入

先 `await initializeOpenApiThrift()`，之后转换/校验继续同步返回。初始化支持 `wasmURL` 或调用方传入的字节/预编译 `WebAssembly.Module`；同一模块重复调用共用初始化 promise，加载失败可重试。输入不离开当前运行环境，不解析远程 `$ref`，也不执行 mockScript。

构建固定 Go 1.26.6，WASM 与 `wasm_exec.js` 必须成对分发。后者按 package.json sideEffects 保留，禁止被 tree-shaking 丢弃。资源以 `application/wasm` 提供；CSP 允许 `script-src 'self' 'wasm-unsafe-eval'` 和对应 `connect-src`。示例服务仅监听 loopback，且只允许示例/构建资源路径。没有独立 converter HTTP API。

包体从旧 TypeScript 的约 26 KB gzip 增加为约 1.45 MB gzip / 5.30 MB 解包；包门禁上限为 2.5 MB 压缩 / 10 MB 解包。它换取单核心维护，不能宣称浏览器更轻。按需加载并缓存静态资源；大文档使用 Web Worker。原生 CLI 无此下载和 JS 初始化需求。包仍是本地未发布状态，真实前端 owner 必须独立执行初始化和资源部署迁移。

## 验证与发布边界

原生验证 `go test -race ./...`、`go vet ./...`；CLI 测试显式清空 Node PATH，执行 YAML validate 与真实 fixture 的文件生成/替换，并确认失败时保留旧文件。浏览器维护侧运行 lint/typecheck、43 项 WASM/生命周期测试与 pack 内容白名单；Node 在这里只用于 TS/npm 生态。

2026-09-10 Edge 的本机页面实测：默认 YAML 成功生成 EmptyResponse/Health 方法；oneOf 显示两个明确 issue 并隐藏旧输出；非法 YAML 显示解析错误。浏览器资源清单只有 example.js、index.js、wasm_exec.js、WASM，均为 loopback；控制台 error/warn 为空。该结果不代表大型文档性能、所有浏览器或所有打包器均已验收。

源码候选为 `0.3.0-rc.1`，未发布 npm/Go 版本，未修改真实消费者。核心字节由 `.gitattributes` 在各平台保留；backend 的来源锁固定 clean commit、8 个文件哈希和完整树哈希。候选与 CI 边界见 [交付说明](releases/unreleased/2026-09-10-workspace-go-candidate.md)。backend 完整无 Node/Python PATH 的 scaffold/codegen/drift/补丁、真实 JSON/YAML 和双框架验收已通过，见 [跨仓验收](releases/unreleased/2026-09-10-workspace-backend-go-integration.md)。该结果不代表后端性能/存储实验或生产准入。
