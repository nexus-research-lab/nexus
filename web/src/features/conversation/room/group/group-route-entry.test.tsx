// INPUT: Room 降级导航面与用户对返回、联系人和交接动作的键盘选择。
// OUTPUT: 证明共享目录主动作仍进入原来的目标路由。
// POS: Room 降级页 DOM 回归；Room 历史筛选仍由原模型合同覆盖。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { GroupRouteEntry } from "./group-route-entry";

function Location() {
  return <output aria-label="Current route">{useLocation().pathname}</output>;
}

describe("GroupRouteEntry", () => {
  it.each([
    ["room.route_back_launcher", "/launcher", 1],
    ["room.route_browse_agents", "/contacts", 2],
    ["room.route_handoff", "/launcher", 3],
  ] as const)("keeps %s keyboard navigation on a shared catalog action", async (label, route, tabCount) => {
    const user = userEvent.setup();
    render(<MemoryRouter initialEntries={["/rooms/unavailable"]}>
      <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>
        <GroupRouteEntry agents={[]} conversations={[]} roomId="unavailable" />
        <Location />
      </I18N_CONTEXT.Provider>
    </MemoryRouter>);
    for (let index = 0; index < tabCount; index += 1) await user.tab();
    const action = screen.getByRole("button", { name: label });
    expect(document.activeElement).toBe(action);
    expect(action.getAttribute("data-slot")).toBe("catalog-primary-action");
    await user.keyboard("{Enter}");
    expect(screen.getByLabelText("Current route").textContent).toBe(route);
  });
});
