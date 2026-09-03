// INPUT: 窄窗 Room 标题、会话标题、切换状态、返回与尾部动作。
// OUTPUT: 证明 Room Header 与普通应用页头共享几何、排版和按钮原语。
// POS: Room 专注模式页头 DOM 合同；会话切换器内容由上层控制器负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { RoomMobileHeader } from "./room-mobile-header";

describe("RoomMobileHeader", () => {
  it("shares shell geometry while preserving Room navigation actions", () => {
    const onBack = vi.fn();
    const onOpenConversations = vi.fn();
    const { container } = render(
      <I18nProvider>
        <RoomMobileHeader
          conversationTitle="需求讨论"
          isConversationSwitcherOpen={false}
          onBack={onBack}
          onOpenConversations={onOpenConversations}
          roomTitle="产品 Room"
          trailing={<button type="button">更多</button>}
        />
      </I18nProvider>,
    );

    const header = container.querySelector("header");
    const back = screen.getByRole("button", { name: /back|返回/i });
    const switcher = screen.getByRole("button", { name: /switch conversation|切换会话/i });

    expect(header?.className).toContain("h-[var(--mobile-shell-header-height,52px)]");
    expect(screen.getByText("产品 Room").className).toContain("ui-type-section-title");
    expect(screen.getByText("需求讨论").className).toContain("ui-type-caption");
    expect(back.className).toContain("rounded-full");
    expect(switcher.className).toContain("border-transparent");
    expect(switcher.className).toContain("radius-control-sm");
    expect(switcher.getAttribute("aria-expanded")).toBe("false");

    fireEvent.click(back);
    fireEvent.click(switcher);
    expect(onBack).toHaveBeenCalledOnce();
    expect(onOpenConversations).toHaveBeenCalledOnce();
  });
});
