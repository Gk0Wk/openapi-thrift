import assert from "node:assert/strict"
import { execFile as execFileCallback } from "node:child_process"
import { fileURLToPath } from "node:url"
import { promisify } from "node:util"

const execFile = promisify(execFileCallback)

const allowedFiles = new Set([
  "LICENSE",
  "README.md",
  "package.json",
  "dist/cli.d.ts",
  "dist/cli.js",
  "dist/index.d.ts",
  "dist/index.js",
  "dist/model.d.ts",
  "dist/model.js",
  "dist/projector.d.ts",
  "dist/projector.js",
  "dist/thrift-route-index.d.ts",
  "dist/thrift-route-index.js",
])

const maxCompressedSizeBytes = 20_000
const maxUnpackedSizeBytes = 80_000
const packageRoot = fileURLToPath(new URL("..", import.meta.url))

async function main() {
  const { stdout } =
    process.platform === "win32"
      ? await execFile(
          process.env.ComSpec ?? "cmd.exe",
          ["/d", "/s", "/c", "npm pack --dry-run --json"],
          { cwd: packageRoot },
        )
      : await execFile("npm", ["pack", "--dry-run", "--json"], {
          cwd: packageRoot,
        })
  const report = JSON.parse(stdout)
  assert.equal(report.length, 1, "npm pack --dry-run 应只返回一个 tarball 条目")

  const [entry] = report
  assert.equal(entry.name, "@sttot/openapi-thrift", "包名与预期不一致")
  assert.ok(
    entry.size <= maxCompressedSizeBytes,
    `压缩包体过大: ${entry.size} bytes > ${maxCompressedSizeBytes} bytes`,
  )
  assert.ok(
    entry.unpackedSize <= maxUnpackedSizeBytes,
    `解包体积过大: ${entry.unpackedSize} bytes > ${maxUnpackedSizeBytes} bytes`,
  )

  const filePaths = entry.files.map((file) => file.path)
  for (const filePath of filePaths) {
    assert.ok(allowedFiles.has(filePath), `tarball 包含未允许文件: ${filePath}`)
  }

  for (const requiredFile of allowedFiles) {
    assert.ok(filePaths.includes(requiredFile), `tarball 缺少预期文件: ${requiredFile}`)
  }

  process.stdout.write(
    JSON.stringify(
      {
        package: entry.name,
        version: entry.version,
        compressedSize: entry.size,
        unpackedSize: entry.unpackedSize,
        files: filePaths,
      },
      null,
      2,
    ) + "\n",
  )
}

void main().catch((error) => {
  const message = error instanceof Error ? error.message : String(error)
  process.stderr.write(`${message}\n`)
  process.exit(1)
})
