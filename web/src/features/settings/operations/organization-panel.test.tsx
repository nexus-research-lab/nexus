// INPUT: 当前组织身份、成员与邀请 API 结果。
// OUTPUT: 统一组织页展示成员和角色，并通过短弹窗创建一次性邀请链接。
// POS: 组织治理页的信息架构与邀请主流程回归。
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { OrganizationPanel } from "./organization-panel";

const api = vi.hoisted(() => ({
  create: vi.fn(),
  list: vi.fn(),
  listMembers: vi.fn(),
  revoke: vi.fn(),
  updateMember: vi.fn(),
}));

vi.mock("@/lib/api/account/control-api", () => ({
  createControlOrganizationInvitationApi: api.create,
  listControlMembersApi: api.listMembers,
  listControlOrganizationInvitationsApi: api.list,
  revokeControlOrganizationInvitationApi: api.revoke,
  updateControlMemberApi: api.updateMember,
}));
vi.mock("@/shared/auth/auth-context", () => ({
  useAuth: () => ({ status: { organization_name: "Nexus Research", role: "owner", user_id: "self" } }),
}));

beforeEach(() => {
  vi.resetAllMocks();
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

it("在同一组织页展示成员、角色说明与邀请入口，邀请链接只在弹窗内显示", async () => {
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
  expect(screen.getByText(/角色权限|Role permissions/)).toBeTruthy();
  const history = screen.getByText(/邀请记录|Invitation history/).closest("details")!;
  expect(history.open).toBe(false);
  await user.click(screen.getByText(/邀请记录|Invitation history/));
  expect(history.open).toBe(true);
  expect(await screen.findByText(/还没有邀请记录|No invitations yet/)).toBeTruthy();
  expect(screen.queryByDisplayValue(joinURL)).toBeNull();

  await user.click(screen.getByRole("button", { name: /邀请成员|Invite member/ }));
  await user.click(screen.getByRole("button", { name: /创建邀请链接|Create invite link/ }));

  expect(await screen.findByDisplayValue(joinURL)).toBeTruthy();
  expect(api.create).toHaveBeenCalledWith("member");
});
