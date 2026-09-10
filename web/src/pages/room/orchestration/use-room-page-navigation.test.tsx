// INPUT: Real Room route navigation, tab controller and owner-bound store with local creation results.
// OUTPUT: New and historical conversations append tabs across route transitions instead of replacing them.
// POS: Page/feature integration; no backend writes, runtime commands or browser geometry assertions.

import { act, renderHook } from "@testing-library/react";
import { useState, type ReactNode } from "react";
import { MemoryRouter, Route, Routes, useParams } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useRoomConversationTabs } from "@/features/navigation/conversation-tabs/use-room-conversation-tabs";
import { setRoomNavigationOwnerScope } from "@/store/room-navigation";
import type { RoomConversationView } from "@/types/conversation/conversation";

import { resolveSelectedConversationId } from "../controller/model/room-conversation-model";
import { useRoomPageNavigation } from "./use-room-page-navigation";

const item = (id: string, time: number): RoomConversationView => ({
  conversation_id: id, room_id: "room", session_key: `room/${id}`, session_id: null,
  title: id, created_at: time, last_activity_at: time, options: {}, is_draft: false,
});

beforeEach(() => {
  setRoomNavigationOwnerScope(null, () => false);
  setRoomNavigationOwnerScope("user-id:page-tab-test", () => true);
});

describe("Room page session tabs", () => {
  it.each(["/rooms/room", "/rooms/room/conversations/first"])(
    "appends creation and history navigation from %s while retaining the prior session",
    async (initialRoute) => {
      const close = vi.fn(async () => undefined);
      const wrapper = ({ children }: { children: ReactNode }) => (
        <MemoryRouter initialEntries={[initialRoute]}><Routes>
          <Route path="/rooms/:roomId" element={children} />
          <Route path="/rooms/:roomId/conversations/:conversationId" element={children} />
        </Routes></MemoryRouter>
      );
      const { result } = renderHook(() => {
        const params = useParams();
        const [items, setItems] = useState([item("first", 1), item("history", 0)]);
        const selectedId = resolveSelectedConversationId(params.conversationId, items);
        const navigation = useRoomPageNavigation({
          roomId: params.roomId, routeConversationId: params.conversationId, currentRoomId: "room",
          selectedConversationId: selectedId, selectedDraftConversationId: null, isHydrated: true,
          closeConversation: close, deleteConversation: async () => null,
          createConversation: async () => {
            setItems((current) => [...current, item("created", 2)]);
            return "created";
          },
        });
        const tabs = useRoomConversationTabs({
          conversations: items, conversationId: selectedId,
          onSelectConversation: navigation.selectConversation,
          onCreateConversation: navigation.createConversation,
        });
        return { navigation, tabs };
      }, { wrapper });
      expect(result.current.tabs.activeConversationId).toBe("first");
      await act(async () => result.current.tabs.createConversation());
      expect(result.current.tabs.activeConversationId).toBe("created");
      expect(result.current.tabs.orderedConversations.map((value) => value.conversation_id)).toEqual(["first", "created"]);
      act(() => result.current.navigation.selectConversation("history"));
      expect(result.current.tabs.activeConversationId).toBe("history");
      expect(result.current.tabs.orderedConversations.map((value) => value.conversation_id)).toEqual(["history", "first", "created"]);
      expect(close).not.toHaveBeenCalled();
    },
  );
});
