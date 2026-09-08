// INPUT: Room Header 的真实成员入口、延迟目录与 Room/owner 变化。
// OUTPUT: 标准身份、加载防重、关闭和迟到打开隔离的 DOM 回归。
// POS: Header 装配测试；成员表单内容用边界替身隔离其独立 Skill HTTP 读取。

import { act, fireEvent, render, renderHook, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { expect, it, vi } from "vitest";

import type { RoomMemberManagerDialog } from "@/features/conversation/room/members/room-member-manager-dialog";
import { advanceAuthOwnerScopeGeneration, publishAuthOwnerScopeGeneration } from "@/shared/auth/auth-owner-generation";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { GroupConversationHeader } from "./group-conversation-header";
import { useRoomMemberManager } from "../../members/use-room-member-manager";

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
function deferred() {
  let resolve!: () => void;
  const promise = new Promise<void>((finish) => { resolve = finish; });
  return { promise, resolve };
}
function membersButton() { return screen.getByRole("button", { name: /Members|成员/ }); }

it("uses one public 40px identity and coalesces member loading before opening and closing", async () => {
  const user = userEvent.setup();
  const pending = deferred();
  const prepare = vi.fn(() => pending.promise);
  render(view({ onOpenMemberManager: prepare }));
  const avatar = screen.getByRole("img", { name: "Research" });
  expect(avatar.className).toContain("h-10 w-10");
  expect(avatar.className).not.toMatch(/h-full|radius-control-sm|shadow-none/);
  expect(avatar.parentElement!.className).not.toMatch(/border|shadow|radius/);
  const trigger = membersButton();
  fireEvent.click(trigger);
  fireEvent.click(trigger);
  expect(prepare).toHaveBeenCalledOnce();
  expect(trigger.getAttribute("aria-busy")).toBe("true");
  expect(trigger.hasAttribute("disabled")).toBe(true);
  expect(screen.queryByRole("dialog")).toBeNull();
  await act(async () => pending.resolve());
  expect(screen.getByRole("dialog", { name: "Research" })).toBeTruthy();
  expect(trigger.hasAttribute("aria-busy")).toBe(false);
  await user.click(screen.getByRole("button", { name: "Close members" }));
  expect(screen.queryByRole("dialog")).toBeNull();
});

it("discards a pending open across A to B to A without consuming a fresh request", async () => {
  const old = deferred();
  const fresh = deferred();
  const prepare = vi.fn().mockReturnValueOnce(old.promise).mockReturnValueOnce(fresh.promise);
  const rendered = render(view({ onOpenMemberManager: prepare }));
  fireEvent.click(membersButton());
  rendered.rerender(view({ roomId: "room-b", onOpenMemberManager: prepare }));
  expect(membersButton().hasAttribute("disabled")).toBe(false);
  rendered.rerender(view({ onOpenMemberManager: prepare }));
  fireEvent.click(membersButton());
  await act(async () => old.resolve());
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(membersButton().getAttribute("aria-busy")).toBe("true");
  await act(async () => fresh.resolve());
  expect(screen.getByRole("dialog", { name: "Research" })).toBeTruthy();
  expect(prepare).toHaveBeenCalledTimes(2);
});

it("clears open state on scope change but preserves it when only the title or catalog changes", async () => {
  const rendered = render(view());
  await userEvent.click(membersButton());
  rendered.rerender(view({ currentRoomTitle: "Updated" }));
  expect(screen.getByRole("dialog", { name: "Updated" })).toBeTruthy();
  rendered.rerender(view({ roomId: "room-b" }));
  expect(screen.queryByRole("dialog")).toBeNull();
  rendered.rerender(view());
  expect(screen.queryByRole("dialog")).toBeNull();
  await userEvent.click(membersButton());
  act(() => { advanceAuthOwnerScopeGeneration(); publishAuthOwnerScopeGeneration(); });
  expect(screen.queryByRole("dialog")).toBeNull();
});

it("does not let an old form completion close a new dialog after returning to the same Room", async () => {
  const prepare = vi.fn(async () => undefined);
  const { result, rerender } = renderHook(({ roomId }) => useRoomMemberManager(roomId, prepare), { initialProps: { roomId: "a" } });
  const oldClose = result.current.close;
  const oldOpen = result.current.open;
  rerender({ roomId: "b" });
  rerender({ roomId: "a" });
  act(() => oldOpen());
  expect(prepare).not.toHaveBeenCalled();
  await act(async () => result.current.open());
  expect(result.current.isOpen).toBe(true);
  act(() => oldClose());
  expect(result.current.isOpen).toBe(true);
  act(() => result.current.close());
  expect(result.current.isOpen).toBe(false);
});

it("rejects a late owner response and does not load without a Room", async () => {
  const pending = deferred();
  const prepare = vi.fn(() => pending.promise);
  const rendered = render(view({ roomId: null, onOpenMemberManager: prepare }));
  fireEvent.click(membersButton());
  expect(prepare).not.toHaveBeenCalled();
  rendered.rerender(view({ onOpenMemberManager: prepare }));
  fireEvent.click(membersButton());
  act(() => { advanceAuthOwnerScopeGeneration(); publishAuthOwnerScopeGeneration(); });
  await act(async () => pending.resolve());
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(membersButton().hasAttribute("disabled")).toBe(false);
});

it("keeps existing members available when the auxiliary catalog rejects and ignores completion after unmount", async () => {
  const log = vi.spyOn(console, "error").mockImplementation(() => undefined);
  try {
    const rendered = render(view({
      roomMembers: [{ agent_id: "private-id", name: "Nova", created_at: 1, options: {}, status: "idle", workspace_path: "/workspace" }],
      onOpenMemberManager: vi.fn(async () => { throw new Error("read failed"); }),
    }));
    await userEvent.click(membersButton());
    expect(within(screen.getByRole("dialog", { name: "Research" })).getByText("Nova")).toBeTruthy();
    rendered.unmount();
    const pending = deferred();
    const another = render(view({ onOpenMemberManager: vi.fn(() => pending.promise) }));
    fireEvent.click(membersButton());
    another.unmount();
    await act(async () => pending.resolve());
    render(view());
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(membersButton().hasAttribute("disabled")).toBe(false);
  } finally { log.mockRestore(); }
});
