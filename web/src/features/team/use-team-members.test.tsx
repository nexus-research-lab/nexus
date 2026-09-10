// INPUT: 当前远程账号与 Control Deployment 真人目录。
// OUTPUT: 只保留可邀请的其他 Team 成员。
// POS: 在线建群成员目录隔离回归。
import { renderHook, waitFor } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { expect, it, vi } from "vitest";

import { AUTH_CONTEXT } from "@/shared/auth/auth-context";

import { useTeamMembers } from "./use-team-members";

const { listMock } = vi.hoisted(() => ({ listMock: vi.fn() }));
vi.mock("@/lib/api/account/control-api", () => ({
  listControlMemberDirectoryApi: listMock,
}));

it("excludes the current user from online Room invite choices", async () => {
  listMock.mockResolvedValue([
    { user_id: "self", username: "self", display_name: "Self" },
    { user_id: "other", username: "other", display_name: "Other" },
  ]);
  const wrapper = ({ children }: PropsWithChildren) => (
    <AUTH_CONTEXT.Provider value={{
      error: null, isBootstrapped: true, loading: false,
      login: vi.fn(), logout: vi.fn(), refreshStatus: vi.fn(),
      status: { auth_method: "password", auth_required: true, authenticated: true, password_login_enabled: true, user_id: "self", username: "self" },
    }}>
      {children}
    </AUTH_CONTEXT.Provider>
  );

  const { result } = renderHook(() => useTeamMembers(true), { wrapper });
  await waitFor(() => expect(result.current.map((member) => member.user_id)).toEqual(["other"]));
});
