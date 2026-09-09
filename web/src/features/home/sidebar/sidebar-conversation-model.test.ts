// INPUT: Relay 在线 Room 与普通本地 Room 的活动时间。
// OUTPUT: 证明在线 Room 不固定置顶，并按统一会话规则参与排序。
// POS: 聊天目录纯投影回归；不覆盖 Relay 读取或导航行为。

import { describe, expect, it } from "vitest";

import {
  buildTeamConversationItem,
  sortConversationItems,
  type SidebarConversationItem,
} from "./sidebar-conversation-model";

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
