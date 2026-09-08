// INPUT: Scoped Composer drafts, Room candidates and the existing host set_goal command.
// OUTPUT: Evidence for explicit unavailable owners, preserved drafts and current-member validation at dispatch.
// POS: Real Room Goal/Composer hook integration with an offline conversation transport.
import type { ReactNode } from "react";
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { goalTestI18n } from "@/features/conversation/shared/goal/goal.test-support";
import { resetComposerDraftOwnerScope, useComposerDraftStore } from "@/features/conversation/shared/composer/composer-draft-store";
import { buildComposerDraftScopeKey } from "@/features/conversation/shared/composer/composer-draft-scope";
import { ROOM_GOAL_MEMBERS } from "../../room-goal.test-support";
import { useGroupChatComposerModel } from "./use-group-chat-composer-model";
import { useRoomGoalComposer } from "./use-room-goal-composer";

function Wrapper({ children }: { children: ReactNode }) {
  return <I18N_CONTEXT.Provider value={goalTestI18n("en")}>{children}</I18N_CONTEXT.Provider>;
}
const base = { roomId: "room-1", roomHostAgentId: "alpha", roomMembers: ROOM_GOAL_MEMBERS, sessionKey: "session-1" };
beforeEach(() => { resetComposerDraftOwnerScope(); });

describe("Room Goal owner drafts", () => {
  it("restores explicit per-session choices and leaves unavailable members visible until the user chooses", () => {
    const { result, rerender } = renderHook(useRoomGoalComposer, { initialProps: base, wrapper: Wrapper });
    expect(result.current.leadAgentId).toBe("alpha");
    act(() => result.current.setLeadAgentId("beta"));
    rerender({ ...base, sessionKey: "session-2" });
    expect(result.current.leadAgentId).toBe("alpha");
    act(() => result.current.setLeadAgentId(""));
    expect(result.current.createDisabledReason).toBe(goalTestI18n("en").t("room.goal_lead_required"));
    rerender(base); expect(result.current.leadAgentId).toBe("beta");
    rerender({ ...base, roomMembers: ROOM_GOAL_MEMBERS.slice(0, 1) });
    expect(result.current.leadAgentId).toBe("beta");
    expect(result.current.createDisabledReason).toBe(goalTestI18n("en").t("room.goal_lead_unavailable"));
    const key = buildComposerDraftScopeKey(base);
    expect(useComposerDraftStore.getState().drafts_by_scope[key].goalLeadAgentId).toBe("beta");
    act(() => result.current.setLeadAgentId("alpha"));
    expect(result.current.createDisabledReason).toBeNull();
    rerender(base); expect(result.current.leadAgentId).toBe("alpha");
  });
  it("does not erase an existing choice while the roster is empty, and honors the shared owner reset", () => {
    const { result, rerender } = renderHook(useRoomGoalComposer, { initialProps: base, wrapper: Wrapper });
    act(() => result.current.setLeadAgentId("beta"));
    rerender({ ...base, roomMembers: [] });
    expect(result.current.leadAgentId).toBe("beta");
    expect(result.current.createDisabledReason).toBe(goalTestI18n("en").t("room.goal_no_assignable_agent"));
    rerender(base); expect(result.current.leadAgentId).toBe("beta");
    act(() => resetComposerDraftOwnerScope());
    expect(result.current.leadAgentId).toBe("alpha");
  });
});

describe("Room Goal host dispatch", () => {
  function conversation(): Parameters<typeof useGroupChatComposerModel>[0]["conversation"] {
    return {
      command_catalog: { status: "ready", commands: [] }, context_usage: null, context_usage_by_agent: {},
      delete_input_queue_message: vi.fn(async () => {}), enqueue_input_queue_message: vi.fn(async () => {}),
      guide_input_queue_message: vi.fn(async () => {}), input_queue_items: [], is_loading: false,
      pending_agent_slots: [], reorder_input_queue_messages: vi.fn(async () => {}), room_agent_execution_states: [],
      runtime_phase: "idle", send_message: vi.fn(async () => {}), set_goal: vi.fn(async () => {}),
      stop_generation: vi.fn(), stopping_agent_round_ids: [],
    };
  }
  it("rejects a removed member at the command boundary and submits a valid Goal only through set_goal", async () => {
    const transport = conversation();
    const { result, rerender } = renderHook((props: typeof base) => {
      const goal = useRoomGoalComposer(props);
      const composer = useGroupChatComposerModel({
        ...props, agentId: "alpha", conversationId: "conversation-1", conversation: transport,
        goal, initialDraft: null, runtimeKind: "nxs", scrollToBottom: vi.fn(),
      });
      return { goal, composer };
    }, { initialProps: base, wrapper: Wrapper });
    act(() => result.current.goal.setLeadAgentId("beta"));
    rerender({ ...base, roomMembers: ROOM_GOAL_MEMBERS.slice(0, 1) });
    await expect(result.current.composer.onCreateGoal!("Objective")).rejects.toThrow(goalTestI18n("en").t("room.goal_lead_unavailable"));
    expect(transport.set_goal).not.toHaveBeenCalled();
    expect(transport.send_message).not.toHaveBeenCalled();
    act(() => result.current.goal.setLeadAgentId("alpha"));
    await result.current.composer.onCreateGoal!("Objective");
    expect(transport.set_goal).toHaveBeenCalledExactlyOnceWith("Objective", {
      replace_existing: true, target_agent_ids: ["alpha"], token_budget: null,
    });
    expect(result.current.composer.goalScopeLabel).toBe("Room Goal");
    expect(transport.send_message).not.toHaveBeenCalled();
  });
});
