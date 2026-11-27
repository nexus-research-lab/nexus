// INPUT: Room Header 的真实成员入口、延迟目录与 Room/owner 变化。
// OUTPUT: 标准身份、加载防重、关闭和迟到打开隔离的 DOM 回归。
// POS: Header 装配测试；成员表单内容用边界替身隔离其独立 Skill HTTP 读取。

import { act, render, screen } from "@testing-library/react";
import type { ComponentProps } from "react";
import { expect, it, vi } from "vitest";

import type { RoomMemberManagerDialog } from "@/features/conversation/room/members/room-member-manager-dialog";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { GroupConversationHeader } from "./group-conversation-header";
import { useSidebarStore } from "@/store/sidebar";

vi.mock("@/features/conversation/room/members/room-member-manager-dialog", () => ({
  RoomMemberManagerDialog: ({ isOpen, initialName, onClose, roomMembers }: ComponentProps<typeof RoomMemberManagerDialog>) =>
    isOpen ? <div role="dialog" aria-label={initialName}><p>{roomMembers.map((member) => member.name).join(", ")}</p><button onClick={onClose}>Close members</button></div> : null,
}));

const base: ComponentProps<typeof GroupConversationHeader> = {
  roomId: "room-a", currentRoomTitle: "Research", activeTab: "chat", conversationId: null,
  availableRoomAgents: [], conversations: [], roomMembers: [], roomSkillNames: [],
  roomHostAutoReplyEnabled: true, roomPrivateMessagesEnabled: true,
  onChangeTab: vi.fn(), onCloseConversation: vi.fn(async () => undefined),
  onCreateConversation: vi.fn(async () => null), onReplaceFinalConversation: vi.fn(async () => undefined),
  onDeleteConversation: vi.fn(async () => null), onManageRoom: vi.fn(async () => undefined),
  onSelectConversation: vi.fn(), onOpenMemberManager: vi.fn(async () => undefined),
};

function view(props: Partial<typeof base> = {}) {
  return <I18nProvider><GroupConversationHeader {...base} {...props} /></I18nProvider>;
}

it("renders avatar when sidebar is expanded, and removes avatar while showing title when collapsed", () => {
  act(() => {
    useSidebarStore.setState({ wide_panel_collapsed: false });
  });
  const { rerender } = render(view({ currentRoomTitle: "Research" }));
  expect(screen.getByRole("img", { name: "Research" })).toBeTruthy();
  expect(screen.queryByText("Research", { selector: ".ui-type-section-title" })).toBeNull();

  act(() => {
    useSidebarStore.setState({ wide_panel_collapsed: true });
  });
  rerender(view({ currentRoomTitle: "Research" }));
  expect(screen.queryByRole("img", { name: "Research" })).toBeNull();
  expect(screen.getByText("Research", { selector: ".ui-type-section-title" })).toBeTruthy();
  act(() => {
    useSidebarStore.setState({ wide_panel_collapsed: false });
  });
});


