import type {
  OpenApiDocument,
  OpenApiExample,
  OpenApiMediaType,
  OpenApiOperation,
  OpenApiParameter,
  OpenApiParameterOrReference,
  OpenApiPathItem,
  OpenApiReference,
  OpenApiRequestBody,
  OpenApiResponse,
  OpenApiSchema,
  OpenApiSchemaObject,
  OpenApiSecurityRequirement,
  OpenApiSecurityScheme,
} from "./model.js"
import { convertOpenApiToThrift, OpenApiProjectionError } from "./projector.js"

const SUPPORTED_HTTP_METHODS = [
  "get",
  "post",
  "put",
  "delete",
  "patch",
  "head",
  "options",
] as const
const NO_REQUEST_BODY_METHODS = new Set(["get", "delete", "head", "options"])
const SUPPORTED_STRING_FORMATS = new Set([
  "base64",
  "binary",
  "date",
  "date-time",
  "email",
  "e164",
  "hexcolor",
  "hostname",
  "ipv4",
  "ipv6",
  "json",
  "jwt",
  "uri",
  "url",
  "ulid",
  "uuid",
  "uuid3",
  "uuid4",
  "uuid5",
])
const SUPPORTED_INTEGER_FORMATS = new Set(["int32", "int64"])
const SUPPORTED_NUMBER_FORMATS = new Set(["float", "double"])

type SupportedHttpMethod = (typeof SUPPORTED_HTTP_METHODS)[number]

export type OpenApiRenderProfile = "apifox-hz-thrift"
export type OpenApiRenderIssueSeverity = "error" | "warning"

export interface OpenApiRenderValidationIssue {
  severity: OpenApiRenderIssueSeverity
  code: string
  pointer: string
  message: string
}

export interface OpenApiRenderValidationOptions {
  profile?: OpenApiRenderProfile
  validateProjection?: boolean
}

export interface OpenApiRenderValidationResult {
  profile: OpenApiRenderProfile
  issues: OpenApiRenderValidationIssue[]
  errorCount: number
  warningCount: number
  pathCount: number
  operationCount: number
  schemaCount: number
}

interface ProfileContext {
  document: OpenApiDocument
  issues: OpenApiRenderValidationIssue[]
  operationIds: Set<string>
  componentSchemas: Record<string, OpenApiSchema>
  componentRequestBodies: Record<string, OpenApiRequestBody>
  componentResponses: Record<string, OpenApiResponse>
  componentParameters: Record<string, OpenApiParameter>
  securitySchemes: Record<string, OpenApiSecurityScheme>
}

export class OpenApiRenderValidationError extends Error {
  readonly issues: OpenApiRenderValidationIssue[]

  constructor(issues: OpenApiRenderValidationIssue[]) {
    super(formatOpenApiRenderValidationIssues(issues))
    this.name = "OpenApiRenderValidationError"
    this.issues = issues
  }
}

export function validateOpenApiRenderDocument(
  input: string | OpenApiDocument,
  options: OpenApiRenderValidationOptions = {},
): OpenApiRenderValidationResult {
  const profile = options.profile ?? "apifox-hz-thrift"
  const document = parseProfileInput(input)
  const context: ProfileContext = {
    document,
    issues: [],
    operationIds: new Set<string>(),
    componentSchemas: document.components?.schemas ?? {},
    componentRequestBodies: document.components?.requestBodies ?? {},
    componentResponses: document.components?.responses ?? {},
    componentParameters: document.components?.parameters ?? {},
    securitySchemes: document.components?.securitySchemes ?? {},
  }

  validateRoot(context)
  validateSecuritySchemes(context)
  validateSecurityRequirements(context, document.security, "#/security")
  validatePaths(context)
  validateComponents(context)
  scanApifoxExtensions(context, document, "#")

  if (options.validateProjection !== false) {
    validateThriftProjection(context)
  }

  const errorCount = context.issues.filter(
    (issue) => issue.severity === "error",
  ).length
  const warningCount = context.issues.filter(
    (issue) => issue.severity === "warning",
  ).length
  const paths = document.paths ?? {}
  return {
    profile,
    issues: context.issues,
    errorCount,
    warningCount,
    pathCount: Object.keys(paths).length,
    operationCount: collectOperationCount(paths),
    schemaCount: Object.keys(context.componentSchemas).length,
  }
}

export function assertOpenApiRenderDocument(
  input: string | OpenApiDocument,
  options: OpenApiRenderValidationOptions = {},
): OpenApiRenderValidationResult {
  const result = validateOpenApiRenderDocument(input, options)
  if (result.errorCount > 0) {
    throw new OpenApiRenderValidationError(result.issues)
  }
  return result
}

export function formatOpenApiRenderValidationIssues(
  issues: OpenApiRenderValidationIssue[],
): string {
  return issues
    .map(
      (issue) =>
        `${issue.severity.toUpperCase()} ${issue.code} ${issue.pointer}: ${issue.message}`,
    )
    .join("\n")
}

function parseProfileInput(input: string | OpenApiDocument): OpenApiDocument {
  if (typeof input !== "string") {
    return input
  }
  const parsed = JSON.parse(input) as unknown
  if (!isObject(parsed)) {
    throw new OpenApiRenderValidationError([
      {
        severity: "error",
        code: "openapi.root.type",
        pointer: "#",
        message: "OpenAPI 输入必须是 JSON object",
      },
    ])
  }
  return parsed as unknown as OpenApiDocument
}

