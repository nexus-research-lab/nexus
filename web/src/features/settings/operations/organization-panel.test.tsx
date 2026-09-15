// INPUT: 当前组织身份、成员与邀请 API 结果。
// OUTPUT: 统一组织页展示成员和角色，并通过短弹窗创建一次性邀请链接。
// POS: 组织治理页的信息架构与邀请主流程回归。
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { OrganizationPanel } from "./organization-panel";

const auth = vi.hoisted(() => ({ organizationRole: "owner" as string | undefined }));
const api = vi.hoisted(() => ({
  create: vi.fn(),
  list: vi.fn(),
  listMembers: vi.fn(),
  revoke: vi.fn(),
  remove: vi.fn(),
  updateMember: vi.fn(),
}));

vi.mock("@/lib/api/account/control-api", () => ({
  createControlOrganizationInvitationApi: api.create,
  listControlMembersApi: api.listMembers,
  listControlOrganizationInvitationsApi: api.list,
  revokeControlOrganizationInvitationApi: api.revoke,
  deleteControlOrganizationInvitationApi: api.remove,
  updateControlMemberApi: api.updateMember,
}));
vi.mock("@/shared/auth/auth-context", async (importOriginal) => ({
  ...await importOriginal<typeof import("@/shared/auth/auth-context")>(),
  useAuth: () => ({ status: { authenticated: true, auth_method: "password", organization_id: "org-1", organization_role: auth.organizationRole, organization_name: "Nexus Research", role: "member", user_id: "self" } }),
}));

beforeEach(() => {
  vi.resetAllMocks();
  auth.organizationRole = "owner";
  api.list.mockResolvedValue([]);
  api.listMembers.mockResolvedValue([{
    user_id: "self",
    username: "owner",
    display_name: "Organization Owner",
    role: "owner",
    membership_status: "active",
  }]);
});
afterEach(cleanup);

it.each(["owner", "admin", "member", undefined])("组织动作依据组织角色 %s，缺失时不冒充普通成员", async (role) => {
  auth.organizationRole = role;
  render(<I18nProvider><OrganizationPanel /></I18nProvider>);
  await screen.findByText("Organization Owner");
  expect(Boolean(screen.queryByRole("button", { name: /解散组织|Dissolve organization/ }))).toBe(role === "owner");
  expect(Boolean(screen.queryByRole("button", { name: /退出组织|Leave organization/ }))).toBe(role === "admin" || role === "member");
  expect(screen.queryByText(/^(操作|Actions)$/)).toBeNull();
  if (!role) expect(screen.getByText(/未获取到组织角色|Your organization role is unavailable/)).toBeTruthy();
});

it("终态邀请删除成功后刷新记录，失败时保留记录和错误反馈", async () => {
  const user = userEvent.setup();
  api.list.mockResolvedValue([{
    invitation_id: "expired", role: "member", expires_at: "2020-01-01T00:00:00Z", created_at: "2019-01-01T00:00:00Z",
  }]);
  render(<I18nProvider><OrganizationPanel /></I18nProvider>);
  await screen.findByText("Organization Owner");
  await user.click(screen.getByRole("button", { name: /邀请记录|Invitation history/ }));
  api.remove.mockRejectedValueOnce(new Error("offline"));
  await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: /删除记录|Delete record/ }));
  expect(api.remove).toHaveBeenCalledWith("expired");
  expect(within(screen.getByRole("dialog")).getByRole("button", { name: /删除记录|Delete record/ })).toBeTruthy();
  api.remove.mockResolvedValueOnce(undefined);
  api.list.mockResolvedValue([]);
  await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: /删除记录|Delete record/ }));
  expect(await screen.findByText(/还没有邀请记录|No invitations yet/)).toBeTruthy();
  expect(api.revoke).not.toHaveBeenCalled();
});

it("组织页保留成员与邀请入口，不展示角色说明侧栏，邀请链接只在弹窗内显示", async () => {
  const user = userEvent.setup();
  const joinURL = `https://app.nexusos.cn/join/${"a".repeat(64)}`;
  api.create.mockResolvedValue({
    invitation_id: "invite-1",
    organization_id: "org-1",
    organization_name: "Nexus Research",
    role: "member",
    created_by_user_id: "self",
    expires_at: "2099-01-01T00:00:00Z",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    token: "a".repeat(64),
    join_url: joinURL,
  });

  render(<I18nProvider><OrganizationPanel /></I18nProvider>);
  expect(screen.getByText("Nexus Research")).toBeTruthy();
  expect(await screen.findByText("Organization Owner")).toBeTruthy();
  expect(screen.queryByRole("complementary")).toBeNull();
  expect(screen.queryByRole("button", { name: /更多操作|More actions/ })).toBeNull();
  expect(screen.queryByRole("button", { name: /Organization Owner/ })).toBeNull();
  expect(screen.queryByText(/角色权限|Role permissions/)).toBeNull();
  expect(screen.queryByRole("dialog")).toBeNull();
  await user.click(screen.getByRole("button", { name: /邀请记录|Invitation history/ }));
  expect(screen.getByRole("dialog")).toBeTruthy();
  expect(await screen.findByText(/还没有邀请记录|No invitations yet/)).toBeTruthy();
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(screen.queryByDisplayValue(joinURL)).toBeNull();

  await user.click(screen.getByRole("button", { name: /邀请成员|Invite member/ }));
  await user.click(screen.getByRole("button", { name: /创建邀请链接|Create invite link/ }));

  expect(await screen.findByDisplayValue(joinURL)).toBeTruthy();
  expect(api.create).toHaveBeenCalledWith("member");
});
