// INPUT: MessageDetailToggle 的展开态、tone 与点击动作。
// OUTPUT: 证明共享 Button 所有权、ARIA 展开语义、箭头状态与动作触发。
// POS: Message 明细展开行 DOM 行为测试；不覆盖各业务明细正文。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Brain } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import { MessageDetailToggle } from "./message-detail-toggle";

describe("MessageDetailToggle", () => {
  it("exposes one expandable Button action and rotates its shared arrow", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    const { rerender } = render(
      <MessageDetailToggle
        expanded={false}
        leading={<Brain aria-hidden="true" />}
        onClick={onToggle}
      >
        Thought
      </MessageDetailToggle>,
    );

    const button = screen.getByRole("button", { name: "Thought" });
    expect(button.getAttribute("type")).toBe("button");
    expect(button.getAttribute("aria-expanded")).toBe("false");
    expect(button.className).toContain("focus-visible:ring-2");
    await user.click(button);
    expect(onToggle).toHaveBeenCalledOnce();

    rerender(
      <MessageDetailToggle
        expanded
        leading={<Brain aria-hidden="true" />}
        onClick={onToggle}
        tone="active"
      >
        Thought
      </MessageDetailToggle>,
    );
    expect(button.getAttribute("aria-expanded")).toBe("true");
    expect(button.querySelector("svg:last-child")?.getAttribute("class"))
      .toContain("rotate-90");
  });
});