function validateRoot(context: ProfileContext): void {
  const { document } = context
  if (
    typeof document.openapi !== "string" ||
    (!document.openapi.startsWith("3.0") && !document.openapi.startsWith("3.1"))
  ) {
    addError(
      context,
      "openapi.version",
      "#/openapi",
      "OpenAPI Render 当前只接受 OpenAPI 3.0/3.1 文档",
    )
  }
  if (!isObject(document.paths) || Object.keys(document.paths).length === 0) {
    addError(context, "openapi.paths.missing", "#/paths", "paths 不能为空")
  }
  if (!isObject(document.components?.schemas)) {
    addWarning(
      context,
      "openapi.components.schemas_missing",
      "#/components/schemas",
      "建议使用 components.schemas 承载可复用数据模型，便于 APIFox 和 Thrift 共用引用",
    )
  }
}

function validateSecuritySchemes(context: ProfileContext): void {
  for (const [name, scheme] of Object.entries(context.securitySchemes)) {
    const pointer = `#/components/securitySchemes/${escapePointer(name)}`
    if (!isObject(scheme)) {
      addError(
        context,
        "auth.scheme.type",
        pointer,
        "securityScheme 必须是 object",
      )
      continue
    }
    switch (scheme.type) {
      case "apiKey":
        if (typeof scheme.name !== "string" || !scheme.name.trim()) {
          addError(
            context,
            "auth.apikey.name",
            `${pointer}/name`,
            "apiKey securityScheme 必须声明 name",
          )
        }
        if (!["query", "header", "cookie"].includes(String(scheme.in))) {
          addError(
            context,
            "auth.apikey.in",
            `${pointer}/in`,
            "apiKey securityScheme.in 只能是 query/header/cookie",
          )
        }
        break
      case "http":
        if (typeof scheme.scheme !== "string" || !scheme.scheme.trim()) {
          addError(
            context,
            "auth.http.scheme",
            `${pointer}/scheme`,
            "http securityScheme 必须声明 scheme，例如 bearer",
          )
        }
        break
      case "oauth2":
        if (!isObject(scheme.flows)) {
          addError(
            context,
            "auth.oauth2.flows",
            `${pointer}/flows`,
            "oauth2 securityScheme 必须声明 flows",
          )
        }
        break
      case "openIdConnect":
        if (
          typeof scheme.openIdConnectUrl !== "string" ||
          !scheme.openIdConnectUrl.trim()
        ) {
          addError(
            context,
            "auth.openid.url",
            `${pointer}/openIdConnectUrl`,
            "openIdConnect securityScheme 必须声明 openIdConnectUrl",
          )
        }
        break
      default:
        addError(
          context,
          "auth.scheme.unsupported",
          `${pointer}/type`,
          "securityScheme.type 必须是 apiKey/http/oauth2/openIdConnect",
        )
        break
    }
  }
}

function validateSecurityRequirements(
  context: ProfileContext,
  requirements: OpenApiSecurityRequirement[] | undefined,
  pointer: string,
): void {
  if (requirements === undefined) {
    return
  }
  if (!Array.isArray(requirements)) {
    addError(
      context,
      "auth.requirement.type",
      pointer,
      "security 必须是 requirement object 数组；公开接口请使用 []",
    )
    return
  }
  for (const [index, requirement] of requirements.entries()) {
    const itemPointer = `${pointer}/${index}`
    if (!isObject(requirement)) {
      addError(
        context,
        "auth.requirement.item_type",
        itemPointer,
        "security requirement 必须是 object",
      )
      continue
    }
    for (const [schemeName, scopes] of Object.entries(requirement)) {
      const schemePointer = `${itemPointer}/${escapePointer(schemeName)}`
      if (!context.securitySchemes[schemeName]) {
        addError(
          context,
          "auth.requirement.unresolved",
          schemePointer,
          `security requirement 引用了不存在的 securityScheme: ${schemeName}`,
        )
      }
      if (!Array.isArray(scopes)) {
        addError(
          context,
          "auth.requirement.scopes",
          schemePointer,
          "security requirement 的 scopes 必须是 string[]",
        )
      }
    }
  }
}

function validatePaths(context: ProfileContext): void {
  const paths = context.document.paths ?? {}
  for (const [path, pathItem] of Object.entries(paths)) {
    const pointer = `#/paths/${escapePointer(path)}`
    if (!isObject(pathItem)) {
      addError(context, "path_item.type", pointer, "path item 必须是 object")
      continue
    }
    const pathItemObject = pathItem as OpenApiPathItem
    validateParameters(
      context,
      pathItemObject.parameters ?? [],
      `${pointer}/parameters`,
      "path",
    )
    for (const method of SUPPORTED_HTTP_METHODS) {
      const operation = pathItemObject[method]
      if (!operation) {
        continue
      }
      validateOperation(
        context,
        path,
        method,
        operation,
        `${pointer}/${method}`,
        pathItemObject,
      )
    }
  }
}

