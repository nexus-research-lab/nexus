// INPUT: Thread 收起/展开状态与切换回调。
// OUTPUT: 证明微型共享按钮保留可访问状态和点击行为。
// POS: Room Agent 执行条 Thread 动作的 DOM 行为回归。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { ThreadActionButton } from "./thread-action-button";

describe("ThreadActionButton", () => {
  it("uses the shared micro action while preserving its pressed state", () => {
    const onClick = vi.fn();
    const { rerender } = render(
      <I18nProvider>
        <ThreadActionButton active={false} onClick={onClick} />
      </I18nProvider>,
    );

    const button = screen.getByRole("button", { name: /View Thread|查看 Thread/ });
    expect(button.className).toContain("min-h-6");
    expect(button.className).toContain("ui-type-caption");
    expect(button.getAttribute("aria-pressed")).toBe("false");
    fireEvent.click(button);
    expect(onClick).toHaveBeenCalledOnce();

    rerender(
      <I18nProvider>
        <ThreadActionButton active onClick={onClick} />
      </I18nProvider>,
    );
    expect(
      screen.getByRole("button", { name: /Close Thread|关闭 Thread/ })
        .getAttribute("aria-pressed"),
    ).toBe("true");
  });
});
