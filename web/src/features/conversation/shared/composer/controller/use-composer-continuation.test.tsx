// INPUT: DM/Room 消息、暂停队列、WorkGraph 权威资格和发送回执。
// OUTPUT: 继续动作按领域路由，防重且不重放未知写入。
// POS: 共享 Composer 继续协议回归。
import { StrictMode, type ReactNode } from "react";
import { act, renderHook } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import type { Message } from "@/types/conversation/message/entity";
import type { InputQueueItem } from "@/types/agent/agent-conversation";
import type { ExecutionView } from "@/types/conversation/execution";
import type { ExecutionResource } from "../../execution/use-execution-resource";
import { ContinuationContext } from "../../execution/execution-continuation-state";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { useComposerContinuation } from "./use-composer-continuation";
const messages = [{ role: "user", message_id: "u", round_id: "r" }, { role: "assistant", message_id: "a", agent_id: "agent", round_id: "r", agent_round_id: "ar", result_summary: { subtype: "interrupted" } }] as Message[];
function options() { return { scope: "s", sessionKey: "s", messages, ready: true, inputQueueItems: [] as InputQueueItem[], onSendMessage: vi.fn().mockResolvedValue(undefined), onGuideQueuedMessage: vi.fn().mockResolvedValue(undefined) }; }
const strict = ({ children }: { children: ReactNode }) => <StrictMode><I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>{children}</I18N_CONTEXT.Provider></StrictMode>;
it("continues a DM once without rewriting, and directs Room continuation to interrupted members", async () => {
  const input = options();
  const { result, rerender } = renderHook((props) => useComposerContinuation(props), { initialProps: { ...input, memberIds: undefined as string[] | undefined }, wrapper: strict });
  await act(async () => { void result.current.run(); await result.current.run(); });
  expect(input.onSendMessage).toHaveBeenCalledExactlyOnceWith(expect.any(String), "queue", [], []);
  expect(result.current.disabled).toBe(true);
  rerender({ ...input, scope: "room", memberIds: ["agent"] });
  await act(async () => result.current.run());
  expect(input.onSendMessage).toHaveBeenLastCalledWith(expect.any(String), "queue", [], ["agent"]);
});
it("resumes a paused queue rather than adding a synthetic message", async () => {
  const input = { ...options(), inputQueueItems: [{ id: "q", dispatch_paused: true }] as InputQueueItem[] };
  const { result } = renderHook(() => useComposerContinuation(input), { wrapper: strict });
  await act(async () => result.current.run());
  expect(input.onGuideQueuedMessage).toHaveBeenCalledWith("q");
  expect(input.onSendMessage).not.toHaveBeenCalled();
});
it("blocks disconnected sends and unknown acceptance, while allowing an explicit rejection to be retried", async () => {
  const input = options();
  const { result, rerender } = renderHook((props) => useComposerContinuation(props), { initialProps: { ...input, ready: false }, wrapper: strict });
  await act(async () => result.current.run());
  expect(input.onSendMessage).not.toHaveBeenCalled();
  rerender(input);
  input.onSendMessage.mockRejectedValueOnce(new Error("rejected"));
  await act(async () => result.current.run());
  expect(result.current.disabled).toBe(false);
  input.onSendMessage.mockRejectedValueOnce(Object.assign(new Error("unknown"), { name: "RequestAcceptanceUnknownError" }));
  await act(async () => result.current.run());
  await act(async () => result.current.run());
  expect(input.onSendMessage).toHaveBeenCalledTimes(2);
  expect(result.current.disabled).toBe(true);
});
it("uses exact recoverable WorkGraph nodes sequentially and does not bypass stale or denied authority", async () => {
  const input = options();
  const nodes = [{ id: "n1", can_recover: true, attempt_id: "a1", work_item_id: "w1", agent_round_id: "ar" }, { id: "n2", can_recover: true, attempt_id: "a2", work_item_id: "w2" }];
  const resource = { sessionKey: "s", execution: { id: "e", graph: { nodes } } as ExecutionView, isStale: true, isLoading: false } as ExecutionResource;
  let finish!: () => void;
  const run = vi.fn().mockImplementationOnce(() => new Promise<void>((resolve) => { finish = resolve; })).mockResolvedValue(undefined);
  const wrapper = ({ children }: { children: ReactNode }) => <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}><ContinuationContext.Provider value={{ resource, action: { run, states: {} } }}>{children}</ContinuationContext.Provider></I18N_CONTEXT.Provider>;
  const { result, rerender } = renderHook(() => useComposerContinuation(input), { wrapper });
  await act(async () => result.current.run());
  expect(run).not.toHaveBeenCalled();
  resource.isStale = false; rerender();
  let pending!: Promise<void>;
  act(() => { pending = result.current.run(); });
  expect(run).toHaveBeenCalledTimes(1);
  await act(async () => { finish(); await pending; });
  expect(run.mock.calls.map(([node]) => node.attempt_id)).toEqual(["a1", "a2"]);
  expect(input.onSendMessage).not.toHaveBeenCalled();
  nodes.forEach((node) => { node.can_recover = false; }); rerender();
  expect(result.current.available).toBe(false);
});
it("does not apply an old session's late failure to the new session", async () => {
  const input = options(); let reject!: (error: Error) => void;
  input.onSendMessage.mockImplementationOnce(() => new Promise<void>((_, fail) => { reject = fail; }));
  const { result, rerender } = renderHook((props) => useComposerContinuation(props), { initialProps: input, wrapper: strict });
  let pending!: Promise<void>;
  act(() => { pending = result.current.run(); });
  rerender({ ...input, scope: "next" });
  await act(async () => { reject(new Error("old")); await pending; });
  expect(result.current.error).toBeNull();
  expect(result.current.disabled).toBe(false);
});
