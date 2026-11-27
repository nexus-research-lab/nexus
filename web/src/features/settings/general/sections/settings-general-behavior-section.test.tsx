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

function preferences(): Props {
  return {
    agentSdkDiagnosticsEnabled: false, autoMemoryEnabled: true, autoDreamEnabled: true,
    chatDefaultDeliveryPolicy: "queue", emotionEnabled: false,
    echoDisabled: false, echoEnabled: true, echoFeedback: null, echoLoading: false, echoSaving: false,
    echoRecovery: { canCheckLatest: false, canCompare: false, checking: false,
      checkLatest: vi.fn(), reapplyChange: vi.fn() },
    onAgentSdkDiagnosticsChange: vi.fn(), onAutoMemoryEnabledChange: vi.fn(), onAutoDreamEnabledChange: vi.fn(),
    onEmotionEnabledChange: vi.fn(), onEchoEnabledChange: vi.fn(), onDefaultDeliveryPolicyChange: vi.fn(),
    preferencesLoading: false, preferencesSaving: false, preferencesFeedback: null,
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
  it("uses one accessible delivery group without a repeated visible title and preserves exact preference selection and saving locks", async () => {
    const user = userEvent.setup();
    const props = preferences();
    const { rerender } = render(view(<SettingsGeneralBehaviorSection {...props} />));
    const group = screen.getByRole("group", { name: "默认消息行为" });
    expect(group.getAttribute("aria-label")).toBe("默认消息行为");
    expect(screen.queryByText("默认消息行为")).toBeNull();
    expect(screen.queryByText("服务 / 模型")).toBeNull();
    await user.click(within(group).getByRole("button", { name: "打断" }));
    expect(props.onDefaultDeliveryPolicyChange).toHaveBeenCalledExactlyOnceWith("interrupt");
    expect(props.onEchoEnabledChange).not.toHaveBeenCalled();
    rerender(view(<SettingsGeneralBehaviorSection {...props} preferencesSaving />));
    await user.click(within(group).getByRole("button", { name: "排队" }));
    expect(props.onDefaultDeliveryPolicyChange).toHaveBeenCalledTimes(1);
  });
});
