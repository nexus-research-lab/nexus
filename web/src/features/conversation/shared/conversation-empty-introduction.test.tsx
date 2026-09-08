// INPUT: 中文空会话身份、共享欢迎组件与建议选择动作。
// OUTPUT: 验证 Agent 名称插值与建议按钮行为。
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

  it("keeps spaces around the Agent name and dispatches the selected suggestion", () => {
    const onSelect = vi.fn();
    render(
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
    const suggestionButtons = screen.getAllByRole("button");
    expect(suggestionButtons).toHaveLength(4);

    fireEvent.click(screen.getByRole("button", {
      name: "处理当前工作区中的文件与内容",
    }));
    expect(onSelect).toHaveBeenCalledWith("处理当前工作区中的文件与内容");
  });
});
