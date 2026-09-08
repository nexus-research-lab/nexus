// INPUT: 问答作用域、待返回的提交、拒绝结果和组件生命周期。
// OUTPUT: 证明重复提交保护与旧作用域/StrictMode 的提交收口互不污染。
// POS: 问答提交控制器回归；传输受理仍由调用方返回布尔事实。

import { act, renderHook } from "@testing-library/react";
import { StrictMode } from "react";
import { describe, expect, it, vi } from "vitest";
import { useQuestionSubmission } from "./use-question-submission";

const defaults = () => ({
  draft: [{ customAnswer: "", selectedOptions: new Set(["Yes"]) }],
  onAccepted: vi.fn(), onCollapse: vi.fn(), onSubmit: vi.fn(async () => true),
  scopeKey: "A", submissionReady: true, toolUseId: "tool-A",
});

describe("question submission scope", () => {
  it("accepts a current submission after StrictMode effect replay", async () => {
    const props = defaults();
    const { result } = renderHook(() => useQuestionSubmission(props), { wrapper: StrictMode });
    await act(async () => { await result.current.submit(); });
    expect(props.onAccepted).toHaveBeenCalledOnce();
    expect(props.onCollapse).toHaveBeenCalledOnce();
    expect(result.current.isSubmitting).toBe(false);
  });

  it("does not apply an old response or release a new attempt after A to B to A", async () => {
    let finishOld!: (accepted: boolean) => void;
    let finishNew!: (accepted: boolean) => void;
    const props = defaults();
    props.onSubmit.mockImplementationOnce(() => new Promise<boolean>((resolve) => { finishOld = resolve; }))
      .mockImplementationOnce(() => new Promise<boolean>((resolve) => { finishNew = resolve; }));
    const { result, rerender } = renderHook(({ scopeKey }) => useQuestionSubmission({ ...props, scopeKey }), { initialProps: { scopeKey: "A" } });
    let oldAttempt!: Promise<void>;
    act(() => { oldAttempt = result.current.submit(); });
    rerender({ scopeKey: "B" });
    rerender({ scopeKey: "A" });
    let newAttempt!: Promise<void>;
    act(() => { newAttempt = result.current.submit(); });
    expect(props.onSubmit).toHaveBeenCalledTimes(2);
    await act(async () => { finishOld(true); await oldAttempt; });
    expect(props.onAccepted).not.toHaveBeenCalled();
    expect(result.current.isSubmitting).toBe(true);
    await act(async () => { finishNew(true); await newAttempt; });
    expect(props.onAccepted).toHaveBeenCalledOnce();
    expect(result.current.isSubmitting).toBe(false);
  });

  it("blocks same-turn duplicates and preserves the answer after explicit non-acceptance", async () => {
    let finish!: (accepted: boolean) => void;
    const props = defaults();
    props.onSubmit.mockImplementationOnce(() => new Promise<boolean>((resolve) => { finish = resolve; }));
    const { result } = renderHook(() => useQuestionSubmission(props));
    let attempt!: Promise<void>;
    act(() => { attempt = result.current.submit(); void result.current.submit(); });
    expect(props.onSubmit).toHaveBeenCalledOnce();
    await act(async () => { finish(false); await attempt; });
    expect(props.onAccepted).not.toHaveBeenCalled();
    expect(props.onCollapse).not.toHaveBeenCalled();
    expect(result.current.submitEnabled).toBe(true);
    await act(async () => { await result.current.submit(); });
    expect(props.onSubmit.mock.calls[1]).toEqual(props.onSubmit.mock.calls[0]);
  });

  it("cleans up rejection and unmount without accepting or replaying", async () => {
    const props = defaults();
    const failure = new Error("submission failed");
    props.onSubmit.mockRejectedValueOnce(failure);
    const { result, unmount } = renderHook(() => useQuestionSubmission(props));
    await act(async () => { await expect(result.current.submit()).rejects.toBe(failure); });
    expect(result.current.isSubmitting).toBe(false);
    let finish!: (accepted: boolean) => void;
    props.onSubmit.mockImplementationOnce(() => new Promise<boolean>((resolve) => { finish = resolve; }));
    let attempt!: Promise<void>;
    act(() => { attempt = result.current.submit(); });
    unmount();
    await act(async () => { finish(true); await attempt; });
    expect(props.onAccepted).not.toHaveBeenCalled();
    expect(props.onCollapse).not.toHaveBeenCalled();
    expect(props.onSubmit).toHaveBeenCalledTimes(2);
  });
});
