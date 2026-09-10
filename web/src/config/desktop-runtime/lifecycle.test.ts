// INPUT: 桌面宿主桥、关联 ID 与可恢复异常。
// OUTPUT: 诊断写入独立事件，桥失效不会再次抛出异常。
// POS: 桌面诊断消息边界的行为回归。
import { afterEach, expect, it, vi } from "vitest";
import { notifyDesktopDiagnostic } from "./lifecycle";

afterEach(() => {
  delete window.__NEXUS_DESKTOP_RUNTIME__;
  delete window.webkit;
});

it("reports bounded recoverable diagnostics and tolerates a failed host bridge", () => {
  const postMessage = vi.fn();
  window.webkit = { messageHandlers: { nexusDesktopLifecycle: { postMessage } } };
  const details = { client_request_id: "req-1", duration_ms: 10000 };
  notifyDesktopDiagnostic("request_ack.timeout", details);
  expect(postMessage).not.toHaveBeenCalled();

  window.__NEXUS_DESKTOP_RUNTIME__ = { app_mode: "desktop" };
  const error = new Error("x".repeat(5000));
  notifyDesktopDiagnostic("request_ack.failed", details, error);
  expect(postMessage).toHaveBeenCalledWith(expect.objectContaining({
    kind: "web.diagnostic", source: "request_ack.failed",
    context: JSON.stringify(details), name: "Error",
  }));
  expect(postMessage.mock.calls[0][0].message.length).toBeLessThan(4200);
  postMessage.mockImplementation(() => { throw new Error("bridge closed"); });
  expect(() => notifyDesktopDiagnostic("request_ack.failed", details, error)).not.toThrow();
});
