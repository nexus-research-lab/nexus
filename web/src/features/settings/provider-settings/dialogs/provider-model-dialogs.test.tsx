// INPUT: 三类 Provider 弹窗、模型草稿、能力开关、使用者目录与命令状态。
// OUTPUT: 证明焦点/字段身份、草稿更新、具名忙碌动作和使用者展示不改变命令目标。
// POS: 真实弹窗行为测试；命令为本地回调，不执行 Provider 请求或删除。

import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { ProviderConfigRecord, ProviderModelRecord } from "@/types/capability/provider";

import type { ModelOptionsState } from "../model/provider-settings-types";
import { ProviderAddModelDialog } from "./provider-settings-add-model-dialog";
import { ProviderDeleteUsageDialog } from "./provider-settings-delete-usage-dialog";
import { ProviderModelOptionsDialog } from "./provider-settings-model-options-dialog";

const MODEL: ProviderModelRecord = {
  id: "model-record", provider_id: "provider-record", model_id: "tenant/deployment/long-model-identity",
  display_name: "Model", category: "chat", enabled: true, is_default: false,
  capabilities_auto: {}, capabilities_override: {}, provider_options: { budget: 1 },
};
const OPTIONS: ModelOptionsState = {
  model: MODEL,
  capabilities: { vision: true, image_output: false, tool_calling: true, reasoning: false, embedding: false },
  context_window: "128000", max_output_tokens: "4000", provider_options_text: '{"budget":1}',
};
const PROVIDER: ProviderConfigRecord = {
  id: "provider-record", visibility: "private", provider_kind: "llm", provider: "example", preset_key: "custom",
  api_format: "responses", display_name: "Example Provider", auth_token_masked: "", base_url: "https://example.com",
  models_path: "/models", enabled: true, usage_count: 3, last_test_status: "", last_test_error: "",
  configuration_version: 1, can_manage: true, agent_runtime_supported: true, models: [MODEL],
  used_by_agents: [
    { agent_id: "private-agent-one", name: "worker", display_name: "Readable Agent", is_main: true },
    { agent_id: "private-agent-two", name: "  Named Worker  ", display_name: "  ", is_main: false },
    { agent_id: "private-agent-three", name: "", display_name: "", is_main: false },
  ],
};
const I18N = { locale: "en" as const, setLocale: () => undefined, t: (key: string) => key };
function view(children: ReactNode) { return <I18N_CONTEXT.Provider value={I18N}>{children}</I18N_CONTEXT.Provider>; }

