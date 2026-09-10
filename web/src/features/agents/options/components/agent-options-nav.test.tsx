// INPUT: Agent Options 当前栏目与栏目切换命令。
// OUTPUT: 证明导航复用共享 Button、只标记当前页并正确转发选择。
// POS: Agent Options 导航 DOM 行为测试；响应式几何由组件 class 合同负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { AgentOptionsNav } from "./agent-options-nav";

describe("AgentOptionsNav", () => {
  it("uses shared navigation buttons and changes the selected section", async () => {
    const user = userEvent.setup();
    const onTabChange = vi.fn();
    render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <AgentOptionsNav
          activeTab="identity"
          onTabChange={onTabChange}
        />
      </I18N_CONTEXT.Provider>,
    );

    const identity = screen.getByRole("button", { name: "agent_options.nav.identity" });
    const tools = screen.getByRole("button", { name: "agent_options.nav.tools" });
    expect(identity.getAttribute("aria-current")).toBe("page");
    expect(tools.hasAttribute("aria-current")).toBe(false);
    expect(identity.className).toContain("ui-type-control");
    expect(identity.className).toContain("aria-[current=page]");

    await user.click(tools);
    expect(onTabChange).toHaveBeenCalledWith("advanced");
  });
});
