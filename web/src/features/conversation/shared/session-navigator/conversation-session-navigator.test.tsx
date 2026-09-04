// INPUT: 会话导航项、预览状态与跳转/清理命令。
// OUTPUT: 证明刻度和预览命中区保留指针、焦点和点击语义，预览文字由 App Typography 持有。
// POS: Session Navigator 视图行为测试；导航项投影和跳转事务由各自纯模型/控制器测试负责。

import { createRef } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { ConversationTimeline } from "@/features/conversation/shared/timeline/timeline-model";

import { ConversationSessionNavigator } from "./conversation-session-navigator";

const navigation = vi.hoisted(() => {
  const items = [
    {
      agentIds: ["agent-1"],
      hasUserMessage: true,
      index: 0,
      isLive: false,
      meta: "1m 20s",
      roundId: "round-1",
      summary: "First response",
      time: "09:10",
      title: "First request",
    },
    {
      agentIds: ["agent-2"],
      hasUserMessage: true,
      index: 1,
      isLive: true,
      meta: "Running",
      roundId: "round-2",
      summary: "Second response",
      time: "09:12",
      title: "Second request",
    },
  ];
  return {
    clearPreview: vi.fn(),
    items,
    jumpToRound: vi.fn(),
    previewItemAt: vi.fn(),
  };
});

vi.mock("./use-conversation-session-navigation", () => ({
  useConversationSessionNavigation: () => ({
    activeItem: navigation.items[0],
    clearPreview: navigation.clearPreview,
    items: navigation.items,
    jumpToRound: navigation.jumpToRound,
    previewIndex: 1,
    previewItem: navigation.items[1],
    previewItemAt: navigation.previewItemAt,
  }),
}));

const EMPTY_TIMELINE: ConversationTimeline = {
  feed_round_ids: [],
  indexed_window: { roundIds: [], unloadedRoundIds: [] },
  live_round_ids: [],
  loaded_round_ids: [],
  message_groups: new Map(),
  pending_permission_groups: new Map(),
  pending_slot_groups: new Map(),
  room_agent_execution_state_groups: new Map(),
  round_index_items: [],
};

describe("ConversationSessionNavigator", () => {
  beforeEach(() => {
    navigation.clearPreview.mockClear();
    navigation.jumpToRound.mockClear();
    navigation.previewItemAt.mockClear();
  });

  it("keeps geometry-owned hit targets and semantic preview typography", () => {
    const scrollRef = createRef<HTMLDivElement>();
    const { container } = render(
      <I18nProvider>
        <ConversationSessionNavigator
          scopeKey="agent:session"
          scrollRef={scrollRef}
          timeline={EMPTY_TIMELINE}
        />
      </I18nProvider>,
    );

    const preview = container.querySelector<HTMLElement>(
      "[data-session-navigator-preview='true']",
    );
    expect(preview).toBeTruthy();
    const tickButtons = screen.getAllByRole("button").filter(
      (button) => !button.hasAttribute("data-session-navigator-preview"),
    );
    expect(tickButtons).toHaveLength(2);

    fireEvent.pointerEnter(tickButtons[1]);
    fireEvent.focus(tickButtons[1]);
    expect(navigation.previewItemAt).toHaveBeenCalledTimes(2);
    expect(navigation.previewItemAt).toHaveBeenLastCalledWith(navigation.items[1]);

    fireEvent.click(tickButtons[1]);
    fireEvent.click(preview!);
    expect(navigation.jumpToRound).toHaveBeenCalledTimes(2);
    expect(navigation.jumpToRound).toHaveBeenLastCalledWith(navigation.items[1]);

    fireEvent.mouseLeave(screen.getByRole("navigation"));
    expect(navigation.clearPreview).toHaveBeenCalledTimes(1);

    expect(screen.getByText("Second request").className).toContain(
      "ui-type-supporting",
    );
    expect(screen.getByText("09:12").className).toContain("ui-type-caption");
    expect(screen.getByText("Second response").className).toContain(
      "ui-type-metadata",
    );
    expect(screen.getByText("Running").parentElement?.className).toContain(
      "ui-type-caption",
    );
  });
});
