// INPUT: 配对目标草稿、异步 Room 目录和版本化写命令。
// OUTPUT: 明确选择话题后才保存，读失败不误切目标。
// POS: IM 目标选择的 DOM 回归。
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { PairingView } from "@/lib/api/capability/channel-api";
import { getRoomContexts, listRooms } from "@/lib/api/conversation/room-resource-api";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { PairingSessionTarget } from "./pairing-session-target";

vi.mock("@/lib/api/conversation/room-resource-api", () => ({ listRooms: vi.fn(), getRoomContexts: vi.fn() }));
const pairing: PairingView = { pairing_id: "pair", agent_id: "agent", channel_type: "feishu", chat_type: "dm", external_ref: "phone", session_key: "session", status: "active", source: "manual", created_at: "", updated_at: "", binding_version: 4 };
const room = { id: "room", room_type: "room", name: "产品开发", description: "", skill_names: [], host_auto_reply_enabled: false, private_messages_enabled: true };
const members = [{ id: "member", room_id: "room", member_type: "agent", member_agent_id: "agent", participation_paused: false }];
function view(onUpdate = vi.fn()) {
 return render(<I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => MESSAGES.zh[key] }}>
  <PairingSessionTarget item={pairing} busy={false} onUpdate={onUpdate} />
 </I18N_CONTEXT.Provider>);
}

describe("PairingSessionTarget", () => {
 it("saves the selected Room topic with the observed binding version", async () => {
  vi.mocked(listRooms).mockResolvedValue([{ room, members }]);
  vi.mocked(getRoomContexts).mockResolvedValue([{ room, members, member_agents: [], sessions: [], conversation: { id: "topic", room_id: "room", conversation_type: "main", title: "登录改造" } }]);
  const user = userEvent.setup(); const update = vi.fn(); view(update);
  await user.click(screen.getByRole("button", { name: /会话目标 · 独立/ }));
  await waitFor(() => expect(screen.getByRole("button", { name: "保存" }).hasAttribute("disabled")).toBe(false));
  await user.click(screen.getByRole("button", { name: "产品开发" }));
  await screen.findByRole("button", { name: "登录改造" });
  expect(screen.getByRole("button", { name: "保存" }).hasAttribute("disabled")).toBe(true);
  const search = screen.getByRole("searchbox");
  await user.type(search, "不存在的会话");
  expect(screen.queryByRole("button", { name: "登录改造" })).toBeNull();
  expect(update).not.toHaveBeenCalled();
  await user.clear(search);
  await user.click(screen.getByRole("button", { name: "登录改造" }));
  expect(screen.getByText("将使用： 产品开发 / 登录改造")).toBeTruthy();
  await user.click(screen.getByRole("button", { name: "保存" }));
  expect(update).toHaveBeenCalledWith(pairing, { session_target: { room_id: "room", conversation_id: "topic" }, binding_version: 4 });
 });
 it("lists this agent's DM without requiring group private messages", async () => {
  const direct = { ...room, id: "direct", name: "Amy", room_type: "dm", private_messages_enabled: false };
  vi.mocked(listRooms).mockResolvedValue([{ room: direct, members }, { room: { ...direct, id: "other", name: "Other" }, members: [{ ...members[0], member_agent_id: "other" }] }]);
  vi.mocked(getRoomContexts).mockResolvedValue([{ room: direct, members, member_agents: [], sessions: [], conversation: { id: "existing-dm", room_id: "direct", conversation_type: "main", title: "之前的调研" } }]);
  const user = userEvent.setup(); const update = vi.fn(); view(update);
  await user.click(screen.getByRole("button", { name: /会话目标 · 独立/ }));
  const dm = await screen.findByRole("button", { name: "Amy · 私聊" });
  expect(screen.queryByRole("button", { name: "Other · 私聊" })).toBeNull();
  await user.click(dm);
  await user.click(await screen.findByRole("button", { name: "之前的调研" }));
  await user.click(screen.getByRole("button", { name: "保存" }));
  expect(update).toHaveBeenCalledWith(pairing, { session_target: { room_id: "direct", conversation_id: "existing-dm" }, binding_version: 4 });
 });
 it("keeps saving disabled when the Room catalog fails", async () => {
  vi.mocked(listRooms).mockRejectedValue(new Error("offline"));
  const user = userEvent.setup(); const update = vi.fn(); view(update);
  await user.click(screen.getByRole("button", { name: /会话目标 · 独立/ }));
  await screen.findByRole("button", { name: "读取失败，点击重试" });
  expect(screen.getByRole("button", { name: "保存" }).hasAttribute("disabled")).toBe(true);
  expect(update).not.toHaveBeenCalled();
 });
});
