import assert from "node:assert/strict"
import fs from "node:fs"
import test from "node:test"

import {
  buildRouteKey,
  convertOpenApiToThrift,
  extractRouteMethodNameMapFromThriftSources,
  OpenApiProjectionError,
} from "../dist/index.js"

function loadFixtureDocument(name) {
  return JSON.parse(
    fs.readFileSync(new URL(`./fixtures/${name}`, import.meta.url), "utf8"),
  )
}

function loadFixtureText(name) {
  return fs.readFileSync(new URL(`./fixtures/${name}`, import.meta.url), "utf8")
}

test("projects the constrained OpenAPI profile into thrift", () => {
  const document = {
    openapi: "3.0.3",
    info: {
      title: "Project API",
    },
    paths: {
      "/api/v1/projects/{project_id}": {
        get: {
          operationId: "getProject",
          summary: "Get project detail",
          parameters: [
            {
              name: "project_id",
              in: "path",
              required: true,
              schema: { type: "string" },
            },
            {
              name: "include_jobs",
              in: "query",
              schema: { type: "boolean" },
            },
          ],
          responses: {
            200: {
              description: "Project detail",
              content: {
                "application/json": {
                  schema: { $ref: "#/components/schemas/Project" },
                },
              },
            },
          },
        },
      },
      "/api/v1/projects": {
        post: {
          operationId: "createProject",
          summary: "Create project",
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  required: ["title"],
                  properties: {
                    title: { type: "string" },
                    metadata: {
                      type: "object",
                      properties: {
                        locale: {
                          type: "string",
                          enum: ["zh-CN", "en-US"],
                        },
                      },
                    },
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "Created project",
              content: {
                "application/json": {
                  schema: { $ref: "#/components/schemas/Project" },
                },
              },
            },
          },
        },
      },
    },
    components: {
      schemas: {
        Project: {
          type: "object",
          properties: {
            id: { type: "string" },
            title: { type: "string" },
          },
        },
      },
    },
  }

  const result = convertOpenApiToThrift(document, {
    namespace: "dramawork.project",
    serviceName: "ProjectService",
  })

  assert.match(result.thrift, /namespace go dramawork\.project/)
  assert.match(result.thrift, /struct GetProjectRequest/)
  assert.match(
    result.thrift,
    /1: required string project_id \(api\.path="project_id"\)/,
  )
  assert.match(
    result.thrift,
    /2: optional bool include_jobs \(api\.query="include_jobs"\)/,
  )
  assert.match(result.thrift, /struct CreateProjectRequestMetadata/)
  assert.match(
    result.thrift,
    /locale \(go\.tag='validate:"oneof=zh-CN en-US"'\)/,
  )
  assert.match(
    result.thrift,
    /Project GetProject\(1: GetProjectRequest req\) \(api\.get="\/api\/v1\/projects\/:project_id"\)/,
  )
  assert.match(
    result.thrift,
    /Project CreateProject\(1: CreateProjectRequest req\) \(api\.post="\/api\/v1\/projects"\)/,
  )
})

