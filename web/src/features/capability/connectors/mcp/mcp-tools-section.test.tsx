// INPUT: 同配置工具快照、刷新/访问失败与可用性。
// OUTPUT: 普通刷新失败保留原文，访问失效隐藏目录，重试仅由用户触发。
// POS: 固定和自定义 Connector 共用工具区的状态展示回归。
import { fireEvent, render, screen } from "@testing-library/react";
import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { MCPToolsSection } from "./mcp-tools-section";

function setup(overrides: Partial<ComponentProps<typeof MCPToolsSection>> = {}) {
  const props: ComponentProps<typeof MCPToolsSection> = {
    available: true, loading: false, onRetry: vi.fn(),
    catalog: { inspection_state: "connected", supports_tools: true, tools: [
      { name: "read_mail", title: "Read mail", description: "Original server description", arguments: [] },
    ] },
    failure: { access: null, message: "private server diagnostic" },
    ...overrides,
  };
  render(<I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
    <MCPToolsSection {...props} />
  </I18N_CONTEXT.Provider>);
  return props;
}

describe("MCPToolsSection refresh states", () => {
  it("keeps the catalog and explains its stale state without exposing diagnostics", () => {
    const props = setup();
    expect(screen.getByRole("heading", { name: "Read mail" })).toBeTruthy();
    expect(screen.getByText("Original server description")).toBeTruthy();
    expect(screen.getByText("capability.custom_mcp_tools_refresh_failed_impact")).toBeTruthy();
    expect(screen.queryByText("private server diagnostic")).toBeNull();
    expect(props.onRetry).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "state.retry" }));
    expect(props.onRetry).toHaveBeenCalledOnce();
  });

  it.each(["forbidden", "authentication_required"] as const)("hides previous tools after %s", (access) => {
    setup({ failure: { access, message: "private server diagnostic" } });
    expect(screen.queryByRole("heading", { name: "Read mail" })).toBeNull();
    expect(screen.queryByText("capability.custom_mcp_tools_refresh_failed_impact")).toBeNull();
    expect(screen.getByText("capability.custom_mcp_tools_load_failed_impact")).toBeTruthy();
  });

  it("uses the initial error state when no successful snapshot exists", () => {
    setup({ catalog: null });
    expect(screen.getByText("capability.custom_mcp_tools_load_failed_impact")).toBeTruthy();
    expect(screen.queryByText("capability.custom_mcp_tools_refresh_failed_impact")).toBeNull();
  });

  it("does not offer a stale retry action while unavailable", () => {
    setup({ available: false });
    expect(screen.queryByRole("button", { name: "state.retry" })).toBeNull();
    expect(screen.queryByRole("heading", { name: "Read mail" })).toBeNull();
    expect(screen.getByText("capability.custom_mcp_tools_disabled")).toBeTruthy();
  });

  it("prevents repeated retry while loading", () => {
    const props = setup({ loading: true });
    const retry = screen.getByRole("button", { name: "state.retry" });
    expect((retry as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(retry);
    expect(props.onRetry).not.toHaveBeenCalled();
    expect(screen.getByRole("heading", { name: "Read mail" })).toBeTruthy();
  });
});
