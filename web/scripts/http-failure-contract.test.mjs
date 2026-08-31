import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { fileURLToPath } from "node:url";

import ts from "typescript";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const httpResponse = await loadTypeScriptModule("src/lib/api/core/http-response.ts");
const httpErrorModuleURL = await transpiledTypeScriptModuleURL(
  "src/lib/api/core/http-error.ts",
);
const httpError = await import(httpErrorModuleURL);
const errorMessage = await loadTypeScriptModule(
  "src/lib/error-message.ts",
  (source) => source.replace(
    '"@/lib/api/core/http-error"',
    JSON.stringify(httpErrorModuleURL),
  ),
);

test("legacy HTTP failures keep diagnostic IDs out of user messages", async () => {
  const { buildApiErrorMessage, getApiFailure } = httpResponse;
  const payload = {
    data: {
      detail: "旧接口错误",
      request_id: "legacy-diagnostic-id",
    },
    message: "failed",
  };

  assert.equal(getApiFailure(payload), null);
  assert.equal(
    buildApiErrorMessage({ status: 409, statusText: "Conflict" }, payload),
    "旧接口错误",
  );
});

test("unknown HTTP bodies never become user-facing error copy", async () => {
  const { buildApiErrorMessage, parseApiResponseBody } = httpResponse;
  const payload = await parseApiResponseBody({
    status: 502,
    statusText: "Bad Gateway",
    text: async () => "<html>proxy secret and upstream stack</html>",
  });

  assert.deepEqual(payload, { message: "服务暂时无法完成请求" });
  assert.equal(
    buildApiErrorMessage({ status: 502, statusText: "Bad Gateway" }, payload),
    "服务暂时无法完成请求",
  );
  assert.equal(
    buildApiErrorMessage(
      { status: 500, statusText: "Internal Server Error" },
      { data: { detail: { sql: "private table" } } },
    ),
    "服务暂时无法完成请求",
  );

  for (const rawText of [
    '"proxy stack and secret"',
    "42",
    '["proxy stack and secret"]',
  ]) {
    const nonEnvelope = await parseApiResponseBody({
      status: 502,
      statusText: "Bad Gateway",
      text: async () => rawText,
    });
    assert.deepEqual(nonEnvelope, { message: "服务暂时无法完成请求" });
    assert.equal(
      buildApiErrorMessage(
        { status: 502, statusText: "Bad Gateway" },
        nonEnvelope,
      ),
      "服务暂时无法完成请求",
    );
  }

  const { ApiRequestError } = httpError;
  assert.equal(
    errorMessage.getErrorMessage(
      new ApiRequestError("服务暂时无法完成请求", 502),
      "Service is temporarily unavailable",
    ),
    "Service is temporarily unavailable",
  );
});

test("FailureCore keeps only result facts and a diagnostic identity", async () => {
  const { buildApiErrorMessage, getApiFailure } = httpResponse;
  const payload = {
    data: {
      detail: "工作图已被其他操作更新",
      request_id: "legacy-copy",
      failure: {
        version: 1,
        code: "workgraph.revision_conflict",
        category: "conflict",
        effect: "not_applied",
        transport_request_id: "http-attempt-1",
      },
    },
  };

  const failure = getApiFailure(payload);
  assert.equal(failure?.transport_request_id, "http-attempt-1");
  assert.equal(
    buildApiErrorMessage({ status: 409, statusText: "Conflict" }, payload),
    "工作图已被其他操作更新",
  );
});

test("future FailureCore values decode without inventing behavior", async () => {
  const { parseFailureCore } = httpResponse;
  const failure = parseFailureCore({
    version: 7,
    code: "future.new_failure",
    category: "future_category",
    effect: "future_effect",
    future_field: true,
  });

  assert.deepEqual(failure, {
    version: 7,
    code: "future.new_failure",
    category: "future_category",
    effect: "future_effect",
  });
  assert.equal(parseFailureCore({ version: 1, code: "incomplete" }), null);
  assert.equal(parseFailureCore({
    version: 1.5,
    code: "invalid.version",
    category: "validation",
    effect: "not_applied",
  }), null);
  assert.equal(parseFailureCore({
    version: Number.MAX_SAFE_INTEGER + 1,
    code: "unsafe.version",
    category: "validation",
    effect: "not_applied",
  }), null);
});

