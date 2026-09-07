// INPUT: 移动 Thread 视图模型、身份、空消息流与返回命令。
// OUTPUT: 证明 Thread 复用平台页头、语义排版和圆形图标动作。
// POS: 共享 Thread 纯视图行为测试；消息投影和滚动状态由各自测试负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";

import type { ConversationThreadModel } from "./conversation-thread-model";
import { ConversationThreadView } from "./conversation-thread-view";

const MOBILE_MODEL: ConversationThreadModel = {
  allMessages: [],
  isMobile: true,
  leadingAction: "back",
  presentation: "inspector",
  rounds: [],
  sessionKey: "thread-session",
  trailingAction: null,
  workspaceAgentId: "agent-1",
};

describe("ConversationThreadView", () => {
  it.each(["en", "zh"] as const)("uses the shared mobile header and localized navigation in %s", (locale) => {
    const onClose = vi.fn();
    const { container } = render(
      <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}>
        <ConversationThreadView
          agentAvatar={null}
          agentName="研究助手"
          bottomAnchorRef={createRef<HTMLDivElement>()}
          emptyContent={<p>暂无过程</p>}
          feedRef={createRef<HTMLDivElement>()}
          footer={null}
          headerAction={null}
          headerAvatar={null}
          messageContext={{
            agentAvatar: null,
            agentName: "研究助手",
            workspaceAgentId: "agent-1",
          }}
          model={{ ...MOBILE_MODEL, trailingAction: "close" }}
          notice={null}
          onClose={onClose}
          onPointerDown={vi.fn()}
          onScroll={vi.fn()}
          onScrollToLatest={vi.fn()}
          onTouchEnd={vi.fn()}
          onTouchMove={vi.fn()}
          onTouchStart={vi.fn()}
          onWheel={vi.fn()}
          scrollRef={createRef<HTMLDivElement>()}
          showScrollToLatest={false}
          subtitle="Thread"
        />
      </I18N_CONTEXT.Provider>,
    );

    const header = container.querySelector("header");
    const back = screen.getByRole("button", { name: MESSAGES[locale]["common.back"] });
    expect(header?.className).toContain("h-[var(--mobile-shell-header-height,52px)]");
    expect(header?.className).toContain("border-b");
    expect(header?.hasAttribute("data-desktop-window-drag-region")).toBe(true);
    expect(screen.getByText("研究助手").className).toContain("ui-type-supporting");
    expect(screen.getByText("Thread").className).toContain("ui-type-caption");
    expect(back.className).toContain("rounded-full");

    fireEvent.click(back);
    expect(onClose).toHaveBeenCalledOnce();
    fireEvent.click(screen.getByRole("button", { name: MESSAGES[locale]["room.thread_close"] }));
    expect(onClose).toHaveBeenCalledTimes(2);
  });
});
