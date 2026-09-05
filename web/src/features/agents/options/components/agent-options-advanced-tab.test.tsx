// INPUT: Agent 高级权限模式和 Connector 资源状态。
// OUTPUT: 证明高危模式警告与加载态复用共享反馈和 Spinner 所有者。
// POS: Agent Options 高级页 DOM 合同；草稿持久化由 editor controller 测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { ConnectorInfo } from "@/types/capability/connector";

import { AgentOptionsAdvancedTab } from "./agent-options-advanced-tab";

function renderAdvancedTab({
  connectorsLoading = false,
  connectors = [] as ConnectorInfo[],
  connectorIds = [] as string[],
  onToggleConnector = vi.fn(),
  onToggleTool = vi.fn(),
  onPermissionModeChange = vi.fn(),
  permissionMode = "bypassPermissions",
} = {}) {
  return {
    ...render(
    <I18N_CONTEXT.Provider
      value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
    >
      <AgentOptionsAdvancedTab
        allowedTools={[]}
        connectorIds={connectorIds}
        connectors={connectors}
        connectorsError={null}
        connectorsLoading={connectorsLoading}
        onPermissionModeChange={onPermissionModeChange}
        onRetryConnectors={vi.fn()}
        onToggleConnector={onToggleConnector}
        onToggleTool={onToggleTool}
        permissionMode={permissionMode}
      />
    </I18N_CONTEXT.Provider>,
    ),
    onPermissionModeChange,
    onToggleConnector,
    onToggleTool,
  };
}

describe("AgentOptionsAdvancedTab", () => {
  it("keeps tool and Connector switches as the only authorization hit targets", async () => {
    const user = userEvent.setup();
    const connector = (id: string, connection_state: ConnectorInfo["connection_state"]): ConnectorInfo => ({
      connector_id: id, name: id, title: id, connection_state,
      auth_type: "oauth2", category: "productivity", description: "Connector purpose", icon: "github",
      is_configured: true, kind: "connector", status: "available",
    });
    const { onToggleTool, onToggleConnector } = renderAdvancedTab({
      connectorIds: ["Existing disconnected"],
      connectors: [connector("Connected", "connected"), connector("Unavailable", "disconnected"), connector("Existing disconnected", "disconnected")],
    });
    await user.click(screen.getByText("Bash"));
    await user.click(screen.getByText("Connected"));
    expect(onToggleTool).not.toHaveBeenCalled();
    expect(onToggleConnector).not.toHaveBeenCalled();
    await user.click(screen.getByRole("switch", { name: "Bash" }));
    expect(onToggleTool).toHaveBeenCalledWith("Bash", "allowed");
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

  it("uses the shared warning notice for bypass permissions", () => {
    renderAdvancedTab();

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

    const defaultMode = screen.getByRole("button", {
      name: /agent_options\.advanced\.permission\.default\.label/,
    });
    const planMode = screen.getByRole("button", {
      name: /agent_options\.advanced\.permission\.plan\.label/,
    });
    expect(defaultMode.getAttribute("aria-pressed")).toBe("true");
    expect(defaultMode.className).toContain("bg-(--surface-interactive-active-background)");
    expect(defaultMode.className).not.toContain("shadow-[");
    await user.click(planMode);
    expect(onPermissionModeChange).toHaveBeenCalledWith("plan");
  });
});
