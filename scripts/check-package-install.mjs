import assert from "node:assert/strict"
import { execFile as execFileCallback } from "node:child_process"
import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises"
import { dirname, join, resolve } from "node:path"
import { fileURLToPath } from "node:url"
import { promisify } from "node:util"

const execFile = promisify(execFileCallback)
const root = fileURLToPath(new URL("..", import.meta.url))

async function main() {
  const [input, option] = process.argv.slice(2)
  assert.ok(input, "usage: npm run install:check -- <package.tgz> [--keep]")
  assert.ok(option === undefined || option === "--keep", "unknown option")
  const npmCLI = process.env.npm_execpath
  assert.ok(
    npmCLI?.endsWith("npm-cli.js"),
    "invoke through npm run install:check",
  )
  const manifest = JSON.parse(
    await readFile(join(root, "package.json"), "utf8"),
  )
  await mkdir(join(root, ".tmp"), { recursive: true })
  const directory = await mkdtemp(join(root, ".tmp", "npm-consumer-"))
  try {
    await writeFile(
      join(directory, "package.json"),
      JSON.stringify({ private: true, type: "module" }),
    )
    const commandOptions = {
      cwd: directory,
      // Installation and consumption must not call Go or package lifecycle hooks.
      env: { ...process.env, PATH: dirname(process.execPath) },
    }
    await execFile(
      process.execPath,
      [
        npmCLI,
        "install",
        resolve(input),
        "--offline",
        "--ignore-scripts",
        "--no-audit",
        "--no-fund",
        "--no-package-lock",
        "--userconfig",
        join(directory, "unused.npmrc"),
      ],
      commandOptions,
    )
    await writeFile(
      join(directory, "consumer.mjs"),
      await readFile(new URL("./package-consumer.mjs", import.meta.url)),
    )
    const { stdout } = await execFile(
      process.execPath,
      ["consumer.mjs", manifest.version],
      commandOptions,
    )
    assert.match(stdout, /Installed npm consumer passed/)
    process.stdout.write(
      `${JSON.stringify({
        package: manifest.name,
        version: manifest.version,
        tarball: resolve(input),
        consumerDirectory: option === "--keep" ? directory : undefined,
        result: "passed",
        node: process.version,
      })}\n`,
    )
  } finally {
    if (option !== "--keep")
      await rm(directory, { recursive: true, force: true })
  }
}

void main().catch((error) => {
  process.stderr.write(
    `${error instanceof Error ? error.message : String(error)}\n`,
  )
  process.exitCode = 1
})
