// INPUT: 侧栏动作标签、账号/设置权限、路由与命令。
// OUTPUT: 验证紧凑设置、帮助、账号退出和折叠动作的独立行为。
// POS: 侧栏底部动作 DOM 行为测试；路由和更新桥接由各自所有者负责。

import { MemoryRouter, useLocation } from "react-router-dom";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { AppRouteBuilders } from "@/shared/navigation/route-paths";

import { SidebarFooterActions, SidebarPanelToggleAction } from "./sidebar-utility-actions";

describe("SidebarPanelToggleAction", () => {
  it("uses the shared round icon action and keeps panel commands distinct", async () => {
    const user = userEvent.setup();
    const onCollapse = vi.fn();
    const onExpand = vi.fn();
    render(
      <SidebarPanelToggleAction
        labels={{ collapse: "收起侧栏", expand: "展开侧栏" }}
        onCollapse={onCollapse}
        onExpand={onExpand}
        showPanelToggle
        variant="panel"
      />,
    );

    const action = screen.getByRole("button", { name: "收起侧栏" });
    expect(action.getAttribute("type")).toBe("button");

    await user.click(action);
    expect(onCollapse).toHaveBeenCalledOnce();
    expect(onExpand).not.toHaveBeenCalled();
  });
});

vi.mock("./use-sidebar-update-version", () => ({ useSidebarUpdateVersion: () => null }));
vi.mock("@/shared/i18n/i18n-context", () => ({ useI18n: () => ({ t: (key: string) => key }) }));


function CurrentLocation() {
  return <output data-testid="location">{useLocation().pathname}</output>;
}

it("shows the account name and opens settings inside the account menu with action visibility", async () => {
  const user = userEvent.setup();
  const props = {
    accountName: "Local User",
    guideOpen: true,
    labels: { collapse: "收起", expand: "展开", settings: "设置", login: "登录远程账户", logout: "退出登录", guide: "帮助" },
    onCollapse: vi.fn(), onExpand: vi.fn(), onLogin: vi.fn(), onLogout: vi.fn(), onOpenGuide: vi.fn(),
    settingsActive: true, showLogin: false, showLogout: false, showPanelToggle: true, showSettings: true,
  };
  const view = render(<MemoryRouter><SidebarFooterActions {...props} /><CurrentLocation /></MemoryRouter>);
  expect(screen.getByRole("button", { name: "Local User" })).toBeTruthy();

  await user.click(screen.getByRole("button", { name: "Local User" }));
  const settings = screen.getByRole("menuitem", { name: "设置" });
  await user.click(settings);
  expect(screen.getByTestId("location").textContent).toBe(AppRouteBuilders.settings());
  expect(screen.queryByRole("menu")).toBeNull();
  await user.click(screen.getByRole("button", { name: "Local User" }));
  expect(screen.queryByRole("menuitem", { name: "退出登录" })).toBeNull();
  await user.click(screen.getByRole("menuitem", { name: "帮助" }));
  expect(props.onOpenGuide).toHaveBeenCalledOnce();
  expect(props.onLogout).not.toHaveBeenCalled();
  view.rerender(<MemoryRouter><SidebarFooterActions {...props} showSettings={false} /><CurrentLocation /></MemoryRouter>);
  expect(screen.queryByRole("button", { name: "设置" })).toBeNull();
  expect(screen.queryByRole("button", { name: "帮助" })).toBeNull();
});
