// INPUT: 中文空会话身份、共享欢迎组件与建议选择动作。
// OUTPUT: 验证 Agent 名称插值与建议按钮行为。
// POS: DM/Room 共享空会话欢迎面的 DOM 行为测试。

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { LOCALE_STORAGE_KEY } from "@/shared/i18n/messages";

import { ConversationEmptyIntroduction } from "./conversation-empty-introduction";

describe("ConversationEmptyIntroduction", () => {
  afterEach(() => vi.restoreAllMocks());
  beforeEach(() => {
    window.localStorage.setItem(LOCALE_STORAGE_KEY, "zh");
  });

  it.each(["dm", "room"] as const)("contains rejected %s suggestion submissions", async (kind) => {
    const error = new Error("受理状态未知");
    error.name = "RequestAcceptanceUnknownError";
    const onSelect = vi.fn().mockRejectedValue(error);
    const log = vi.spyOn(console, "error").mockImplementation(() => {});
    render(<I18nProvider><ConversationEmptyIntroduction kind={kind} onSelect={onSelect} /></I18nProvider>);
    fireEvent.click(screen.getAllByRole("button")[0]);
    await waitFor(() => expect(log).toHaveBeenCalledWith(
      "[ConversationEmptyIntroduction] 快捷建议发送失败", error,
    ));
    expect(onSelect).toHaveBeenCalledTimes(1);
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
