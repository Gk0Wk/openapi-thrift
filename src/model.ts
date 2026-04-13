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
  components?: {
    schemas?: Record<string, OpenApiSchema>
    requestBodies?: Record<string, OpenApiRequestBody>
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
  parameters?: OpenApiParameterOrReference[]
  requestBody?: OpenApiRequestBody | OpenApiReference
  responses?: Record<string, OpenApiResponse>
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
}

export interface OpenApiReference {
  $ref: string
}

export interface OpenApiSchemaObject {
  type?: string
  format?: string
  description?: string
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
  "x-dramawork-validate"?: string | string[]
  "x-dramawork-allow-unsupported-validation"?: boolean
}

export type OpenApiSchema = OpenApiSchemaObject | OpenApiReference

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
