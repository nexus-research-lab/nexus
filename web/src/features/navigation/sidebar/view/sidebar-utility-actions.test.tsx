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
    expect(action.className).toContain("h-8 w-8");
    expect(action.className).toContain("rounded-full");

    await user.click(action);
    expect(onCollapse).toHaveBeenCalledOnce();
    expect(onExpand).not.toHaveBeenCalled();
  });
});

vi.mock("./use-sidebar-update-version", () => ({ useSidebarUpdateVersion: () => null }));

function CurrentRoute() {
  const location = useLocation();
  return <output data-testid="route">{location.pathname}{location.search}</output>;
}

it("opens personal settings from the account row and keeps logout explicit", async () => {
  const user = userEvent.setup();
  const onLogout = vi.fn();
  const onOpenGuide = vi.fn();
  render(<MemoryRouter><SidebarFooterActions
    accountName="测试用户"
    guideOpen={false}
    labels={{ collapse: "收起", expand: "展开", settings: "设置", login: "登录远程账户", logout: "退出登录", guide: "帮助" }}
    onCollapse={vi.fn()}
    onExpand={vi.fn()}
    onLogin={vi.fn()}
    onLogout={onLogout}
    onOpenGuide={onOpenGuide}
    settingsActive={false}
    showLogin={false}
    showLogout
    showPanelToggle
    showSettings
  /><CurrentRoute /></MemoryRouter>);
  expect(screen.queryByRole("menuitem", { name: "退出登录" })).toBeNull();
  await user.click(screen.getByRole("button", { name: "测试用户" }));
  expect(screen.queryByRole("menuitem", { name: "设置" })).toBeNull();
  expect(screen.getByRole("button", { name: "设置" })).toBeTruthy();
  expect(screen.getAllByRole("separator")).toHaveLength(1);
  const account = screen.getByRole("menuitem", { name: "测试用户" });
  expect(account.className).toBe(screen.getByRole("menuitem", { name: "帮助" }).className);
  await user.click(account);
  expect(screen.getByTestId("route").textContent).toBe("/settings?section=personal");
  expect(screen.queryByRole("menu")).toBeNull();
  await user.click(screen.getByRole("button", { name: "测试用户" }));
  expect(onLogout).not.toHaveBeenCalled();
  await user.click(screen.getByRole("menuitem", { name: "退出登录" }));
  expect(onLogout).toHaveBeenCalledOnce();
  expect(screen.queryByRole("menu")).toBeNull();
  await user.click(screen.getByRole("button", { name: "测试用户" }));
  await user.click(screen.getByRole("menuitem", { name: "帮助" }));
  expect(onOpenGuide).toHaveBeenCalledOnce();
});

function CurrentLocation() {
  return <output data-testid="location">{useLocation().pathname}</output>;
}

it("opens settings directly without a local account bar and respects action visibility", async () => {
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
  expect(screen.queryByText("Local User")).toBeNull();
  const settings = screen.getByRole("button", { name: "设置" });
  expect(settings.getAttribute("aria-pressed")).toBe("true");
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

it("exposes optional remote sign-in from the Desktop account menu", async () => {
  const user = userEvent.setup();
  const onLogin = vi.fn();
  render(<MemoryRouter><SidebarFooterActions
    accountName="本地用户"
    guideOpen={false}
    labels={{ collapse: "收起", expand: "展开", settings: "设置", login: "登录远程账户", logout: "退出登录", guide: "帮助" }}
    onCollapse={vi.fn()}
    onExpand={vi.fn()}
    onLogin={onLogin}
    onLogout={vi.fn()}
    onOpenGuide={vi.fn()}
    settingsActive={false}
    showLogin
    showLogout={false}
    showPanelToggle
    showSettings
  /></MemoryRouter>);

  await user.click(screen.getByRole("button", { name: "本地用户" }));
  await user.click(screen.getByRole("menuitem", { name: "登录远程账户" }));
  expect(onLogin).toHaveBeenCalledOnce();
});
