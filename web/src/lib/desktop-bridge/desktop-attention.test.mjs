// @vitest-environment node

import assert from "node:assert/strict";

import { test } from "vitest";
import { fileURLToPath } from "node:url";

// 多会话共享提醒，不能让一个会话的清理取消另一个会话仍在等待的请求。
test("桌面待确认提醒聚合来源，清空后取消，Web 模式安全跳过", async () => {
  const root = fileURLToPath(new URL("../../../", import.meta.url));

  const previousWindow = globalThis.window;
  try {
    const { setDesktopAttention } = await import("@/lib/desktop-bridge/desktop-bridge.ts");
    const requests = [];
    globalThis.window = { __NEXUS_DESKTOP_BRIDGE__: { invoke: async (request) => {
      requests.push(request);
      return { updated: true };
    } } };
    const first = Symbol("first");
    const second = Symbol("second");
    setDesktopAttention(first, 2);
    setDesktopAttention(second, 3);
    setDesktopAttention(first, 0);
    setDesktopAttention(second, 0);
    assert.deepEqual(requests.map((request) => request.payload.count), [2, 5, 3, 0]);
    assert.ok(requests.every((request) => request.schema_version === 1 && request.kind === "app.set_attention"));
    globalThis.window = {};
    setDesktopAttention(first, 2);
    setDesktopAttention(first, 0);
    assert.equal(requests.length, 4);
  } finally {
    if (previousWindow === undefined) delete globalThis.window;
    else globalThis.window = previousWindow;

  }
});
