// INPUT: Provider 身份、权限、测试候选与用户点击/键盘事件。
// OUTPUT: 证明测试只由显式动作提交，菜单随身份/能力边界收口，标题和状态保持可读。
// POS: Provider 详情头离线 DOM 回归；不调用模型或网络，不替代视觉验收。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { ProviderSettingsDetailHeader } from "./components/provider-settings-detail-header";

type Props = ComponentProps<typeof ProviderSettingsDetailHeader>;
const options = [{ label: "Auto", value: "auto" }, { label: "Model Alpha", value: "alpha" }];
function props(overrides: Partial<Props> = {}): Props {
  return { detailTitle: "Provider Alpha", enabled: true, hasSelectedRecord: true,
    isApiFormatConfigurable: true, isEditing: true, onEnabledChange: vi.fn(),
    onTestSelection: vi.fn(), pendingAction: null, providerId: "provider-a", selectedCanManage: true,
    testModelOptions: options, ...overrides };
}
function testTrigger() { return screen.getByRole("button", { expanded: false }); }

describe("Provider detail test actions", () => {
  it.each(["click", "Enter", "Space"])("opens with %s and runs only the explicitly activated model", async (opening) => {
    const user = userEvent.setup();
    const input = props();
    render(<ProviderSettingsDetailHeader {...input} />, { wrapper: I18nProvider });
    const trigger = testTrigger();
    trigger.focus();
    await user.keyboard("{ArrowDown}");
    expect(input.onTestSelection).not.toHaveBeenCalled();
    expect(screen.queryByRole("menu")).toBeNull();
    if (opening === "click") await user.click(trigger);
    else await user.keyboard(opening === "Enter" ? "{Enter}" : " ");
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "Auto" }));
    await user.keyboard("{End}{Home}{ArrowDown}");
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "Model Alpha" }));
    expect(input.onTestSelection).not.toHaveBeenCalled();
    await user.keyboard("{Enter}");
    expect(input.onTestSelection).toHaveBeenCalledExactlyOnceWith("alpha");
    expect(screen.queryByRole("menu")).toBeNull();
    expect(document.activeElement).toBe(trigger);
    await user.click(trigger);
    await user.click(screen.getByRole("menuitem", { name: "Auto" }));
    expect(vi.mocked(input.onTestSelection).mock.calls).toEqual([["alpha"], ["auto"]]);
  });

  it.each([
    { pendingAction: { kind: "test-provider" } },
    { pendingAction: { kind: "save-provider" } },
    { selectedCanManage: false }, { isApiFormatConfigurable: false },
    { providerId: null }, { testModelOptions: [] },
  ] satisfies Partial<Props>[])("disables unavailable testing for %j", async (overrides) => {
    const user = userEvent.setup();
    const input = props(overrides);
    render(<ProviderSettingsDetailHeader {...input} />, { wrapper: I18nProvider });
    const trigger = testTrigger() as HTMLButtonElement;
    expect(trigger.disabled).toBe(true);
    expect(trigger.getAttribute("aria-busy")).toBe(input.pendingAction?.kind === "test-provider" ? "true" : null);
    await user.click(trigger);
    expect(screen.queryByRole("menu")).toBeNull();
    expect(input.onTestSelection).not.toHaveBeenCalled();
  });

  it.each(["provider", "options", "permission", "editing"])("consumes open state when %s changes and never resurrects it", async (boundary) => {
    const user = userEvent.setup();
    const input = props();
    const view = render(<ProviderSettingsDetailHeader {...input} />, { wrapper: I18nProvider });
    await user.click(testTrigger());
    const changed: Partial<Props> = boundary === "provider" ? { providerId: "provider-b" }
      : boundary === "options" ? { testModelOptions: [{ label: "Beta", value: "beta" }] }
        : boundary === "permission" ? { selectedCanManage: false } : { isEditing: false };
    view.rerender(<ProviderSettingsDetailHeader {...input} {...changed} />);
    expect(screen.queryByRole("menu")).toBeNull();
    view.rerender(<ProviderSettingsDetailHeader {...input} />);
    expect(screen.queryByRole("menu")).toBeNull();
    expect(input.onTestSelection).not.toHaveBeenCalled();
  });

  it("keeps identity labels and switch behavior through presentation-only updates", async () => {
    const user = userEvent.setup();
    const input = props();
    const view = render(<ProviderSettingsDetailHeader {...input} />, { wrapper: I18nProvider });
    await user.click(testTrigger());
    const longName = "A full custom Provider name that must remain readable";
    view.rerender(<ProviderSettingsDetailHeader {...input} detailTitle={longName}
      testModelOptions={options.map((option) => ({ ...option, label: `${option.label} renamed` }))} />);
    expect(screen.getByRole("menu")).toBeTruthy();
    expect(screen.getByRole("menuitem", { name: "Auto renamed" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: longName }).className).not.toContain("truncate");
    await user.keyboard("{Escape}");
    await user.click(screen.getByRole("switch"));
    expect(input.onEnabledChange).toHaveBeenCalledExactlyOnceWith(false);
    expect(input.onTestSelection).not.toHaveBeenCalled();
  });
});
