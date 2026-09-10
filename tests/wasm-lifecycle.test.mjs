import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import test from "node:test"

import {
  assertOpenApiRenderDocument,
  convertOpenApiToThrift,
  initializeOpenApiThrift,
  OpenApiRenderValidationError,
  parseOpenApiDocument,
  renderThriftDocument,
} from "../dist/index.js"

test("initialization fails clearly, retries, and shares one instance", async () => {
  assert.throws(() => convertOpenApiToThrift("{}"), /initializeOpenApiThrift/)
  await assert.rejects(
    initializeOpenApiThrift({ wasm: new Uint8Array([0, 1, 2]) }),
  )
  const options = {
    wasm: readFileSync(new URL("../dist/openapi-thrift.wasm", import.meta.url)),
  }
  const first = initializeOpenApiThrift(options)
  assert.equal(first, initializeOpenApiThrift(options))
  await first
  const yaml =
    "openapi: 3.0.3\npaths:\n  /health:\n    get:\n      operationId: health\n      responses:\n        204:\n          description: OK\n"
  const parsed = parseOpenApiDocument(yaml)
  assert.equal(parsed.openapi, "3.0.3")
  const output = convertOpenApiToThrift(parsed)
  assert.equal(renderThriftDocument(output.document), output.thrift)
  assert.throws(
    () => assertOpenApiRenderDocument({ openapi: "2.0", paths: {} }),
    OpenApiRenderValidationError,
  )
  assert.throws(() => convertOpenApiToThrift("openapi: ["))
  assert.equal(
    convertOpenApiToThrift(yaml).thrift,
    output.thrift,
    "errors must not kill the Go runtime",
  )
})
