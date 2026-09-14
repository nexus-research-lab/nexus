// INPUT: Agent 高级权限模式和 Connector 资源状态。
// OUTPUT: 证明高危模式警告与加载态复用共享反馈和 Spinner 所有者。
// POS: Agent Options 高级页 DOM 合同；草稿持久化由 editor controller 测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router-dom";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { ConnectorInfo } from "@/types/capability/connector";

import { AgentOptionsAdvancedTab } from "./agent-options-advanced-tab";

function renderAdvancedTab({
  connectorsLoading = false,
  connectors = [] as ConnectorInfo[],
  connectorIds = [] as string[],
  onToggleConnector = vi.fn(),
  onPermissionModeChange = vi.fn(),
  permissionMode = "bypassPermissions",
} = {}) {
  return {
    ...render(
    <I18N_CONTEXT.Provider
      value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
    >
      <MemoryRouter>
        <AgentOptionsAdvancedTab
          connectorIds={connectorIds}
          connectors={connectors}
          connectorsError={null}
          connectorsLoading={connectorsLoading}
          onPermissionModeChange={onPermissionModeChange}
          onRetryConnectors={vi.fn()}
          onToggleConnector={onToggleConnector}
          permissionMode={permissionMode}
        />
      </MemoryRouter>
    </I18N_CONTEXT.Provider>,
    ),
    onPermissionModeChange,
    onToggleConnector,
  };
}

describe("AgentOptionsAdvancedTab", () => {
  it("does not expose tool preauthorization and keeps Connector switches independent", async () => {
    const user = userEvent.setup();
    const connector = (id: string, connection_state: ConnectorInfo["connection_state"]): ConnectorInfo => ({
      connector_id: id, name: id, title: id, connection_state,
      auth_type: "oauth2", category: "productivity", description: "Connector purpose", icon: "github",
      is_configured: true, kind: "connector", status: "available",
    });
    const { onToggleConnector } = renderAdvancedTab({
      connectorIds: ["Existing disconnected"],
      connectors: [connector("Connected", "connected"), connector("Unavailable", "disconnected"), connector("Existing disconnected", "disconnected")],
    });
    expect(screen.queryByRole("switch", { name: "Bash" })).toBeNull();
    await user.click(screen.getByText("Connected"));
    expect(onToggleConnector).not.toHaveBeenCalled();
    await user.click(screen.getByRole("switch", { name: "Connected" }));
    expect(onToggleConnector).toHaveBeenLastCalledWith("Connected");
    const unavailable = screen.getByRole("switch", { name: "Unavailable" }) as HTMLButtonElement;
    expect(unavailable.disabled).toBe(true);
    await user.click(unavailable);
    expect(onToggleConnector).toHaveBeenCalledTimes(1);
    const existing = screen.getByRole("switch", { name: "Existing disconnected" }) as HTMLButtonElement;
    expect(existing.disabled).toBe(false);
    await user.click(existing);
    expect(onToggleConnector).toHaveBeenLastCalledWith("Existing disconnected");
  });

  it("uses the shared warning notice for bypass permissions", async () => {
    const user = userEvent.setup();
    renderAdvancedTab();
    const summary = screen.getByText("agent_options.advanced.permission_settings");
    const disclosure = summary.closest("details") as HTMLDetailsElement;
    expect(disclosure.open).toBe(false);
    await user.click(summary);
    expect(disclosure.open).toBe(true);

    const notice = screen.getByRole("status");
    expect(notice.getAttribute("data-inline-notice-tone")).toBe("warning");
    expect(notice.textContent).toContain("agent_options.advanced.bypass_warning");
  });

  it("uses the reduced-motion shared spinner recipe for Connector loading", () => {
    const { container } = renderAdvancedTab({ connectorsLoading: true });

    const spinner = container.querySelector("svg.animate-spin");
    expect(spinner).not.toBeNull();
    expect(spinner?.getAttribute("class")).toContain("motion-reduce:animate-none");
  });

  it("projects permission modes as shared neutral choices", async () => {
    const user = userEvent.setup();
    const { onPermissionModeChange } = renderAdvancedTab({
      permissionMode: "default",
    });
    await user.click(screen.getByText("agent_options.advanced.permission_settings"));

    const defaultMode = screen.getByRole("button", {
      name: /agent_options\.advanced\.permission\.default\.label/,
    });
    const bypassMode = screen.getByRole("button", {
      name: /agent_options\.advanced\.permission\.bypass\.label/,
    });
    expect(defaultMode.getAttribute("aria-pressed")).toBe("true");
    expect(defaultMode.className).toContain("bg-(--surface-interactive-active-background)");
    expect(defaultMode.className).not.toContain("shadow-");
    for (const control of document.querySelectorAll<HTMLButtonElement>("[data-agent-permission-mode]")) {
      expect(document.getElementById(control.getAttribute("aria-describedby")!)).not.toBeNull();
    }
    await user.click(bypassMode);
    expect(onPermissionModeChange).toHaveBeenCalledWith("bypassPermissions");
  });
});
