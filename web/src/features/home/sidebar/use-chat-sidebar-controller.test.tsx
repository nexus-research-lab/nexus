// INPUT: Concurrent online Room creation intents.
// OUTPUT: One request while pending, original identity reused after failure.
// POS: Sidebar online creation boundary regression.
import { act, renderHook } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { expect, it, vi } from "vitest";
import { useChatSidebarController } from "./use-chat-sidebar-controller";
const api = vi.hoisted(() => ({ create: vi.fn() }));
vi.mock("@/shared/i18n/i18n-context", () => ({useI18n: () => ({locale: "en", t: (key: string) => key})}));
vi.mock("@/lib/api/conversation/team-api", () => ({createTeamRoom: api.create}));
vi.mock("@/features/team/use-team-rooms", () => ({useTeamRooms: () => ({rooms: [], refresh: vi.fn(), isAvailable: true})}));
vi.mock("../room-activity-resource", () => ({useRoomActivity: () => ({})}));
vi.mock("./sidebar-directory", () => ({useSidebarDirectory: () => ({agents: [], conversations: [], rooms: [], hasLoaded: true})}));
it("deduplicates same-turn submits and retains the request id after failure", async () => {
  let reject!: (error: Error) => void;
  api.create.mockImplementation(() => new Promise((_, fail) => {reject = fail;}));
  const {result} = renderHook(() => useChatSidebarController({untitledRoomLabel: "Room"}), {wrapper: MemoryRouter});
  let pending!: Promise<void>;
  act(() => {pending = result.current.onlineCreate.submit("Room"); void result.current.onlineCreate.submit("Room");});
  expect(api.create).toHaveBeenCalledTimes(1);
  await act(async () => {reject(new Error("offline")); await pending;});
  act(() => {void result.current.onlineCreate.submit("Room");});
  expect(api.create).toHaveBeenCalledTimes(2);
  expect(api.create.mock.calls[1]).toEqual(api.create.mock.calls[0]);
});
