// INPUT: Concurrent online Room creation intents.
// OUTPUT: One request while pending, original identity reused after failure.
// POS: Sidebar online creation boundary regression.
import { act, renderHook } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, expect, it, vi } from "vitest";
import { useChatSidebarController } from "./use-chat-sidebar-controller";
const api = vi.hoisted(() => ({ agents: [] as Array<{ id: string; name: string; avatar?: string }>, create: vi.fn(), publish: vi.fn() }));
vi.mock("@/shared/i18n/i18n-context", () => ({useI18n: () => ({locale: "en", t: (key: string) => key})}));
vi.mock("@/lib/api/conversation/team-api", () => ({createTeamRoom: api.create}));
vi.mock("@/lib/api/account/control-api", () => ({publishControlAgentApi: api.publish}));
vi.mock("@/features/team/use-team-rooms", () => ({useTeamRooms: () => ({rooms: [], refresh: vi.fn(), isAvailable: true})}));
vi.mock("@/features/team/use-team-members", () => ({useTeamMembers: () => []}));
vi.mock("@/features/team/use-team-invitations", () => ({useTeamInvitations: () => ({busyRoomId: null, errorRoomId: null, invitations: [], refresh: vi.fn(), resolve: vi.fn()})}));
vi.mock("../room-activity-resource", () => ({useRoomActivity: () => ({})}));
vi.mock("./sidebar-directory", () => ({useSidebarDirectory: () => ({agents: api.agents, conversations: [], rooms: [], hasLoaded: true})}));
beforeEach(() => {
  api.agents = [];
  api.create.mockReset();
  api.publish.mockReset();
});
it("deduplicates same-turn submits and retains the request id after failure", async () => {
  let reject!: (error: Error) => void;
  api.create.mockImplementation(() => new Promise((_, fail) => {reject = fail;}));
  const {result} = renderHook(() => useChatSidebarController({untitledRoomLabel: "Room"}), {wrapper: MemoryRouter});
  let pending!: Promise<void>;
  const submission = {agentIds: [], hostAgentId: null, hostAutoReplyEnabled: false, location: "online" as const, name: "Room", pausedAgentIds: [], privateMessagesEnabled: false, skillNames: [], userIds: []};
  act(() => {pending = result.current.create.submit(submission); void result.current.create.submit(submission);});
  expect(api.create).toHaveBeenCalledTimes(1);
  await act(async () => {reject(new Error("offline")); await pending.catch(() => undefined);});
  act(() => {void result.current.create.submit(submission);});
  expect(api.create).toHaveBeenCalledTimes(2);
  expect(api.create.mock.calls[1]).toEqual(api.create.mock.calls[0]);
});

it("publishes selected local Agents before creating an online Room", async () => {
  api.agents = [{ id: "local-agent", name: "Nova", avatar: "avatar://nova" }];
  api.publish.mockResolvedValue({ agent_id: "agent-online", source_agent_id: "local-agent" });
  api.create.mockResolvedValue({ room: { id: "room-online" } });
  const {result} = renderHook(() => useChatSidebarController({untitledRoomLabel: "Room"}), {wrapper: MemoryRouter});
  await act(async () => {
    await result.current.create.submit({
      agentIds: ["local-agent"], hostAgentId: "local-agent", hostAutoReplyEnabled: false,
      location: "online", name: "Room", pausedAgentIds: [], privateMessagesEnabled: false,
      skillNames: [], userIds: [],
    });
  });
  expect(api.publish).toHaveBeenCalledWith("local-agent", { name: "Nova", avatar: "avatar://nova" });
  expect(api.create.mock.calls[0][0]).toEqual(expect.objectContaining({
    agent_ids: ["agent-online"],
    coordinator_agent_id: "agent-online",
  }));
});
