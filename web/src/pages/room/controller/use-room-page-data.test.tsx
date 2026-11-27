// INPUT: 路由 roomId 切换、Room 上下文读取结果与 owner 代次推进。
// OUTPUT: 验证回访 Room 直接用缓存渲染、读取失败丢弃未确认缓存、owner 变化作废缓存。
// POS: 防止切换 Room 时整页退回全屏加载形成空白帧。

import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";

import type { RoomContextAggregate } from "@/types/conversation/room";

const getRoomContexts = vi.fn<(roomId: string) => Promise<RoomContextAggregate[]>>();

vi.mock("@/lib/api/conversation/room-resource-api", () => ({
  getRoomContexts: (roomId: string) => getRoomContexts(roomId),
}));

const { advanceAuthOwnerScopeGeneration } = await import("@/shared/auth/auth-owner-generation");
const { useRoomPageData } = await import("./use-room-page-data");

function contexts(roomId: string): RoomContextAggregate[] {
  return [{ room: { id: roomId } } as unknown as RoomContextAggregate];
}

beforeEach(() => {
  // 每个用例从新的 owner 代次开始，互不共享缓存。
  advanceAuthOwnerScopeGeneration();
  getRoomContexts.mockReset();
});

it("renders a revisited room from cache while refreshing in the background", async () => {
  getRoomContexts.mockImplementation(async (roomId) => contexts(roomId));
  const { result, rerender } = renderHook(({ roomId }) => useRoomPageData({ roomId }), {
    initialProps: { roomId: "room-a" },
  });
  await waitFor(() => expect(result.current.isRoomLoading).toBe(false));

  rerender({ roomId: "room-b" });
  expect(result.current.isRoomLoading).toBe(true);
  await waitFor(() => expect(result.current.isRoomLoading).toBe(false));

  let resolveRefresh: (value: RoomContextAggregate[]) => void = () => undefined;
  getRoomContexts.mockImplementation(() => new Promise((resolve) => { resolveRefresh = resolve; }));
  rerender({ roomId: "room-a" });
  // 回访当帧即可渲染，不退回全屏加载。
  expect(result.current.isRoomLoading).toBe(false);
  expect(result.current.roomContexts).toEqual(contexts("room-a"));

  const refreshed = contexts("room-a-refreshed");
  await act(async () => resolveRefresh(refreshed));
  expect(result.current.roomContexts).toEqual(refreshed);
});

it("drops unverified cached contexts when the background refresh fails", async () => {
  getRoomContexts.mockImplementation(async (roomId) => contexts(roomId));
  const { result, rerender } = renderHook(({ roomId }) => useRoomPageData({ roomId }), {
    initialProps: { roomId: "room-a" },
  });
  await waitFor(() => expect(result.current.isRoomLoading).toBe(false));
  rerender({ roomId: "room-b" });
  await waitFor(() => expect(result.current.isRoomLoading).toBe(false));

  getRoomContexts.mockRejectedValue(new Error("room deleted"));
  rerender({ roomId: "room-a" });
  await waitFor(() => expect(result.current.roomContexts).toEqual([]));

  // 失败后缓存也被移除：再次进入时重新经历加载。
  rerender({ roomId: "room-b" });
  rerender({ roomId: "room-a" });
  expect(result.current.isRoomLoading).toBe(true);
});

it("does not reuse cached contexts across owner generations", async () => {
  getRoomContexts.mockImplementation(async (roomId) => contexts(roomId));
  const { result, rerender } = renderHook(({ roomId }) => useRoomPageData({ roomId }), {
    initialProps: { roomId: "room-a" },
  });
  await waitFor(() => expect(result.current.isRoomLoading).toBe(false));
  rerender({ roomId: "room-b" });
  await waitFor(() => expect(result.current.isRoomLoading).toBe(false));

  advanceAuthOwnerScopeGeneration();
  getRoomContexts.mockImplementation(() => new Promise(() => undefined));
  rerender({ roomId: "room-a" });
  expect(result.current.isRoomLoading).toBe(true);
  expect(result.current.roomContexts).toEqual([]);
});
