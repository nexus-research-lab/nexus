// INPUT: 两条本地 Room 历史会话与历史菜单选择动作。
// OUTPUT: 证明菜单入口复用共享 IconButton，批量全选使用真实 mixed checkbox。
// POS: RoomHistoryMenu DOM 行为测试；删除事务和锚定位置算法由各自所有者测试。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { RoomHistoryMenu } from "@/features/conversation/room/surface/history/room-history-menu";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { RoomConversationView } from "@/types/conversation/conversation";

const CONVERSATIONS: RoomConversationView[] = [
  {
    conversation_id: "conversation-alpha",
    created_at: 1,
    is_draft: false,
    last_activity_at: 2,
    options: {},
    room_id: "room-1",
    session_id: null,
    session_key: "alpha",
    title: "Alpha",
  },
  {
    conversation_id: "conversation-beta",
    created_at: 2,
    is_draft: false,
    last_activity_at: 3,
    options: {},
    room_id: "room-1",
    session_id: null,
    session_key: "beta",
    title: "Beta",
  },
];

describe("RoomHistoryMenu", () => {
  it("uses shared trigger and mixed selection controls", async () => {
    const user = userEvent.setup();
    render(
      <I18nProvider>
        <RoomHistoryMenu
          conversationId="conversation-alpha"
          conversations={CONVERSATIONS}
          onCreateConversation={vi.fn(async () => null)}
          onDeleteConversation={vi.fn(async () => null)}
          onSelectConversation={vi.fn()}
          onUpdateConversationTitle={vi.fn(async () => undefined)}
        />
      </I18nProvider>,
    );

    const trigger = screen.getByRole("button", { name: /History|历史/ });
    expect(trigger.className).toContain("h-9 w-9");
    expect(trigger.className).toContain("border-transparent");
    await user.click(trigger);
    expect(screen.getByRole("dialog", { name: /History|历史/ })).toBeTruthy();

    await user.click(screen.getByRole("button", { name: /Select|多选/ }));
    const checkboxes = screen.getAllByRole("checkbox") as HTMLInputElement[];
    expect(checkboxes).toHaveLength(3);
    await user.click(checkboxes[1]);
    expect(checkboxes[0].indeterminate).toBe(true);
    expect(checkboxes[0].getAttribute("aria-checked")).toBe("mixed");
  });
});
