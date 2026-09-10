import "./wasm_exec.js"
import type {
  OpenApiDocument,
  ProjectionOptions,
  ProjectionResult,
  ThriftDocument,
} from "./model.js"
import type {
  OpenApiRenderValidationIssue,
  OpenApiRenderValidationOptions,
  OpenApiRenderValidationResult,
  ThriftSourceFile,
} from "./validation-types.js"

export type {
  OpenApiDocument,
  ProjectionOptions,
  ProjectionResult,
  ThriftDocument,
  ThriftField,
  ThriftServiceMethod,
  ThriftStruct,
} from "./model.js"
export type {
  OpenApiRenderIssueSeverity,
  OpenApiRenderProfile,
  OpenApiRenderValidationIssue,
  OpenApiRenderValidationOptions,
  OpenApiRenderValidationResult,
  ThriftSourceFile,
} from "./validation-types.js"

interface GoRuntime {
  importObject: WebAssembly.Imports
  run(instance: WebAssembly.Instance): Promise<void>
}
interface CoreGlobal {
  Go: new () => GoRuntime
  __openapiThriftDispatch?: (request: string) => string
}
interface CoreError {
  name: string
  message: string
  pointer?: string
  issues?: OpenApiRenderValidationIssue[]
}
interface CoreResponse<T> {
  result?: T
  error?: CoreError
}
export interface InitializeOptions {
  /** Compiled module or bytes, useful for workers and runtimes without fetch. */
  wasm?: BufferSource | WebAssembly.Module
  /** Defaults to the asset shipped beside index.js. Serve as application/wasm. */
  wasmURL?: string | URL
}

let ready: Promise<void> | undefined
let dispatch: ((request: string) => string) | undefined

/** Initialize once before using the synchronous conversion API. No Node APIs. */
export function initializeOpenApiThrift(
  options: InitializeOptions = {},
): Promise<void> {
  if (!ready) {
    ready = initialize(options).catch((error: unknown) => {
      ready = undefined
      throw error
    })
  }
  return ready
}
async function initialize(options: InitializeOptions): Promise<void> {
  const runtimeGlobal = globalThis as typeof globalThis & CoreGlobal
  const runtime = new runtimeGlobal.Go()
  let source = options.wasm
  if (!source) {
    const response = await fetch(
      options.wasmURL ?? new URL("./openapi-thrift.wasm", import.meta.url),
    )
    if (!response.ok)
      throw new Error(
        `Cannot load OpenAPI Thrift WASM: HTTP ${response.status}`,
      )
    source = await response.arrayBuffer()
  }
  const module =
    source instanceof WebAssembly.Module
      ? source
      : await WebAssembly.compile(source)
  const instance = await WebAssembly.instantiate(module, runtime.importObject)
  // Go starts synchronously up to its first suspended goroutine. Keep the exit
  // promise observed, but do not await a runtime designed to serve more calls.
  const exit = runtime.run(instance)
  const initialized = runtimeGlobal.__openapiThriftDispatch
  if (!initialized) throw new Error("OpenAPI Thrift WASM did not initialize")
  dispatch = initialized
  void exit.then(
    () => {
      if (dispatch === initialized) dispatch = undefined
    },
    () => {
      if (dispatch === initialized) dispatch = undefined
    },
  )
}
function call<T>(action: string, fields: Record<string, unknown>): T {
  if (!dispatch)
    throw new Error(
      "Call and await initializeOpenApiThrift() before using the Go core",
    )
  const response = JSON.parse(
    dispatch(JSON.stringify({ action, ...fields })),
  ) as CoreResponse<T>
  if (response.error) {
    const error = response.error
    if (error.name === "OpenApiProjectionError")
      throw new OpenApiProjectionError(error.message, error.pointer)
    if (error.name === "OpenApiRenderValidationError")
      throw new OpenApiRenderValidationError(error.issues ?? [])
    throw new Error(error.message)
  }
  return response.result as T
}
export class OpenApiProjectionError extends Error {
  readonly pointer?: string
  constructor(message: string, pointer?: string) {
    super(pointer ? `${message} (${pointer})` : message)
    this.name = "OpenApiProjectionError"
    this.pointer = pointer
  }
}
export class OpenApiRenderValidationError extends Error {
  readonly issues: OpenApiRenderValidationIssue[]
  constructor(issues: OpenApiRenderValidationIssue[]) {
    super(formatOpenApiRenderValidationIssues(issues))
    this.name = "OpenApiRenderValidationError"
    this.issues = issues
  }
}
export function convertOpenApiToThrift(
  input: string | OpenApiDocument,
  options: ProjectionOptions = {},
): ProjectionResult {
  return call("convertOpenApiToThrift", { input, options })
}
export function parseOpenApiDocument(
  input: string | OpenApiDocument,
): OpenApiDocument {
  return call("parseOpenApiDocument", { input })
}
export function projectDocument(
  input: OpenApiDocument,
  options: ProjectionOptions = {},
): ThriftDocument {
  return call("projectDocument", { input, options })
}
export function renderThriftDocument(input: ThriftDocument): string {
  return call("renderThriftDocument", { input })
}
export function validateOpenApiRenderDocument(
  input: string | OpenApiDocument,
  options: OpenApiRenderValidationOptions = {},
): OpenApiRenderValidationResult {
  return call("validateOpenApiRenderDocument", { input, options })
}
export function assertOpenApiRenderDocument(
  input: string | OpenApiDocument,
  options: OpenApiRenderValidationOptions = {},
): OpenApiRenderValidationResult {
  return call("assertOpenApiRenderDocument", { input, options })
}
export function formatOpenApiRenderValidationIssues(
  issues: OpenApiRenderValidationIssue[],
): string {
  return call("formatOpenApiRenderValidationIssues", { issues })
}
export function buildRouteKey(method: string, path: string): string {
  return call("buildRouteKey", { method, path })
}
export function normalizeRoutePath(path: string): string {
  return call("normalizeRoutePath", { path })
}
export function extractRouteMethodNameMapFromThriftSources(
  files: ThriftSourceFile[],
): Record<string, string> {
  return call("extractRouteMethodNameMapFromThriftSources", { files })
}
