// INPUT: 当前 Organization 成员读写结果与用户的刷新操作。
// OUTPUT: 并发写入受阻，未知结果在读取失败后继续禁写，手动读取可恢复。
// POS: 运营成员目录状态与恢复回归。
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { OrganizationMembersPanel } from "./control-members-panel";

const api = vi.hoisted(() => ({
  list: vi.fn(), update: vi.fn(),
}));
vi.mock("@/lib/api/account/control-api", () => ({
  listControlMembersApi: api.list,
  updateControlMemberApi: api.update,
}));
vi.mock("@/shared/auth/auth-context", () => ({ useAuth: () => ({ status: { organization_role: "owner", role: "member", user_id: "self" } }) }));
const members = ["alice", "bob"].map((username) => ({ user_id: username, username, display_name: username, role: "member", membership_status: "active" }));
beforeEach(() => {
  vi.resetAllMocks();
  api.list.mockResolvedValue(members);
});
afterEach(cleanup);

it("一次成员写入期间禁止其他行和手动刷新，失败核对前保留锁", async () => {
  const user = userEvent.setup();
  let rejectUpdate!: (reason: Error) => void;
  api.update.mockImplementation(() => new Promise((_, reject) => { rejectUpdate = reject; }));
  render(<I18nProvider><OrganizationMembersPanel /></I18nProvider>);
  await screen.findByText("alice");
  await user.click(screen.getAllByRole("button", { name: /移除:|Remove:/ })[0]);
  expect((screen.getAllByRole("button", { name: /移除:|Remove:/ })[1] as HTMLButtonElement).disabled).toBe(true);
  expect(screen.queryByRole("button", { name: /刷新|Refresh/ })).toBeNull();
  api.list.mockRejectedValueOnce(new Error("offline"));
  await act(async () => rejectUpdate(new Error("unknown")));
  await waitFor(() => expect(api.list).toHaveBeenCalledTimes(2));
  expect(screen.getAllByRole("button", { name: /移除:|Remove:/ }).every((button) => (button as HTMLButtonElement).disabled)).toBe(true);
  expect((screen.getByRole("button", { name: /刷新|Refresh/ }) as HTMLButtonElement).disabled).toBe(false);
  await user.click(screen.getByRole("button", { name: /刷新|Refresh/ }));
  await waitFor(() => expect(screen.getAllByRole("button", { name: /移除:|Remove:/ }).every((button) => !(button as HTMLButtonElement).disabled)).toBe(true));
  expect(api.update).toHaveBeenCalledTimes(1);
});

it("按姓名或用户名筛选成员并保留清晰的空结果", async () => {
  const user = userEvent.setup();
  render(<I18nProvider><OrganizationMembersPanel /></I18nProvider>);
  await screen.findByText("alice");

  await user.type(screen.getByRole("searchbox", { name: /搜索成员|Search members/ }), "bob");
  expect(screen.queryByText("alice")).toBeNull();
  expect(screen.getByText("bob")).toBeTruthy();

  await user.clear(screen.getByRole("searchbox", { name: /搜索成员|Search members/ }));
  await user.type(screen.getByRole("searchbox", { name: /搜索成员|Search members/ }), "nobody");
  expect(screen.getByText(/没有匹配的成员|No matching members/)).toBeTruthy();
});


it("直接移除组织成员，保留账号并从列表移除该行", async () => {
  api.list.mockResolvedValue([...members, {...members[0], user_id: "removed", username: "removed", display_name: "removed", membership_status: "revoked"}]);
  api.update.mockResolvedValue({...members[0], membership_status: "revoked"});
  const user = userEvent.setup();
  render(<I18nProvider><OrganizationMembersPanel /></I18nProvider>);
  await screen.findByText("alice");
  expect(screen.queryByText("removed")).toBeNull();
  expect(screen.queryByRole("button", {name: /更多操作|More actions/})).toBeNull();
  await user.click(screen.getByRole("button", {name: /移除: alice|Remove: alice/}));
  expect(api.update).toHaveBeenCalledExactlyOnceWith("alice", {status: "revoked"});
  await waitFor(() => expect(screen.queryByText("alice")).toBeNull());
  expect(screen.getByText("bob")).toBeTruthy();
});
