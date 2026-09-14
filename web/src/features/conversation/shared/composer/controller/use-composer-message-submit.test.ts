// INPUT: 附件准备失败、明确拒绝和受理未知的发送结果。
// OUTPUT: 所有失败被捕获，只有已认领且明确失败的草稿可恢复。
// POS: Composer 提交事务的异常边界回归。
import { act, renderHook } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { EMPTY_COMPOSER_DRAFT } from "../composer-draft-store";
import { useComposerMessageSubmit } from "./use-composer-message-submit";

afterEach(() => vi.restoreAllMocks());

it.each(["attachments", "rejected", "unknown"])("contains %s failures without losing draft ownership", async (failure) => {
  vi.spyOn(console, "error").mockImplementation(() => {});
  const error = new Error("submission failed");
  if (failure === "unknown") error.name = "RequestAcceptanceUnknownError";
  const draft = { ...EMPTY_COMPOSER_DRAFT, input: "hello" };
  const claimDraftSubmission = vi.fn(() => draft);
  const restoreFailedDraftSubmission = vi.fn(() => true);
  const deliver = vi.fn().mockRejectedValue(error);
  const { result } = renderHook(() => useComposerMessageSubmit({
    attachmentCount: 0, claimDraftSubmission, clearAttachmentError: vi.fn(),
    defaultDeliveryPolicy: "queue", input: draft.input, isLoading: false,
    isPreparingAttachments: false, onEnqueueMessage: deliver, onSendMessage: deliver,
    prepareAttachments: failure === "attachments"
      ? vi.fn().mockRejectedValue(error) : vi.fn().mockResolvedValue([]),
    queueItemCount: 0, queueWhenSessionBusy: false, recordHistory: vi.fn(),
    resetTextareaHeight: vi.fn(), restoreFailedDraftSubmission,
    runtimePhase: null, targetAgentIDs: [],
  }));
  await act(async () => { await expect(result.current()).resolves.toBeUndefined(); });
  expect(claimDraftSubmission).toHaveBeenCalledTimes(failure === "attachments" ? 0 : 1);
  expect(deliver).toHaveBeenCalledTimes(failure === "attachments" ? 0 : 1);
  expect(restoreFailedDraftSubmission).toHaveBeenCalledTimes(failure === "rejected" ? 1 : 0);
});