describe("Provider model dialogs", () => {
  it("uses shared initial focus and restores the opener while preserving raw add-model input and enable choice", async () => {
    const user = userEvent.setup();
    const onAdd = vi.fn();
    function Harness() {
      const [open, setOpen] = useState(false);
      const [id, setId] = useState("");
      const [enabled, setEnabled] = useState(true);
      return <>
        <button onClick={() => setOpen(true)}>Open model</button>
        <ProviderAddModelDialog isOpen={open} manualModelEnabled={enabled} manualModelId={id} manualModelPlaceholder="model"
          onAdd={() => onAdd({ id, enabled })} onClose={() => setOpen(false)} pendingAction={null}
          selectedCanManage setManualModelEnabled={setEnabled} setManualModelId={setId} />
      </>;
    }
    render(view(<Harness />));
    const opener = screen.getByRole("button", { name: "Open model" });
    await user.click(opener);
    const input = screen.getByRole("textbox", { name: "settings.providers.model_id" });
    await waitFor(() => expect(document.activeElement).toBe(input));
    const description = document.getElementById(input.getAttribute("aria-describedby")!);
    expect(description?.textContent).toBe("settings.providers.add_model_description");
    await user.type(input, "  tenant/model  ");
    const enable = screen.getByRole("switch", { name: "settings.providers.enable_after_add" });
    expect(document.getElementById(enable.getAttribute("aria-describedby")!)?.textContent)
      .toBe("settings.providers.enable_after_add_description");
    await user.click(enable);
    await user.click(screen.getByRole("button", { name: "settings.providers.add" }));
    expect(onAdd).toHaveBeenCalledWith({ id: "  tenant/model  ", enabled: false });
    await user.click(screen.getByRole("button", { name: "common.cancel" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    await waitFor(() => expect(document.activeElement).toBe(opener));
  });

  it("updates each model capability independently and keeps raw numeric and JSON drafts", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    function Harness() {
      const [options, setOptions] = useState<ModelOptionsState | null>(OPTIONS);
      return <>
        <ProviderModelOptionsDialog modelOptions={options} onClose={vi.fn()} onSave={onSave} pendingAction={null}
          selectedCanManage setModelOptions={setOptions} />
        <output data-testid="options">{JSON.stringify(options)}</output>
      </>;
    }
    render(view(<Harness />));
    const draft = () => JSON.parse(screen.getByTestId("options").textContent!) as ModelOptionsState;
    expect(screen.getByText(MODEL.model_id)).toBeTruthy();
    for (const key of ["vision", "image_output", "tool_calling", "reasoning", "embedding"] as const) {
      const before = draft();
      await user.click(screen.getByRole("switch", { name: `settings.providers.capability_${key}` }));
      expect(draft()).toEqual({ ...before, capabilities: { ...before.capabilities, [key]: !before.capabilities[key] } });
    }
    await user.click(screen.getAllByText("settings.providers.provider_options_json").find((node) => node.closest("summary"))!);
    for (const [label, key, value] of [
      ["context_window", "context_window", " 256000 "],
      ["max_output_tokens", "max_output_tokens", ""],
      ["provider_options_json", "provider_options_text", "{ unfinished JSON"],
    ] as const) {
      const input = screen.getByLabelText(`settings.providers.${label}`);
      fireEvent.change(input, { target: { value } });
      expect(draft()[key]).toBe(value);
      expect(onSave).not.toHaveBeenCalled();
    }
    await user.click(screen.getByRole("button", { name: "common.save" }));
    expect(onSave).toHaveBeenCalledOnce();
  });

  it("keeps model saving named and blocks add/save commands according to their existing busy and permission flags", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    const onAdd = vi.fn();
    const renderOptions = (saving: boolean, canManage: boolean) => view(<ProviderModelOptionsDialog modelOptions={OPTIONS}
      onClose={vi.fn()} onSave={onSave} pendingAction={saving ? { kind: "save-model-options", modelId: MODEL.model_id } : null}
      selectedCanManage={canManage} setModelOptions={vi.fn()} />);
    const { rerender } = render(renderOptions(true, true));
    const saving = screen.getByRole("button", { name: "common.saving" }) as HTMLButtonElement;
    expect(saving.disabled).toBe(true);
    expect(saving.getAttribute("aria-busy")).toBe("true");
    await user.click(saving);
    expect(onSave).not.toHaveBeenCalled();
    rerender(renderOptions(false, false));
    expect((screen.getByRole("button", { name: "common.save" }) as HTMLButtonElement).disabled).toBe(true);
    rerender(view(<ProviderAddModelDialog isOpen manualModelEnabled manualModelId="model" manualModelPlaceholder=""
      onAdd={onAdd} onClose={vi.fn()} pendingAction={{ kind: "add-model", modelId: "model" }} selectedCanManage
      setManualModelEnabled={vi.fn()} setManualModelId={vi.fn()} />));
    const add = screen.getByRole("button", { name: "settings.providers.add_and_enable" }) as HTMLButtonElement;
    expect(add.disabled).toBe(true);
    expect(add.getAttribute("aria-busy")).toBe("true");
    await user.click(add);
    expect(onAdd).not.toHaveBeenCalled();
  });

  it.each([
    { canManage: false, pendingAction: null },
    { canManage: true, pendingAction: { kind: "test-provider" as const } },
  ])("locks add and options drafts across permission and unrelated busy states: %j", async ({ canManage, pendingAction }) => {
    const user = userEvent.setup();
    const onAdd = vi.fn();
    const setId = vi.fn();
    const setEnabled = vi.fn();
    const setOptions = vi.fn();
    const { rerender } = render(view(<ProviderAddModelDialog isOpen manualModelEnabled manualModelId="model" manualModelPlaceholder=""
      onAdd={onAdd} onClose={vi.fn()} pendingAction={pendingAction} selectedCanManage={canManage}
      setManualModelEnabled={setEnabled} setManualModelId={setId} />));
    const input = screen.getByRole("textbox") as HTMLInputElement;
    expect(input.disabled).toBe(true);
    await user.type(input, "changed");
    await user.click(screen.getByRole("switch"));
    fireEvent.submit(input.closest("form")!);
    expect(onAdd).not.toHaveBeenCalled();
    expect(setId).not.toHaveBeenCalled();
    expect(setEnabled).not.toHaveBeenCalled();
    rerender(view(<ProviderModelOptionsDialog modelOptions={OPTIONS} onClose={vi.fn()} onSave={vi.fn()}
      pendingAction={pendingAction} selectedCanManage={canManage} setModelOptions={setOptions} />));
    for (const input of screen.getAllByRole("textbox")) expect((input as HTMLInputElement).disabled).toBe(true);
    for (const control of screen.getAllByRole("switch")) {
      expect((control as HTMLButtonElement).disabled).toBe(true);
      await user.click(control);
    }
    expect((screen.getByRole("button", { name: "common.save" }) as HTMLButtonElement).disabled).toBe(true);
    expect(setOptions).not.toHaveBeenCalled();
  });

  it("binds every dialog title and editable field to its own instance", () => {
    render(view(<>{["first", "second"].map((key) => <div key={key}>
      <ProviderAddModelDialog isOpen manualModelEnabled manualModelId={key} manualModelPlaceholder=""
        onAdd={vi.fn()} onClose={vi.fn()} pendingAction={null} selectedCanManage setManualModelEnabled={vi.fn()} setManualModelId={vi.fn()} />
      <ProviderModelOptionsDialog modelOptions={OPTIONS} onClose={vi.fn()} onSave={vi.fn()} pendingAction={null}
        selectedCanManage setModelOptions={vi.fn()} />
      <ProviderDeleteUsageDialog deleteTargetRecord={PROVIDER} isOpen onCancel={vi.fn()} onForceDelete={vi.fn()} pendingAction={null} />
    </div>)}</>));
    const ids = [...document.querySelectorAll("[id]")].map((element) => element.id);
    expect(new Set(ids).size).toBe(ids.length);
    const dialogs = document.querySelectorAll('[role="dialog"]');
    expect(dialogs).toHaveLength(6);
    for (const dialog of dialogs) {
      expect(dialog.contains(document.getElementById(dialog.getAttribute("aria-labelledby")!))).toBe(true);
      for (const label of dialog.querySelectorAll<HTMLLabelElement>("label[for]")) {
        expect(label.control?.closest('[role="dialog"]')).toBe(dialog);
      }
    }
  });

  it("shows readable usage names, replaces missing names without exposing IDs and preserves explicit deletion", async () => {
    const user = userEvent.setup();
    const onForceDelete = vi.fn();
    const onCancel = vi.fn();
    const { rerender } = render(view(<ProviderDeleteUsageDialog deleteTargetRecord={PROVIDER} isOpen
      onCancel={onCancel} onForceDelete={onForceDelete} pendingAction={null} />));
    const dialog = screen.getByRole("dialog");
    for (const name of ["Readable Agent", "Named Worker", "settings.providers.agent_name_unavailable"]) {
      expect(within(dialog).getByText(name)).toBeTruthy();
    }
    for (const agent of PROVIDER.used_by_agents) expect(dialog.textContent).not.toContain(agent.agent_id);
    expect(within(dialog).getByText("settings.providers.main_agent_badge")).toBeTruthy();
    expect(onForceDelete).not.toHaveBeenCalled();
    await user.click(within(dialog).getByRole("button", { name: "settings.providers.force_delete" }));
    expect(onForceDelete).toHaveBeenCalledOnce();
    rerender(view(<ProviderDeleteUsageDialog deleteTargetRecord={PROVIDER} isOpen onCancel={onCancel}
      onForceDelete={onForceDelete} pendingAction={{ kind: "delete-provider" }} />));
    const button = screen.getByRole("button", { name: "settings.providers.force_delete" }) as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    await user.click(button);
    expect(onForceDelete).toHaveBeenCalledOnce();
  });

  it("uses the stale-usage explanation when the current agent directory is empty", () => {
    render(view(<ProviderDeleteUsageDialog deleteTargetRecord={{ ...PROVIDER, used_by_agents: [] }} isOpen
      onCancel={vi.fn()} onForceDelete={vi.fn()} pendingAction={null} />));
    expect(screen.getByText("settings.providers.delete_usage_stale")).toBeTruthy();
    expect(screen.queryByText("settings.providers.used_by_agents")).toBeNull();
  });
});
