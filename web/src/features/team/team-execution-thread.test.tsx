// INPUT: 本机精确轮次、历史失败与权限事件。
// OUTPUT: Thread 复用边界的回归检查。
// POS: 不启动运行时、不发送真实权限决定。
import { act, fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { TeamExecutionObserver, TeamExecutionThread } from "./team-execution-thread";
import { useTeamRefresh } from "./use-team-refresh";

const model = vi.hoisted(() => ({ load: vi.fn(), stop: vi.fn(), panel: vi.fn(), session: vi.fn() }));

it("observes the native Room before a job exists, ignores stream chunks and reconciles reconnects", () => {
  const binding = {agent_id: "remote", local_agent_id: "local", room_id: "room", conversation_id: "conversation"};
  const changed = vi.fn();
  const state = {round_id: "round", agent_id: "local", agent_round_id: "execution", phase: "active", status: "streaming"};
  model.session.mockReturnValue({ws_state: "connected", room_agent_execution_states: []});
  const view = render(<TeamExecutionObserver binding={binding} onChange={changed} />);
  expect(model.session.mock.lastCall?.[0].identity).toMatchObject({chat_type: "group", room_id: "room", conversation_id: "conversation"});
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

it("releases native controls when the binding unmounts", () => {
  const binding = {agent_id: "remote", local_agent_id: "local", room_id: "room", conversation_id: "conversation"};
  const controls = vi.fn();
  const session = {pending_permissions: [], stop_generation: vi.fn(), send_permission_response: vi.fn(), room_agent_execution_states: [], stopping_agent_round_ids: []};
  model.session.mockReturnValue(session);
  const view = render(<TeamExecutionObserver binding={binding} onChange={vi.fn()} onControls={controls} />);
  expect(controls).toHaveBeenLastCalledWith("conversation", session);
  view.unmount();
  expect(controls).toHaveBeenLastCalledWith("conversation", null);
});

it("does not poll native jobs and coalesces notifications arriving during a read", async () => {
  vi.useFakeTimers();
  try {
    let finish!: () => void;
    const load = vi.fn().mockImplementationOnce(() => new Promise<void>((resolve) => { finish = resolve; })).mockResolvedValue(undefined);
    const {result, unmount} = renderHook(() => useTeamRefresh("room", load, false));
    act(() => { result.current(); result.current(); });
    expect(load).toHaveBeenCalledTimes(1);
    await act(async () => { finish(); });
    expect(load).toHaveBeenCalledTimes(2);
    await act(async () => { await vi.advanceTimersByTimeAsync(60_000); });
    expect(load).toHaveBeenCalledTimes(2);
    unmount();
  } finally { vi.useRealTimers(); }
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
  model.load.mockResolvedValueOnce(false).mockResolvedValue(true);
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
});

it("separates history fetching from execution and reloads after the final output arrives", async () => {
  model.load.mockReset().mockResolvedValue(true);
  model.session.mockReturnValue({load_round_window: model.load, stop_generation: model.stop,
    is_history_loading: true, is_session_loading: true, pending_permissions: [], messages: [],
  });
  const job = {id: "job", agent_id: "remote", state: "running" as const, room_id: "room", conversation_id: "conversation", local_agent_id: "local", round_id: "round"};
  const view = render(<TeamExecutionThread job={job} name="Agent" compact={false} onClose={vi.fn()} />);
  await waitFor(() => expect(model.load).toHaveBeenCalledTimes(1));
  expect(model.panel.mock.lastCall?.[0].isLoading).toBe(true);
  view.rerender(<TeamExecutionThread job={job} hasFinalOutput name="Agent" compact={false} onClose={vi.fn()} />);
  await waitFor(() => expect(model.load).toHaveBeenCalledTimes(2));
  expect(model.panel.mock.lastCall?.[0].isLoading).toBe(false);
});

it("renders an interrupted job as stopped instead of an execution error", async () => {
  model.load.mockReset().mockResolvedValue(true);
  model.session.mockReturnValue({load_round_window: model.load, stop_generation: model.stop, pending_permissions: [], messages: []});
  render(<TeamExecutionThread job={{id: "job", agent_id: "remote", state: "cancelled", room_id: "room", conversation_id: "conversation", local_agent_id: "local", round_id: "round"}} name="Agent" compact={false} onClose={vi.fn()} />);
  await waitFor(() => expect(model.load).toHaveBeenCalledTimes(1));
  expect(model.panel.mock.lastCall?.[0].unresolvedToolStatus).toBe("stopped");
  expect(model.panel.mock.lastCall?.[0].isLoading).toBe(false);
});
