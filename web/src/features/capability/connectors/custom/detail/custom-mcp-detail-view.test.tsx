// INPUT: 已就绪的自定义 MCP 快照与详情动作回调。
// OUTPUT: 证明对象身份、状态、连接目标和动作复用能力详情公共合同并正确派发。
// POS: 自定义 MCP 详情 DOM 合同；请求、CRUD 互斥和重试对账由上层控制器负责。

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { CustomMCPServer } from "@/types/capability/connector";

import { CustomMCPDetailView } from "./custom-mcp-detail-view";

const SERVER = {
  args: ["server.js"],
  command: "node",
  configuration_state: "ready",
  connector_id: "custom-mcp:test-server",
  enabled: true,
  name: "test-server",
  type: "stdio",
} satisfies CustomMCPServer;

describe("CustomMCPDetailView", () => {
  it("projects ready-server identity and actions through the capability detail pattern", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    const onEdit = vi.fn();
    const onToggle = vi.fn();
    const { container } = render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <CustomMCPDetailView
          busy={false}
          catalog={null}
          failure={null}
          loading={false}
          onBack={vi.fn()}
          onDelete={onDelete}
          onEdit={onEdit}
          onRetry={vi.fn()}
          onToggle={onToggle}
          server={SERVER}
          serverLoading={false}
        />
      </I18N_CONTEXT.Provider>,
    );

    expect(container.querySelector("[data-slot='capability-detail-page']")).toBeTruthy();
    expect(container.querySelector("[data-slot='capability-detail-identity']")).toBeTruthy();
    expect(container.querySelector("[data-slot='capability-detail-identity-actions']")).toBeTruthy();
    expect(screen.getByRole("heading", { name: SERVER.name }).className)
      .toContain("ui-type-object-title");
    expect(screen.getByText("node server.js").className).toContain("ui-type-code");
    expect(screen.getByText("capability.custom_mcp_enabled")).toBeTruthy();

    fireEvent.click(screen.getByRole("switch", {
      name: "capability.custom_mcp_available_in_chat",
    }));
    await user.click(screen.getByRole("button", { name: "common.edit" }));
    await user.click(screen.getByRole("button", { name: "common.delete" }));

    expect(onToggle).toHaveBeenCalledWith(false);
    expect(onEdit).toHaveBeenCalledWith(SERVER);
    expect(onDelete).toHaveBeenCalledWith(SERVER);
  });
});
