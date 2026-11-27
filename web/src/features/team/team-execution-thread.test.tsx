// INPUT: 本机精确轮次、历史失败与权限事件。
// OUTPUT: Thread 复用边界的回归检查。
// POS: 不启动运行时、不发送真实权限决定。
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { TeamExecutionObserver, TeamExecutionThread } from "./team-execution-thread";

const model = vi.hoisted(() => ({ load: vi.fn(), stop: vi.fn(), panel: vi.fn(), session: vi.fn(), recover: vi.fn() }));
vi.mock("@/lib/api/conversation/team-node-api", () => ({recoverTeamJob: model.recover}));

it("keeps unknown execution recoverable after a rejected verification", async () => {
  model.load.mockResolvedValue(true);
  model.session.mockReturnValue({load_round_window: model.load, messages: [], pending_permissions: []});
  model.recover.mockRejectedValueOnce(new Error("not stopped")).mockResolvedValueOnce(undefined);
  render(<TeamExecutionThread job={{id: "unknown", agent_id: "remote", state: "review_required", room_id: "room", conversation_id: "conversation", local_agent_id: "local", round_id: "round"}} name="Agent" compact={false} onClose={vi.fn()} />);
  fireEvent.click(await screen.findByRole("button", {name: "team.recover_execution"}));
  expect((await screen.findByRole("alert")).textContent).toContain("team.recovery_failed");
  fireEvent.click(screen.getByRole("button", {name: "team.recover_execution"}));
  await waitFor(() => expect(model.recover).toHaveBeenCalledTimes(2));
  expect(model.recover).toHaveBeenLastCalledWith("unknown");
});

it("observes the native Room before a job exists, ignores stream chunks and reconciles reconnects", () => {
  const binding = {agent_id: "remote", local_agent_id: "local", room_id: "room", conversation_id: "conversation"};
  const changed = vi.fn();
  const state = {round_id: "round", agent_id: "local", agent_round_id: "execution", phase: "active", status: "streaming"};
  model.session.mockReturnValue({ws_state: "connected", room_agent_execution_states: []});
  const view = render(<TeamExecutionObserver binding={binding} onChange={changed} />);
  expect(model.session.mock.lastCall?.[0].identity).toMatchObject({agent_id: "local", chat_type: "group", room_id: "room", conversation_id: "conversation"});
  expect(changed).toHaveBeenCalledTimes(1);
  model.session.mockReturnValue({ws_state: "connected", room_agent_execution_states: [state]});
  view.rerender(<TeamExecutionObserver binding={binding} onChange={changed} />);
  expect(changed).toHaveBeenCalledTimes(2);
  model.session.mockReturnValue({ws_state: "connected", room_agent_execution_states: [{...state}], messages: [{content: "delta"}]});
  view.rerender(<TeamExecutionObserver binding={binding} onChange={changed} />);
  expect(changed).toHaveBeenCalledTimes(2);
  model.session.mockReturnValue({ws_state: "disconnected", room_agent_execution_states: [state]});
  view.rerender(<TeamExecutionObserver binding={binding} onChange={changed} />);
  model.session.mockReturnValue({ws_state: "connected", room_agent_execution_states: [state]});
  view.rerender(<TeamExecutionObserver binding={binding} onChange={changed} />);
  expect(changed).toHaveBeenCalledTimes(4);
});

it("reacts to admission slots without waiting for the first model stream", () => {
  const binding = {agent_id: "remote", local_agent_id: "local", room_id: "room", conversation_id: "conversation"};
  const changed = vi.fn();
  model.session.mockReturnValue({ws_state: "connected", room_agent_execution_states: [], pending_agent_slots: []});
  const view = render(<TeamExecutionObserver binding={binding} onChange={changed} />);
  model.session.mockReturnValue({ws_state: "connected", room_agent_execution_states: [], pending_agent_slots: [{round_id: "round", agent_id: "local", status: "pending"}]});
  view.rerender(<TeamExecutionObserver binding={binding} onChange={changed} />);
  expect(changed).toHaveBeenCalledTimes(2);
});
vi.mock("@/shared/i18n/i18n-context", () => ({ useI18n: () => ({ t: (key: string) => key }) }));
vi.mock("@/hooks/agent/use-agent-conversation", () => ({ useAgentConversation: model.session }));
vi.mock("@/features/conversation/shared/thread/conversation-thread-panel", () => ({
  ConversationThreadPanel: (props: {footer: React.ReactNode; onStopMessage: (id: string) => void}) => {
    model.panel(props);
    return <>{props.footer}<button onClick={() => props.onStopMessage("message")}>stop</button></>;
  },
}));

it("retries history and stops only the exact local Agent execution", async () => {
  model.load.mockReset().mockResolvedValueOnce(false).mockResolvedValue(true);
  model.session.mockReturnValue({ load_round_window: model.load, stop_generation: model.stop,
    pending_permissions: [], messages: [
      {message_id: "message", role: "assistant", agent_id: "local", round_id: "round", agent_round_id: "execution", content: []},
      {message_id: "other", role: "assistant", agent_id: "local", round_id: "other", content: []},
    ],
  });
  render(<TeamExecutionThread job={{id: "job", agent_id: "remote", state: "running", room_id: "room", conversation_id: "conversation", local_agent_id: "local", round_id: "round"}} name="Agent" compact={false} onClose={vi.fn()} />);
  fireEvent.click(await screen.findByRole("button", {name: "state.retry"}));
  await waitFor(() => expect(model.load).toHaveBeenCalledTimes(2));
  fireEvent.click(screen.getByRole("button", {name: "stop"}));
  expect(model.stop).toHaveBeenCalledExactlyOnceWith("execution");
  expect(model.panel.mock.lastCall?.[0].messages).toHaveLength(1);
  expect(model.session.mock.lastCall?.[0].identity.chat_type).toBe("group");
  expect(model.session.mock.lastCall?.[0].identity.agent_id).toBe("local");
});
