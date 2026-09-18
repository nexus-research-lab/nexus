// INPUT: 模型设置视图、可用模型目录与保留的用户默认选择。
// OUTPUT: 四类默认模型独立分派、加载保存锁和凭据清空后的真实空选择。
// POS: 默认模型页面组合回归；不替代 Provider 凭据事务和真实浏览器验收。
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps, ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { enSettingsMessages } from "@/shared/i18n/catalog/en/settings";
import { zhSettingsMessages } from "@/shared/i18n/catalog/zh/settings";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { UserPreferences } from "@/types/settings/preferences";

import {
  EMPTY_DEFAULT_MODEL_CATALOG,
  buildDefaultModelPreferencesView,
  type DefaultModelCatalog,
} from "./default-model-preferences-model";
import { SettingsDefaultModelsView } from "./settings-default-models-section";

type Props = ComponentProps<typeof SettingsDefaultModelsView>;
const modelRows = [
  ["default_model", "agent_runtime"],
  ["default_image_model", "image_generation"],
  ["default_vision_model", "vision_understanding"],
  ["default_background_model", "background_task"],
] as const;

function preferences(): Props {
  const options = [{ value: '["provider","model"]', label: "Provider / Model" }];
  return {
    defaultBackgroundModelOptions: options, defaultBackgroundModelValue: "",
    defaultImageModelOptions: options, defaultImageModelValue: "",
    defaultVisionModelOptions: options, defaultVisionModelValue: "",
    defaultModelCatalogFailed: false, defaultModelOptions: options,
    defaultModelSavingRole: null, defaultModelValue: "",
    onDefaultModelChange: vi.fn(), onRetryDefaultModelCatalog: vi.fn(),
    preferencesLoading: false, preferencesSaving: false,
    preferencesFeedback: null, providerOptionsLoading: false,
    preferencesRecovery: { canCompare: false, canRepairProjection: false, checking: false,
      checkLatest: vi.fn(), reapplyDraft: vi.fn(), repairProjection: vi.fn(), repairing: false },
  };
}

function view(children: ReactNode, locale: "zh" | "en" = "zh") {
  const messages = locale === "zh" ? zhSettingsMessages : enSettingsMessages;
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => {
    const value = messages[key as keyof typeof messages];
    return typeof value === "string" ? value : key;
  } }}>{children}</I18N_CONTEXT.Provider>;
}

function catalogProps(catalog: DefaultModelCatalog, saved: UserPreferences) {
  const result = buildDefaultModelPreferencesView(catalog, saved, "订阅");
  return {
    defaultModelOptions: result.options.agent, defaultModelValue: result.values.agent,
    defaultImageModelOptions: result.options.image, defaultImageModelValue: result.values.image,
    defaultVisionModelOptions: result.options.vision, defaultVisionModelValue: result.values.vision,
    defaultBackgroundModelOptions: result.options.background, defaultBackgroundModelValue: result.values.background,
  };
}

describe("Default model settings", () => {
  it.each(["zh", "en"] as const)("names all four %s roles and dispatches the exact selection", async (locale) => {
    const user = userEvent.setup();
    const props = preferences();
    const messages = locale === "zh" ? zhSettingsMessages : enSettingsMessages;
    render(view(<SettingsDefaultModelsView {...props} />, locale));
    for (const [name, role] of modelRows) {
      await user.click(screen.getByRole("button", { name: messages[`settings.general.${name}_title`] }));
      await user.click(screen.getByRole("option", { name: "Provider / Model" }));
      expect(props.onDefaultModelChange).toHaveBeenLastCalledWith('["provider","model"]', role);
    }
    expect(props.onDefaultModelChange).toHaveBeenCalledTimes(4);
    expect(screen.queryByText("服务 / 模型")).toBeNull();
  });

  it.each(["preferencesLoading", "preferencesSaving", "providerOptionsLoading"] as const)("locks all populated selectors during %s", (flag) => {
    const props = { ...preferences(), [flag]: true };
    const { rerender } = render(view(<SettingsDefaultModelsView {...props} />));
    for (const [name] of modelRows) {
      const control = screen.getByRole("button", { name: zhSettingsMessages[`settings.general.${name}_title`] });
      expect((control as HTMLButtonElement).disabled).toBe(true);
    }
    rerender(view(<SettingsDefaultModelsView {...props} {...{ [flag]: false }} />));
    for (const [name] of modelRows) {
      const control = screen.getByRole("button", { name: zhSettingsMessages[`settings.general.${name}_title`] });
      expect((control as HTMLButtonElement).disabled).toBe(false);
    }
  });

  it("blocks duplicate catalog retries while its read is pending", async () => {
    const user = userEvent.setup();
    const props = { ...preferences(), defaultModelCatalogFailed: true };
    const { rerender } = render(view(<SettingsDefaultModelsView {...props} />));
    await user.click(screen.getByRole("button", { name: zhSettingsMessages["settings.general.default_model_catalog_retry"] }));
    expect(props.onRetryDefaultModelCatalog).toHaveBeenCalledOnce();
    rerender(view(<SettingsDefaultModelsView {...props} providerOptionsLoading />));
    const retry = screen.getByRole("button", { name: zhSettingsMessages["settings.general.default_model_loading"] });
    expect((retry as HTMLButtonElement).disabled).toBe(true);
    await user.click(retry);
    expect(props.onRetryDefaultModelCatalog).toHaveBeenCalledOnce();
  });

  it("shows empty choices after the last credential is cleared and restores saved selections only when available again", () => {
    const selection = { provider: "provider", model: "model" };
    const saved: UserPreferences = {
      chat_default_delivery_policy: "queue",
      default_agent_options: { ...selection },
      default_image_model_selection: { ...selection },
      default_vision_model_selection: { ...selection },
      default_background_model_selection: { ...selection },
    };
    const original = structuredClone(saved);
    const options = [{ provider: "provider", display_name: "Provider", models: [
      { model_id: "model", display_name: "Model", is_default: true },
    ] }];
    const catalog: DefaultModelCatalog = {
      agentDefault: selection, imageDefault: selection,
      agentOptions: options, imageOptions: options, visionOptions: options, backgroundOptions: options,
    };
    const props = preferences();
    const { rerender } = render(view(<SettingsDefaultModelsView {...props} {...catalogProps(catalog, saved)} />));
    for (const [name] of modelRows) {
      expect(screen.getByRole("button", { name: zhSettingsMessages[`settings.general.${name}_title`] }).textContent).toBe("Provider / Model");
    }

    rerender(view(<SettingsDefaultModelsView {...props} {...catalogProps(EMPTY_DEFAULT_MODEL_CATALOG, saved)} />));
    for (const [name] of modelRows) {
      const control = screen.getByRole("button", { name: zhSettingsMessages[`settings.general.${name}_title`] });
      expect(control.textContent).toBe(zhSettingsMessages[`settings.general.${name}_empty`]);
      expect((control as HTMLButtonElement).disabled).toBe(true);
    }
    expect(saved).toEqual(original);
    expect(props.onDefaultModelChange).not.toHaveBeenCalled();

    rerender(view(<SettingsDefaultModelsView {...props} {...catalogProps(catalog, saved)} />));
    for (const [name] of modelRows) {
      expect(screen.getByRole("button", { name: zhSettingsMessages[`settings.general.${name}_title`] }).textContent).toBe("Provider / Model");
    }
  });
});
