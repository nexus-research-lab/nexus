// INPUT: Local Room API responses, real catalog refresh/commands, route navigation and header controls.
// OUTPUT: Browser-style session tabs survive creation, history selection, closing and owner-bound reload.
// POS: Page integration regression; replaces only HTTP results and jsdom's absent scrolling APIs.

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes, useParams } from "react-router-dom";
import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { DmConversationHeader } from "@/features/conversation/room/dm/dm-conversation-header";
import { GroupConversationHeader } from "@/features/conversation/room/group/header/group-conversation-header";
import { resolveSelectedDraftConversationId } from "@/features/navigation/conversation-tabs/room-conversation-tabs-model";
import { closeRoomConversationRuntime, createRoomConversation } from "@/lib/api/conversation/room-command-api";
import { getRoomContexts } from "@/lib/api/conversation/room-resource-api";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { resetRoomNavigationOwnerScope, setRoomNavigationOwnerScope, useRoomNavigationStore } from "@/store/room-navigation";
import type { RoomContextAggregate } from "@/types/conversation/room";

import { useRoomPageCommands } from "../controller/commands/use-room-page-commands";
import { buildRoomConversationViews, resolveSelectedConversationId } from "../controller/model/room-conversation-model";
import { useRoomPageData } from "../controller/use-room-page-data";
import { useRoomPageNavigation } from "./use-room-page-navigation";

vi.mock("@/lib/api/conversation/room-resource-api", () => ({ getRoomContexts: vi.fn() }));
vi.mock("@/lib/api/conversation/room-command-api", () => ({
  closeRoomConversationRuntime: vi.fn(async () => undefined),
  createRoomConversation: vi.fn(),
}));

// These shims do not prove browser geometry, hit testing or native-host behavior.
const methods = ["scrollTo", "hasPointerCapture"] as const;
const descriptors = methods.map((name) => Object.getOwnPropertyDescriptor(HTMLElement.prototype, name));
beforeAll(() => {
  Object.defineProperty(HTMLElement.prototype, "scrollTo", { configurable: true, value: vi.fn() });
  Object.defineProperty(HTMLElement.prototype, "hasPointerCapture", { configurable: true, value: () => false });
});
afterAll(() => methods.forEach((name, index) => {
  const descriptor = descriptors[index];
  if (descriptor) Object.defineProperty(HTMLElement.prototype, name, descriptor);
  else Reflect.deleteProperty(HTMLElement.prototype, name);
}));

const owner = "user-id:session-page-regression";
beforeEach(() => {
  vi.clearAllMocks();
  setRoomNavigationOwnerScope(null, () => false);
  setRoomNavigationOwnerScope(owner, () => true);
});

function context(id: string, roomType: string, day: number, draft = false): RoomContextAggregate {
  return {
    room: {
      id: "room", room_type: roomType, description: "", skill_names: [],
      host_auto_reply_enabled: true, private_messages_enabled: true,
    },
    members: [], member_agents: [], sessions: [],
    conversation: {
      id, room_id: "room", conversation_type: roomType, title: id, is_draft: draft,
      created_at: `2026-09-0${day}T00:00:00Z`, last_activity_at: `2026-09-0${day}T00:00:00Z`,
    },
  };
}

function HeaderPage() {
  const params = useParams();
  const data = useRoomPageData({ roomId: params.roomId });
  const conversations = buildRoomConversationViews(data.roomContexts);
  const tabs = useRoomNavigationStore((state) => state.conversation_tabs_by_room.room);
  const selectedId = resolveSelectedConversationId(params.conversationId, conversations,
    tabs ? [tabs.active_conversation_id, ...tabs.open_conversation_ids] : []);
  const commands = useRoomPageCommands({
    roomId: params.roomId, roomMembers: [], refreshRoomContexts: data.refreshRoomContexts,
    saveExistingAgentOptions: async () => undefined,
  });
  const navigation = useRoomPageNavigation({
    roomId: params.roomId, routeConversationId: params.conversationId,
    currentRoomId: data.roomContexts[0]?.room.id ?? null, selectedConversationId: selectedId,
    selectedDraftConversationId: resolveSelectedDraftConversationId(conversations, selectedId),
    isHydrated: !data.isRoomLoading, closeConversation: commands.handleCloseConversation,
    createConversation: commands.handleCreateConversation, deleteConversation: commands.handleDeleteConversation,
  });
  if (data.isRoomLoading) return null;
  const headerProps = {
    activeTab: "chat" as const, conversations, conversationId: selectedId,
    onChangeTab: () => undefined,
    onSelectConversation: navigation.selectConversation, onCreateConversation: navigation.createConversation,
    onCloseConversation: commands.handleCloseConversation, onReplaceFinalConversation: navigation.replaceFinalConversation,
    onDeleteConversation: navigation.deleteConversation,
  };
  return data.roomContexts[0]?.room.room_type === "dm"
    ? <DmConversationHeader {...headerProps} currentAgentName="Nova" />
    : <GroupConversationHeader {...headerProps} roomId="room" currentRoomTitle="Research"
        roomMembers={[]} availableRoomAgents={[]} roomHostAutoReplyEnabled roomPrivateMessagesEnabled
        roomSkillNames={[]} onManageRoom={async () => undefined} onOpenMemberManager={async () => undefined} />;
}

