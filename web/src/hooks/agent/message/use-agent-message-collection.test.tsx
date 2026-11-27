// INPUT: 连续的消息窗口写入。
// OUTPUT: 验证 windowRef 在 React 渲染前已同步前进，且多次写入按顺序叠加。
// POS: 防止历史分页拿新游标配旧窗口头部而重复请求同一页。

import { act, renderHook } from "@testing-library/react";
import { expect, it } from "vitest";

import type { Message } from "@/types/conversation/message/entity";

import { useAgentMessageCollection } from "./use-agent-message-collection";

function message(id: string, roundId: string, timestamp: number): Message {
  return {
    content: "text",
    message_id: id,
    role: "assistant",
    round_id: roundId,
    timestamp,
  } as unknown as Message;
}

it("advances windowRef synchronously before React renders", () => {
  const { result } = renderHook(() => useAgentMessageCollection());
  act(() => {
    result.current.updateMessageWindow((current) => ({
      ...current,
      messages: [...current.messages, message("m2", "r2", 2)],
    }));
    // 同一批次内、渲染前读取：必须已经看到刚写入的 round。
    expect(result.current.windowRef.current.messages.map((item) => item.round_id)).toEqual(["r2"]);
    result.current.updateMessageWindow((current) => ({
      ...current,
      messages: [message("m1", "r1", 1), ...current.messages],
    }));
    expect(result.current.windowRef.current.messages.map((item) => item.round_id)).toEqual(["r1", "r2"]);
  });
  expect(result.current.messages.map((item) => item.round_id)).toEqual(["r1", "r2"]);
});
