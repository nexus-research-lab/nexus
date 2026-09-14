// INPUT: 根视图渲染失败；没有任何应用 Provider。
// OUTPUT: 仍可显示安全恢复界面，并将原始错误仅送往诊断。
// POS: Bootstrap 的最后恢复边界回归。
import { render, screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { RootErrorBoundary, RootFailureScreen } from "./root-failure-view";

beforeEach(() => { localStorage.setItem("nexus-locale", "zh"); });

const diagnostics = vi.hoisted(() => ({ fatal: vi.fn(), recover: vi.fn() }));
vi.mock("@/config/desktop-runtime", () => ({ notifyDesktopWebFatal: diagnostics.fatal }));
vi.mock("./recovery/chunk-error-recovery", () => ({ recoverFromChunkLoadError: diagnostics.recover }));

it("renders its recovery action without an application provider", () => {
  render(<RootFailureScreen title="启动失败" message="请刷新页面" />);
  expect(screen.getByRole("heading", { name: "启动失败" })).toBeTruthy();
  expect(screen.getByRole("button", { name: "重试" }).getAttribute("type")).toBe("button");
  expect(screen.getByText("请刷新页面")).toBeTruthy();
});

it("contains a render failure and keeps diagnostic details out of the screen", () => {
  const error = new Error("private diagnostic detail");
  const log = vi.spyOn(console, "error").mockImplementation(() => {});
  function Broken(): never { throw error; }
  try {
    render(<RootErrorBoundary><Broken /></RootErrorBoundary>);
    expect(screen.getByRole("heading", { name: "页面暂时无法显示" })).toBeTruthy();
    expect(screen.queryByText(error.message)).toBeNull();
    expect(diagnostics.fatal).toHaveBeenCalledWith("react.render", error, expect.any(Object));
    expect(diagnostics.recover).toHaveBeenCalledWith("react.render", error);
  } finally {
    log.mockRestore();
  }
});

it("recovers in English even when preference storage cannot be read", () => {
  const language = vi.spyOn(window.navigator, "language", "get").mockReturnValue("en-US");
  const storage = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("blocked"); });
  try {
    render(<RootFailureScreen title="Recovery" message="Reload" />);
    expect(screen.getByRole("button", { name: "Retry" })).toBeTruthy();
  } finally { storage.mockRestore(); language.mockRestore(); }
});
