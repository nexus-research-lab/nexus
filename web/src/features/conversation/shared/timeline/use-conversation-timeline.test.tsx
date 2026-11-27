// INPUT: 跳转加载、连续窗口淘汰前后的消息窗口、运行态证据与已解析轮次。
// OUTPUT: 导航索引保留投影过消息正文的轮次，只移除确认无可见内容的轮次；被丢弃的渲染不改写该结论。
// POS: 时间线对已解析轮次的保留规则回归。
import { renderHook } from "@testing-library/react";
import { expect, it } from "vitest";

import type { SessionRoundIndexItem } from "@/types/conversation/history";
import type { Message } from "@/types/conversation/message/entity";

import {
  useConversationTimeline,
  type UseConversationTimelineOptions,
} from "./use-conversation-timeline";

function roundIndexItem(roundId: string): SessionRoundIndexItem {
  return {
    agentIds: [],
    durationMs: null,
    hasUserMessage: true,
    isLive: false,
    roundId,
    status: null,
    timestamp: null,
    title: roundId,
  };
}

function userMessage(roundId: string, overrides: Partial<Message> = {}): Message {
  return {
    agent_id: "agent",
    content: roundId,
    message_id: `${roundId}-user`,
    role: "user",
    round_id: roundId,
    session_key: "session",
    timestamp: 1,
    ...overrides,
  } as Message;
}

function timelineOptions(
  messages: Message[],
  resolvedRoundIds: string[],
  overrides: Partial<UseConversationTimelineOptions> = {},
): UseConversationTimelineOptions {
  return {
    chat_type: "dm",
    live_round_ids: [],
    messages,
    resolved_history_round_ids: resolvedRoundIds,
    round_index_items: ["round-40", "round-hidden", "round-520"].map(roundIndexItem),
    ...overrides,
  };
}

function useNavigatorRoundIds(options: UseConversationTimelineOptions): string[] {
  return useConversationTimeline(options).round_index_items.map((item) => item.roundId);
}

it("keeps a jumped-to round in the navigator after the window evicts it", () => {
  const latest = userMessage("round-520");
  const hidden = userMessage("round-hidden", { hidden_from_user: true });
  const hook = renderHook(
    useNavigatorRoundIds,
    { initialProps: timelineOptions([latest], []) },
  );
  expect(hook.result.current).toEqual(["round-40", "round-hidden", "round-520"]);

  // 跳转加载把两轮都标记为已解析：一轮有正文，另一轮只有对用户隐藏的续跑输入。
  hook.rerender(timelineOptions(
    [userMessage("round-40"), hidden],
    ["round-40", "round-hidden"],
  ));
  expect(hook.result.current).toEqual(["round-40", "round-520"]);

  // 回到最新后连续窗口淘汰旧轮次；淘汰不等于该轮没有内容。
  hook.rerender(timelineOptions([latest], ["round-40", "round-hidden"]));
  expect(hook.result.current).toEqual(["round-40", "round-520"]);
});

it("drops a resolved round once its runtime-only evidence clears", () => {
  const latest = userMessage("round-520");
  const slot = {
    agent_id: "agent",
    agent_round_id: "agent-round",
    msg_id: "slot-message",
    round_id: "round-hidden",
    status: "streaming" as const,
    timestamp: 1,
  };
  const hook = renderHook(useNavigatorRoundIds, {
    initialProps: timelineOptions([latest], ["round-hidden"], {
      chat_type: "group",
      pending_agent_slots: [slot],
    }),
  });
  expect(hook.result.current).toEqual(["round-40", "round-hidden", "round-520"]);

  // slot 没有留下任何消息正文就结束：该轮从未有过可见内容，不能留下空刻度。
  hook.rerender(timelineOptions([latest], ["round-hidden"], { chat_type: "group" }));
  expect(hook.result.current).toEqual(["round-40", "round-520"]);
});
