// INPUT: 桌面本地免登录认证状态。
// OUTPUT: Relay Room 目录保持关闭且不发起网络请求。
// POS: 本地单人模式与远程多人能力的前端隔离回归。

import { renderHook } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { describe, expect, it, vi } from "vitest";

import { AUTH_CONTEXT } from "@/shared/auth/auth-context";

import { useTeamRooms } from "./use-team-rooms";

const { listRoomsMock } = vi.hoisted(() => ({ listRoomsMock: vi.fn() }));

vi.mock("@/lib/api/conversation/team-api", () => ({
  listTeamRooms: listRoomsMock,
}));

describe("useTeamRooms", () => {
  it("does not open Relay for a local desktop user", () => {
    const wrapper = ({ children }: PropsWithChildren) => (
      <AUTH_CONTEXT.Provider value={{
        error: null,
        isBootstrapped: true,
        loading: false,
        login: vi.fn(),
        logout: vi.fn(),
        refreshStatus: vi.fn(),
        status: {
          auth_method: "local",
          auth_required: false,
          authenticated: true,
          password_login_enabled: false,
          user_id: "__system__",
          username: "local",
        },
      }}>
        {children}
      </AUTH_CONTEXT.Provider>
    );
    const { result } = renderHook(() => useTeamRooms(), { wrapper });

    expect(result.current.isAvailable).toBe(false);
    expect(result.current.rooms).toEqual([]);
    expect(listRoomsMock).not.toHaveBeenCalled();
  });
});
