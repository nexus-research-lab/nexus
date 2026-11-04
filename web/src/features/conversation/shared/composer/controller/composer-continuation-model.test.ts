// INPUT: 最新用户轮次、Room 成员与不同助手终态。
// OUTPUT: 仅当前未完成的普通对话获得继续目标。
// POS: 继续资格的跨 DM/Room 业务回归。
import { expect, it } from "vitest";
import type { Message } from "@/types/conversation/message/entity";
import { resolveConversationContinuation } from "./composer-continuation-model";
const user = (round = "r") => ({ role: "user", round_id: round, message_id: `user-${round}` }) as Message;
const assistant = (id: string, subtype = "interrupted", round = "r") => ({ role: "assistant", round_id: round, message_id: id, agent_id: id, agent_round_id: `agent-${id}`, result_summary: { subtype } }) as Message;

it("continues only interrupted members of the latest visible request", () => {
  const messages = [user("old"), assistant("old", "interrupted", "old"), user(), assistant("a"), assistant("b", "success"), assistant("c"), assistant("removed")];
  expect(resolveConversationContinuation(messages, ["a", "b", "c"])?.targetAgentIds).toEqual(["a", "c"]);
  expect(resolveConversationContinuation([user(), assistant("a")])?.targetAgentIds).toEqual([]);
  expect(resolveConversationContinuation([...messages, user("next")])).toBeNull();
});
it("does not infer unfinished work from successful, hidden or still running output", () => {
  expect(resolveConversationContinuation([user(), assistant("a", "success")])).toBeNull();
  expect(resolveConversationContinuation([user(), { ...assistant("a"), hidden_from_user: true }])).toBeNull();
  expect(resolveConversationContinuation([user(), assistant("a"), { ...assistant("a"), result_summary: undefined } as Message])).toBeNull();
  expect(resolveConversationContinuation([])).toBeNull();
});
