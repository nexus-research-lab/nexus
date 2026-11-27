// INPUT: 本地与 IM Room 历史会话、菜单选择动作和外部会话身份。
// OUTPUT: 证明共享触发/多选控件，并以公共列表分隔线隔离普通历史和 IM 历史。
// POS: RoomHistoryMenu DOM 行为测试；删除事务和锚定位置算法由各自所有者测试。

import { render, screen, within } from "@testing-library/react";
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
const EXTERNAL_CONVERSATION: RoomConversationView = {
  conversation_id: "external-session:feishu-account",
  created_at: 3,
  is_draft: false,
  last_activity_at: 4,
  options: {
    channel_type: "feishu",
    external_identity: {
      account_hint: "816684",
      can_delete: true,
      channel_type: "feishu",
      current_pairing: false,
      pairing_status: "paired",
    },
    external_session: true,
  },
  room_id: "room-1",
  session_id: null,
  session_key: "fs:feishu-account",
  title: "飞书系统测试",
};

describe("RoomHistoryMenu", () => {
  it("keeps unconfirmed batch deletion feedback in the scrollable body without replaying commands", async () => {
    const user = userEvent.setup();
    const remove = vi.fn(async (id: string) => {
      if (id === "conversation-beta") throw new Error("Connection lost");
      return null;
    });
    const create = vi.fn(async () => "replacement");
    const select = vi.fn();
    render(<I18nProvider><RoomHistoryMenu
      conversationId="conversation-alpha" conversations={CONVERSATIONS}
      onCreateConversation={create} onDeleteConversation={remove} onSelectConversation={select}
    /></I18nProvider>);
    await user.click(screen.getByRole("button", { name: /History|历史/ }));
    await user.click(screen.getByRole("button", { name: /Select|多选/ }));
    await user.click(screen.getByRole("checkbox", { name: /Select all|全选/ }));
    await user.click(screen.getByRole("button", { name: /Clear history|清空历史/ }));
    const confirm = screen.getByRole("dialog");
    await user.click(within(confirm).getByRole("button", { name: /Clear history|清空历史/ }));
    const status = await screen.findByRole("status");
    expect(status.closest("[data-room-history-scroll-viewport]")).not.toBeNull();
    expect(status.getAttribute("data-inline-notice-tone")).toBe("warning");
    expect(status.textContent).toMatch(/1 conversations|1 个会话/);
    expect(status.textContent).toMatch(/Do not delete them again|不要重复删除/);
    expect(within(status).queryByRole("button")).toBeNull();
    expect(create).toHaveBeenCalledOnce();
    expect(remove.mock.calls.map(([id]) => id)).toEqual(["conversation-beta", "conversation-alpha"]);
    expect(select).toHaveBeenCalledExactlyOnceWith("replacement");
    expect(screen.getAllByRole("checkbox").some((checkbox) => (checkbox as HTMLInputElement).disabled)).toBe(true);
    expect((screen.getByRole("button", { name: /Delete|删除/ }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("captures failed renames and prevents replay for that conversation", async () => {
    const user = userEvent.setup();
    const rename = vi.fn(async () => { throw new Error("lost response"); });
    render(<I18nProvider><RoomHistoryMenu conversationId="conversation-alpha" conversations={CONVERSATIONS}
      onCreateConversation={vi.fn(async () => null)} onDeleteConversation={vi.fn(async () => null)}
      onSelectConversation={vi.fn()} onUpdateConversationTitle={rename} /></I18nProvider>);
    await user.click(screen.getByRole("button", { name: /History|历史/ }));
    const row = screen.getByRole("button", { name: /Alpha/ });
    await user.click(within(row).getByRole("button", { name: /Rename|重命名/ }));
    await user.clear(screen.getByRole("textbox"));
    await user.type(screen.getByRole("textbox"), "Changed{Enter}");
    expect((await screen.findByRole("status")).textContent).toMatch(/could not be confirmed|未确认/);
    expect(within(screen.getByRole("button", { name: /Alpha/ })).queryByRole("button", { name: /Rename|重命名/ })).toBeNull();
    expect(rename).toHaveBeenCalledTimes(1);
  });

  it("reopens history with feedback when a single deletion cannot be confirmed", async () => {
    const user = userEvent.setup();
    const remove = vi.fn(async () => { throw new Error("lost response"); });
    render(<I18nProvider><RoomHistoryMenu conversationId="conversation-alpha" conversations={CONVERSATIONS}
      onCreateConversation={vi.fn(async () => null)} onDeleteConversation={remove} onSelectConversation={vi.fn()} /></I18nProvider>);
    await user.click(screen.getByRole("button", { name: /History|历史/ }));
    await user.click(within(screen.getByRole("button", { name: /Alpha/ })).getByRole("button", { name: /Delete|删除/ }));
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: /Delete|删除/ }));
    expect((await screen.findByRole("status")).textContent).toMatch(/could not be confirmed|未确认/);
    expect(within(screen.getByRole("button", { name: /Alpha/ })).queryByRole("button", { name: /Delete|删除/ })).toBeNull();
    expect(remove).toHaveBeenCalledTimes(1);
  });

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

    await user.click(trigger);
    expect(screen.getByRole("dialog", { name: /History|历史/ })).toBeTruthy();

    await user.click(screen.getByRole("button", { name: /Select|多选/ }));
    const checkboxes = screen.getAllByRole("checkbox") as HTMLInputElement[];
    expect(checkboxes).toHaveLength(3);
    await user.click(checkboxes[1]);
    expect(checkboxes[0].indeterminate).toBe(true);
    expect(checkboxes[0].getAttribute("aria-checked")).toBe("mixed");
  });

  it("separates IM sessions from ordinary history with the shared divider", async () => {
    const user = userEvent.setup();
    render(
      <I18nProvider>
        <RoomHistoryMenu
          conversationId="conversation-alpha"
          conversations={[...CONVERSATIONS, EXTERNAL_CONVERSATION]}
          onCreateConversation={vi.fn(async () => null)}
          onDeleteConversation={vi.fn(async () => null)}
          onSelectConversation={vi.fn()}
          onUpdateConversationTitle={vi.fn(async () => undefined)}
        />
      </I18nProvider>,
    );

    await user.click(screen.getByRole("button", { name: /History|历史/ }));
    const historySection = document.querySelector("[data-room-history-section='history']");
    const imSection = document.querySelector("[data-room-history-section='im']");
    const divider = screen.getByRole("separator", { name: "IM" });

    expect(historySection?.textContent).toContain("Alpha");
    expect(historySection?.textContent).not.toContain("飞书系统测试");
    expect(imSection?.textContent).toContain("飞书系统测试");
    expect(imSection?.textContent).toContain("飞书 · 账号 816684 · 历史");
    expect(historySection?.nextElementSibling).toBe(divider);
    expect(divider.nextElementSibling).toBe(imSection);
  });
});
