// INPUT: 运行时主身份与最新会话目录快照。
// OUTPUT: 主 DM 始终置顶禁删，普通聊天随最新活动重新排序。
// POS: 侧栏目录投影回归，不按可见名称识别系统身份。
import { describe, expect, it } from "vitest";
import { buildConversationItems, buildTeamConversationItem, sortConversationItems, type SidebarConversationItem } from "./sidebar-conversation-model";
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
it("reorders fresh activity below the protected main DM", () => {
  const items = buildConversationItems({ ...input, mainAgentId: "main", conversations: conversations.map((item) => item.room_id === "a" ? { ...item, last_activity: "2026-09-09T12:00:00Z" } : item) });
  expect(items.map((item) => item.id)).toEqual(["main-room", "a", "b"]);
});

// INPUT: Relay 在线 Room 与普通本地 Room 的活动时间。
// OUTPUT: 证明在线 Room 不固定置顶，并按统一会话规则参与排序。
// POS: 聊天目录纯投影回归；不覆盖 Relay 读取或导航行为。

describe("Relay Room sidebar projection", () => {
  it("sorts as a normal unpinned Room by its last activity", () => {
    const general = buildTeamConversationItem({
      fallbackTitle: "General",
      locale: "zh",
      summary: "Shared messages",
      team: {
        room: {
          id: "general", team_id: "team", name: "General", description: "", avatar: "",
          configuration_version: 1, membership_version: 1,
          created_at: "2026-09-09T00:00:00Z", updated_at: "2026-09-09T01:00:00Z",
        },
        conversation: {
          id: "team-conversation", room_id: "general", type: "main",
          high_water_message_seq: 3, last_activity_at: "2026-09-09T01:00:00Z",
          sync_stream_id: "stream", stream_epoch: "epoch",
          high_water_sync_event_seq: 3,
        },
        current_user_role: "owner",
      },
    });
    const older: SidebarConversationItem = {
      activityStatus: null,
      canDelete: true,
      id: "older",
      isPinned: false,
      kind: "room",
      lastActivityAt: new Date("2026-09-08T01:00:00Z").getTime(),
      members: [],
      messageCount: 1,
      summary: "",
      timeLabel: "",
      title: "Older",
    };

    expect(general.isPinned).toBe(false);
    expect(general.timeLabel).not.toBe("");
    expect(sortConversationItems([older, general], "zh").map((item) => item.id))
      .toEqual([general.id, older.id]);
  });
});
