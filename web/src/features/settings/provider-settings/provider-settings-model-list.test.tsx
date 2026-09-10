// INPUT: 含默认模型的 Provider 目录与默认模型禁用尝试命令。
// OUTPUT: 证明默认模型只渲染一个可操作 switch，并把尝试交给业务处理。
// POS: Provider 模型目录行为测试；提示内容与 mutation 事务由 action 层负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type {
  ProviderConfigRecord,
  ProviderModelRecord,
} from "@/types/capability/provider";

import { ProviderSettingsModelList } from "./components/provider-settings-model-list";

function renderWithI18n(children: ReactNode) {
  return render(
    <I18N_CONTEXT.Provider
      value={{
        locale: "zh",
        setLocale: vi.fn(),
        t: (key) => key,
      }}
    >
      {children}
    </I18N_CONTEXT.Provider>,
  );
}

const defaultModel: ProviderModelRecord = {
  capabilities_auto: {},
  capabilities_override: {},
  category: "chat",
  display_name: "Default Model",
  enabled: true,
  id: "model-default",
  is_default: true,
  model_id: "default-model",
  provider_id: "provider-one",
  provider_options: {},
};

const provider: ProviderConfigRecord = {
  agent_runtime_supported: true,
  api_format: "responses",
  auth_token_masked: "",
  base_url: "https://example.test",
  can_manage: true,
  configuration_version: 1,
  display_name: "Provider One",
  enabled: true,
  id: "provider-one",
  last_test_error: "",
  last_test_status: "success",
  models: [defaultModel],
  models_path: "/models",
  preset_key: "custom",
  provider: "provider-one",
  provider_kind: "llm",
  usage_count: 0,
  used_by_agents: [],
  visibility: "private",
};

describe("ProviderSettingsModelList", () => {
  it("routes a default-model switch attempt without a fake wrapper button", async () => {
    const user = userEvent.setup();
    const onDefaultModelDisableAttempt = vi.fn();
    renderWithI18n(
      <ProviderSettingsModelList
        displayedModels={[defaultModel]}
        hasModelsEndpoint
        isApiFormatConfigurable
        isEditing
        modelQuery=""
        onDefaultModelDisableAttempt={onDefaultModelDisableAttempt}
        onFetchModels={vi.fn()}
        onModelOptions={vi.fn()}
        onModelQueryChange={vi.fn()}
        onOpenAddModel={vi.fn()}
        onRequestDeleteModel={vi.fn()}
        onSetDefaultModel={vi.fn()}
        onToggleModel={vi.fn()}
        pendingAction={null}
        selectedCanManage
        selectedRecord={provider}
      />,
    );

    const control = screen.getByRole("switch", {
      name: "settings.providers.toggle_model",
    }) as HTMLButtonElement;
    expect(control.disabled).toBe(false);
    expect(control.closest('[role="button"]')).toBeNull();
    await user.click(control);
    expect(onDefaultModelDisableAttempt).toHaveBeenCalledWith(defaultModel);
  });
});
