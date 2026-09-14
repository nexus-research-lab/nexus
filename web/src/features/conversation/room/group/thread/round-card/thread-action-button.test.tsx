// INPUT: Thread 收起/展开状态与切换回调。
// OUTPUT: 证明紧凑共享按钮保留具名展开状态和点击行为。
// POS: Room Agent 执行条 Thread 动作的 DOM 行为回归。

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { ThreadActionButton } from "./thread-action-button";

describe("ThreadActionButton", () => {
  it.each(["en", "zh"] as const)("keeps its visible label stable and names the current Agent in %s", async (locale) => {
    const user = userEvent.setup();
    const click = vi.fn();
    const view = (name: string, active: boolean) => <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(),
      t: (key, params) => Object.entries(params ?? {}).reduce((text, [param, value]) => text.replaceAll(`{${param}}`, String(value)), MESSAGES[locale][key]) }}>
      <ThreadActionButton agentName={name} active={active} onClick={click} />
    </I18N_CONTEXT.Provider>;
    const rendered = render(view(" Nova ", false));
    const button = screen.getByRole("button", { name: MESSAGES[locale]["room.thread_action_open"].replace("{name}", "Nova") });
    expect(button.textContent).toBe(MESSAGES[locale]["room.thread_label"]);
    expect(button.hasAttribute("aria-pressed")).toBe(false);
    await user.tab();
    await user.keyboard("{Enter}");
    expect(click).toHaveBeenCalledOnce();
    rendered.rerender(view("Nova", true));
    expect(button.getAttribute("aria-expanded")).toBe("true");
    expect(button.getAttribute("aria-label")).toBe(MESSAGES[locale]["room.thread_action_close"].replace("{name}", "Nova"));
    expect(button.textContent).toBe(MESSAGES[locale]["room.thread_label"]);
    await user.keyboard(" ");
    expect(click).toHaveBeenCalledTimes(2);
    rendered.rerender(view("  ", false));
    expect(button.getAttribute("aria-label")).toBe(MESSAGES[locale]["room.thread_action_open"].replace("{name}", MESSAGES[locale]["agent.name_fallback"]));
  });

  it("uses the shared compact action while exposing its expanded state", () => {
    const onClick = vi.fn();
    const { rerender } = render(
      <I18nProvider>
        <ThreadActionButton agentName="Nova" active={false} onClick={onClick} />
      </I18nProvider>,
    );

    const button = screen.getByRole("button", { name: /View Thread for Nova|查看Nova的 Thread/ });
    expect(button.className).toContain("min-h-7");
    expect(button.className).toContain("ui-type-metadata");
    expect(button.getAttribute("aria-expanded")).toBe("false");
    fireEvent.click(button);
    expect(onClick).toHaveBeenCalledOnce();

    rerender(
      <I18nProvider>
        <ThreadActionButton agentName="Nova" active onClick={onClick} />
      </I18nProvider>,
    );
    expect(
      screen.getByRole("button", { name: /Close Thread for Nova|关闭Nova的 Thread/ })
        .getAttribute("aria-expanded"),
    ).toBe("true");
  });
});
