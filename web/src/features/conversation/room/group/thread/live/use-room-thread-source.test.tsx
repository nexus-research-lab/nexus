// INPUT: Live Room source lifecycle and replaceable scope-aware file callbacks.
// OUTPUT: Stable publication forwards both arguments to the latest handler and cleans up on exit.
// POS: Room Thread bridge integration test using the real hook, context and live store.

import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, expect, it, vi } from "vitest";
import { ThreadControlContext } from "../group-thread-state";
import { useRoomThreadLiveStore } from "./room-thread-live-store";
import { useRoomThreadSource } from "./use-room-thread-source";

afterEach(() => act(() => useRoomThreadLiveStore.getState().clearSource()));

it("preserves exact workspace identity across stable Thread callbacks, replacement and unavailable preview", () => {
  const first = vi.fn();
  const second = vi.fn();
  const closeThread = vi.fn();
  const options: Parameters<typeof useRoomThreadSource>[0] = {
    agentAvatarMap: {}, agentNameMap: {}, conversationId: "room-conversation", messageGroups: new Map(), onOpenWorkspaceFile: first,
    pendingPermissionGroups: new Map(), pendingSlotGroups: new Map(), roomAgentExecutionStateGroups: new Map(), sendPermissionResponse: vi.fn(() => true),
  };
  const { rerender, unmount } = renderHook(useRoomThreadSource, {
    initialProps: options,
    wrapper: ({ children }: { children: ReactNode }) => <ThreadControlContext.Provider value={{ activeThread: null, openThread: vi.fn(), closeThread }}>{children}</ThreadControlContext.Provider>,
  });
  const source = useRoomThreadLiveStore.getState().source!;
  source.onOpenWorkspaceFile!("report.md", "source-author");
  expect(first).toHaveBeenCalledExactlyOnceWith("report.md", "source-author");
  rerender({ ...options, onOpenWorkspaceFile: second });
  expect(useRoomThreadLiveStore.getState().source).toBe(source);
  source.onOpenWorkspaceFile!("next.md", "other-author");
  expect(second).toHaveBeenCalledExactlyOnceWith("next.md", "other-author");
  source.onOpenWorkspaceFile!("unknown.md", null);
  expect(second).toHaveBeenLastCalledWith("unknown.md", null);
  rerender({ ...options, onOpenWorkspaceFile: undefined });
  expect(useRoomThreadLiveStore.getState().source?.onOpenWorkspaceFile).toBeUndefined();
  source.onOpenWorkspaceFile!("stale.md", "source-author");
  expect(second).toHaveBeenCalledTimes(2);
  rerender({ ...options, conversationId: "next-conversation", onOpenWorkspaceFile: second });
  expect(closeThread).toHaveBeenCalledTimes(2);
  unmount();
  expect(useRoomThreadLiveStore.getState().source).toBeNull();
});
