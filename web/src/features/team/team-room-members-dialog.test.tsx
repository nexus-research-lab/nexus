// INPUT: 不含本人的邀请目录和远程账号头像。
// OUTPUT: 群设置使用远程本人头像，不误用本地 owner 身份。
// POS: 在线群成员身份展示回归。
import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { TeamRoomMembersDialog } from "./team-room-members-dialog";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

vi.mock("@/shared/auth/auth-context", () => ({useAuth: () => ({status: {
  user_id: "local-owner", control_user_id: "remote-me", avatar: "/icon/agent/7.png",
}})}));
vi.mock("@/lib/api/account/control-api", () => ({
  listControlAgentsApi: async () => [], listControlAgentDirectoryApi: async () => [],
}));
vi.mock("./use-team-room-members", () => ({useTeamRoomMembers: () => ({
  details: {room: {name: "Example", avatar: "", coordinator_agent_id: ""}, members: [
    {member_type: "user", member_id: "remote-me", state: "active", role: "member"},
  ]}, busy: false,
})}));

it("shows my remote avatar even though the invitation directory excludes me", async () => {
  render(<I18N_CONTEXT.Provider value={{locale: "zh", setLocale: vi.fn(), t: (key) => key}}>
    <TeamRoomMembersDialog agents={[]} currentUserId="remote-me" directory={[]}
      onClose={() => {}} onChanged={() => {}} onDeparted={() => {}} open roomId="room" />
  </I18N_CONTEXT.Provider>);
  await screen.findByRole("dialog");
  expect(document.querySelector('img[src="/icon/agent/7.png"]')).not.toBeNull();
});
