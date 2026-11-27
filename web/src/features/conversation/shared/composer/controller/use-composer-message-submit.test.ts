// INPUT: 附件准备失败、明确拒绝和受理未知的发送结果。
// OUTPUT: 所有失败被捕获，只有已认领且明确失败的草稿可恢复。
// POS: Composer 提交事务的异常边界回归。
import { act, renderHook } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { EMPTY_COMPOSER_DRAFT } from "../composer-draft-store";
import { useComposerMessageSubmit } from "./use-composer-message-submit";

afterEach(() => vi.restoreAllMocks());

it("queues input when the runtime phase is active before isLoading catches up", async () => {
  const enqueue = vi.fn().mockResolvedValue(undefined);
  const send = vi.fn().mockResolvedValue(undefined);
  const draft = { ...EMPTY_COMPOSER_DRAFT, input: "follow up" };
  const { result } = renderHook(() => useComposerMessageSubmit({
    attachmentCount: 0,
    claimDraftSubmission: vi.fn(() => draft),
    clearAttachmentError: vi.fn(),
    defaultDeliveryPolicy: "queue",
    input: draft.input,
    isLoading: false,
    isPreparingAttachments: false,
    onEnqueueMessage: enqueue,
    onSendMessage: send,
    prepareAttachments: vi.fn().mockResolvedValue([]),
    queueItemCount: 0,
    queueWhenSessionBusy: true,
    recordHistory: vi.fn(),
    resetTextareaHeight: vi.fn(),
    restoreFailedDraftSubmission: vi.fn(() => true),
    runtimePhase: "streaming",
    targetAgentIDs: [],
  }));

  await act(async () => { await result.current(); });

  expect(enqueue).toHaveBeenCalledWith("follow up", "queue", [], []);
  expect(send).not.toHaveBeenCalled();
});
