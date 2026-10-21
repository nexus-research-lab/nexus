// INPUT: 中文界面下的首屏历史加载占位。
// OUTPUT: 验证长会话首开期间呈现可播报的加载状态，而不是空会话。
// POS: DM/Room 共享首屏历史占位的 DOM 行为测试。

import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { LOCALE_STORAGE_KEY } from "@/shared/i18n/messages";

import { ConversationHistoryLoadingState } from "./conversation-history-loading-state";

describe("ConversationHistoryLoadingState", () => {
  beforeEach(() => {
    window.localStorage.setItem(LOCALE_STORAGE_KEY, "zh");
  });

  it("announces that the conversation history is still loading", () => {
    render(<I18nProvider><ConversationHistoryLoadingState /></I18nProvider>);
    expect(screen.getByRole("status").textContent).toContain("加载对话…");
    expect(screen.getByText("对话较长时，首次打开需要先整理历史记录。")).toBeTruthy();
  });
});
