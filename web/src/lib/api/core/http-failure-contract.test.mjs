// @vitest-environment node

import assert from "node:assert/strict";

import { test } from "vitest";

const httpResponse = await import("@/lib/api/core/http-response");

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