function validateOperation(
  context: ProfileContext,
  path: string,
  method: SupportedHttpMethod,
  operation: OpenApiOperation,
  pointer: string,
  pathItem: OpenApiPathItem,
): void {
  if (!isObject(operation)) {
    addError(context, "operation.type", pointer, "operation 必须是 object")
    return
  }
  const operationObject = operation as OpenApiOperation
  const operationLabel = `${method.toUpperCase()} ${path}`
  if (
    typeof operationObject.operationId !== "string" ||
    !operationObject.operationId
  ) {
    addError(
      context,
      "operation.operation_id.missing",
      `${pointer}/operationId`,
      `${operationLabel} 必须声明稳定 operationId`,
    )
  } else if (context.operationIds.has(operationObject.operationId)) {
    addError(
      context,
      "operation.operation_id.duplicate",
      `${pointer}/operationId`,
      `operationId 重复: ${operationObject.operationId}`,
    )
  } else {
    context.operationIds.add(operationObject.operationId)
  }

  if (
    !Array.isArray(operationObject.tags) ||
    operationObject.tags.length === 0
  ) {
    addWarning(
      context,
      "operation.tags.missing",
      `${pointer}/tags`,
      `${operationLabel} 建议声明 tags，APIFox 会用它辅助目录和过滤`,
    )
  }

  validateSecurityRequirements(
    context,
    operationObject.security,
    `${pointer}/security`,
  )
  validateParameters(
    context,
    [...(pathItem.parameters ?? []), ...(operationObject.parameters ?? [])],
    `${pointer}/parameters`,
    "operation",
  )
  validateRequestBody(
    context,
    method,
    operationObject.requestBody,
    `${pointer}/requestBody`,
  )
  validateResponses(context, operationObject.responses, `${pointer}/responses`)
}

function validateParameters(
  context: ProfileContext,
  parameters: OpenApiParameterOrReference[],
  pointer: string,
  owner: "operation" | "path",
): void {
  for (const [index, parameterOrReference] of parameters.entries()) {
    const itemPointer = `${pointer}/${index}`
    if (isReference(parameterOrReference)) {
      const parameter = resolveParameterReference(
        context,
        parameterOrReference,
        itemPointer,
      )
      if (parameter) {
        addError(
          context,
          "hz.parameter_ref.unsupported",
          itemPointer,
          "Hz/Thrift 生成不支持 parameter $ref；中央契约应在进入投影前写成内联 parameter",
        )
        validateParameter(context, parameter, itemPointer, owner)
      }
      continue
    }
    validateParameter(context, parameterOrReference, itemPointer, owner)
  }
}

function validateParameter(
  context: ProfileContext,
  parameter: OpenApiParameter,
  pointer: string,
  owner: "operation" | "path",
): void {
  if (!isObject(parameter)) {
    addError(context, "parameter.type", pointer, "parameter 必须是 object")
    return
  }
  if (!["path", "query", "header", "cookie"].includes(parameter.in)) {
    addError(
      context,
      "parameter.location",
      `${pointer}/in`,
      "parameter.in 必须是 path/query/header/cookie",
    )
  }
  if (typeof parameter.name !== "string" || !parameter.name.trim()) {
    addError(
      context,
      "parameter.name",
      `${pointer}/name`,
      "parameter 必须声明非空 name",
    )
  }
  if (parameter.in === "path" && parameter.required !== true) {
    addError(
      context,
      "parameter.path.required",
      `${pointer}/required`,
      "path parameter 必须 required=true",
    )
  }
  if (parameter.content && Object.keys(parameter.content).length > 0) {
    addError(
      context,
      "hz.parameter_content.unsupported",
      `${pointer}/content`,
      "Hz/Thrift profile 不支持 parameter.content，请改用 schema 参数",
    )
  }
  if (!parameter.schema) {
    addError(
      context,
      "parameter.schema.missing",
      `${pointer}/schema`,
      "parameter 必须声明 schema",
    )
    return
  }
  validateSchema(context, parameter.schema, `${pointer}/schema`)
  validateParameterSerialization(context, parameter, `${pointer}/schema`)
  validateParameterExamples(context, parameter, pointer, owner)
}

function validateParameterSerialization(
  context: ProfileContext,
  parameter: OpenApiParameter,
  pointer: string,
): void {
  if (parameter.in !== "query" || !parameter.schema) {
    return
  }
  const resolved = resolveSchema(context, parameter.schema, pointer)
  if (!resolved) {
    return
  }
  const style = parameter.style ?? "form"
  const explode = parameter.explode ?? style === "form"
  if (style === "deepObject") {
    addError(
      context,
      "hz.query.deep_object",
      pointer,
      "不支持 deepObject query 参数；请拆成简单字段",
    )
  }
  if (style === "spaceDelimited" || style === "pipeDelimited") {
    addError(
      context,
      "hz.query.delimited",
      pointer,
      `不支持 ${style} query 参数；数组只能使用 form + explode=true`,
    )
  }
  if (looksLikeObjectSchema(resolved)) {
    addError(
      context,
      "hz.query.object",
      pointer,
      "不支持 object query 参数自动投影；请拆成简单字段",
    )
  }
  if (resolved.type === "array" && (style !== "form" || !explode)) {
    addError(
      context,
      "hz.query.array_serialization",
      pointer,
      "query array 只能使用 form + explode=true（重复 key）语义",
    )
  }
}

function validateParameterExamples(
  context: ProfileContext,
  parameter: OpenApiParameter,
  pointer: string,
  owner: "operation" | "path",
): void {
  const record = parameter as unknown as Record<string, unknown>
  const hasExample =
    record.example !== undefined ||
    (isObject(record.examples) && Object.keys(record.examples).length > 0) ||
    (parameter.schema !== undefined &&
      schemaHasExample(context, parameter.schema))
  if (!hasExample && owner === "operation" && parameter.in !== "header") {
    addWarning(
      context,
      "apifox.parameter.example_missing",
      pointer,
      "APIFox 参数建议提供 example/examples；query/path-only 接口尤其需要",
    )
  }
  if (record.example !== undefined && parameter.schema) {
    validateExampleValue(
      context,
      parameter.schema,
      record.example,
      `${pointer}/example`,
    )
  }
}

