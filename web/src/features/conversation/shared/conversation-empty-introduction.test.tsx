// INPUT: 中文空会话身份、共享欢迎组件与建议选择动作。
// OUTPUT: 验证品牌/Agent 名称排版、可见区居中、四块边框与建议按钮行为。
// POS: DM/Room 共享空会话欢迎面的 DOM 行为测试。

import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { LOCALE_STORAGE_KEY } from "@/shared/i18n/messages";

import { ConversationEmptyIntroduction } from "./conversation-empty-introduction";

describe("ConversationEmptyIntroduction", () => {
  beforeEach(() => {
    window.localStorage.setItem(LOCALE_STORAGE_KEY, "zh");
  });

  it("centers the shared surface and keeps spaces around an interpolated Agent name", () => {
    const onSelect = vi.fn();
    const { container } = render(
      <I18nProvider>
        <ConversationEmptyIntroduction
          agentName="nexus"
          kind="dm"
          onSelect={onSelect}
        />
      </I18nProvider>,
    );

    expect(screen.getByRole("heading", {
      name: "想让 nexus 帮你做什么？",
    })).toBeTruthy();
    expect(container.querySelector("[data-conversation-empty-introduction]")?.className)
      .toContain("flex-1");
    const suggestionButtons = screen.getAllByRole("button");
    expect(suggestionButtons).toHaveLength(4);
    for (const button of suggestionButtons) {
      expect(button.className).toContain("border-(--modal-btn-secondary-border)");
      expect(button.className).toContain("bg-transparent");
      expect(button.className).toContain("min-h-24");
      expect(button.className).toContain("flex-col");
    }

    fireEvent.click(screen.getByRole("button", {
      name: "处理当前工作区中的文件与内容",
    }));
    expect(onSelect).toHaveBeenCalledWith("处理当前工作区中的文件与内容");
  });
});
