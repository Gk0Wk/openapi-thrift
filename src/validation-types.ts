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
export interface ThriftSourceFile {
  path: string
  content: string
}
