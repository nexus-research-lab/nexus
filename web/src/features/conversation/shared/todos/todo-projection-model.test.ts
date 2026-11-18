import { expect, it } from "vitest";

import type { AssistantMessage, Message } from "@/types/conversation/message/entity";

import {
  createTodoGroupProjectionCache,
  projectConversationTaskRuns,
  projectConversationTodoProcesses,
} from "./todo-projection-model";

const SESSION = "room:group:conv-1";

function todoWrite(
  id: string,
  agentId: string,
  agentRoundId: string,
  todos: string[],
): AssistantMessage {
  return {
    message_id: id,
    session_key: SESSION,
    agent_id: agentId,
    round_id: "root-1",
    agent_round_id: agentRoundId,
    timestamp: 0,
    role: "assistant",
    content: [{
      type: "tool_use",
      id: `tool-${id}`,
      name: "TodoWrite",
      input: { todos: todos.map((content) => ({ content, status: "pending" })) },
    }],
  };
}

it("reuses unchanged agent groups and matches an uncached projection", () => {
  const history: Message[] = [
    todoWrite("a1", "alice", "ar-1", ["plan A"]),
    todoWrite("b1", "bob", "br-1", ["plan B"]),
  ];
  const processCache = createTodoGroupProjectionCache();
  const runCache = createTodoGroupProjectionCache();
  const firstProcesses = projectConversationTodoProcesses(history, SESSION, processCache);
  const firstRuns = projectConversationTaskRuns(history, SESSION, runCache);

  // 只有 bob 继续输出：alice 的组内消息引用未变，投影应原样复用。
  const streamed = [...history, todoWrite("b2", "bob", "br-2", ["plan B2"])];
  const processes = projectConversationTodoProcesses(streamed, SESSION, processCache);
  const runs = projectConversationTaskRuns(streamed, SESSION, runCache);

  expect(processes).toEqual(projectConversationTodoProcesses(streamed, SESSION));
  expect(runs).toEqual(projectConversationTaskRuns(streamed, SESSION));
  expect(processes[0].todos).toBe(firstProcesses[0].todos);
  expect(processes[1].todos.map((todo) => todo.content)).toEqual(["plan B2"]);
  expect(runs.find((run) => run.agentRoundId === "ar-1")?.todos).toBe(firstRuns[0].todos);
  expect(runs.map((run) => [run.agentId, run.agentRoundId, run.latestTaskEventIndex]))
    .toEqual([["alice", "ar-1", 0], ["bob", "br-1", 1], ["bob", "br-2", 2]]);
});

it("drops cached groups when the session changes", () => {
  const messages: Message[] = [todoWrite("a1", "alice", "ar-1", ["plan A"])];
  const cache = createTodoGroupProjectionCache();
  expect(projectConversationTodoProcesses(messages, SESSION, cache)).toHaveLength(1);
  // 同一批消息换到另一个会话：session 过滤后不再属于它，不能沿用旧投影。
  expect(projectConversationTodoProcesses(messages, "room:group:conv-2", cache)).toEqual([]);
  expect(cache.groups.size).toBe(0);
});