function validateRequestBody(
  context: ProfileContext,
  method: SupportedHttpMethod,
  requestBodyLike: OpenApiRequestBody | OpenApiReference | undefined,
  pointer: string,
): void {
  if (!requestBodyLike) {
    return
  }
  if (NO_REQUEST_BODY_METHODS.has(method)) {
    addError(
      context,
      "hz.request_body.method",
      pointer,
      `${method.toUpperCase()} 不允许声明 requestBody`,
    )
  }
  const requestBody = resolveRequestBody(context, requestBodyLike, pointer)
  if (!requestBody) {
    return
  }
  const content = requestBody.content ?? {}
  validateMediaContent(context, content, pointer, "request")
  if (!hasMediaExample(context, content)) {
    addWarning(
      context,
      "apifox.request_body.example_missing",
      pointer,
      "APIFox requestBody 建议提供 media example/examples 或 schema 字段 example",
    )
  }
}

function validateResponses(
  context: ProfileContext,
  responses: Record<string, OpenApiResponse> | undefined,
  pointer: string,
): void {
  if (!responses || !isObject(responses)) {
    addError(
      context,
      "operation.responses.missing",
      pointer,
      "operation 必须声明 responses",
    )
    return
  }
  const successCodes = Object.keys(responses)
    .filter((statusCode) => /^2\d\d$/.test(statusCode))
    .sort()
  if (successCodes.length === 0) {
    addError(
      context,
      "hz.response.success_missing",
      pointer,
      "Hz/Thrift profile 要求每个 operation 有且只有一个 2xx success response",
    )
  }
  if (successCodes.length > 1) {
    addError(
      context,
      "hz.response.multiple_success",
      pointer,
      "Hz/Thrift profile 不支持多个 2xx success response",
    )
  }
  for (const [statusCode, responseLike] of Object.entries(responses)) {
    const responsePointer = `${pointer}/${statusCode}`
    const response = resolveResponse(context, responseLike, responsePointer)
    if (!response) {
      continue
    }
    validateMediaContent(
      context,
      response.content ?? {},
      responsePointer,
      /^2\d\d$/.test(statusCode) ? "success_response" : "error_response",
    )
  }
}

function validateMediaContent(
  context: ProfileContext,
  content: Record<string, OpenApiMediaType>,
  pointer: string,
  owner: "request" | "success_response" | "error_response",
): void {
  if (!content || Object.keys(content).length === 0) {
    return
  }
  const entries = Object.entries(content)
  if (entries.length > 1) {
    addError(
      context,
      "hz.media.multiple_content_types",
      `${pointer}/content`,
      "Hz/Thrift profile 不支持同一 request/response 的多个 content-type 主线",
    )
  }
  for (const [contentType, mediaType] of entries) {
    const mediaPointer = `${pointer}/content/${escapePointer(contentType)}`
    const normalized = contentType.trim().toLowerCase()
    if (owner === "request" && normalized === "json") {
      addError(
        context,
        "apifox.request_body.json_alias",
        mediaPointer,
        "APIFox 稳定导出必须使用 application/json，不要使用 json 简写 content-type",
      )
    }
    if (owner === "request" && !isSupportedRequestContentType(normalized)) {
      addError(
        context,
        "hz.request_body.content_type",
        mediaPointer,
        "请求体只支持 application/json、multipart/form-data、application/x-www-form-urlencoded",
      )
    }
    if (mediaType.schema) {
      validateSchema(context, mediaType.schema, `${mediaPointer}/schema`)
      if (
        owner === "success_response" &&
        !isJsonContentType(normalized) &&
        !isRawSuccessSchemaSupported(context, mediaType.schema, mediaPointer)
      ) {
        addError(
          context,
          "hz.response.raw_schema",
          `${mediaPointer}/schema`,
          "非 JSON success response 只支持 string/binary 标量或省略 schema",
        )
      }
    }
    validateMediaExamples(context, mediaType, mediaPointer)
  }
}

function validateMediaExamples(
  context: ProfileContext,
  mediaType: OpenApiMediaType,
  pointer: string,
): void {
  if (!mediaType.schema) {
    return
  }
  if (mediaType.example !== undefined) {
    validateExampleValue(
      context,
      mediaType.schema,
      mediaType.example,
      `${pointer}/example`,
    )
  }
  if (isObject(mediaType.examples)) {
    for (const [name, exampleOrReference] of Object.entries(
      mediaType.examples,
    )) {
      const examplePointer = `${pointer}/examples/${escapePointer(name)}`
      if (isReference(exampleOrReference)) {
        validateLocalReference(context, exampleOrReference.$ref, examplePointer)
        continue
      }
      if (!isObject(exampleOrReference)) {
        addError(
          context,
          "apifox.example.type",
          examplePointer,
          "media examples 的每个条目必须是 Example Object 或 $ref",
        )
        continue
      }
      const example = exampleOrReference as OpenApiExample
      if (example.value !== undefined) {
        validateExampleValue(
          context,
          mediaType.schema,
          example.value,
          `${examplePointer}/value`,
        )
      } else if (typeof example.externalValue !== "string") {
        addWarning(
          context,
          "apifox.example.value_missing",
          examplePointer,
          "media example 建议提供 value；externalValue 只适合真实外链样例",
        )
      }
    }
  }
}

