// INPUT: 现有 Conversation bottom stack 的 Goal、可靠性状态与 Composer children。
// OUTPUT: Goal 仍复用原节点，但锚在 Composer 上缘并从正常 status flow 脱离，避免改变正文 viewport 的最低点。
// POS: Conversation layout 几何合同；不测试 Goal 业务状态或 Composer 内部表单。

import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { ConversationPanelBottomArea } from "./conversation-panel-layout";

const I18N_VALUE = {
  locale: "zh" as const,
  setLocale: vi.fn(),
  t: (key: string) => key,
};

const EMPTY_RELIABILITY = {
  failure: null,
  provider_retry: null,
  transport_phase: "healthy" as const,
};

function renderBottomArea(goal: React.ReactNode, activity?: React.ReactNode) {
  return render(
    <I18N_CONTEXT.Provider value={I18N_VALUE}>
      <ConversationPanelBottomArea
        activity={activity}
        goal={goal}
        isMobileLayout={false}
        isReconciling={false}
        onReconcile={vi.fn()}
        providerWarningVisible={false}
        reliability={EMPTY_RELIABILITY}
        scrollToLatest={{ isGenerating: false, onClick: vi.fn(), visible: false }}
      >
        <div data-test-composer>composer</div>
      </ConversationPanelBottomArea>
    </I18N_CONTEXT.Provider>,
  );
}

describe("ConversationPanelBottomArea Goal anchoring", () => {
  it("anchors the existing Goal node to the Composer edge without occupying the status flow", () => {
    const { container } = renderBottomArea(<div data-test-goal>goal</div>);
    const floatingGoal = container.querySelector("[data-conversation-goal-float]");
    const statusStack = container.querySelector("[data-conversation-status-stack]");
    const composerAnchor = container.querySelector("[data-conversation-composer-anchor]");

    expect(floatingGoal?.className).toContain("bottom-full");
    expect(composerAnchor?.contains(floatingGoal)).toBe(true);
    expect(statusStack?.querySelector("[data-test-goal]")).toBeNull();
    expect(composerAnchor?.querySelector("[data-test-composer]")).toBeTruthy();
  });

  it("uses the same compact gap for Activity, Goal and Composer", () => {
    const { container } = renderBottomArea(
      <div data-test-goal>goal</div>,
      <div data-test-activity>activity</div>,
    );
    const floatingGoal = container.querySelector<HTMLElement>("[data-conversation-goal-float]");
    const activityDock = floatingGoal?.querySelector<HTMLElement>("[data-conversation-activity-dock]");

    expect(floatingGoal?.className).toContain("pb-3");
    expect(activityDock?.className).toContain("mb-2");
    expect(activityDock?.querySelector("[data-test-activity]")).toBeTruthy();
  });

  it("keeps Goal hidden when a higher-priority reliability notice owns the status slot", () => {
    const { container } = render(
      <I18N_CONTEXT.Provider value={I18N_VALUE}>
        <ConversationPanelBottomArea
          goal={<div data-test-goal>goal</div>}
          isMobileLayout={false}
          isReconciling={false}
          onReconcile={vi.fn()}
          providerWarningVisible={false}
          reliability={{
            failure: { code: "delivery_unknown", session_key: "session-1" },
            provider_retry: null,
            transport_phase: "healthy",
          }}
          scrollToLatest={{ isGenerating: false, onClick: vi.fn(), visible: false }}
        >
          <div data-test-composer>composer</div>
        </ConversationPanelBottomArea>
      </I18N_CONTEXT.Provider>,
    );

    expect(container.querySelector("[data-conversation-goal-float]")).toBeNull();
    expect(container.querySelector("[data-test-goal]")).toBeNull();
  });
});
