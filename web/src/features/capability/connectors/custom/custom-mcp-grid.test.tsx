// INPUT: 自定义 MCP 的加载、筛选空集、配置恢复和忙碌状态。
// OUTPUT: 公共资源状态及添加/启停/编辑/删除命令保留精确目标和独立命中。
// POS: 真实目录视图回归；回调不写 API，也不绕过恢复控制器。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { LOCALE_STORAGE_KEY } from "@/shared/i18n/messages";
import type { CustomMCPServer } from "@/types/capability/connector";

import { CustomMCPGrid } from "./custom-mcp-grid";

const SERVER: CustomMCPServer = {
  configuration_state: "ready", connector_id: "custom-mcp:test", enabled: true,
  name: "Workspace tools", type: "stdio", command: "node", args: ["tools.mjs"],
};

function props(): ComponentProps<typeof CustomMCPGrid> {
  return { busy: false, hasServers: true, loading: false, onAdd: vi.fn(), onDelete: vi.fn(),
    onEdit: vi.fn(), onOpen: vi.fn(), onToggle: vi.fn(), servers: [SERVER] };
}
beforeEach(() => localStorage.setItem(LOCALE_STORAGE_KEY, "en"));

describe("Custom MCP directory", () => {
  it("distinguishes loading, empty owner catalog and filtered no-results without creating resources", async () => {
    const user = userEvent.setup();
    const actions = props();
    const { rerender } = render(<CustomMCPGrid {...actions} loading />, { wrapper: I18nProvider });
    expect(screen.getByRole("status").getAttribute("aria-busy")).toBe("true");
    expect(screen.queryByRole("button")).toBeNull();
    rerender(<CustomMCPGrid {...actions} servers={[]} hasServers={false} />);
    expect(screen.getByRole("heading", { name: "No custom MCP servers yet" })).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Add MCP" }));
    expect(actions.onAdd).toHaveBeenCalledOnce();
    rerender(<CustomMCPGrid {...actions} servers={[]} hasServers={false} busy />);
    expect((screen.getByRole("button", { name: "Add MCP" }) as HTMLButtonElement).disabled).toBe(true);
    rerender(<CustomMCPGrid {...actions} servers={[]} />);
    expect(screen.getByRole("heading", { name: "No matching MCP servers" })).toBeTruthy();
    expect(screen.queryByRole("button")).toBeNull();
    expect(actions.onAdd).toHaveBeenCalledOnce();
  });

  it("isolates switch, edit and keyboard delete from opening the server", async () => {
    const user = userEvent.setup();
    const actions = props();
    render(<CustomMCPGrid {...actions} />, { wrapper: I18nProvider });
    await user.click(screen.getByRole("switch", { name: "Available in chat" }));
    expect(actions.onToggle).toHaveBeenCalledExactlyOnceWith(SERVER, false);
    await user.click(screen.getByRole("button", { name: "Edit" }));
    expect(actions.onEdit).toHaveBeenCalledExactlyOnceWith(SERVER);
    await user.tab();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Delete" }));
    await user.keyboard("{Enter}");
    expect(actions.onDelete).toHaveBeenCalledExactlyOnceWith(SERVER);
    expect(actions.onOpen).not.toHaveBeenCalled();
    await user.click(screen.getByText(SERVER.name));
    expect(actions.onOpen).toHaveBeenCalledExactlyOnceWith(SERVER);
  });

  it("keeps details readable while busy prevents every mutation", async () => {
    const user = userEvent.setup();
    const actions = props();
    render(<CustomMCPGrid {...actions} busy />, { wrapper: I18nProvider });
    for (const control of [screen.getByRole("switch"), screen.getByRole("button", { name: "Edit" }),
      screen.getByRole("button", { name: "Delete" })]) {
      expect((control as HTMLButtonElement).disabled).toBe(true);
      await user.click(control);
    }
    expect(actions.onToggle).not.toHaveBeenCalled();
    expect(actions.onEdit).not.toHaveBeenCalled();
    expect(actions.onDelete).not.toHaveBeenCalled();
    await user.click(screen.getByText(SERVER.name));
    expect(actions.onOpen).toHaveBeenCalledExactlyOnceWith(SERVER);
  });

  it("keeps recovery disabled for chat while preserving explicit reconfiguration", async () => {
    const user = userEvent.setup();
    const actions = props();
    const recovering = { ...SERVER, configuration_state: "recovery_required" as const };
    render(<CustomMCPGrid {...actions} servers={[recovering]} />, { wrapper: I18nProvider });
    const control = screen.getByRole("switch") as HTMLButtonElement;
    expect(control.disabled).toBe(true);
    expect(control.getAttribute("aria-checked")).toBe("false");
    await user.click(control);
    expect(actions.onToggle).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Reconfigure" }));
    expect(actions.onEdit).toHaveBeenCalledExactlyOnceWith(recovering);
    expect(actions.onOpen).not.toHaveBeenCalled();
  });
});
