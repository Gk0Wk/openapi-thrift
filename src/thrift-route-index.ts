export interface ThriftSourceFile {
  path: string
  content: string
}

const ROUTE_METHOD_PATTERN =
  /[A-Za-z_][A-Za-z0-9_<>.]*\s+(?<methodName>[A-Za-z_][A-Za-z0-9_]*)\s*\([^)]*\)\s*\(\s*api\.(?<httpMethod>get|post|put|delete|patch|head|options|any)\s*=\s*"(?<path>[^"]+)"/g

export function extractRouteMethodNameMapFromThriftSources(
  files: ThriftSourceFile[],
): Record<string, string> {
  const routeMethodNames: Record<string, string> = {}

  for (const file of files) {
    const matches = file.content.matchAll(ROUTE_METHOD_PATTERN)
    for (const match of matches) {
      const methodName = match.groups?.methodName
      const httpMethod = match.groups?.httpMethod
      const path = match.groups?.path
      if (!methodName || !httpMethod || !path) {
        continue
      }
      routeMethodNames[buildRouteKey(httpMethod, path)] = methodName
    }
  }

  return routeMethodNames
}

export function buildRouteKey(httpMethod: string, path: string): string {
  return `${httpMethod.toUpperCase()} ${normalizeRoutePath(path)}`
}

export function normalizeRoutePath(path: string): string {
  return path.replaceAll(/\{([^}]+)\}/g, ":$1")
}