function validateSchema(
  context: ProfileContext,
  schemaOrReference: OpenApiSchema,
  pointer: string,
): void {
  const schema = resolveSchema(context, schemaOrReference, pointer)
  if (!schema) {
    return
  }
  validateSchemaObject(context, schema, pointer)
}

function validateSchemaObject(
  context: ProfileContext,
  schema: OpenApiSchemaObject,
  pointer: string,
): void {
  validateSchemaExamples(context, schema, pointer)
  validateSchemaMocks(context, schema, pointer)
  validateUnsupportedSchemaKeywords(context, schema, pointer)

  if (schema.additionalProperties === true) {
    addError(
      context,
      "hz.schema.additional_properties_true",
      `${pointer}/additionalProperties`,
      "Hz/Thrift profile 不支持 additionalProperties: true",
    )
  }
  if (schema.additionalProperties === false) {
    addError(
      context,
      "hz.schema.additional_properties_false",
      `${pointer}/additionalProperties`,
      "additionalProperties:false 需要 binder/decoder 级语义，不能投影到字段 validator",
    )
  }
  if (
    typeof schema.additionalProperties === "object" &&
    schema.properties &&
    Object.keys(schema.properties).length > 0
  ) {
    addError(
      context,
      "hz.schema.properties_plus_map",
      pointer,
      "不支持 properties 与 additionalProperties schema 混用的 object",
    )
  }
  if (
    schema.enum?.some((value) => typeof value === "string" && /\s/.test(value))
  ) {
    addError(
      context,
      "hz.schema.enum_whitespace",
      `${pointer}/enum`,
      "string enum 值不能包含空白字符，Hertz validator oneof 无法稳定表达",
    )
  }
  if (schema.properties) {
    for (const [propertyName, propertySchema] of Object.entries(
      schema.properties,
    )) {
      validateSchema(
        context,
        propertySchema,
        `${pointer}/properties/${escapePointer(propertyName)}`,
      )
    }
  }
  if (schema.items) {
    validateSchema(context, schema.items, `${pointer}/items`)
  }
  if (typeof schema.additionalProperties === "object") {
    validateSchema(
      context,
      schema.additionalProperties,
      `${pointer}/additionalProperties`,
    )
  }
  for (const [index, branch] of (schema.anyOf ?? []).entries()) {
    validateSchema(context, branch, `${pointer}/anyOf/${index}`)
  }
  for (const [index, branch] of (schema.oneOf ?? []).entries()) {
    validateSchema(context, branch, `${pointer}/oneOf/${index}`)
  }
  for (const [index, branch] of (schema.allOf ?? []).entries()) {
    validateSchema(context, branch, `${pointer}/allOf/${index}`)
  }
}

function validateSchemaExamples(
  context: ProfileContext,
  schema: OpenApiSchemaObject,
  pointer: string,
): void {
  if (schema.example !== undefined) {
    validateExampleValue(context, schema, schema.example, `${pointer}/example`)
  }
  if (schema.examples !== undefined) {
    if (!Array.isArray(schema.examples)) {
      addError(
        context,
        "apifox.schema.examples_type",
        `${pointer}/examples`,
        "schema.examples 必须是数组；media.examples 才是命名 Example Object map",
      )
      return
    }
    for (const [index, example] of schema.examples.entries()) {
      validateExampleValue(
        context,
        schema,
        example,
        `${pointer}/examples/${index}`,
      )
    }
  }
}

function validateSchemaMocks(
  context: ProfileContext,
  schema: OpenApiSchemaObject,
  pointer: string,
): void {
  const record = schema as Record<string, unknown>
  if (record["x-apifox-mock"] !== undefined) {
    if (
      typeof record["x-apifox-mock"] !== "string" ||
      !record["x-apifox-mock"].trim()
    ) {
      addError(
        context,
        "apifox.mock.expression",
        `${pointer}/x-apifox-mock`,
        "x-apifox-mock 必须是非空字符串表达式，例如 @id、@date、@pick(...)",
      )
    }
  }
  if (record.mockScript !== undefined) {
    if (typeof record.mockScript !== "string" || !record.mockScript.trim()) {
      addError(
        context,
        "apifox.mock_script.type",
        `${pointer}/mockScript`,
        "mockScript 必须是非空字符串；只在需要条件分支 mock 时使用",
      )
    } else {
      addWarning(
        context,
        "apifox.mock_script.review",
        `${pointer}/mockScript`,
        "mockScript 应保持确定性；优先使用字段级 x-apifox-mock",
      )
    }
  }
}