function page(initialRoute: string) {
  return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
    <MemoryRouter initialEntries={[initialRoute]}><Routes>
      <Route path="/rooms/:roomId" element={<HeaderPage />} />
      <Route path="/rooms/:roomId/conversations/:conversationId" element={<HeaderPage />} />
    </Routes></MemoryRouter>
  </I18N_CONTEXT.Provider>;
}

describe("session header navigation through page commands", () => {
  it.each(["dm", "group"])("retains %s tabs through delayed creation, real history, close/reopen and reload", async (roomType) => {
    const user = userEvent.setup();
    let contexts = [context("First", roomType, 2), context("History", roomType, 1)];
    vi.mocked(getRoomContexts).mockImplementation(async () => contexts);
    vi.mocked(createRoomConversation).mockImplementation(async () => {
      const draft = contexts.find((value) => value.conversation.is_draft);
      if (draft) return draft;
      const created = context("New session", roomType, 3, true);
      contexts = [...contexts, created];
      return created;
    });
    const view = render(page("/rooms/room/conversations/First"));
    const nav = within(await screen.findByRole("navigation", { name: "room.session_tabs_label" }));
    expect(nav.getByRole("button", { name: "First" })).toBeTruthy();
    expect(nav.queryByRole("button", { name: "History" })).toBeNull();

    let finishRefresh!: (value: RoomContextAggregate[]) => void;
    vi.mocked(getRoomContexts).mockImplementationOnce(() => new Promise((resolve) => { finishRefresh = resolve; }));
    const create = nav.getByRole("button", { name: "room.new_conversation" });
    await user.click(create);
    expect(create.hasAttribute("disabled")).toBe(true);
    expect(nav.getByRole("button", { name: "First" }).getAttribute("aria-current")).toBe("page");
    await act(async () => finishRefresh(contexts));
    await waitFor(() => expect(nav.getByRole("button", { name: "New session" }).getAttribute("aria-current")).toBe("page"));
    expect(nav.getByRole("button", { name: "First" })).toBeTruthy();
    await user.click(create);
    expect(createRoomConversation).toHaveBeenCalledExactlyOnceWith("room", { title: undefined });

    await user.click(nav.getByRole("button", { name: "room.history" }));
    const history = screen.getByRole("dialog", { name: "room.history" });
    await user.click(within(history).getByRole("button", { name: /^History/ }));
    expect(screen.queryByRole("dialog", { name: "room.history" })).toBeNull();
    expect(nav.getByRole("button", { name: "History" }).getAttribute("aria-current")).toBe("page");
    for (const name of ["First", "New session"]) expect(nav.getByRole("button", { name })).toBeTruthy();
    await user.click(nav.getByRole("button", { name: "First" }));
    expect(nav.getByRole("button", { name: "First" }).getAttribute("aria-current")).toBe("page");
    await user.click(nav.getByRole("button", { name: "New session" }));
    expect(nav.getByRole("button", { name: "New session" }).getAttribute("aria-current")).toBe("page");
    expect(nav.getByRole("button", { name: "History" })).toBeTruthy();
    expect(closeRoomConversationRuntime).not.toHaveBeenCalled();

    const firstTab = nav.getByRole("button", { name: "First" }).parentElement!;
    await user.click(within(firstTab).getByRole("button", { name: "room.pin_conversation" }));
    await user.click(within(firstTab).getByRole("button", { name: "room.close_conversation" }));
    expect(nav.queryByRole("button", { name: "First" })).toBeNull();
    expect(closeRoomConversationRuntime).toHaveBeenCalledExactlyOnceWith("room", "First");
    expect(useRoomNavigationStore.getState().pinned_conversations[0].conversation_id).toBe("First");
    await user.click(nav.getByRole("button", { name: "room.history" }));
    await user.click(within(screen.getByRole("dialog", { name: "room.history" })).getByRole("button", { name: /^First/ }));
    expect(nav.getByRole("button", { name: "First" }).getAttribute("aria-current")).toBe("page");

    view.unmount();
    resetRoomNavigationOwnerScope();
    setRoomNavigationOwnerScope(owner, () => true);
    render(page("/rooms/room"));
    const restored = within(await screen.findByRole("navigation", { name: "room.session_tabs_label" }));
    for (const name of ["First", "History", "New session"]) expect(restored.getByRole("button", { name })).toBeTruthy();
    expect(restored.getByRole("button", { name: "First" }).getAttribute("aria-current")).toBe("page");
    expect(restored.getByRole("button", { name: "room.unpin_conversation" }).getAttribute("aria-pressed")).toBe("true");
  });
});
