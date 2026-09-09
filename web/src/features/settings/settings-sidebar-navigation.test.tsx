// INPUT: 设置导航查询、账户权限与入口选择动作。
// OUTPUT: 验证筛选、空结果、清空恢复和入口点击。
// POS: 设置侧栏搜索交互回归，不请求后端。
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { MESSAGES, type TranslationKey } from "@/shared/i18n/messages";
import { OPERATIONS_SECTIONS, parseSettingsSection } from "./settings-navigation-model";
import { SettingsSidebarNavigation } from "./settings-sidebar-navigation";

const auth = vi.hoisted(() => ({ role: "member" }));
afterEach(() => { auth.role = "member"; });
vi.mock("@/hooks/settings/use-project-permissions-enabled", () => ({ useProjectPermissionsEnabled: () => true }));
const selectSection = vi.hoisted(() => vi.fn());
vi.mock("@/shared/auth/auth-context", () => ({ useAuth: () => ({ status: auth }) }));
vi.mock("@/shared/i18n/i18n-context", () => ({ useI18n: () => ({ t: (key: TranslationKey) => MESSAGES.zh[key] }) }));
vi.mock("./use-settings-navigation", () => ({ useSettingsNavigation: () => ({ activeSection: "general", backToWorkspace: vi.fn(), selectSection }) }));

it("filters names and groups, reports no results and restores navigation on clear", async () => {
  const user = userEvent.setup();
  render(<SettingsSidebarNavigation variant="panel" />);
  const search = screen.getByRole("searchbox", { name: "搜索设置…" });
  await user.type(search, "外观");
  expect(screen.queryByRole("button", { name: "常规" })).toBeNull();
  await user.click(screen.getByRole("button", { name: "外观" }));
  expect(selectSection).toHaveBeenCalledWith("appearance");
  await user.clear(search);
  await user.type(search, "偏好");
  expect(screen.getByRole("button", { name: "常规" })).toBeTruthy();
  expect(screen.queryByRole("button", { name: "个人" })).toBeNull();
  await user.type(search, "不存在");
  expect(screen.getByRole("status").textContent).toBe("未找到匹配的设置");
  await user.clear(search);
  expect(screen.getByRole("button", { name: "个人" })).toBeTruthy();
});

it("finds setting descriptions beneath their module without exposing restricted modules", async () => {
  const user = userEvent.setup();
  render(<SettingsSidebarNavigation variant="panel" />);
  const search = screen.getByRole("searchbox", { name: "搜索设置…" });
  await user.type(search, "长期");
  expect(screen.getByRole("button", { name: "常规" })).toBeTruthy();
  await user.click(screen.getByRole("button", { name: "自动记忆" }));
  expect(selectSection).toHaveBeenCalledWith("general", "settings.general.auto_memory_title");
  await user.clear(search);
  await user.type(search, "部署成员");
  expect(screen.queryByRole("button", { name: "运营" })).toBeNull();
  expect(screen.getByRole("status")).toBeTruthy();
});

it("运营分组直接导航到五个独立子页，搜索保留管理员权限过滤", async () => {
  auth.role = "admin";
  const user = userEvent.setup();
  render(<SettingsSidebarNavigation variant="panel" />);
  expect(screen.getByText("运营管理")).toBeTruthy();
  for (const item of OPERATIONS_SECTIONS) {
    await user.click(screen.getByRole("button", { name: MESSAGES.zh[item.labelKey] }));
    expect(selectSection).toHaveBeenLastCalledWith(item.key);
    expect(parseSettingsSection(new URLSearchParams({ section: item.key }))).toBe(item.key);
  }
  expect(parseSettingsSection(new URLSearchParams("section=operations"))).toBe("operations-members");
  await user.type(screen.getByRole("searchbox"), "套餐管理");
  expect(screen.getByRole("button", { name: "套餐管理" })).toBeTruthy();
  expect(screen.queryByRole("button", { name: "部署成员" })).toBeNull();
});
