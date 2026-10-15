import assert from "node:assert/strict"
import fs from "node:fs"
import test from "node:test"

import {
  compileRawJsonContract,
  convertOpenApiToThrift,
  initializeOpenApiThrift,
} from "../dist/index.js"

await initializeOpenApiThrift({
  wasm: fs.readFileSync(
    new URL("../dist/openapi-thrift.wasm", import.meta.url),
  ),
})

test("WASM compiles strict raw JSON without changing the source document", () => {
  const document = JSON.parse(
    fs.readFileSync(
      new URL("./fixtures/raw-json-envelope.openapi.json", import.meta.url),
      "utf8",
    ),
  )
  const before = JSON.stringify(document)
  const contract = compileRawJsonContract(document)
  assert.equal(contract.schemaVersion, 1)
  assert.equal(contract.operations.length, 1)
  assert.equal(contract.operations[0].requestBodyRequired, true)
  assert.deepEqual(compileRawJsonContract(document), contract)
  assert.equal(JSON.stringify(document), before)
  const output = convertOpenApiToThrift(document)
  assert.match(output.thrift, /typedef binary DraftBodyRequest/)
  assert.match(output.thrift, /typedef binary DraftBodyResponse/)
  document.components.schemas.DraftBody["x-ispark-raw-json"] = false
  assert.throws(() => compileRawJsonContract(document), /raw JSON/)
})
