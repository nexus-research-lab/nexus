// INPUT: Agent 高级权限模式和 Connector 资源状态。
// OUTPUT: 证明高危模式警告与加载态复用共享反馈和 Spinner 所有者。
// POS: Agent Options 高级页 DOM 合同；草稿持久化由 editor controller 测试负责。

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { AgentOptionsAdvancedTab } from "./agent-options-advanced-tab";

function renderAdvancedTab({ connectorsLoading = false } = {}) {
  return render(
    <I18N_CONTEXT.Provider
      value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
    >
      <AgentOptionsAdvancedTab
        allowedTools={[]}
        connectorIds={[]}
        connectors={[]}
        connectorsError={null}
        connectorsLoading={connectorsLoading}
        onPermissionModeChange={vi.fn()}
        onRetryConnectors={vi.fn()}
        onToggleConnector={vi.fn()}
        onToggleTool={vi.fn()}
        permissionMode="bypassPermissions"
      />
    </I18N_CONTEXT.Provider>,
  );
}

describe("AgentOptionsAdvancedTab", () => {
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
});
