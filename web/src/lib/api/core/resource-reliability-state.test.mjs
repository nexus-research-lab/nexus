// @vitest-environment node

import assert from "node:assert/strict";

import { test } from "vitest";

test("resource failures only project access loss from HTTP facts", async () => {
  const { ApiRequestError, UnauthorizedError } = await import("@/lib/api/core/http-error.ts");
  const { getResourceFailure } = await import("@/lib/error-message.ts");

  assert.deepEqual(
    getResourceFailure(new Error("network unavailable"), "fallback"),
    { access: null, message: "fallback" },
  );
  assert.equal(
    getResourceFailure(new UnauthorizedError("sign in"), "fallback").access,
    "authentication_required",
  );
  assert.equal(
    getResourceFailure(new ApiRequestError("forbidden", 403), "fallback").access,
    "forbidden",
  );
  assert.equal(
    getResourceFailure(new ApiRequestError("conflict", 409), "fallback").access,
    null,
  );
  assert.equal(
    getResourceFailure(new ApiRequestError("denied", 500, {
      category: "authorization",
      code: "test.authorization",
      effect: "not_applicable",
      version: 1,
    }), "fallback").access,
    "forbidden",
  );

});

test("requestApi preserves structured 401 facts through the real request path", async () => {
  const { requestApi } = await import("@/lib/api/core/http.ts");
  const { UnauthorizedError } = await import("@/lib/api/core/http-error.ts");
  let calls = 0;
  const error = await captureError(() => withFetch(async () => {
    calls += 1;
    return new Response(JSON.stringify({
      code: "401",
      message: "failed",
      success: false,
      data: {
        detail: "未授权",
        failure: {
          version: 1,
          code: "auth.authentication_required",
          category: "authentication",
          effect: "not_applicable",
          transport_request_id: "auth-http-attempt",
        },
      },
    }), {
      headers: { "Content-Type": "application/json" },
      status: 401,
      statusText: "Unauthorized",
    });
  }, () => requestApi("/nexus/v1/private", { method: "GET" })));

  assert.ok(error instanceof UnauthorizedError);
  assert.equal(error.failure?.code, "auth.authentication_required");
  assert.equal(error.transportRequestId, "auth-http-attempt");
  assert.equal(calls, 1, "401 must not trigger an automatic retry");
});

test("desktop token recovery prefers the stable FailureCore code", async () => {
  const { requestApi } = await import("@/lib/api/core/http.ts");
  const { UnauthorizedError } = await import("@/lib/api/core/http-error.ts");
  await withDesktopRuntime(async ({ reloadCount }) => {
    let calls = 0;
    const error = await captureError(() => withFetch(async (_input, init) => {
      calls += 1;
      assert.equal(init?.headers.get("X-Nexus-Desktop-Token"), "expired-token");
      return new Response(JSON.stringify({
        code: "401",
        message: "failed",
        success: false,
        data: {
          // 401 安全文案不会包含旧版桌面 token 专用中文提示。
          detail: "未授权",
          failure: {
            version: 1,
            code: "auth.desktop_session_invalid",
            category: "authentication",
            effect: "not_applicable",
          },
        },
      }), {
        headers: { "Content-Type": "application/json" },
        status: 401,
        statusText: "Unauthorized",
      });
    }, () => requestApi("http://nexus.local/nexus/v1/private", {
      method: "GET",
    })));

    assert.ok(error instanceof UnauthorizedError);
    assert.equal(error.failure?.code, "auth.desktop_session_invalid");
    assert.equal(calls, 1);
    await new Promise((resolve) => setTimeout(resolve, 0));
    assert.equal(reloadCount(), 1);
  });
});

