// INPUT: 实际常规设置视图、双语文案、独立 Preferences/Echo 状态和本地变更回调。
// OUTPUT: 具名开关及实例说明、精确叶子更新、互不混用的加载/保存/恢复禁用条件。
// POS: 设置行组合回归；不替代后端偏好事务与实际浏览器几何验收。

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps, ReactNode } from "react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { enSettingsMessages } from "@/shared/i18n/catalog/en/settings";
import { zhSettingsMessages } from "@/shared/i18n/catalog/zh/settings";

import { SettingsGeneralBehaviorSection } from "./settings-general-behavior-section";

type Props = ComponentProps<typeof SettingsGeneralBehaviorSection>;
const toggles = [
  ["agent_sdk_diagnostics", "onAgentSdkDiagnosticsChange", false],
  ["auto_memory", "onAutoMemoryEnabledChange", true],
  ["auto_dream", "onAutoDreamEnabledChange", true],
  ["emotion", "onEmotionEnabledChange", false],
  ["echo", "onEchoEnabledChange", true],
] as const;

function preferences(): Props {
  return {
    agentSdkDiagnosticsEnabled: false, autoMemoryEnabled: true, autoDreamEnabled: true,
    chatDefaultDeliveryPolicy: "queue", emotionEnabled: false,
    echoDisabled: false, echoEnabled: true, echoFeedback: null, echoLoading: false, echoSaving: false,
    echoRecovery: { canCheckLatest: false, canCompare: false, canFinishDisabling: false, checking: false,
      checkLatest: vi.fn(), finishDisabling: vi.fn(), reapplyChange: vi.fn(), repairing: false },
    defaultBackgroundModelOptions: [], defaultBackgroundModelValue: "", defaultImageModelOptions: [],
    defaultImageModelValue: "", defaultVisionModelOptions: [], defaultVisionModelValue: "",
    defaultModelCatalogFailed: false, defaultModelOptions: [], defaultModelSavingRole: null, defaultModelValue: "",
    onAgentSdkDiagnosticsChange: vi.fn(), onAutoMemoryEnabledChange: vi.fn(), onAutoDreamEnabledChange: vi.fn(),
    onEmotionEnabledChange: vi.fn(), onEchoEnabledChange: vi.fn(), onDefaultDeliveryPolicyChange: vi.fn(),
    onDefaultModelChange: vi.fn(), onRetryDefaultModelCatalog: vi.fn(), onResetTours: vi.fn(),
    preferencesLoading: false, preferencesSaving: false, preferencesFeedback: null, providerOptionsLoading: false,
    preferencesRecovery: { canCompare: false, canRepairProjection: false, checking: false,
      checkLatest: vi.fn(), reapplyDraft: vi.fn(), repairProjection: vi.fn(), repairing: false },
  };
}

function view(children: ReactNode, locale: "zh" | "en" = "zh") {
  const messages = locale === "zh" ? zhSettingsMessages : enSettingsMessages;
  return <MemoryRouter><I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => {
    const value = messages[key as keyof typeof messages];
    return typeof value === "string" ? value : key;
  } }}>{children}</I18N_CONTEXT.Provider></MemoryRouter>;
}

