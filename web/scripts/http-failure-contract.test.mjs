import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { fileURLToPath } from "node:url";

import ts from "typescript";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const httpResponse = await loadTypeScriptModule("src/lib/api/core/http-response.ts");
const httpError = await loadTypeScriptModule("src/lib/api/core/http-error.ts");

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

test("FailureCore exposes a distinct transport diagnostic identity", async () => {
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
        resolution: {
          actor: "user",
          action: "workgraph.refresh_editor",
        },
      },
    },
  };

  const failure = getApiFailure(payload);
  assert.equal(failure?.transport_request_id, "http-attempt-1");
  assert.equal(failure?.resolution?.action, "workgraph.refresh_editor");
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
    resolution: {
      actor: "future_actor",
      action: "future.action",
    },
  });

  assert.deepEqual(failure, {
    version: 7,
    code: "future.new_failure",
    category: "future_category",
    effect: "future_effect",
    resolution: {
      actor: "future_actor",
      action: "future.action",
    },
  });
  assert.equal(parseFailureCore({ version: 1, code: "incomplete" }), null);
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

async function loadTypeScriptModule(relativePath) {
  const source = await readFile(new URL(relativePath, `file://${webRoot}/`), "utf8");
  const output = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
    fileName: relativePath,
  }).outputText;
  return import(`data:text/javascript;base64,${Buffer.from(output).toString("base64")}`);
}
