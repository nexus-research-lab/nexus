// INPUT: User 消息尾部动作与点击回调。
// OUTPUT: 证明消息动作保留可访问名称和精确命令行为。
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
  it("dispatches each accessible message action exactly once", () => {
    const onCopy = vi.fn().mockResolvedValue(undefined);
    const onEdit = vi.fn();
    const onRerun = vi.fn();
    render(
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

    fireEvent.click(rerun);
    fireEvent.click(edit);
    fireEvent.click(copy);
    expect(onRerun).toHaveBeenCalledOnce();
    expect(onEdit).toHaveBeenCalledOnce();
    expect(onCopy).toHaveBeenCalledOnce();
  });
});
