---
type: feature
scope: runtime
audience: developer
summary: 增加显式标记的严格 raw JSON 契约与请求响应方向独立 DTO
breaking: true
demo_ready: false
tests:
  - go test -race ./...
  - go vet ./...
  - node --test tests/*.test.mjs
  - corepack pnpm@10.14.0 run lint
artifacts:
  - raw_json.go
  - rawjson/
  - directional.go
  - tests/raw-json.test.mjs
---

## What changed

只有组件显式写入 `x-ispark-raw-json: true` 才进入严格 raw JSON 闭包；重复键、未知字段、非安全整数、未声明 nullable 和不支持的 schema 语义都会失败。请求和成功响应共同引用的组件递归生成独立的 Request/Response 类型，线上 JSON 字段名保持原样。

## Why it matters

转换器、原生 CLI 和 WASM 共享同一份可版本化描述符，backend 可以在绑定请求和写出响应前执行相同校验，避免宽松解码、base64 二次编码和请求响应序列化规则互相污染。DTO 名称变化是已批准的公共生成契约变化，消费者重新生成前不能升级。

## Demo posture / limitations

这次不代表真实消费者迁移、发布、部署或生产准入。Hertz Core 扩展仍只在 NNM 的显式边界保留；本轮没有扩大到其他服务，也没有合并旧分支。