test("future FailureCore known values remain diagnostic-only", () => {
  const { ApiRequestError } = httpError;
  const { getResourceFailure, projectMutationFailure } = errorMessage;
  const rawFailure = httpResponse.parseFailureCore({
    version: 7,
    code: "auth.authorization_required",
    category: "authorization",
    effect: "not_applied",
    transport_request_id: "future-http-attempt",
  });
  assert.ok(rawFailure);
  const error = new ApiRequestError("future failure", 500, rawFailure);

  assert.equal(error.failure, rawFailure, "raw future facts stay available for diagnostics");
  assert.deepEqual(projectMutationFailure(error, "fallback"), {
    category: null,
    code: null,
    effect: "unknown",
    message: "fallback",
    transportRequestId: "future-http-attempt",
  });
  assert.deepEqual(getResourceFailure(error, "fallback"), {
    access: null,
    message: "fallback",
  });
});

test("transport messages use the caller's operation-specific copy", () => {
  const { ApiTransportError } = httpError;
  const { getErrorMessage, projectMutationFailure } = errorMessage;
  const error = new ApiTransportError(
    "连接中断，暂时无法确认操作是否已经生效",
    "network",
    "unknown",
  );

  assert.equal(getErrorMessage(error, "Task could not be saved"), "Task could not be saved");
  assert.equal(
    projectMutationFailure(error, "Task could not be saved").message,
    "Task could not be saved",
  );
});

test("ordinary exceptions never become user-facing copy", () => {
  const { getErrorMessage, projectMutationFailure } = errorMessage;
  const error = new Error("/private/path provider-secret SQL table_name");

  assert.equal(getErrorMessage(error, "The action could not be completed"), "The action could not be completed");
  assert.equal(
    projectMutationFailure(error, "The action could not be completed").message,
    "The action could not be completed",
  );
});

test("ApiRequestError remains backwards compatible and only adds optional facts", async () => {
  const { ApiRequestError, UnauthorizedError } = httpError;
  const legacy = new ApiRequestError("failed", 500);
  assert.equal(legacy.name, "ApiRequestError");
  assert.equal(legacy.message, "failed");
  assert.equal(legacy.status, 500);
  assert.equal(legacy.failure, null);
  assert.equal(legacy.transportRequestId, null);

  const structured = new ApiRequestError("conflict", 409, {
    version: 1,
    code: "workgraph.revision_conflict",
    category: "conflict",
    effect: "not_applied",
    transport_request_id: "http-attempt-2",
  });
  assert.equal(structured.status, 409);
  assert.equal(structured.transportRequestId, "http-attempt-2");
  assert.equal(structured.failure?.effect, "not_applied");

  const legacyUnauthorized = new UnauthorizedError("sign in");
  assert.equal(legacyUnauthorized.status, 401);
  assert.equal(legacyUnauthorized.failure, null);
  assert.equal(legacyUnauthorized.transportRequestId, null);

  const structuredUnauthorized = new UnauthorizedError("sign in", {
    version: 1,
    code: "auth.required",
    category: "authentication",
    effect: "not_applicable",
    transport_request_id: "http-attempt-3",
  });
  assert.equal(structuredUnauthorized.status, 401);
  assert.equal(structuredUnauthorized.transportRequestId, "http-attempt-3");
});

async function loadTypeScriptModule(relativePath, transform = (source) => source) {
  return import(await transpiledTypeScriptModuleURL(relativePath, transform));
}

async function transpiledTypeScriptModuleURL(
  relativePath,
  transform = (source) => source,
) {
  const source = transform(
    await readFile(new URL(relativePath, `file://${webRoot}/`), "utf8"),
  );
  const output = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
    fileName: relativePath,
  }).outputText;
  return `data:text/javascript;base64,${Buffer.from(output).toString("base64")}`;
}
