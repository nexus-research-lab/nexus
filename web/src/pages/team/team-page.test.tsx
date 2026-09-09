// INPUT: 已建立的 Relay 在线 Room 与可提交草稿。
// OUTPUT: 证明 Team 页面复用 Room Header/Composer，并沿原 Team 动作发送。
// POS: Team 到共享 Room UI 的最小组件回归；同步协议由 features/team 测试负责。

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router-dom";

import { AUTH_CONTEXT } from "@/shared/auth/auth-context";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { TeamPage } from "./team-page";

const { sendMock, useRoomMock } = vi.hoisted(() => ({
  sendMock: vi.fn(),
  useRoomMock: vi.fn(),
}));

vi.mock("@/features/team/use-team-room", () => ({
  useTeamRoom: (roomId: string | null) => {
    useRoomMock(roomId);
    return {
    room: {
      room: {
        id: "research", team_id: "team", name: "Research", description: "", avatar: "",
        configuration_version: 1, membership_version: 1,
        created_at: "2026-09-09T00:00:00Z", updated_at: "2026-09-09T00:00:00Z",
      },
      conversation: {
        id: "conversation", room_id: "research", type: "main",
        high_water_message_seq: 0, last_activity_at: null, sync_stream_id: "stream",
        stream_epoch: "epoch", high_water_sync_event_seq: 0,
      },
      current_user_role: "owner",
    },
    error: null,
    isLoading: false,
    isSending: false,
    messages: [],
    reload: vi.fn(),
    send: sendMock,
    };
  },
}));

describe("TeamPage", () => {
  beforeEach(() => {
    sendMock.mockReset().mockResolvedValue(true);
    useRoomMock.mockReset();
  });

  it("uses the Room surface and sends through the Team controller", async () => {
    const { container } = render(
      <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>
        <AUTH_CONTEXT.Provider value={{
          error: null,
          isBootstrapped: true,
          loading: false,
          login: vi.fn(),
          logout: vi.fn(),
          refreshStatus: vi.fn(),
          status: {
            auth_required: true,
            authenticated: true,
            auth_method: "password",
            password_login_enabled: true,
            user_id: "owner",
            username: "owner",
          },
        }}>
          <MemoryRouter initialEntries={["/team?room_id=research"]}>
            <TeamPage />
          </MemoryRouter>
        </AUTH_CONTEXT.Provider>
      </I18N_CONTEXT.Provider>,
    );

    expect(container.querySelector(".workspace-surface-header")).toBeTruthy();
    expect(useRoomMock).toHaveBeenCalledWith("research");
    expect(container.querySelector(".nexus-chat-composer-shell")).toBeTruthy();
    const input = screen.getByRole("textbox", { name: "team.message" });
    fireEvent.change(input, { target: { value: "hello" } });
    fireEvent.click(screen.getByRole("button", { name: "team.send" }));

    await waitFor(() => expect(sendMock).toHaveBeenCalledExactlyOnceWith("hello"));
    await waitFor(() => expect((input as HTMLTextAreaElement).value).toBe(""));
  });
});
