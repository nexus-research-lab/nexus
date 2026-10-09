// INPUT: 平台角色、部署用户 API 和未知创建结果。
// OUTPUT: 无组织创建、密码清除与按原用户名核对的回归证据。
// POS: 独立 Web 用户创建流程测试，不访问线上。
import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ApiRequestError } from "@/lib/api/core/http-error";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { DeploymentMembersPanel } from "./deployment-members-panel";

const api = vi.hoisted(() => ({ list: vi.fn(), create: vi.fn() }));
const auth = vi.hoisted(() => ({ role: "owner" }));
vi.mock("@/lib/api/account/control-api", () => ({
  listControlDeploymentMembersApi: api.list,
  createControlDeploymentMemberApi: api.create,
}));
vi.mock("@/shared/auth/auth-context", () => ({ useAuth: () => ({ status: auth }) }));
const member = { user_id: "new-user", username: "lijie", display_name: "Lijie", role: "member", membership_status: "active", web_access_disabled: false };
beforeEach(() => { vi.resetAllMocks(); auth.role = "owner"; api.list.mockResolvedValue([]); });
afterEach(cleanup);

async function fillForm() {
  const user = userEvent.setup();
  await waitFor(() => expect((screen.getByRole("button", { name: /^(新建用户|Create user)$/ }) as HTMLButtonElement).disabled).toBe(false));
  await user.click(screen.getByRole("button", { name: /^(新建用户|Create user)$/ }));
  await user.type(screen.getByLabelText(/^(用户名|Username)/), "lijie");
  await user.type(screen.getByLabelText(/^(显示名称|Display name)$/), "Lijie");
  await user.type(screen.getByLabelText(/^(初始密码|Initial password)/), "password-123");
  await user.type(screen.getByLabelText(/^(确认密码|Confirm password)/), "password-123");
  return user;
}

it("无组织的平台 owner 可创建 Web 用户，提交不包含组织且结束即清除密码", async () => {
  api.create.mockResolvedValue(member);
  render(<I18nProvider><DeploymentMembersPanel /></I18nProvider>);
  const user = await fillForm();
  await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: /^(新建用户|Create user)$/ }));
  expect(api.create).toHaveBeenCalledExactlyOnceWith({ username: "lijie", display_name: "Lijie", password: "password-123", role: "member" });
  expect(await screen.findByText("@lijie")).toBeTruthy();
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(screen.getByText(/可访问 Web|Web enabled/)).toBeTruthy();
  await user.click(screen.getByRole("button", { name: /^(新建用户|Create user)$/ }));
  expect((screen.getByLabelText(/^(初始密码|Initial password)/) as HTMLInputElement).value).toBe("");
});

it("未知创建期间拒绝重复写入，空目录不能解除禁写，找到原用户名才恢复", async () => {
  auth.role = "admin";
  let reject!: (error: Error) => void;
  api.create.mockImplementation(() => new Promise((_, fail) => { reject = fail; }));
  render(<I18nProvider><DeploymentMembersPanel /></I18nProvider>);
  const user = await fillForm();
  expect(screen.getAllByRole("option")).toHaveLength(1);
  const submit = within(screen.getByRole("dialog")).getByRole("button", { name: /^(新建用户|Create user)$/ });
  await user.click(submit);
  await user.click(submit);
  expect(api.create).toHaveBeenCalledTimes(1);
  await user.keyboard("{Escape}");
  expect(screen.getByRole("dialog")).toBeTruthy();
  expect((screen.getByRole("button", { name: /取消|Cancel/ }) as HTMLButtonElement).disabled).toBe(true);
  await act(async () => reject(new Error("connection lost")));
  expect(screen.queryByRole("dialog")).toBeNull();
  const trigger = screen.getByRole("button", { name: /^(新建用户|Create user)$/ }) as HTMLButtonElement;
  await user.click(screen.getByRole("button", { name: /刷新|Refresh/ }));
  expect(trigger.disabled).toBe(true);
  api.list.mockRejectedValueOnce(new Error("offline"));
  await user.click(screen.getByRole("button", { name: /刷新|Refresh/ }));
  expect(screen.getAllByRole("button", { name: /刷新|Refresh/ })).toHaveLength(1);
  expect(trigger.disabled).toBe(true);
  api.list.mockResolvedValue([member]);
  await user.click(screen.getByRole("button", { name: /刷新|Refresh/ }));
  await screen.findByText("@lijie");
  expect(trigger.disabled).toBe(false);
  expect(api.create).toHaveBeenCalledTimes(1);
});


it("取消丢弃密码与草稿；密码不一致定位字段，明确拒绝保留账号信息并清空密码", async () => {
  api.create.mockRejectedValue(new ApiRequestError("username conflict", 409));
  render(<I18nProvider><DeploymentMembersPanel /></I18nProvider>);
  const user = await fillForm();
  await user.click(screen.getByRole("button", { name: /取消|Cancel/ }));
  expect(screen.queryByRole("dialog")).toBeNull();
  await user.click(screen.getByRole("button", { name: /^(新建用户|Create user)$/ }));
  expect((screen.getByLabelText(/^(初始密码|Initial password)/) as HTMLInputElement).value).toBe("");
  expect((screen.getByLabelText(/^(用户名|Username)/) as HTMLInputElement).value).toBe("");
  await user.type(screen.getByLabelText(/^(用户名|Username)/), "lijie");
  await user.type(screen.getByLabelText(/^(初始密码|Initial password)/), "password-123");
  const confirm = screen.getByLabelText(/^(确认密码|Confirm password)/);
  await user.type(confirm, "password-wrong");
  const submit = within(screen.getByRole("dialog")).getByRole("button", { name: /^(新建用户|Create user)$/ });
  await user.click(submit);
  expect(api.create).not.toHaveBeenCalled();
  expect(document.activeElement).toBe(confirm);
  expect(confirm.getAttribute("aria-invalid")).toBe("true");
  await user.clear(confirm);
  await user.type(confirm, "password-123");
  await user.click(submit);
  expect(await screen.findByRole("alert")).toBeTruthy();
  expect(screen.getByRole("dialog")).toBeTruthy();
  expect((screen.getByLabelText(/^(用户名|Username)/) as HTMLInputElement).value).toBe("lijie");
  expect((screen.getByLabelText(/^(初始密码|Initial password)/) as HTMLInputElement).value).toBe("");
  expect((confirm as HTMLInputElement).value).toBe("");
});

it("首次读取失败提供恢复入口，不显示空目录或允许创建", async () => {
  api.list.mockRejectedValueOnce(new Error("offline"));
  render(<I18nProvider><DeploymentMembersPanel /></I18nProvider>);
  const refresh = await screen.findByRole("button", { name: /刷新|Refresh/ });
  expect(screen.queryByText(/暂无部署用户|No deployment users yet/)).toBeNull();
  const create = screen.getByRole("button", { name: /^(新建用户|Create user)$/ }) as HTMLButtonElement;
  expect(create.disabled).toBe(true);
  await userEvent.click(refresh);
  await waitFor(() => expect(create.disabled).toBe(false));
});
