// INPUT: User 消息尾部动作、复制完成态与点击回调。
// OUTPUT: 证明消息动作直接复用共享微型 Button，并保留可访问名称和命令行为。
// POS: User 消息头 DOM 行为回归；不覆盖正文或编辑器流程。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { UserMessageHeader } from "./user-message-header";

const PRESENTATION = {
  displayContent: "测试消息",
  goal: false,
  guided: false,
  hasContent: true,
  timestamp: "10:24",
};

describe("UserMessageHeader", () => {
  it("uses shared icon actions and projects copied state without changing geometry", () => {
    const onCopy = vi.fn().mockResolvedValue(undefined);
    const onEdit = vi.fn();
    const onRerun = vi.fn();
    const { rerender } = render(
      <I18nProvider>
        <UserMessageHeader
          copied={false}
          onCopy={onCopy}
          onEdit={onEdit}
          onRerun={onRerun}
          presentation={PRESENTATION}
        />
      </I18nProvider>,
    );

    const rerun = screen.getByRole("button", { name: /Run again|重新运行/ });
    const edit = screen.getByRole("button", { name: /Edit message|编辑消息/ });
    const copy = screen.getByRole("button", { name: /Copy message|复制消息/ });
    for (const action of [rerun, edit, copy]) {
      expect(action.className).toContain("h-6");
      expect(action.className).toContain("radius-control-xs");
    }

    fireEvent.click(rerun);
    fireEvent.click(edit);
    fireEvent.click(copy);
    expect(onRerun).toHaveBeenCalledOnce();
    expect(onEdit).toHaveBeenCalledOnce();
    expect(onCopy).toHaveBeenCalledOnce();

    rerender(
      <I18nProvider>
        <UserMessageHeader
          copied
          onCopy={onCopy}
          onEdit={onEdit}
          onRerun={onRerun}
          presentation={PRESENTATION}
        />
      </I18nProvider>,
    );
    const copied = screen.getByRole("button", { name: /Copy message|复制消息/ });
    expect(copied.className).toContain("text-(--success)");
    expect(copied.className).toContain("h-6");
  });
});