test("accepts OpenAPI 3.1 documents", () => {
  const document = {
    openapi: "3.1.0",
    info: {
      title: "Project API",
    },
    paths: {
      "/api/v1/ping": {
        get: {
          operationId: "ping",
          responses: {
            200: {
              description: "pong",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  const result = convertOpenApiToThrift(document, {
    namespace: "dramawork.project",
    serviceName: "ProjectService",
  })

  assert.match(result.thrift, /service ProjectService/)
})

test("fails fast on unsupported oneOf", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Bad API" },
    paths: {
      "/api/v1/bad": {
        post: {
          operationId: "badOperation",
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  oneOf: [{ type: "string" }, { type: "integer" }],
                },
              },
            },
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  assert.throws(
    () => convertOpenApiToThrift(document),
    (error) =>
      error instanceof OpenApiProjectionError &&
      error.message.includes("oneOf"),
  )
})

test("supports nullable anyOf and route-based method fallback", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "JobSeek API" },
    paths: {
      "/api/v1/auth/wx-login": {
        post: {
          summary: "微信登录",
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  properties: {
                    redirect_url: {
                      anyOf: [{ type: "string" }, { type: "null" }],
                    },
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      qr_code: {
                        anyOf: [{ type: "string" }, { type: "null" }],
                      },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  const routeMethodNames = extractRouteMethodNameMapFromThriftSources([
    {
      path: "auth.thrift",
      content:
        'service AuthService {\n  WxLoginResponse WxLogin(1: WxLoginRequest req) (api.post="/api/v1/auth/wx-login")\n}\n',
    },
  ])

  assert.equal(
    routeMethodNames[buildRouteKey("post", "/api/v1/auth/wx-login")],
    "WxLogin",
  )

  const result = convertOpenApiToThrift(document, {
    routeMethodNames,
  })

  assert.match(result.thrift, /string redirect_url/)
  assert.match(
    result.thrift,
    /WxLoginResponse WxLogin\(1: WxLoginRequest req\) \(api\.post="\/api\/v1\/auth\/wx-login"\)/,
  )
})

test("supports json alias and scalar request body fallback", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Scalar Body API" },
    paths: {
      "/api/v1/demands/search": {
        post: {
          operationId: "SearchDemands",
          requestBody: {
            required: true,
            content: {
              json: {
                schema: { type: "string" },
              },
            },
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  const result = convertOpenApiToThrift(document)

  assert.match(
    result.thrift,
    /1: optional string body \(api\.raw_body="", go\.tag='validate:"required"'\)/,
  )
})

test("supports query array parameters with default repeated-key semantics", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Query Array API" },
    paths: {
      "/api/v1/media/urls": {
        get: {
          operationId: "GetMediaUrls",
          parameters: [
            {
              name: "file_paths",
              in: "query",
              required: true,
              schema: {
                type: "array",
                items: { type: "string" },
              },
            },
          ],
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  const result = convertOpenApiToThrift(document)

  assert.match(
    result.thrift,
    /1: optional list<string> file_paths \(api\.query="file_paths", go\.tag='validate:"required"'\)/,
  )
})

test("supports header and cookie parameters", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Auth Meta API" },
    paths: {
      "/api/v1/profile": {
        get: {
          operationId: "GetProfile",
          parameters: [
            {
              name: "X-Request-ID",
              in: "header",
              required: true,
              schema: { type: "string", minLength: 16, maxLength: 64 },
            },
            {
              name: "csrf_token",
              in: "cookie",
              schema: { type: "string", minLength: 8 },
            },
          ],
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  const result = convertOpenApiToThrift(document)

  assert.match(
    result.thrift,
    /x_request_id \(api\.header="X-Request-ID", go\.tag='validate:"required,min=16,max=64"'\)/,
  )
  assert.match(
    result.thrift,
    /csrf_token \(api\.cookie="csrf_token", go\.tag='validate:"min=8"'\)/,
  )
})

test("supports 204 empty success response", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Delete API" },
    paths: {
      "/api/v1/items/{item_id}": {
        delete: {
          operationId: "DeleteItem",
          parameters: [
            {
              name: "item_id",
              in: "path",
              required: true,
              schema: { type: "string" },
            },
          ],
          responses: {
            204: {
              description: "deleted",
            },
          },
        },
      },
    },
  }

  const result = convertOpenApiToThrift(document)

  assert.match(result.thrift, /struct EmptyResponse \{/)
  assert.match(
    result.thrift,
    /EmptyResponse DeleteItem\(1: DeleteItemRequest req\) \(api\.delete="\/api\/v1\/items\/:item_id"\)/,
  )
})

test("fails fast on deepObject query parameters", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Deep Query API" },
    paths: {
      "/api/v1/search": {
        get: {
          operationId: "Search",
          parameters: [
            {
              name: "filter",
              in: "query",
              style: "deepObject",
              explode: true,
              schema: {
                type: "object",
                properties: {
                  city: { type: "string" },
                },
              },
            },
          ],
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  assert.throws(
    () => convertOpenApiToThrift(document),
    (error) =>
      error instanceof OpenApiProjectionError &&
      error.message.includes("deepObject"),
  )
})

test("fails fast on non-default query array serialization", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Delimited Query API" },
    paths: {
      "/api/v1/search": {
        get: {
          operationId: "SearchDelimited",
          parameters: [
            {
              name: "ids",
              in: "query",
              style: "pipeDelimited",
              explode: false,
              schema: {
                type: "array",
                items: { type: "string" },
              },
            },
          ],
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  assert.throws(
    () => convertOpenApiToThrift(document),
    (error) =>
      error instanceof OpenApiProjectionError &&
      error.message.includes("pipeDelimited"),
  )
})

test("fails fast on parameter content", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Parameter Content API" },
    paths: {
      "/api/v1/search": {
        get: {
          operationId: "SearchByContent",
          parameters: [
            {
              name: "filter",
              in: "query",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      city: { type: "string" },
                    },
                  },
                },
              },
            },
          ],
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  assert.throws(
    () => convertOpenApiToThrift(document),
    (error) =>
      error instanceof OpenApiProjectionError &&
      error.message.includes("parameter content"),
  )
})

test("projects numeric length array and format validators into go tags", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Validation API" },
    paths: {
      "/api/v1/teachers": {
        post: {
          operationId: "CreateTeacher",
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  required: ["profile"],
                  properties: {
                    age: {
                      type: "integer",
                      minimum: 18,
                      maximum: 65,
                    },
                    nickname: {
                      type: "string",
                      minLength: 2,
                      maxLength: 20,
                    },
                    tags: {
                      type: "array",
                      minItems: 1,
                      maxItems: 5,
                      items: { type: "string" },
                    },
                    email: {
                      type: "string",
                      format: "email",
                    },
                    homepage: {
                      type: "string",
                      format: "uri",
                    },
                    host: {
                      type: "string",
                      format: "hostname",
                    },
                    phone: {
                      type: "string",
                      format: "e164",
                    },
                    birthday: {
                      type: "string",
                      format: "date",
                    },
                    submitted_at: {
                      type: "string",
                      format: "date-time",
                    },
                    session_token: {
                      type: "string",
                      format: "jwt",
                    },
                    profile: {
                      $ref: "#/components/schemas/Profile",
                    },
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
    components: {
      schemas: {
        Profile: {
          type: "object",
          properties: {
            bio: { type: "string" },
          },
        },
      },
    },
  }

  const result = convertOpenApiToThrift(document)

  assert.match(
    result.thrift,
    /age \(api\.body="age", go\.tag='validate:"gte=18,lte=65"'\)/,
  )
  assert.match(
    result.thrift,
    /nickname \(api\.body="nickname", go\.tag='validate:"min=2,max=20"'\)/,
  )
  assert.match(
    result.thrift,
    /tags \(api\.body="tags", go\.tag='validate:"min=1,max=5"'\)/,
  )
  assert.match(
    result.thrift,
    /email \(api\.body="email", go\.tag='validate:"email"'\)/,
  )
  assert.match(
    result.thrift,
    /homepage \(api\.body="homepage", go\.tag='validate:"uri"'\)/,
  )
  assert.match(
    result.thrift,
    /host \(api\.body="host", go\.tag='validate:"hostname_rfc1123"'\)/,
  )
  assert.match(
    result.thrift,
    /phone \(api\.body="phone", go\.tag='validate:"e164"'\)/,
  )
  assert.match(
    result.thrift,
    /birthday \(api\.body="birthday", go\.tag='validate:"datetime=2006-01-02"'\)/,
  )
  assert.match(
    result.thrift,
    /submitted_at \(api\.body="submitted_at", go\.tag='validate:"datetime=2006-01-02T15:04:05Z07:00"'\)/,
  )
  assert.match(
    result.thrift,
    /session_token \(api\.body="session_token", go\.tag='validate:"jwt"'\)/,
  )
  assert.match(
    result.thrift,
    /profile \(api\.body="profile", go\.tag='validate:"required"'\)/,
  )
})

test("accepts standard numeric scalar formats", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Numeric Format API" },
    paths: {
      "/api/v1/metrics": {
        post: {
          operationId: "CreateMetric",
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  properties: {
                    score: {
                      type: "number",
                      format: "float",
                    },
                    total: {
                      type: "integer",
                      format: "int64",
                    },
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  const result = convertOpenApiToThrift(document)

  assert.match(result.thrift, /score/)
  assert.match(result.thrift, /total/)
})

test("fails fast on unsupported string format", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Format API" },
    paths: {
      "/api/v1/format": {
        post: {
          operationId: "FormatOperation",
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  properties: {
                    secret: {
                      type: "string",
                      format: "password",
                    },
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  assert.throws(
    () => convertOpenApiToThrift(document),
    (error) =>
      error instanceof OpenApiProjectionError &&
      error.message.includes("format=password"),
  )
})

test("fails fast on parameter $ref", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Parameter Ref API" },
    paths: {
      "/api/v1/ref": {
        get: {
          operationId: "GetByRef",
          parameters: [
            {
              $ref: "#/components/parameters/Keyword",
            },
          ],
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  assert.throws(
    () => convertOpenApiToThrift(document),
    (error) =>
      error instanceof OpenApiProjectionError &&
      error.message.includes("parameter $ref"),
  )
})

test("supports requestBody $ref via local components.requestBodies", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Request Body Ref API" },
    paths: {
      "/api/v1/ref-body": {
        post: {
          operationId: "PostByRef",
          requestBody: {
            $ref: "#/components/requestBodies/CreateBody",
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
    components: {
      requestBodies: {
        CreateBody: {
          required: true,
          content: {
            "application/json": {
              schema: {
                type: "object",
                required: ["title"],
                properties: {
                  title: { type: "string", minLength: 1 },
                },
              },
            },
          },
        },
      },
    },
  }

  const result = convertOpenApiToThrift(document)

  assert.match(
    result.thrift,
    /1: optional string title \(api\.body="title", go\.tag='validate:"required,min=1"'\)/,
  )
})

test("captures current APIFox boundary lab export as regression fixture", () => {
  const document = loadFixtureDocument(
    "apifox-boundary-lab.export.openapi.json",
  )

  assert.equal(
    document.paths["/v1/lab/mutation/ref-request-body"].post.requestBody
      .content["application/json"].schema.$ref,
    "#/components/schemas/CreateProfileRequest",
  )
  assert.equal(
    document.paths["/v1/lab/mutation/union-contact"].post.requestBody.content[
      "application/json"
    ].schema.oneOf.length,
    2,
  )
  assert.equal(
    document.components.schemas.MutationMultipleOfPayload.properties.price
      .multipleOf,
    0.01,
  )
  assert.equal(
    document.components.schemas.MutationStrictObjectPayload.properties.profile
      .additionalProperties,
    false,
  )
  assert.equal(
    document.paths["/v1/lab/mutation/upload-file"].post.requestBody.content[
      "multipart/form-data"
    ].schema.properties.file.format,
    "binary",
  )
  assert.ok(
    document.paths["/v1/lab/mutation/non-json-success"].get.responses["200"]
      .content["application/octet-stream"],
  )
})

test("captures APIFox followup export shape for 204 and multiple content-types", () => {
  const document = loadFixtureDocument(
    "apifox-boundary-lab.followup.export.openapi.json",
  )

  assert.equal(
    document.paths["/v1/lab/mutation/no-content"].delete.responses["204"]
      .description,
    "deleted",
  )
  assert.deepEqual(
    Object.keys(
      document.paths["/v1/lab/mutation/multi-content-type"].post.requestBody
        .content,
    ),
    ["application/json"],
  )
  assert.deepEqual(
    Object.keys(
      document.paths["/v1/lab/mutation/multi-content-type"].post.responses[
        "200"
      ].content,
    ),
    ["application/json"],
  )
})

test("captures APIFox export shape for header and cookie parameters", () => {
  const document = loadFixtureDocument(
    "apifox-boundary-lab.header-cookie.export.openapi.json",
  )
  const parameters =
    document.paths["/v1/lab/mutation/header-cookie"].get.parameters

  assert.equal(parameters.length, 2)
  assert.deepEqual(
    parameters.map((parameter) => parameter.in),
    ["cookie", "header"],
  )
  assert.equal(parameters[0].name, "csrf_token")
  assert.equal(parameters[0].schema.minLength, 8)
  assert.equal(parameters[1].name, "X-Request-ID")
  assert.equal(parameters[1].required, true)
  assert.equal(parameters[1].schema.minLength, 16)
  assert.equal(parameters[1].schema.maxLength, 64)
})

test("converts supported subset fixture end-to-end", () => {
  const document = loadFixtureDocument(
    "apifox-boundary-lab.supported.openapi.json",
  )
  const expectedThrift = loadFixtureText("apifox-boundary-lab.supported.thrift")

  const result = convertOpenApiToThrift(document, {
    namespace: "dramawork.boundarylab.supported",
    serviceName: "BoundaryLabSupportedService",
  })

  assert.equal(result.thrift, expectedThrift)
})

test("fails fast on multiple 2xx success responses", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Multiple Success API" },
    paths: {
      "/api/v1/multi-success": {
        post: {
          operationId: "MultiSuccess",
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
            202: {
              description: "accepted",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      queued: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  assert.throws(
    () => convertOpenApiToThrift(document),
    (error) =>
      error instanceof OpenApiProjectionError &&
      error.message.includes("多个 2xx success response"),
  )
})

test("fails fast on request body with parallel multiple content-types", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Multiple Request Content API" },
    paths: {
      "/api/v1/multi-request-content": {
        post: {
          operationId: "MultiRequestContent",
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  properties: { name: { type: "string" } },
                },
              },
              "application/x-www-form-urlencoded": {
                schema: {
                  type: "object",
                  properties: { name: { type: "string" } },
                },
              },
            },
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: { ok: { type: "boolean" } },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  assert.throws(
    () => convertOpenApiToThrift(document),
    (error) =>
      error instanceof OpenApiProjectionError &&
      error.message.includes("并行多 content-type 主线"),
  )
})

test("fails fast on success response with parallel multiple content-types", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Multiple Response Content API" },
    paths: {
      "/api/v1/multi-response-content": {
        get: {
          operationId: "MultiResponseContent",
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: { ok: { type: "boolean" } },
                  },
                },
                "text/plain": {
                  schema: { type: "string" },
                },
              },
            },
          },
        },
      },
    },
  }

  assert.throws(
    () => convertOpenApiToThrift(document),
    (error) =>
      error instanceof OpenApiProjectionError &&
      error.message.includes("并行多 content-type 主线"),
  )
})

test("fails fast on oneOf with discriminator", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Discriminator API" },
    paths: {
      "/api/v1/contact": {
        post: {
          operationId: "CreateContact",
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  oneOf: [
                    {
                      type: "object",
                      properties: {
                        kind: { type: "string", enum: ["email"] },
                        email: { type: "string" },
                      },
                    },
                    {
                      type: "object",
                      properties: {
                        kind: { type: "string", enum: ["phone"] },
                        phone: { type: "string" },
                      },
                    },
                  ],
                  discriminator: {
                    propertyName: "kind",
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: { ok: { type: "boolean" } },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  assert.throws(
    () => convertOpenApiToThrift(document),
    (error) =>
      error instanceof OpenApiProjectionError &&
      error.message.includes("oneOf"),
  )
})

test("supports non-json success response as raw body wrapper", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "CSV Success API" },
    paths: {
      "/api/v1/export": {
        get: {
          operationId: "ExportCsv",
          responses: {
            200: {
              description: "ok",
              content: {
                "text/csv": {
                  schema: {
                    type: "string",
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  const result = convertOpenApiToThrift(document)

  assert.match(result.thrift, /struct ExportCsvRawBodyResponse/)
  assert.match(result.thrift, /1: optional string body \(api\.raw_body=""\)/)
  assert.match(
    result.thrift,
    /ExportCsvRawBodyResponse ExportCsv\(1: ExportCsvRequest req\)/,
  )
})

test("fails fast on non-json success response with object schema", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Bad Binary Success API" },
    paths: {
      "/api/v1/export-bad": {
        get: {
          operationId: "ExportBad",
          responses: {
            200: {
              description: "ok",
              content: {
                "application/octet-stream": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  assert.throws(
    () => convertOpenApiToThrift(document),
    (error) =>
      error instanceof OpenApiProjectionError &&
      error.message.includes("非 JSON success response 只支持 string/binary"),
  )
})

test("fails fast on object schemas mixing properties and additionalProperties", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Mixed Object API" },
    paths: {
      "/api/v1/mixed-map": {
        post: {
          operationId: "CreateMixedMap",
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  properties: {
                    metadata: {
                      type: "object",
                      properties: {
                        fixed: {
                          type: "string",
                        },
                      },
                      additionalProperties: {
                        type: "string",
                      },
                    },
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  assert.throws(
    () => convertOpenApiToThrift(document),
    (error) =>
      error instanceof OpenApiProjectionError &&
      error.message.includes("properties 与 additionalProperties 混用"),
  )
})

test("fails fast on unsupported pattern", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Pattern API" },
    paths: {
      "/api/v1/pattern": {
        post: {
          operationId: "PatternOperation",
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  properties: {
                    phone: {
                      type: "string",
                      pattern: "^1\\d{10}$",
                    },
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  assert.throws(
    () => convertOpenApiToThrift(document),
    (error) =>
      error instanceof OpenApiProjectionError &&
      error.message.includes("pattern 自动投影"),
  )
})

test("fails fast on unsupported multipleOf", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Multiple API" },
    paths: {
      "/api/v1/multiple": {
        post: {
          operationId: "MultipleOperation",
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  properties: {
                    step: {
                      type: "number",
                      multipleOf: 0.5,
                    },
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  assert.throws(
    () => convertOpenApiToThrift(document),
    (error) =>
      error instanceof OpenApiProjectionError &&
      error.message.includes("multipleOf 自动投影"),
  )
})

test("fails fast on unsupported additionalProperties false", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Closed Object API" },
    paths: {
      "/api/v1/closed": {
        post: {
          operationId: "ClosedOperation",
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  additionalProperties: false,
                  properties: {
                    name: {
                      type: "string",
                    },
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  assert.throws(
    () => convertOpenApiToThrift(document),
    (error) =>
      error instanceof OpenApiProjectionError &&
      error.message.includes("additionalProperties: false 自动投影"),
  )
})

test("does not allow manual validator override for additionalProperties false", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Closed Object Manual Override API" },
    paths: {
      "/api/v1/closed-override": {
        post: {
          operationId: "ClosedOverrideOperation",
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  properties: {
                    payload: {
                      type: "object",
                      additionalProperties: false,
                      properties: {
                        name: { type: "string" },
                      },
                      "x-dramawork-allow-unsupported-validation": true,
                      "x-dramawork-validate": "strict_object",
                    },
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  assert.throws(
    () => convertOpenApiToThrift(document),
    (error) =>
      error instanceof OpenApiProjectionError &&
      error.message.includes("不能通过字段 validator 接管"),
  )
})

test("allows explicit manual validator override for unsupported keywords", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Manual Override API" },
    paths: {
      "/api/v1/manual": {
        post: {
          operationId: "ManualOverrideOperation",
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  properties: {
                    phone: {
                      type: "string",
                      pattern: "^1\\d{10}$",
                      "x-dramawork-allow-unsupported-validation": true,
                      "x-dramawork-validate": "cn_mobile",
                    },
                    score: {
                      type: "number",
                      multipleOf: 0.5,
                      "x-dramawork-allow-unsupported-validation": true,
                      "x-dramawork-validate": ["half_step", "gte=0"],
                    },
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  const result = convertOpenApiToThrift(document)

  assert.match(
    result.thrift,
    /phone \(api\.body="phone", go\.tag='validate:"cn_mobile"'\)/,
  )
  assert.match(
    result.thrift,
    /score \(api\.body="score", go\.tag='validate:"half_step,gte=0"'\)/,
  )
})

test("fails when unsupported override flag has no manual validators", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Bad Manual Override API" },
    paths: {
      "/api/v1/manual-bad": {
        post: {
          operationId: "BadManualOverrideOperation",
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  properties: {
                    phone: {
                      type: "string",
                      pattern: "^1\\d{10}$",
                      "x-dramawork-allow-unsupported-validation": true,
                    },
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  assert.throws(
    () => convertOpenApiToThrift(document),
    (error) =>
      error instanceof OpenApiProjectionError &&
      error.message.includes(
        "x-dramawork-allow-unsupported-validation 需要同时提供 x-dramawork-validate",
      ),
  )
})

test("supports application/x-www-form-urlencoded as api.form", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Form API" },
    paths: {
      "/api/v1/form": {
        post: {
          operationId: "SubmitForm",
          requestBody: {
            content: {
              "application/x-www-form-urlencoded": {
                schema: {
                  type: "object",
                  properties: {
                    keyword: {
                      type: "string",
                      minLength: 1,
                    },
                    page: {
                      type: "integer",
                      minimum: 1,
                    },
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  const result = convertOpenApiToThrift(document)

  assert.match(
    result.thrift,
    /keyword \(api\.form="keyword", go\.tag='validate:"min=1"'\)/,
  )
  assert.match(
    result.thrift,
    /page \(api\.form="page", go\.tag='validate:"gte=1"'\)/,
  )
})

test("supports multipart form fields without files", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Multipart API" },
    paths: {
      "/api/v1/multipart": {
        post: {
          operationId: "SubmitMultipart",
          requestBody: {
            content: {
              "multipart/form-data": {
                schema: {
                  type: "object",
                  properties: {
                    title: {
                      type: "string",
                      maxLength: 50,
                    },
                    visible: {
                      type: "boolean",
                    },
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  const result = convertOpenApiToThrift(document)

  assert.match(
    result.thrift,
    /title \(api\.form="title", go\.tag='validate:"max=50"'\)/,
  )
  assert.match(result.thrift, /visible \(api\.form="visible"\)/)
})

test("fails fast on multipart binary file fields", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Multipart File API" },
    paths: {
      "/api/v1/upload": {
        post: {
          operationId: "UploadFile",
          requestBody: {
            content: {
              "multipart/form-data": {
                schema: {
                  type: "object",
                  properties: {
                    file: {
                      type: "string",
                      format: "binary",
                    },
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  assert.throws(
    () => convertOpenApiToThrift(document),
    (error) =>
      error instanceof OpenApiProjectionError &&
      error.message.includes("单文件或多文件"),
  )
})

test("fails fast on multipart multi-file array fields", () => {
  const document = {
    openapi: "3.0.3",
    info: { title: "Multipart Multi File API" },
    paths: {
      "/api/v1/upload-multi": {
        post: {
          operationId: "UploadMultiFile",
          requestBody: {
            content: {
              "multipart/form-data": {
                schema: {
                  type: "object",
                  properties: {
                    files: {
                      type: "array",
                      items: {
                        type: "string",
                        format: "binary",
                      },
                    },
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "ok",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      ok: { type: "boolean" },
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  }

  assert.throws(
    () => convertOpenApiToThrift(document),
    (error) =>
      error instanceof OpenApiProjectionError &&
      error.message.includes("单文件或多文件"),
  )
})
