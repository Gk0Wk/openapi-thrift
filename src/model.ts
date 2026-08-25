export type JsonValue =
  | null
  | boolean
  | number
  | string
  | JsonValue[]
  | { [key: string]: JsonValue }

export interface OpenApiDocument {
  openapi: string
  info?: {
    title?: string
    description?: string
  }
  paths?: Record<string, OpenApiPathItem>
  security?: OpenApiSecurityRequirement[]
  components?: {
    schemas?: Record<string, OpenApiSchema>
    requestBodies?: Record<string, OpenApiRequestBody>
    responses?: Record<string, OpenApiResponse>
    parameters?: Record<string, OpenApiParameter>
    securitySchemes?: Record<string, OpenApiSecurityScheme>
  }
}

export interface OpenApiPathItem {
  parameters?: OpenApiParameterOrReference[]
  get?: OpenApiOperation
  post?: OpenApiOperation
  put?: OpenApiOperation
  delete?: OpenApiOperation
  patch?: OpenApiOperation
  head?: OpenApiOperation
  options?: OpenApiOperation
}

export interface OpenApiOperation {
  operationId?: string
  summary?: string
  description?: string
  tags?: string[]
  parameters?: OpenApiParameterOrReference[]
  requestBody?: OpenApiRequestBody | OpenApiReference
  responses?: Record<string, OpenApiResponse>
  security?: OpenApiSecurityRequirement[]
}

export interface OpenApiParameter {
  name: string
  in: "path" | "query" | "header" | "cookie"
  required?: boolean
  description?: string
  schema?: OpenApiSchema
  style?: string
  explode?: boolean
  content?: Record<string, OpenApiMediaType>
}

export type OpenApiParameterOrReference = OpenApiParameter | OpenApiReference

export interface OpenApiRequestBody {
  required?: boolean
  content?: Record<string, OpenApiMediaType>
}

export interface OpenApiResponse {
  description?: string
  content?: Record<string, OpenApiMediaType>
}

export interface OpenApiMediaType {
  schema?: OpenApiSchema
  example?: JsonValue
  examples?: Record<string, OpenApiExample | OpenApiReference>
}

export interface OpenApiExample {
  summary?: string
  description?: string
  value?: JsonValue
  externalValue?: string
}

export interface OpenApiReference {
  $ref: string
}

export interface OpenApiSchemaObject {
  type?: string
  format?: string
  description?: string
  example?: JsonValue
  examples?: JsonValue[]
  nullable?: boolean
  default?: JsonValue
  minimum?: number
  maximum?: number
  exclusiveMinimum?: number | boolean
  exclusiveMaximum?: number | boolean
  minLength?: number
  maxLength?: number
  pattern?: string
  multipleOf?: number
  minItems?: number
  maxItems?: number
  uniqueItems?: boolean
  minProperties?: number
  maxProperties?: number
  properties?: Record<string, OpenApiSchema>
  required?: string[]
  items?: OpenApiSchema
  enum?: Array<string | number>
  additionalProperties?: boolean | OpenApiSchema
  oneOf?: OpenApiSchema[]
  anyOf?: OpenApiSchema[]
  allOf?: OpenApiSchema[]
  "x-ispark-validate"?: string | string[]
  "x-ispark-allow-unsupported-validation"?: boolean
  /** @deprecated Use x-ispark-validate. */
  "x-dramawork-validate"?: string | string[]
  /** @deprecated Use x-ispark-allow-unsupported-validation. */
  "x-dramawork-allow-unsupported-validation"?: boolean
  "x-apifox-mock"?: string
  mockScript?: string
}

export type OpenApiSchema = OpenApiSchemaObject | OpenApiReference

export type OpenApiSecurityRequirement = Record<string, string[]>

export interface OpenApiSecurityScheme {
  type?: string
  name?: string
  in?: string
  scheme?: string
  bearerFormat?: string
  flows?: Record<string, unknown>
  openIdConnectUrl?: string
}

export interface ProjectionOptions {
  namespace?: string
  serviceName?: string
  routeMethodNames?: Record<string, string>
}

export interface ProjectionResult {
  document: ThriftDocument
  thrift: string
}

export interface ThriftDocument {
  namespace: string
  serviceName: string
  definitions: ThriftStruct[]
  methods: ThriftServiceMethod[]
}

export interface ThriftStruct {
  name: string
  comment?: string[]
  fields: ThriftField[]
}

export interface ThriftField {
  id: number
  requiredness: "required" | "optional"
  type: string
  name: string
  annotations: string[]
  comment?: string[]
}

export interface ThriftServiceMethod {
  name: string
  requestType: string
  responseType: string
  httpMethod: string
  path: string
  comment?: string[]
}