describe("General setting switches", () => {
  it("uses one visible delivery group and preserves exact preference selection and saving locks", async () => {
    const user = userEvent.setup();
    const props = preferences();
    const { rerender } = render(view(<SettingsGeneralBehaviorSection {...props} />));
    const group = screen.getByRole("group", { name: "默认消息行为" });
    expect(document.getElementById(group.getAttribute("aria-labelledby")!)?.textContent).toBe("默认消息行为");
    expect(screen.getAllByText("默认消息行为")).toHaveLength(1);
    await user.click(within(group).getByRole("button", { name: "打断" }));
    expect(props.onDefaultDeliveryPolicyChange).toHaveBeenCalledExactlyOnceWith("interrupt");
    expect(props.onEchoEnabledChange).not.toHaveBeenCalled();
    rerender(view(<SettingsGeneralBehaviorSection {...props} preferencesSaving />));
    await user.click(within(group).getByRole("button", { name: "排队" }));
    expect(props.onDefaultDeliveryPolicyChange).toHaveBeenCalledTimes(1);
  });

  it.each(["zh", "en"] as const)("names each %s setting and changes only the selected leaf", async (locale) => {
    const user = userEvent.setup();
    const props = preferences();
    const messages = locale === "zh" ? zhSettingsMessages : enSettingsMessages;
    render(view(<SettingsGeneralBehaviorSection {...props} />, locale));
    expect(screen.getAllByRole("switch")).toHaveLength(5);
    for (const [name, callback, checked] of toggles) {
      const control = screen.getByRole("switch", { name: messages[`settings.general.${name}_title`] });
      const description = document.getElementById(control.getAttribute("aria-describedby")!);
      expect(description?.textContent).toBe(messages[`settings.general.${name}_description`]);
      expect(control.getAttribute("aria-checked")).toBe(String(checked));
      // Visible explanations remain static; only the switch changes the value.
      await user.click(description!);
      expect(props[callback]).not.toHaveBeenCalled();
      await user.click(control);
      expect(props[callback]).toHaveBeenCalledExactlyOnceWith(!checked);
    }
    expect(props.onDefaultModelChange).not.toHaveBeenCalled();
    expect(props.onDefaultDeliveryPolicyChange).not.toHaveBeenCalled();
  });

  it("associates descriptions and commands with the correct mounted instance", async () => {
    const user = userEvent.setup();
    const first = preferences();
    const second = preferences();
    render(view(<>
      <div data-testid="first"><SettingsGeneralBehaviorSection {...first} /></div>
      <div data-testid="second"><SettingsGeneralBehaviorSection {...second} /></div>
    </>));
    const ids = screen.getAllByRole("switch").map((control) => control.getAttribute("aria-describedby"));
    expect(new Set(ids).size).toBe(10);
    for (const id of ids) expect(document.getElementById(id!)).not.toBeNull();
    await user.click(within(screen.getByTestId("second")).getByRole("switch", { name: "主动跟进" }));
    expect(second.onEchoEnabledChange).toHaveBeenCalledExactlyOnceWith(false);
    expect(first.onEchoEnabledChange).not.toHaveBeenCalled();
  });

  it.each(["preferencesLoading", "preferencesSaving"] as const)("keeps the independent Echo control available during %s", async (flag) => {
    const user = userEvent.setup();
    const props = { ...preferences(), [flag]: true };
    render(view(<SettingsGeneralBehaviorSection {...props} />));
    for (const [name, callback] of toggles.slice(0, 4)) {
      const control = screen.getByRole("switch", { name: zhSettingsMessages[`settings.general.${name}_title`] });
      expect((control as HTMLButtonElement).disabled).toBe(true);
      await user.click(control);
      expect(props[callback]).not.toHaveBeenCalled();
    }
    await user.click(screen.getByRole("switch", { name: "主动跟进" }));
    expect(props.onEchoEnabledChange).toHaveBeenCalledExactlyOnceWith(false);
  });

  it.each(["echoDisabled", "echoLoading", "echoSaving"] as const)("keeps Preferences independent from %s", async (flag) => {
    const user = userEvent.setup();
    const props = { ...preferences(), [flag]: true };
    render(view(<SettingsGeneralBehaviorSection {...props} />));
    const echo = screen.getByRole("switch", { name: "主动跟进" }) as HTMLButtonElement;
    expect(echo.disabled).toBe(true);
    await user.click(echo);
    expect(props.onEchoEnabledChange).not.toHaveBeenCalled();
    await user.click(screen.getByRole("switch", { name: "自动记忆" }));
    expect(props.onAutoMemoryEnabledChange).toHaveBeenCalledExactlyOnceWith(false);
  });
});
