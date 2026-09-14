// INPUT: 已提交的模型删除与随后的目录刷新结果。
// OUTPUT: 验证刷新失败保留已提交/待刷新反馈，不重发删除或伪装完整成功。
// POS: Provider 模型动作的恢复回归。
import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { ProviderConfigRecord, ProviderModelRecord } from "@/types/capability/provider";
import { useProviderModelUpdate } from "./use-provider-model-update";

describe("model deletion refresh feedback", () => {
  it.each([false, true])("preserves committed deletion when refresh returns %s", async (refreshed) => {
    const model = { model_id: "model", display_name: "Model", is_default: false } as ProviderModelRecord;
    const selectedRecord = { provider: "provider", can_manage: true } as ProviderConfigRecord;
    const deleteModel = vi.fn().mockResolvedValue(undefined);
    const refreshAll = vi.fn().mockResolvedValue(refreshed);
    const setFeedback = vi.fn();
    const { result } = renderHook(() => useProviderModelUpdate({
      modelApi: { deleteModel, updateModel: vi.fn(), setDefaultModel: vi.fn() },
      modelOptions: null, refreshAll, runCommand: async (_action, command) => command(),
      selectedCanManage: true, selectedRecord, setFeedback, setModelOptions: vi.fn(), t: (key) => key,
    }));
    act(() => result.current.handleRequestDeleteModel(model));
    act(() => result.current.handleDeleteModel());
    await waitFor(() => expect(setFeedback).toHaveBeenCalledOnce());
    expect(deleteModel).toHaveBeenCalledExactlyOnceWith("provider", "model");
    expect(refreshAll).toHaveBeenCalledExactlyOnceWith("provider");
    expect(result.current.deleteModelTarget).toBeNull();
    expect(setFeedback.mock.calls[0][0]).toMatchObject(refreshed
      ? { tone: "success", title: "settings.providers.model_deleted_title" }
      : { recoveryAction: "refresh", tone: "warning", impact: "state.committed_refresh_impact" });
  });
});