function validateUnsupportedSchemaKeywords(
  context: ProfileContext,
  schema: OpenApiSchemaObject,
  pointer: string,
): void {
  const canonicalOverride = schema["x-ispark-allow-unsupported-validation"]
  const legacyOverride = schema["x-dramawork-allow-unsupported-validation"]
  const canonicalValidators = schema["x-ispark-validate"]
  const legacyValidators = schema["x-dramawork-validate"]
  if (legacyOverride !== undefined) {
    addWarning(
      context,
      "hz.schema.legacy_manual_override",
      `${pointer}/x-dramawork-allow-unsupported-validation`,
      "x-dramawork-allow-unsupported-validation 已弃用，请迁移到 x-ispark-allow-unsupported-validation",
    )
  }
  if (legacyValidators !== undefined) {
    addWarning(
      context,
      "hz.schema.legacy_manual_validator",
      `${pointer}/x-dramawork-validate`,
      "x-dramawork-validate 已弃用，请迁移到 x-ispark-validate",
    )
  }
  if (
    canonicalOverride !== undefined &&
    legacyOverride !== undefined &&
    canonicalOverride !== legacyOverride
  ) {
    addError(
      context,
      "hz.schema.manual_override_conflict",
      `${pointer}/x-ispark-allow-unsupported-validation`,
      "x-ispark-allow-unsupported-validation 与已弃用字段必须保持一致",
    )
  }
  if (
    canonicalValidators !== undefined &&
    legacyValidators !== undefined &&
    JSON.stringify(canonicalValidators) !== JSON.stringify(legacyValidators)
  ) {
    addError(
      context,
      "hz.schema.manual_validator_conflict",
      `${pointer}/x-ispark-validate`,
      "x-ispark-validate 与已弃用字段必须保持一致",
    )
  }
  const manualOverride = Boolean(canonicalOverride ?? legacyOverride ?? false)
  const manualValidators = canonicalValidators ?? legacyValidators
  if (manualOverride && !hasManualValidator(manualValidators)) {
    addError(
      context,
      "hz.schema.manual_validator_missing",
      `${pointer}/x-ispark-validate`,
      "x-ispark-allow-unsupported-validation 需要同时提供 x-ispark-validate",
    )
  }

  if (schema.format && !isSupportedSchemaFormat(schema) && !manualOverride) {
    addError(
      context,
      "hz.schema.format",
      `${pointer}/format`,
      `不支持 format=${schema.format} 自动投影，请收紧 schema 或显式 x-ispark-validate`,
    )
  }
  if (schema.oneOf?.length) {
    addError(context, "hz.schema.one_of", `${pointer}/oneOf`, "不支持 oneOf")
  }
  if (schema.anyOf?.length && !isNullableUnion(schema)) {
    addError(
      context,
      "hz.schema.any_of",
      `${pointer}/anyOf`,
      "只支持 nullable anyOf [T, null]",
    )
  }
  if (
    schema.allOf?.length &&
    !isComposableObjectAllOf(context, schema, pointer)
  ) {
    addError(
      context,
      "hz.schema.all_of",
      `${pointer}/allOf`,
      "只支持可组合 object schema 的 allOf",
    )
  }
  if (schema.pattern && !manualOverride) {
    addError(
      context,
      "hz.schema.pattern",
      `${pointer}/pattern`,
      "pattern 不能自动投影；需要显式 x-ispark-validate",
    )
  }
  if (typeof schema.multipleOf === "number" && !manualOverride) {
    addError(
      context,
      "hz.schema.multiple_of",
      `${pointer}/multipleOf`,
      "multipleOf 不能自动投影；需要显式 x-ispark-validate",
    )
  }
  for (const keyword of [
    "exclusiveMinimum",
    "exclusiveMaximum",
    "uniqueItems",
    "minProperties",
    "maxProperties",
  ]) {
    if (
      (schema as Record<string, unknown>)[keyword] !== undefined &&
      !manualOverride
    ) {
      addError(
        context,
        "hz.schema.unsupported_validator",
        `${pointer}/${keyword}`,
        `${keyword} 不能自动投影；需要显式 x-ispark-validate`,
      )
    }
  }
}

function validateComponents(context: ProfileContext): void {
  for (const [schemaName, schema] of Object.entries(context.componentSchemas)) {
    validateSchema(
      context,
      schema,
      `#/components/schemas/${escapePointer(schemaName)}`,
    )
  }
  for (const [bodyName, body] of Object.entries(
    context.componentRequestBodies,
  )) {
    validateRequestBody(
      context,
      "post",
      body,
      `#/components/requestBodies/${escapePointer(bodyName)}`,
    )
  }
  for (const [responseName, response] of Object.entries(
    context.componentResponses,
  )) {
    validateMediaContent(
      context,
      response.content ?? {},
      `#/components/responses/${escapePointer(responseName)}`,
      "success_response",
    )
  }
  for (const [parameterName, parameter] of Object.entries(
    context.componentParameters,
  )) {
    validateParameter(
      context,
      parameter,
      `#/components/parameters/${escapePointer(parameterName)}`,
      "path",
    )
  }
}

function validateThriftProjection(context: ProfileContext): void {
  try {
    convertOpenApiToThrift(context.document)
  } catch (error) {
    addError(
      context,
      "hz.thrift_projection",
      error instanceof OpenApiProjectionError && error.pointer
        ? error.pointer
        : "#",
      error instanceof Error
        ? `OpenAPI 无法稳定投影为 Thrift IDL: ${error.message}`
        : "OpenAPI 无法稳定投影为 Thrift IDL",
    )
  }
}

function validateExampleValue(
  context: ProfileContext,
  schemaOrReference: OpenApiSchema,
  value: unknown,
  pointer: string,
): void {
  const schema = resolveSchema(context, schemaOrReference, pointer)
  if (!schema || value === null) {
    return
  }
  if (isNullableUnion(schema)) {
    const nonNull = schema.anyOf?.find((item) => !isNullSchema(item))
    if (nonNull) {
      validateExampleValue(context, nonNull, value, pointer)
    }
    return
  }
  const expectedType = inferExampleType(schema)
  if (!expectedType) {
    return
  }
  const matched =
    (expectedType === "object" && isObject(value)) ||
    (expectedType === "array" && Array.isArray(value)) ||
    (expectedType === "string" && typeof value === "string") ||
    (expectedType === "integer" &&
      typeof value === "number" &&
      Number.isInteger(value)) ||
    (expectedType === "number" && typeof value === "number") ||
    (expectedType === "boolean" && typeof value === "boolean")
  if (!matched) {
    addError(
      context,
      "apifox.example.schema_mismatch",
      pointer,
      `example 类型与 schema 不匹配，期望 ${expectedType}`,
    )
  }
}

