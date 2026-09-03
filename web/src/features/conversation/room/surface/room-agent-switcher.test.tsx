// INPUT: 两个 Room Agent、当前选择与切换命令。
// OUTPUT: 证明成员切换器复用共享 Button/Menu 状态并提交精确 Agent 身份。
// POS: RoomAgentSwitcher DOM 行为测试；成员权限和进程归属由上层负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { RoomAgentSwitcher } from "@/features/conversation/room/surface/room-agent-switcher";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { Agent } from "@/types/agent/agent";

const MEMBERS: Agent[] = [
  {
    agent_id: "alpha",
    created_at: 1,
    name: "Alpha",
    options: {},
    status: "idle",
    workspace_path: "/workspace/alpha",
  },
  {
    agent_id: "beta",
    created_at: 2,
    name: "Beta",
    options: {},
    status: "idle",
    workspace_path: "/workspace/beta",
  },
];

describe("RoomAgentSwitcher", () => {
  it("uses the shared trigger state and selects from the shared action menu", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <I18nProvider>
        <RoomAgentSwitcher
          members={MEMBERS}
          onSelect={onSelect}
          selectedId="alpha"
        />
      </I18nProvider>,
    );

    const trigger = screen.getByRole("button", { name: /Alpha/ });
    expect(trigger.className).toContain("min-h-7");
    expect(trigger.className).toContain("border-transparent");
    expect(trigger.getAttribute("aria-expanded")).toBe("false");

    await user.click(trigger);
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
    await user.click(screen.getByRole("menuitem", { name: /Beta/ }));
    expect(onSelect).toHaveBeenCalledWith("beta");
    expect(screen.queryByRole("menu")).toBeNull();
  });
});
