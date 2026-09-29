// INPUT: Provider credential commands and subsequent directory refresh outcomes.
// OUTPUT: Distinct disable/replace/clear payloads, command exclusion and committed feedback.
// POS: Provider persistence regression; fake API, real command serialization.
import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { ProviderConfigRecord } from "@/types/capability/provider";
import { toProviderDraft } from "../../model/provider-config-model";
import { useProviderCommand } from "../use-provider-command";
import { useProviderPersistence } from "./use-provider-persistence";

function setup(refreshed = true, canManage = true) {
  const record = { id: "p1", provider: "example", provider_kind: "llm", preset_key: "custom",
    api_format: "responses", display_name: "Example", base_url: "https://example.test",
    models_path: "/models", enabled: true, auth_token_masked: "test-****", can_manage: canManage,
    usage_count: 0, configuration_version: 1, visibility: "private" } as ProviderConfigRecord;
  const updateConfig = vi.fn().mockResolvedValue(record);
  const refreshAll = vi.fn().mockResolvedValue(refreshed);
  const setFeedback = vi.fn();
  const updateDraft = vi.fn();
  const hook = renderHook(() => {
    const command = useProviderCommand();
    return useProviderPersistence({ currentPreset: null,
      draft: { ...toProviderDraft(record), auth_token: "replacement-test-key" },
      isCreating: false, isEditing: true, isEmptyMode: false,
      providerApi: { createConfig: vi.fn(), updateConfig }, refreshAll,
      runCommand: command.runCommand, selectedCanManage: canManage, selectedRecord: record,
      setFeedback, t: (key) => key, updateDraft, visibilityScope: "private" });
  });
  return { ...hook, updateConfig, refreshAll, setFeedback, updateDraft };
}

describe("Provider credential persistence", () => {
  it.each([false, true])("sets enabled=%s without sending the credential draft", async (enabled) => {
    const test = setup();
    act(() => test.result.current.handleEnabledChange(enabled));
    await waitFor(() => expect(test.refreshAll).toHaveBeenCalledOnce());
    expect(test.updateConfig).toHaveBeenCalledOnce();
    const payload = test.updateConfig.mock.calls[0][1];
    expect(payload.enabled).toBe(enabled);
    expect(payload).not.toHaveProperty("auth_token");
  });

  it("saves a replacement key without changing the Provider's enabled state", async () => {
    const test = setup();
    await act(async () => { await test.result.current.persistProvider(); });
    expect(test.updateConfig).toHaveBeenCalledExactlyOnceWith("example", expect.objectContaining({
      auth_token: "replacement-test-key", enabled: true,
    }));
  });

  it.each([true, false])("clears and disables once, preserving committed feedback when refresh=%s", async (refreshed) => {
    const test = setup(refreshed);
    act(() => {
      test.result.current.handleClearAuthToken();
      test.result.current.handleClearAuthToken();
    });
    await waitFor(() => expect(test.setFeedback).toHaveBeenCalledOnce());
    expect(test.updateConfig).toHaveBeenCalledExactlyOnceWith("example", expect.objectContaining({
      auth_token: "", enabled: false,
    }));
    expect(test.refreshAll).toHaveBeenCalledExactlyOnceWith("example");
    expect(test.setFeedback.mock.calls[0][0]).toMatchObject(refreshed
      ? { tone: "success", title: "settings.providers.key_cleared_title" }
      : { tone: "warning", recoveryAction: "refresh", impact: "state.committed_refresh_impact" });
  });

  it("does not clear or disable a read-only Provider", async () => {
    const test = setup(true, false);
    await act(async () => {
      test.result.current.handleClearAuthToken();
      test.result.current.handleEnabledChange(false);
    });
    expect(test.updateConfig).not.toHaveBeenCalled();
  });
});
