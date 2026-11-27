// INPUT: 运行时主身份与最新会话目录快照。
// OUTPUT: 主 DM 始终置顶禁删，普通聊天随最新活动重新排序。
// POS: 侧栏目录投影回归，不按可见名称识别系统身份。
import { expect, it } from "vitest";
import { buildConversationItems } from "./sidebar-conversation-model";
const rooms = [
  { id: "main-room", room_type: "dm" as const, dm_target_agent_id: "main" },
  { id: "a", room_type: "dm" as const, dm_target_agent_id: "a" },
  { id: "b", room_type: "room" as const, name: "nexus" },
];
const conversations = rooms.map((room, index) => ({ room_id: room.id, room_type: room.room_type, session_key: room.id, title: room.id, last_activity: `2026-09-09T00:0${index}:00Z` }));
const input = { agents: [], rooms, conversations, untitledRoomLabel: "Untitled" };
it("reprojects late main identity without relying on its display name", () => {
  expect(buildConversationItems({ ...input, mainAgentId: "" })[0].id).toBe("b");
  const items = buildConversationItems({ ...input, mainAgentId: "main" });
  expect(items[0]).toMatchObject({ id: "main-room", isPinned: true, canDelete: false });
  expect(items.find((item) => item.id === "b")?.canDelete).toBe(true);
});


it.each([
  ["<nexus_room_no_reply/>", ""],
  ["  <nexus_room_fanout />  ", ""],
  ["已完成整理。<nexus_room_no_reply/>\n<nexus_room_fanout/>", "已完成整理。"],
  ["普通消息 <example> 保留", "普通消息 <example> 保留"],
])("cleans internal control markers from directory previews: %s", (preview, expected) => {
  const source = conversations.map((item) => ({ ...item, last_reply_preview: preview }));
  const items = buildConversationItems({ ...input, conversations: source });
  expect(items).toHaveLength(3);
  items.forEach((item) => expect(item.summary).toBe(expected));
  expect(source[0].last_reply_preview).toBe(preview);
});
