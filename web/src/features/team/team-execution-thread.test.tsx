// INPUT: 本机精确轮次、历史失败与权限事件。
// OUTPUT: Thread 复用边界的回归检查。
// POS: 不启动运行时、不发送真实权限决定。
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { TeamExecutionThread } from "./team-execution-thread";

const model = vi.hoisted(() => ({ load: vi.fn(), stop: vi.fn(), panel: vi.fn(), session: vi.fn() }));
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
