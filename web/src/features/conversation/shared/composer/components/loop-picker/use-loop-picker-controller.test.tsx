// INPUT: 真实 Loop 选择控制器、隔离的目录结果与可控完成时机。
// OUTPUT: 单次启动、失败后重试及关闭旧窗口后不影响新窗口的回归。
// POS: 不调用真实启动命令；保留异步业务结果与 UI 生命周期的边界。

import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { LoopCatalogItem } from "@/types/capability/loop";
import { useLoopPickerController } from "./use-loop-picker-controller";

const list = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api/capability/loop-api", () => ({ listLoopsApi: list }));
const LOOP: LoopCatalogItem = { id: "verify", slug: "verify", title: "Verify", category: "Quality", description: "Review the result.",
  trigger_type: "manual", tags: [], compatible_agents: [], trigger_config: {}, steps: [],
  exit_condition: { type: "manual", description: "Stop after review." }, kickoff_prompt: "Verify", install_bundle: {}, best_for_agents: [],
  author: "Nexus", author_slug: "nexus", author_official: true, source: "builtin", guardrails: [], examples: [],
  copies: 0, installs: 0, views: 0, featured: false, is_published: true, created_at: "" };
function deferred() {
  let resolve!: () => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<void>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
beforeEach(() => list.mockResolvedValue([LOOP]));

describe("Loop picker selection lifecycle", () => {
  it("claims the selection synchronously, blocks repeated commands, and releases after failure", async () => {
    const pending = deferred();
    const onClose = vi.fn();
    const onSelect = vi.fn(() => pending.promise);
    const { result } = renderHook(() => useLoopPickerController({ onClose, onSelect }), { wrapper: I18nProvider });
    await waitFor(() => expect(result.current.state.hasSnapshot).toBe(true));
    let completion!: Promise<void>;
    act(() => {
      completion = result.current.actions.selectLoop(LOOP);
      void result.current.actions.selectLoop(LOOP);
    });
    expect(onSelect).toHaveBeenCalledExactlyOnceWith(LOOP);
    await act(async () => { pending.reject(new Error("Unavailable")); await completion; });
    expect(result.current.state.busySlug).toBeNull();
    expect(result.current.state.actionError).toBeTruthy();
    expect(onClose).not.toHaveBeenCalled();
    onSelect.mockResolvedValueOnce(undefined);
    await act(async () => result.current.actions.selectLoop(LOOP));
    expect(onSelect).toHaveBeenCalledTimes(2);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it.each(["success", "failure"])("ignores a late %s after its picker closes", async (outcome) => {
    const pending = deferred();
    const onClose = vi.fn();
    const onSelect = vi.fn(() => pending.promise);
    const first = renderHook(() => useLoopPickerController({ onClose, onSelect }), { wrapper: I18nProvider });
    await waitFor(() => expect(first.result.current.state.hasSnapshot).toBe(true));
    let completion!: Promise<void>;
    act(() => { completion = first.result.current.actions.selectLoop(LOOP); });
    first.unmount();
    const next = renderHook(() => useLoopPickerController({ onClose, onSelect }), { wrapper: I18nProvider });
    await waitFor(() => expect(next.result.current.state.hasSnapshot).toBe(true));
    await act(async () => {
      if (outcome === "success") pending.resolve(); else pending.reject(new Error("Old result"));
      await completion;
    });
    expect(onClose).not.toHaveBeenCalled();
    expect(next.result.current.state.busySlug).toBeNull();
    expect(next.result.current.state.actionError).toBeNull();
  });
});
