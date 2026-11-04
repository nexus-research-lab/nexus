// INPUT: 真实 Composer 草稿、终态消息和运行/审批状态。
// OUTPUT: 空输入继续、编辑发送与运行停止互斥；提交仍经过同一控制器门禁。
// POS: 共享输入控制器动作切换回归。
import { act, renderHook } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { Message } from "@/types/conversation/message/entity";
import { resetComposerDraftOwnerScope } from "../composer-draft-store";
import type { ComposerPanelProps } from "../composer-model";
import { useComposerController } from "./use-composer-controller";
beforeEach(resetComposerDraftOwnerScope);
function props(): ComposerPanelProps { return {
  compact: false, commandCatalog: { commands: [], status: "cold" }, contextUsage: null,
  draftScopeKey: "s", historyScopeKey: "s", isLoading: false, runtimePhase: "idle", runtimeKind: "nxs", workGraphSessionKey: "s",
  continuationReady: true,
  continuationMessages: [{ role: "user", message_id: "u", round_id: "r" }, { role: "assistant", message_id: "a", agent_id: "agent", round_id: "r", result_summary: { subtype: "interrupted" } }] as Message[],
  onSendMessage: vi.fn().mockResolvedValue(undefined), onStop: vi.fn(),
  inputQueueItems: [], onEnqueueMessage: vi.fn(), onDeleteQueuedMessage: vi.fn(), onGuideQueuedMessage: vi.fn(), onReorderQueueMessages: vi.fn(),
  defaultDeliveryPolicy: "queue", onPrepareAttachments: vi.fn(), goalScopeLabel: "", tourAnchor: "",
}; }
it("changes empty-input continuation to ordinary send when typing, and to stop while running", async () => {
  const input = props();
  const { result, rerender } = renderHook((value) => useComposerController(value), { initialProps: input, wrapper: ({ children }) => <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>{children}</I18N_CONTEXT.Provider> });
  expect(result.current.state.shouldContinue).toBe(true);
  expect(result.current.state.isSendDisabled).toBe(false);
  act(() => result.current.actions.handleInputChange("new task"));
  expect(result.current.state.shouldContinue).toBe(false);
  expect(result.current.state.sendButtonLabel).toBe("composer.send_message");
  act(() => result.current.actions.handleInputChange(""));
  rerender({ ...input, isLoading: true, runtimePhase: "running" });
  expect(result.current.state.shouldContinue).toBe(false);
  expect(result.current.state.shouldShowStopButton).toBe(true);
  rerender({ ...input, interactionIdentity: "approval" });
  await act(async () => result.current.actions.handleSend());
  expect(input.onSendMessage).not.toHaveBeenCalled();
  rerender({ ...input, continuationReady: false });
  await act(async () => result.current.actions.handleSend());
  expect(input.onSendMessage).not.toHaveBeenCalled();
  rerender(input);
  await act(async () => result.current.actions.handleSend());
  expect(input.onSendMessage).toHaveBeenCalledExactlyOnceWith("composer.continue_message", "queue", [], []);
});
