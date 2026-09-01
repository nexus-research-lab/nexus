import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { createServer } from "vite";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({
  configFile: false,
  logLevel: "silent",
  resolve: { alias: { "@": path.join(webRoot, "src") } },
  root: webRoot,
  server: { middlewareMode: true },
});

test.after(async () => {
  await server.close();
});

test("resource failures only project access loss from HTTP facts", async () => {
  const { ApiRequestError, UnauthorizedError } = await server.ssrLoadModule(
    "/src/lib/api/core/http-error.ts",
  );
  const { getResourceFailure } = await server.ssrLoadModule(
    "/src/lib/error-message.ts",
  );

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

  const errorSource = await read("src/lib/error-message.ts");
  const resourceStateSource = await read("src/shared/ui/display/resource-state.tsx");
  assert.doesNotMatch(errorSource, /navigator\.onLine/);
  assert.doesNotMatch(resourceStateSource, /"offline"|"permission"/);
  assert.match(resourceStateSource, /primaryAction\?: UiResourceStateAction/);
});

test("requestApi preserves structured 401 facts through the real request path", async () => {
  const { requestApi } = await server.ssrLoadModule(
    "/src/lib/api/core/http.ts",
  );
  const { UnauthorizedError } = await server.ssrLoadModule(
    "/src/lib/api/core/http-error.ts",
  );
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
  const { requestApi } = await server.ssrLoadModule(
    "/src/lib/api/core/http.ts",
  );
  const { UnauthorizedError } = await server.ssrLoadModule(
    "/src/lib/api/core/http-error.ts",
  );
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

test("future FailureCore codes cannot trigger desktop token recovery", async () => {
  const { requestApi } = await server.ssrLoadModule(
    "/src/lib/api/core/http.ts",
  );
  const { UnauthorizedError } = await server.ssrLoadModule(
    "/src/lib/api/core/http-error.ts",
  );
  await withDesktopRuntime(async ({ reloadCount }) => {
    const error = await captureError(() => withFetch(async () => (
      new Response(JSON.stringify({
        code: "401",
        message: "failed",
        success: false,
        data: {
          detail: "桌面会话 token 无效",
          failure: {
            version: 7,
            code: "auth.desktop_session_invalid",
            category: "authentication",
            effect: "not_applicable",
          },
        },
      }), {
        headers: { "Content-Type": "application/json" },
        status: 401,
        statusText: "Unauthorized",
      })
    ), () => requestApi("http://nexus.local/nexus/v1/private", {
      method: "GET",
      notify_on_401: false,
    })));

    assert.ok(error instanceof UnauthorizedError);
    assert.equal(error.failure?.version, 7);
    await new Promise((resolve) => setTimeout(resolve, 0));
    assert.equal(reloadCount(), 0);
  });
});

test("requestApi classifies fetch and response-body transport failures without replay", async () => {
  const { requestApi } = await server.ssrLoadModule(
    "/src/lib/api/core/http.ts",
  );
  const { ApiTransportError } = await server.ssrLoadModule(
    "/src/lib/api/core/http-error.ts",
  );

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
  const { requestApi } = await server.ssrLoadModule(
    "/src/lib/api/core/http.ts",
  );
  const { ApiTransportError } = await server.ssrLoadModule(
    "/src/lib/api/core/http-error.ts",
  );
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

test("mutation failure projection keeps evidence and downgrades unknown effects", async () => {
  const {
    ApiRequestError,
    ApiTransportError,
  } = await server.ssrLoadModule("/src/lib/api/core/http-error.ts");
  const {
    projectMutationFailure,
  } = await server.ssrLoadModule("/src/lib/error-message.ts");

  const rejected = projectMutationFailure(new ApiRequestError("conflict", 409, {
    version: 1,
    code: "workgraph.revision_conflict",
    category: "conflict",
    effect: "not_applied",
    transport_request_id: "mutation-attempt",
  }), "fallback");
  assert.deepEqual(rejected, {
    category: "conflict",
    code: "workgraph.revision_conflict",
    effect: "not_applied",
    message: "fallback",
    transportRequestId: "mutation-attempt",
  });
  const future = projectMutationFailure(new ApiRequestError("future", 500, {
    version: 7,
    code: "future.failure",
    category: "future_category",
    effect: "future_effect",
  }), "fallback");
  assert.equal(future.effect, "unknown");

  const interrupted = projectMutationFailure(new ApiTransportError(
    "response interrupted",
    "response_interrupted",
    "unknown",
  ), "fallback");
  assert.equal(interrupted.effect, "unknown");
  assert.equal(interrupted.category, "unavailable");
});

test("Loop picker keeps a loaded snapshot during refresh and transient failure", async () => {
  const { projectLoopPickerContentKind } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/loop-picker/loop-picker-model.ts",
  );

  assert.equal(projectLoopPickerContentKind({
    accessBlocked: false,
    error: null,
    hasSnapshot: false,
    isLoading: true,
    loopCount: 0,
  }), "loading");
  assert.equal(projectLoopPickerContentKind({
    accessBlocked: false,
    error: null,
    hasSnapshot: true,
    isLoading: true,
    loopCount: 2,
  }), "list");
  assert.equal(projectLoopPickerContentKind({
    accessBlocked: false,
    error: new Error("refresh failed"),
    hasSnapshot: true,
    isLoading: false,
    loopCount: 2,
  }), "list");
  assert.equal(projectLoopPickerContentKind({
    accessBlocked: true,
    error: new Error("forbidden"),
    hasSnapshot: true,
    isLoading: false,
    loopCount: 2,
  }), "error");
});

test("sensitive snapshots are blocked by access state and refresh stays non-destructive", async () => {
  const [
    memoryView,
    memoryDocument,
    scheduledHistory,
    loopsDirectory,
    workGraphDirectory,
    loopController,
    memoryDocumentResource,
    scheduledDialog,
  ] = await Promise.all([
    read("src/features/memory/agent-memory-view.tsx"),
    read("src/features/memory/document/memory-document-panel.tsx"),
    read("src/features/capability/scheduled/history/view/scheduled-task-run-history-content.tsx"),
    read("src/features/capability/loops/loops-directory.tsx"),
    read("src/features/capability/workgraph-distillations/workgraph-distillations-directory.tsx"),
    read("src/features/conversation/shared/composer/components/loop-picker/use-loop-picker-controller.ts"),
    read("src/features/memory/document/use-memory-document-resource.ts"),
    read("src/features/capability/scheduled/history/scheduled-task-run-history-dialog.tsx"),
  ]);

  assert.ok(
    memoryView.indexOf("memory.resource.error?.access")
      < memoryView.indexOf("memory.resource.isLoading && !memory.resource.snapshot"),
  );
  assert.ok(
    memoryDocument.indexOf("controller.resourceError?.access")
      < memoryDocument.indexOf("controller.isLoading && !controller.content"),
  );
  assert.match(scheduledHistory, /accessBlocked && failure[\s\S]*isLoading && !hasSnapshot/);
  assert.match(loopsDirectory, /loading && !hasSnapshot/);
  assert.match(workGraphDirectory, /loading && !hasSnapshot/);
  assert.match(loopController, /current\.scopeKey === locale[\s\S]*\.\.\.current/);
  assert.doesNotMatch(loopController, /setResource\(INITIAL_RESOURCE\)/);
  assert.match(memoryView, /isOpen=\{!accessBlocked && deleteTarget !== null\}/);
  assert.match(workGraphDirectory, /isOpen=\{!accessBlocked && Boolean\(deleteCandidate\)\}/);
  assert.match(workGraphDirectory, /\{!accessBlocked && editingPreview \? \(/);
  assert.match(
    scheduledDialog,
    /isOpen=\{!accessBlocked && !deletionBlocked && actions\.recoveryTarget !== null\}/,
  );
  assert.match(
    scheduledDialog,
    /if \(accessBlocked \|\| deletionBlocked\) \{[\s\S]*cancelRecovery\(\);[\s\S]*return;/,
  );
  assert.match(
    memoryDocumentResource,
    /if \(accessBlocked\) \{[\s\S]*consumedLiveVersionRef\.current[\s\S]*return;/,
  );
  assert.match(
    memoryDocumentResource,
    /current\.resourceError\?\.access[\s\S]*\? current/,
  );
});

test("uncertain mutation copy names the affected resource without enumerating outcomes", async () => {
  const [capabilityZh, capabilityEn, conversationZh, conversationEn] = await Promise.all([
    read("src/shared/i18n/catalog/zh/capability.ts"),
    read("src/shared/i18n/catalog/en/capability.ts"),
    read("src/shared/i18n/catalog/zh/conversation.ts"),
    read("src/shared/i18n/catalog/en/conversation.ts"),
  ]);
  const copy = [capabilityZh, capabilityEn, conversationZh, conversationEn].join("\n");

  assert.match(copy, /无法确认删除是否已经生效/);
  assert.match(copy, /cannot yet confirm whether deletion took effect/);
  assert.match(copy, /操作结果待核对，同一 Agent、路径和操作已暂停/);
  assert.match(copy, /operation result needs verification, and the same Agent, path, and action are paused/i);
  assert.match(copy, /消息状态未确认/);
  assert.match(copy, /message status isn't confirmed/i);
  assert.doesNotMatch(copy, /可能[^。；]*也可能|may or may not/i);
});

test("Room external tabs and round indexes retain only same-scope read snapshots", async () => {
  const [
    externalSessionsSource,
    roundIndexSource,
    conversationSessionSource,
    noticeSource,
    panelLayoutSource,
    desktopRoomSource,
    mobileRoomSource,
  ] = await Promise.all([
    read("src/pages/room/controller/model/use-room-external-sessions.ts"),
    read("src/hooks/conversation/use-session-round-index.ts"),
    read("src/features/conversation/shared/session/use-conversation-session.ts"),
    read("src/features/conversation/shared/read-resource-reliability-notice.tsx"),
    read("src/features/conversation/shared/conversation-panel-layout.tsx"),
    read("src/features/conversation/room/surface/layout/room-surface-content.tsx"),
    read("src/features/conversation/room/surface/mobile/room-mobile-surface.tsx"),
  ]);

  assert.match(externalSessionsSource, /\$\{roomId\}\\u0000\$\{agentId\}/);
  assert.match(externalSessionsSource, /if \(failure\.access\) \{[\s\S]*setExternalAgentSessions\(\[\]\)/);
  assert.match(externalSessionsSource, /requestId !== refreshRequestId/);
  assert.match(externalSessionsSource, /isExternalSessionCatalogStale/);
  assert.match(externalSessionsSource, /refreshExternalSessions/);

  assert.match(roundIndexSource, /snapshot\.scopeKey === scopeKey/);
  assert.match(roundIndexSource, /items: accessLost \? \[\] : current\.items/);
  assert.match(roundIndexSource, /hasSuccessfulSnapshot/);
  assert.match(roundIndexSource, /retry: \(\) => void/);
  assert.match(conversationSessionSource, /roundIndexResource\.error !== null/);
  assert.match(conversationSessionSource, /roundIndexResource\.items/);
  assert.match(noticeSource, /data-read-resource-state/);
  assert.match(noticeSource, /<RecoverySummary[\s\S]*impact=\{impact\}/);
  assert.match(noticeSource, /onClick=\{onRefresh\}/);
  assert.match(panelLayoutSource, /resource="session-round-index"/);
  assert.match(desktopRoomSource, /resource="room-external-sessions"/);
  assert.match(mobileRoomSource, /resource="room-external-sessions"/);
});

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}

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
