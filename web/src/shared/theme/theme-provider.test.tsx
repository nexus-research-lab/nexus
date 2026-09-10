// INPUT: 浏览器拒绝读写本机偏好。
// OUTPUT: 主题和正文排版仍能应用到当前文档。
// POS: 可选视觉存储失败边界回归。
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ThemeProvider } from "./theme-provider";
import { useTheme } from "./theme-context";
import { DEFAULT_CHAT_TYPOGRAPHY, useChatTypography } from "./chat-typography";
vi.mock("./theme-overlay", () => ({ ThemeOverlay: () => null }));
beforeEach(() => vi.stubGlobal("matchMedia", () => ({ matches: false })));
afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  act(() => useChatTypography.getState().setTypography(DEFAULT_CHAT_TYPOGRAPHY));
});
function ChangeTheme() {
  const { theme, setTheme } = useTheme();
  return <button onClick={() => setTheme("dark")}>{theme}</button>;
}
it("applies a live theme even when stored preferences cannot be read or written", () => {
  vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("unavailable"); });
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("unavailable"); });
  render(<ThemeProvider><ChangeTheme /></ThemeProvider>);
  fireEvent.click(screen.getByRole("button"));
  expect(document.documentElement.dataset.theme).toBe("dark");
  expect(screen.getByRole("button").textContent).toBe("dark");
});
it("applies live typography when persistence fails", () => {
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("quota"); });
  render(<ThemeProvider><ChangeTheme /></ThemeProvider>);
  act(() => useChatTypography.getState().setTypography({ fontSize: 19 }));
  expect(document.documentElement.style.getPropertyValue("--chat-font-size")).toBe("19px");
});
