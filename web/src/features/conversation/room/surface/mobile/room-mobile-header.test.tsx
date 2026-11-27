// INPUT: 窄窗 Room 标题、会话标题、切换状态、返回与尾部动作。
// OUTPUT: 证明 Room Header 与普通应用页头共享几何、排版和按钮原语。
// POS: Room 专注模式页头 DOM 合同；会话切换器内容由上层控制器负责。

import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { MESSAGES } from "@/shared/i18n/messages";
import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { RoomMobileHeader } from "./room-mobile-header";

describe("RoomMobileHeader", () => {

  it("keeps full names available, deduplicates whitespace-equivalent titles and gives each instance its own description", () => {
    const props = { isConversationSwitcherOpen: false, onBack: vi.fn(), onOpenConversations: vi.fn(), trailing: null };
    const rendered = render(<I18nProvider>
      <RoomMobileHeader {...props} roomTitle="  Research  " conversationTitle=" Study details with a long title " />
      <RoomMobileHeader {...props} roomTitle="  Research  " conversationTitle="Research " />
    </I18nProvider>);
    const [first, second] = screen.getAllByRole("button", { name: /switch conversation|切换会话/i });
    const describe = (button: HTMLElement) => button.getAttribute("aria-describedby")!.split(" ").map((id) => document.getElementById(id)!.textContent);
    expect(describe(first)).toEqual(["Research", "Study details with a long title"]);
    expect(describe(second)).toEqual(["Research"]);
    expect(first.getAttribute("aria-describedby")).not.toBe(second.getAttribute("aria-describedby"));
    expect(first.getAttribute("title")).toBeNull();
    expect(within(second).getAllByText("Research")).toHaveLength(1);
    expect(first.querySelector("svg")?.getAttribute("aria-hidden")).toBe("true");
    rendered.rerender(<I18nProvider><RoomMobileHeader {...props} roomTitle=" " conversationTitle=" " /></I18nProvider>);
    const empty = screen.getByRole("button", { name: /switch conversation|切换会话/i });
    expect([MESSAGES.en["room.new_conversation"], MESSAGES.zh["room.new_conversation"]]).toContain(empty.textContent);
  });
});