test("requestApi classifies fetch and response-body transport failures without replay", async () => {
  const { requestApi } = await import("@/lib/api/core/http.ts");
  const { ApiTransportError } = await import("@/lib/api/core/http-error.ts");

  for (const scenario of [
    { effect: "not_applicable", method: "GET" },
    { effect: "not_applicable", method: "HEAD" },
    { effect: "not_applicable", method: "OPTIONS" },
    { effect: "unknown", method: "POST" },
  ]) {
    let calls = 0;
    const error = await captureError(() => withFetch((_input, init) => {
      calls += 1;
      return rejectWhenAborted(init?.signal);
    }, () => requestApi("/nexus/v1/transport-timeout", {
      method: scenario.method,
      timeout_ms: 5,
    })));
    assert.ok(error instanceof ApiTransportError);
    assert.equal(error.kind, "timeout");
    assert.equal(error.effect, scenario.effect);
    if (scenario.effect === "unknown") {
      assert.match(error.message, /无法确认操作是否已经生效/);
      assert.doesNotMatch(error.message, /重试/);
    }
    assert.equal(calls, 1, `${scenario.method} timeout must not replay`);
  }

  let networkCalls = 0;
  const networkError = await captureError(() => withFetch(async () => {
    networkCalls += 1;
    throw new TypeError("socket disconnected");
  }, () => requestApi("/nexus/v1/mutation", { method: "DELETE" })));
  assert.ok(networkError instanceof ApiTransportError);
  assert.equal(networkError.kind, "network");
  assert.equal(networkError.effect, "unknown");
  assert.equal(networkCalls, 1, "network failure must not replay a mutation");

  let bodyTimeoutCalls = 0;
  const bodyTimeout = await captureError(() => withFetch(async (_input, init) => {
    bodyTimeoutCalls += 1;
    return responseWithText(
      () => rejectWhenAborted(init?.signal),
      { requestId: "body-timeout-attempt" },
    );
  }, () => requestApi("/nexus/v1/mutation", {
    method: "POST",
    timeout_ms: 5,
  })));
  assert.ok(bodyTimeout instanceof ApiTransportError);
  assert.equal(bodyTimeout.kind, "timeout");
  assert.equal(bodyTimeout.effect, "unknown");
  assert.equal(bodyTimeout.status, 200);
  assert.equal(bodyTimeout.transportRequestId, "body-timeout-attempt");
  assert.equal(bodyTimeoutCalls, 1, "body timeout must not replay a mutation");

  let bodyDisconnectCalls = 0;
  const bodyDisconnect = await captureError(() => withFetch(async () => {
    bodyDisconnectCalls += 1;
    return responseWithText(async () => {
      throw new TypeError("response terminated");
    });
  }, () => requestApi("/nexus/v1/read", { method: "GET" })));
  assert.ok(bodyDisconnect instanceof ApiTransportError);
  assert.equal(bodyDisconnect.kind, "response_interrupted");
  assert.equal(bodyDisconnect.effect, "not_applicable");
  assert.equal(bodyDisconnectCalls, 1, "body disconnect must not replay a read");
});

test("requestApi leaves an external Abort exception untouched", async () => {
  const { requestApi } = await import("@/lib/api/core/http.ts");
  const { ApiTransportError } = await import("@/lib/api/core/http-error.ts");
  const controller = new AbortController();
  const expected = new DOMException("caller canceled", "AbortError");
  controller.abort();
  let calls = 0;
  const error = await captureError(() => withFetch(async () => {
    calls += 1;
    throw expected;
  }, () => requestApi("/nexus/v1/mutation", {
    method: "POST",
    signal: controller.signal,
  })));

  assert.equal(error, expected);
  assert.equal(error.name, "AbortError");
  assert.equal(error instanceof ApiTransportError, false);
  assert.equal(calls, 1);
});

async function captureError(run) {
  try {
    await run();
  } catch (error) {
    return error;
  }
  assert.fail("expected request to reject");
}

async function withFetch(fetchImplementation, run) {
  const previousFetch = globalThis.fetch;
  globalThis.fetch = fetchImplementation;
  try {
    return await run();
  } finally {
    globalThis.fetch = previousFetch;
  }
}

function rejectWhenAborted(signal) {
  return new Promise((_resolve, reject) => {
    const rejectAbort = () => reject(
      signal?.reason ?? new DOMException("aborted", "AbortError"),
    );
    if (signal?.aborted) {
      rejectAbort();
      return;
    }
    signal?.addEventListener("abort", rejectAbort, { once: true });
  });
}

function responseWithText(text, { requestId = null } = {}) {
  return {
    headers: new Headers(requestId ? { "X-Request-ID": requestId } : undefined),
    ok: true,
    status: 200,
    statusText: "OK",
    text,
  };
}

async function withDesktopRuntime(run) {
  const previousWindow = globalThis.window;
  const previousDocument = globalThis.document;
  const values = new Map();
  let reloads = 0;
  globalThis.window = {
    __NEXUS_DESKTOP_RUNTIME__: {
      api_base_url: "http://nexus.local/nexus/v1",
      app_mode: "desktop",
      auth_token: "expired-token",
    },
    location: {
      hash: "",
      href: "http://nexus.local/app",
      pathname: "/app",
      reload: () => {
        reloads += 1;
      },
      search: "",
    },
    localStorage: {
      getItem: (key) => values.get(key) ?? null,
      setItem: (key, value) => values.set(key, String(value)),
    },
    sessionStorage: {
      getItem: (key) => values.get(key) ?? null,
      setItem: (key, value) => values.set(key, String(value)),
    },
    setTimeout,
  };
  globalThis.document = {
    body: { childElementCount: 0, textContent: "" },
    getElementById: () => null,
    readyState: "complete",
    title: "Nexus",
  };
  try {
    return await run({ reloadCount: () => reloads });
  } finally {
    if (previousWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = previousWindow;
    }
    if (previousDocument === undefined) {
      delete globalThis.document;
    } else {
      globalThis.document = previousDocument;
    }
  }
}
