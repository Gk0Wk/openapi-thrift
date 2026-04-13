import { readdir, readFile, writeFile } from "node:fs/promises"
import path from "node:path"
import process from "node:process"

import {
  convertOpenApiToThrift,
  extractRouteMethodNameMapFromThriftSources,
  OpenApiProjectionError,
} from "./index.js"

interface CliOptions {
  inputPath?: string
  outputPath?: string
  idlDir?: string
  namespace?: string
  serviceName?: string
  help: boolean
}

async function main(argv: string[]): Promise<void> {
  const options = parseArgs(argv)
  if (options.help || !options.inputPath) {
    printUsage()
    process.exit(options.help ? 0 : 1)
  }

  const source = await readFile(options.inputPath, "utf8")
  const routeMethodNames = options.idlDir
    ? await loadRouteMethodNamesFromIdlDir(options.idlDir)
    : undefined
  const result = convertOpenApiToThrift(source, {
    namespace: options.namespace,
    serviceName: options.serviceName,
    routeMethodNames,
  })

  if (options.outputPath) {
    await writeFile(options.outputPath, result.thrift, "utf8")
    process.stdout.write(`${options.outputPath}\n`)
    return
  }

  process.stdout.write(result.thrift)
}

function parseArgs(argv: string[]): CliOptions {
  const options: CliOptions = {
    help: false,
  }

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index]
    switch (arg) {
      case "--help":
      case "-h":
        options.help = true
        break
      case "--input":
      case "-i":
        options.inputPath = argv[index + 1]
        index += 1
        break
      case "--output":
      case "-o":
        options.outputPath = argv[index + 1]
        index += 1
        break
      case "--idl-dir":
        options.idlDir = argv[index + 1]
        index += 1
        break
      case "--namespace":
        options.namespace = argv[index + 1]
        index += 1
        break
      case "--service-name":
        options.serviceName = argv[index + 1]
        index += 1
        break
      default:
        throw new OpenApiProjectionError(`未知参数 ${arg}`)
    }
  }

  return options
}

function printUsage(): void {
  process.stdout.write(`@sttot/openapi-thrift

Usage:
  node dist/cli.js --input <openapi.json> [--output <out.thrift>] [--idl-dir <idlDir>] [--namespace <go.namespace>] [--service-name <ServiceName>]

Examples:
  node dist/cli.js --input ./project.openapi.json --namespace dramawork.project --service-name ProjectService
  node dist/cli.js --input ./project.openapi.json --idl-dir ../existing-idl --output ./idl/project.thrift
  node dist/cli.js --input ./project.openapi.json --output ./idl/project.thrift
`)
}

async function loadRouteMethodNamesFromIdlDir(
  idlDir: string,
): Promise<Record<string, string>> {
  const thriftPaths = await collectThriftFiles(idlDir)
  const files = await Promise.all(
    thriftPaths.map(async (filePath) => ({
      path: filePath,
      content: await readFile(filePath, "utf8"),
    })),
  )
  return extractRouteMethodNameMapFromThriftSources(files)
}

async function collectThriftFiles(rootDir: string): Promise<string[]> {
  const entries = await readdir(rootDir, { withFileTypes: true })
  const files: string[] = []

  for (const entry of entries) {
    const fullPath = path.join(rootDir, entry.name)
    if (entry.isDirectory()) {
      files.push(...(await collectThriftFiles(fullPath)))
      continue
    }
    if (entry.isFile() && entry.name.endsWith(".thrift")) {
      files.push(fullPath)
    }
  }

  return files.sort()
}

void main(process.argv.slice(2)).catch((error) => {
  const message = error instanceof Error ? error.message : String(error)
  process.stderr.write(`${message}\n`)
  process.exit(1)
})
