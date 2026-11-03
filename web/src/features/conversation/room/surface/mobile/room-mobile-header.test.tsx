// INPUT: 窄窗 Room 标题、会话标题、切换状态、返回与尾部动作。
// OUTPUT: 证明 Room Header 与普通应用页头共享几何、排版和按钮原语。
// POS: Room 专注模式页头 DOM 合同；会话切换器内容由上层控制器负责。

import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { MESSAGES } from "@/shared/i18n/messages";
import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { RoomMobileHeader } from "./room-mobile-header";

describe("RoomMobileHeader", () => {
  it("shares shell geometry while preserving Room navigation actions", () => {
    const onBack = vi.fn();
    const onOpenConversations = vi.fn();
    render(
<I18nProvider>
<RoomMobileHeader
conversationTitle="需求讨论"
isConversationSwitcherOpen={false}
onBack={onBack}
onOpenConversations={onOpenConversations}
roomTitle="产品 Room"
trailing={<button type="button">更多</button>} />
</I18nProvider>
);

    const back = screen.getByRole("button", { name: /back|返回/i });
    const switcher = screen.getByRole("button", { name: /switch conversation|切换会话/i });

    expect(switcher.getAttribute("aria-expanded")).toBe("false");

    fireEvent.click(back);
    fireEvent.click(switcher);
    expect(onBack).toHaveBeenCalledOnce();
    expect(onOpenConversations).toHaveBeenCalledOnce();
  });

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

  it("keeps a single title action and exposes its expanded state for Enter and Space", async () => {
    const user = userEvent.setup();
    function Harness() {
      const [open, setOpen] = useState(false);
      return <I18nProvider><RoomMobileHeader roomTitle="Room" conversationTitle="Session" isConversationSwitcherOpen={open}
        onBack={vi.fn()} onOpenConversations={() => setOpen((current) => !current)} trailing={null} /></I18nProvider>;
    }
    render(<Harness />);
    const button = screen.getByRole("button", { name: /switch conversation|切换会话/i });
    button.focus();
    await user.keyboard("{Enter}");
    expect(button.getAttribute("aria-expanded")).toBe("true");
    await user.keyboard(" ");
    expect(button.getAttribute("aria-expanded")).toBe("false");
  });
});
