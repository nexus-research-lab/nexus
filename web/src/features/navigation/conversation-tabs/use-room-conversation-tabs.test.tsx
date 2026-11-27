// INPUT: 已绑定测试 owner 的 Room 事实、持久偏好、路由延迟与可控创建/最终替换事务。
// OUTPUT: 验证恢复、显式打开、关闭邻居、固定保留、单飞事务及最后标签替换时的并发打开保留。
// POS: 标签导航 Feature 回归，直接运行真实 Store 和 Hook，不模拟共享 DOM。

import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { setRoomNavigationOwnerScope, useRoomNavigationStore } from "@/store/room-navigation";
import type { RoomConversationView } from "@/types/conversation/conversation";
import { useRoomConversationTabs } from "./use-room-conversation-tabs";

function conversation(id: string, createdAt: number): RoomConversationView {
  return {
    conversation_id: id,
    room_id: "room",
    session_key: `room/${id}`,
    session_id: null,
    title: id,
    created_at: createdAt,
    last_activity_at: 10 - createdAt,
    options: {},
  };
}

const conversations = [conversation("third", 3), conversation("first", 1), conversation("second", 2)];

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

beforeEach(() => {
  setRoomNavigationOwnerScope(null, () => false);
  setRoomNavigationOwnerScope("user-id:room-tabs-test", () => true);
});

describe("useRoomConversationTabs", () => {

  it("closes the active tab to its next neighbor before the route catches up, preserving its pin", () => {
    const onCloseConversation = vi.fn(async () => undefined);
    const onSelectConversation = vi.fn();
    for (const id of ["first", "second", "third", "second"]) useRoomNavigationStore.getState().remember_last_active_conversation("room", id);
    useRoomNavigationStore.getState().toggle_pinned_conversation({
      room_id: "room", conversation_id: "second", session_key: "room/second", title: "second",
    });
    const { result, rerender } = renderHook((conversationId) => useRoomConversationTabs({
      conversations,
      conversationId,
      onCloseConversation,
      onSelectConversation,
    }), { initialProps: "second" });
    act(() => result.current.closeConversation("second"));
    expect(result.current.activeConversationId).toBe("third");
    expect(result.current.orderedConversations.map((item) => item.conversation_id)).toEqual(["first", "third"]);
    expect(onSelectConversation).toHaveBeenCalledWith("third");
    expect(onCloseConversation).toHaveBeenCalledWith("second");
    expect(useRoomNavigationStore.getState().pinned_conversations[0].conversation_id).toBe("second");
    rerender("third");
    expect(result.current.orderedConversations.map((item) => item.conversation_id)).toEqual(["first", "third"]);
  });

  it("retains the last tab until replacement commits and rejects competing commands", async () => {
    const replacement = deferred<void>();
    let commit!: (id: string) => void;
    const onReplaceFinalConversation = vi.fn(async (_conversation, commitConversation) => {
      commit = commitConversation;
      await replacement.promise;
    });
    const onCreateConversation = vi.fn(async () => "second");
    const onSelectConversation = vi.fn();
    const { result } = renderHook(() => useRoomConversationTabs({
      conversations,
      conversationId: "first",
      onCreateConversation,
      onReplaceFinalConversation,
      onSelectConversation,
    }));
    act(() => result.current.closeConversation("first"));
    act(() => {
      result.current.closeConversation("first");
      result.current.selectConversation("third");
      void result.current.createConversation();
    });
    expect(result.current.activeConversationId).toBe("first");
    expect(result.current.isCreating).toBe(true);
    expect(onReplaceFinalConversation).toHaveBeenCalledOnce();
    expect(onCreateConversation).not.toHaveBeenCalled();
    expect(onSelectConversation).not.toHaveBeenCalled();
    await act(async () => {
      commit("second");
      replacement.resolve();
      await replacement.promise;
    });
    expect(result.current.isCreating).toBe(false);
    expect(result.current.activeConversationId).toBe("second");
    expect(result.current.orderedConversations.map((item) => item.conversation_id)).toEqual(["second"]);
    expect(onSelectConversation).toHaveBeenCalledWith("second");
  });
});
