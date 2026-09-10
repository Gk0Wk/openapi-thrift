// Copied into a fresh consumer directory; all imports resolve from installed npm.
import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import {
  assertOpenApiRenderDocument,
  convertOpenApiToThrift,
  initializeOpenApiThrift,
  OpenApiRenderValidationError,
  parseOpenApiDocument,
} from "@sttot/openapi-thrift"

const entry = new URL(import.meta.resolve("@sttot/openapi-thrift"))
const manifest = JSON.parse(
  await readFile(new URL("../package.json", entry), "utf8"),
)
assert.equal(manifest.version, process.argv[2])
assert.equal(manifest.bin, undefined)
assert.equal(manifest.scripts?.postinstall, undefined)
assert.throws(() => convertOpenApiToThrift("{}"), /initializeOpenApiThrift/)
await initializeOpenApiThrift({
  wasm: await readFile(
    new URL(import.meta.resolve("@sttot/openapi-thrift/openapi-thrift.wasm")),
  ),
})
const source = `openapi: 3.0.3
paths:
  /health:
    get:
      operationId: health
      tags: [health]
      responses:
        204:
          description: healthy
components:
  schemas: {}
`
assert.equal(assertOpenApiRenderDocument(source).errorCount, 0)
const result = convertOpenApiToThrift(source, {
  namespace: "release.smoke",
  serviceName: "SmokeService",
})
assert.match(result.thrift, /namespace go release\.smoke/)
assert.match(result.thrift, /service SmokeService/)
assert.throws(() => convertOpenApiToThrift("openapi: ["))
const unsupported = parseOpenApiDocument(source)
unsupported.components.schemas.Unsupported = {
  oneOf: [{ type: "string" }, { type: "integer" }],
}
assert.throws(
  () => assertOpenApiRenderDocument(unsupported),
  (error) =>
    error instanceof OpenApiRenderValidationError &&
    error.issues.some((issue) => issue.code === "hz.schema.one_of"),
)
assert.equal(
  convertOpenApiToThrift(source, {
    namespace: "release.smoke",
    serviceName: "SmokeService",
  }).thrift,
  result.thrift,
)
console.log("Installed npm consumer passed")
