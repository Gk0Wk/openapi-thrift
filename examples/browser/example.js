import {
  initializeOpenApiThrift,
  validateOpenApiRenderDocument,
  formatOpenApiRenderValidationIssues,
  convertOpenApiToThrift,
} from "/dist/index.js"

const source = document.querySelector("#source")
const button = document.querySelector("#convert")
const status = document.querySelector("#status")
const issues = document.querySelector("#issues")
const output = document.querySelector("#output")

try {
  await initializeOpenApiThrift()
  button.disabled = false
  status.textContent = "已就绪"
} catch (error) {
  status.textContent = `加载失败：${error instanceof Error ? error.message : String(error)}`
}

button.addEventListener("click", () => {
  output.textContent = ""
  output.hidden = true
  issues.textContent = ""
  issues.hidden = true
  try {
    const validation = validateOpenApiRenderDocument(source.value)
    issues.textContent = formatOpenApiRenderValidationIssues(validation.issues)
    issues.hidden = validation.issues.length === 0
    if (validation.errorCount > 0) {
      status.textContent = `校验失败：${validation.errorCount} 个错误`
      return
    }
    output.textContent = convertOpenApiToThrift(source.value).thrift
    output.hidden = false
    status.textContent = "转换完成"
  } catch (error) {
    status.textContent = "转换失败"
    issues.textContent = error instanceof Error ? error.message : String(error)
    issues.hidden = false
  }
})
