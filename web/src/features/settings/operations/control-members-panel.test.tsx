// INPUT: Control 成员读写结果与用户的创建、刷新操作。
// OUTPUT: 并发写入受阻，未知结果在读取失败后继续禁写，手动读取可恢复。
// POS: 运营成员目录状态与恢复回归。
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { ControlMembersPanel } from "./control-members-panel";

const api = vi.hoisted(() => ({ list: vi.fn(), create: vi.fn(), update: vi.fn() }));
vi.mock("@/lib/api/account/control-api", () => ({
  listControlMembersApi: api.list,
  createControlMemberApi: api.create,
  updateControlMemberApi: api.update,
}));
vi.mock("@/shared/auth/auth-context", () => ({ useAuth: () => ({ status: { role: "owner", user_id: "self" } }) }));
const members = ["alice", "bob"].map((username) => ({ user_id: username, username, display_name: username, role: "member", membership_status: "active" }));
beforeEach(() => { vi.resetAllMocks(); api.list.mockResolvedValue(members); });
afterEach(cleanup);

it("一次成员写入期间禁止其他行、创建和手动刷新，失败核对前保留锁", async () => {
  const user = userEvent.setup();
  let rejectUpdate!: (reason: Error) => void;
  api.update.mockImplementation(() => new Promise((_, reject) => { rejectUpdate = reject; }));
  const { container } = render(<I18nProvider><ControlMembersPanel /></I18nProvider>);
  await screen.findByText("alice");
  await user.click(screen.getAllByRole("button", { name: /停用|Suspend/ })[0]);
  expect((screen.getByRole("button", { name: /停用|Suspend/ }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: /刷新|Refresh/ }) as HTMLButtonElement).disabled).toBe(true);
  expect((container.querySelector("input") as HTMLInputElement).disabled).toBe(true);
  api.list.mockRejectedValueOnce(new Error("offline"));
  await act(async () => rejectUpdate(new Error("unknown")));
  await waitFor(() => expect(api.list).toHaveBeenCalledTimes(2));
  expect(screen.getAllByRole("button", { name: /停用|Suspend/ }).every((button) => (button as HTMLButtonElement).disabled)).toBe(true);
  expect((screen.getByRole("button", { name: /刷新|Refresh/ }) as HTMLButtonElement).disabled).toBe(false);
  await user.click(screen.getByRole("button", { name: /刷新|Refresh/ }));
  await waitFor(() => expect(screen.getAllByRole("button", { name: /停用|Suspend/ }).every((button) => !(button as HTMLButtonElement).disabled)).toBe(true));
  expect(api.update).toHaveBeenCalledTimes(1);
});

it("重复原生表单提交只创建一次，处理期间不会丢失新编辑的草稿", async () => {
  const user = userEvent.setup();
  let finishCreate!: (value: unknown) => void;
  api.create.mockImplementation(() => new Promise((resolve) => { finishCreate = resolve; }));
  const { container } = render(<I18nProvider><ControlMembersPanel /></I18nProvider>);
  await screen.findByText("alice");
  await user.click(container.querySelector("summary")!);
  await user.type(screen.getByLabelText(/用户名|Username/), "charlie");
  await user.type(screen.getByLabelText(/初始密码|Initial password/), "password1");
  await user.type(screen.getByLabelText(/确认密码|Confirm password/), "password1");
  act(() => { fireEvent.submit(container.querySelector("form")!); fireEvent.submit(container.querySelector("form")!); });
  expect(api.create).toHaveBeenCalledTimes(1);
  expect((screen.getByLabelText(/用户名|Username/) as HTMLInputElement).disabled).toBe(true);
  await act(async () => finishCreate({ ...members[0], user_id: "charlie", username: "charlie", display_name: "charlie" }));
  expect((screen.getByLabelText(/用户名|Username/) as HTMLInputElement).value).toBe("");
});