function hasMediaExample(
  context: ProfileContext,
  content: Record<string, OpenApiMediaType>,
): boolean {
  for (const mediaType of Object.values(content)) {
    if (
      mediaType.example !== undefined ||
      (mediaType.examples && Object.keys(mediaType.examples).length > 0) ||
      (mediaType.schema && schemaHasExample(context, mediaType.schema))
    ) {
      return true
    }
  }
  return false
}

function schemaHasExample(
  context: ProfileContext,
  schemaOrReference: OpenApiSchema,
  seenRefs = new Set<string>(),
): boolean {
  if (isReference(schemaOrReference)) {
    const ref = schemaOrReference.$ref
    if (seenRefs.has(ref)) {
      return false
    }
    const match = /^#\/components\/schemas\/(?<name>[^/]+)$/.exec(ref)
    if (!match?.groups?.name) {
      return false
    }
    const schema = context.componentSchemas[match.groups.name]
    if (!schema) {
      return false
    }
    seenRefs.add(ref)
    return schemaHasExample(context, schema, seenRefs)
  }
  if (
    schemaOrReference.example !== undefined ||
    (schemaOrReference.examples && schemaOrReference.examples.length > 0)
  ) {
    return true
  }
  if (
    Object.values(schemaOrReference.properties ?? {}).some((propertySchema) =>
      schemaHasExample(context, propertySchema, seenRefs),
    )
  ) {
    return true
  }
  return Boolean(
    schemaOrReference.items &&
      schemaHasExample(context, schemaOrReference.items, seenRefs),
  )
}

function resolveSchema(
  context: ProfileContext,
  schemaOrReference: OpenApiSchema,
  pointer: string,
): OpenApiSchemaObject | undefined {
  if (isReference(schemaOrReference)) {
    const ref = schemaOrReference.$ref
    const match = /^#\/components\/schemas\/(?<name>[^/]+)$/.exec(ref)
    if (!match?.groups?.name) {
      addError(
        context,
        "openapi.ref.schema_scope",
        pointer,
        "schema $ref 只允许指向 #/components/schemas/*",
      )
      return undefined
    }
    const schema = context.componentSchemas[match.groups.name]
    if (!schema) {
      addError(
        context,
        "openapi.ref.unresolved",
        pointer,
        `找不到 schema 引用 ${ref}`,
      )
      return undefined
    }
    return resolveSchema(context, schema, pointer)
  }
  return schemaOrReference
}

function resolveRequestBody(
  context: ProfileContext,
  requestBodyLike: OpenApiRequestBody | OpenApiReference,
  pointer: string,
): OpenApiRequestBody | undefined {
  if (!isReference(requestBodyLike)) {
    return requestBodyLike
  }
  const ref = requestBodyLike.$ref
  const match = /^#\/components\/requestBodies\/(?<name>[^/]+)$/.exec(ref)
  if (!match?.groups?.name) {
    addError(
      context,
      "openapi.ref.request_body_scope",
      pointer,
      "requestBody $ref 只允许指向 #/components/requestBodies/*",
    )
    return undefined
  }
  const requestBody = context.componentRequestBodies[match.groups.name]
  if (!requestBody) {
    addError(
      context,
      "openapi.ref.unresolved",
      pointer,
      `找不到 requestBody 引用 ${ref}`,
    )
    return undefined
  }
  return requestBody
}

function resolveResponse(
  context: ProfileContext,
  responseLike: OpenApiResponse | OpenApiReference,
  pointer: string,
): OpenApiResponse | undefined {
  if (!isReference(responseLike)) {
    return responseLike
  }
  const ref = responseLike.$ref
  const match = /^#\/components\/responses\/(?<name>[^/]+)$/.exec(ref)
  if (!match?.groups?.name) {
    addError(
      context,
      "openapi.ref.response_scope",
      pointer,
      "response $ref 只允许指向 #/components/responses/*",
    )
    return undefined
  }
  const response = context.componentResponses[match.groups.name]
  if (!response) {
    addError(
      context,
      "openapi.ref.unresolved",
      pointer,
      `找不到 response 引用 ${ref}`,
    )
    return undefined
  }
  return response
}

function resolveParameterReference(
  context: ProfileContext,
  parameterLike: OpenApiReference,
  pointer: string,
): OpenApiParameter | undefined {
  const ref = parameterLike.$ref
  const match = /^#\/components\/parameters\/(?<name>[^/]+)$/.exec(ref)
  if (!match?.groups?.name) {
    addError(
      context,
      "openapi.ref.parameter_scope",
      pointer,
      "parameter $ref 只允许指向 #/components/parameters/*",
    )
    return undefined
  }
  const parameter = context.componentParameters[match.groups.name]
  if (!parameter) {
    addError(
      context,
      "openapi.ref.unresolved",
      pointer,
      `找不到 parameter 引用 ${ref}`,
    )
    return undefined
  }
  return parameter
}

