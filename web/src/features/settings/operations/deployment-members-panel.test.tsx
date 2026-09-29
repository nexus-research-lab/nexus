// INPUT: 平台角色、部署用户 API 和未知创建结果。
// OUTPUT: 无组织创建、密码清除与按原用户名核对的回归证据。
// POS: 独立 Web 用户创建流程测试，不访问线上。
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
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
  await waitFor(() => expect((screen.getByRole("button", { name: /刷新|Refresh/ }) as HTMLButtonElement).disabled).toBe(false));
  await user.click(screen.getByText(/^(创建用户|Create user)$/ , { selector: "summary *" }));
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
  await user.click(screen.getByRole("button", { name: /^(创建用户|Create user)$/ }));
  expect(api.create).toHaveBeenCalledExactlyOnceWith({ username: "lijie", display_name: "Lijie", password: "password-123", role: "member" });
  expect(await screen.findByText("@lijie")).toBeTruthy();
  expect((screen.getByLabelText(/^(初始密码|Initial password)/) as HTMLInputElement).value).toBe("");
  expect(screen.getByText(/可访问 Web|Web enabled/)).toBeTruthy();
});

it("未知创建期间拒绝重复写入，空目录不能解除禁写，找到原用户名才恢复", async () => {
  auth.role = "admin";
  let reject!: (error: Error) => void;
  api.create.mockImplementation(() => new Promise((_, fail) => { reject = fail; }));
  render(<I18nProvider><DeploymentMembersPanel /></I18nProvider>);
  const user = await fillForm();
  expect(screen.getAllByRole("option")).toHaveLength(1);
  const submit = screen.getByRole("button", { name: /^(创建用户|Create user)$/ });
  await user.click(submit);
  await user.click(submit);
  expect(api.create).toHaveBeenCalledTimes(1);
  await act(async () => reject(new Error("connection lost")));
  expect((screen.getByLabelText(/^(初始密码|Initial password)/) as HTMLInputElement).value).toBe("");
  await user.click(screen.getByRole("button", { name: /刷新|Refresh/ }));
  expect((submit.closest("fieldset") as HTMLFieldSetElement).disabled).toBe(true);
  api.list.mockResolvedValue([member]);
  await user.click(screen.getByRole("button", { name: /刷新|Refresh/ }));
  await screen.findByText("@lijie");
  expect((submit.closest("fieldset") as HTMLFieldSetElement).disabled).toBe(false);
  expect(api.create).toHaveBeenCalledTimes(1);
});