function validateLocalReference(
  context: ProfileContext,
  ref: string,
  pointer: string,
): void {
  if (!ref.startsWith("#/")) {
    addError(
      context,
      "openapi.ref.external",
      pointer,
      "只允许本地 OpenAPI $ref",
    )
    return
  }
  let value: unknown = context.document
  for (const part of ref.slice(2).split("/").map(unescapePointer)) {
    if (!isObject(value) || !(part in value)) {
      addError(
        context,
        "openapi.ref.unresolved",
        pointer,
        `找不到 OpenAPI 引用 ${ref}`,
      )
      return
    }
    value = value[part]
  }
}

function scanApifoxExtensions(
  context: ProfileContext,
  value: unknown,
  pointer: string,
): void {
  if (Array.isArray(value)) {
    for (const [index, item] of value.entries()) {
      scanApifoxExtensions(context, item, `${pointer}/${index}`)
    }
    return
  }
  if (!isObject(value)) {
    return
  }
  const mock = value["x-apifox-mock"]
  if (mock !== undefined && (typeof mock !== "string" || !mock.trim())) {
    addError(
      context,
      "apifox.mock.expression",
      `${pointer}/x-apifox-mock`,
      "x-apifox-mock 必须是非空字符串表达式",
    )
  }
  const script = value.mockScript
  if (script !== undefined && (typeof script !== "string" || !script.trim())) {
    addError(
      context,
      "apifox.mock_script.type",
      `${pointer}/mockScript`,
      "mockScript 必须是非空字符串",
    )
  }
  for (const [key, child] of Object.entries(value)) {
    scanApifoxExtensions(context, child, `${pointer}/${escapePointer(key)}`)
  }
}

function isComposableObjectAllOf(
  context: ProfileContext,
  schema: OpenApiSchemaObject,
  pointer: string,
): boolean {
  return (schema.allOf ?? []).every((branch, index) => {
    const resolved = resolveSchema(context, branch, `${pointer}/allOf/${index}`)
    return Boolean(resolved && looksLikeObjectSchema(resolved))
  })
}

function isRawSuccessSchemaSupported(
  context: ProfileContext,
  schemaOrReference: OpenApiSchema,
  pointer: string,
): boolean {
  const schema = resolveSchema(context, schemaOrReference, pointer)
  if (!schema) {
    return true
  }
  return !looksLikeObjectSchema(schema) && schema.type !== "array"
}

function isSupportedRequestContentType(contentType: string): boolean {
  return (
    isJsonContentType(contentType) ||
    contentType === "multipart/form-data" ||
    contentType === "application/x-www-form-urlencoded"
  )
}

function isJsonContentType(contentType: string): boolean {
  const normalized = contentType.trim().toLowerCase()
  return (
    normalized === "application/json" ||
    normalized.startsWith("application/json;")
  )
}

function isSupportedSchemaFormat(schema: OpenApiSchemaObject): boolean {
  if (!schema.format) {
    return true
  }
  switch (schema.type) {
    case "string":
      return SUPPORTED_STRING_FORMATS.has(schema.format)
    case "integer":
      return SUPPORTED_INTEGER_FORMATS.has(schema.format)
    case "number":
      return SUPPORTED_NUMBER_FORMATS.has(schema.format)
    default:
      return false
  }
}

function hasManualValidator(value: string | string[] | undefined): boolean {
  if (typeof value === "string") {
    return Boolean(value.trim())
  }
  if (Array.isArray(value)) {
    return value.some((item) => item.trim())
  }
  return false
}

function isNullableUnion(schema: OpenApiSchemaObject): boolean {
  if (!schema.anyOf || schema.anyOf.length !== 2) {
    return false
  }
  return schema.anyOf.some(isNullSchema)
}

function isNullSchema(schemaOrReference: OpenApiSchema): boolean {
  return !isReference(schemaOrReference) && schemaOrReference.type === "null"
}

function looksLikeObjectSchema(schema: OpenApiSchemaObject): boolean {
  return schema.type === "object" || Boolean(schema.properties)
}

function inferExampleType(
  schema: OpenApiSchemaObject,
):
  | "object"
  | "array"
  | "string"
  | "integer"
  | "number"
  | "boolean"
  | undefined {
  if (looksLikeObjectSchema(schema)) {
    return "object"
  }
  if (schema.type === "array") {
    return "array"
  }
  if (
    schema.type === "string" ||
    schema.type === "integer" ||
    schema.type === "number" ||
    schema.type === "boolean"
  ) {
    return schema.type
  }
  return undefined
}

function collectOperationCount(paths: Record<string, OpenApiPathItem>): number {
  let count = 0
  for (const pathItem of Object.values(paths)) {
    for (const method of SUPPORTED_HTTP_METHODS) {
      if (pathItem[method]) {
        count += 1
      }
    }
  }
  return count
}

function addError(
  context: ProfileContext,
  code: string,
  pointer: string,
  message: string,
): void {
  context.issues.push({ severity: "error", code, pointer, message })
}

function addWarning(
  context: ProfileContext,
  code: string,
  pointer: string,
  message: string,
): void {
  context.issues.push({ severity: "warning", code, pointer, message })
}

function isReference(value: unknown): value is OpenApiReference {
  return isObject(value) && typeof value.$ref === "string"
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
}

function escapePointer(value: string): string {
  return value.replaceAll("~", "~0").replaceAll("/", "~1")
}

function unescapePointer(value: string): string {
  return value.replaceAll("~1", "/").replaceAll("~0", "~")
}
