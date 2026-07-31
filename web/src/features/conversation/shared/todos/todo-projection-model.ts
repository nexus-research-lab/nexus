/**
 * INPUT: 当前会话消息、session 与 Room Agent 身份。
 * OUTPUT: 单会话任务列表，以及按 Agent 隔离且带最近任务事件位置的进程集合。
 * POS: Conversation Todo/Task 工具到 DM/Room 进程 UI 的唯一纯投影入口。
 */
import { areEquivalentSessionKeys } from "@/lib/conversation/session-key";
import type {
  AssistantMessage,
  Message,
} from "@/types/conversation/message/entity";
import type { TodoItem } from "@/types/conversation/todo";

import {
  completeOrphanRuntimeTasks,
  mergeTodoPlanWithRuntimeTasks,
  upsertAssistantRuntimeTask,
  upsertSystemRuntimeTask,
} from "./runtime-task-model";
import { projectTaskListToolTodos } from "./task-list-tool-model";

interface MutableTodoRound {
  plan: TodoItem[] | null;
  planMessageIndex: number;
  runtimeTasksById: Map<string, TodoItem>;
  latestTaskEventIndex: number;
  latestSummary: {isError: boolean} | null;
}

type TodoRoundProjectionKind = "hidden" | "plan" | "runtime";

interface TodoRoundProjection {
  completeRuntimeTasks: boolean;
  kind: TodoRoundProjectionKind;
  plan: TodoItem[];
  runtimeTasks: TodoItem[];
}

export interface ConversationTodoProcess {
  agentId: string;
  latestTaskEventIndex: number;
  todos: TodoItem[];
}

interface ConversationTodoProjection {
  latestTaskEventIndex: number;
  todos: TodoItem[];
}

const TODO_ROUND_PROJECTORS: Record<
  TodoRoundProjectionKind,
  (projection: TodoRoundProjection) => TodoItem[]
> = {
  hidden: () => [],
  plan: ({ plan, runtimeTasks }) =>
    mergeTodoPlanWithRuntimeTasks(plan, runtimeTasks),
  runtime: ({ completeRuntimeTasks, runtimeTasks }) =>
    completeOrphanRuntimeTasks(runtimeTasks, completeRuntimeTasks),
};

function createTodoRound(): MutableTodoRound {
  return {
    plan: null,
    planMessageIndex: -1,
    runtimeTasksById: new Map(),
    latestTaskEventIndex: -1,
    latestSummary: null,
  };
}

function isSameSessionMessage(message: Message, sessionKey: string): boolean {
  return !message.session_key || areEquivalentSessionKeys(message.session_key, sessionKey);
}

function extractTodoPlan(message: AssistantMessage): TodoItem[] | null {
  if (!Array.isArray(message.content)) {
    return null;
  }
  let plan: TodoItem[] | null = null;
  for (const block of message.content) {
    if (
      block?.type === "tool_use"
      && block.name === "TodoWrite"
      && block.input
      && Array.isArray(block.input.todos)
    ) {
      plan = block.input.todos;
    }
  }
  return plan;
}

function indexAssistantMessage(
  round: MutableTodoRound,
  message: AssistantMessage,
  messageIndex: number,
) {
  const plan = extractTodoPlan(message);
  if (plan) {
    round.plan = plan;
    round.planMessageIndex = messageIndex;
    round.latestTaskEventIndex = messageIndex;
  }

  if (Array.isArray(message.content)) {
    for (const block of message.content) {
      if (
        block?.type === "task_progress"
        && upsertAssistantRuntimeTask(round.runtimeTasksById, block)
      ) {
        round.latestTaskEventIndex = messageIndex;
      }
    }
  }
  if (message.result_summary) {
    round.latestSummary = {isError: Boolean(message.result_summary.is_error)};
  }
}

function buildTodoRoundIndex(
  messages: Message[],
  sessionKey: string,
): Map<string, MutableTodoRound> {
  const rounds = new Map<string, MutableTodoRound>();
  messages.forEach((message, messageIndex) => {
    if (!message.round_id || !isSameSessionMessage(message, sessionKey)) {
      return;
    }
    const round = rounds.get(message.round_id) ?? createTodoRound();
    rounds.set(message.round_id, round);

    if (message.role === "assistant") {
      indexAssistantMessage(round, message, messageIndex);
    } else if (
      message.role === "system"
      && upsertSystemRuntimeTask(round.runtimeTasksById, message)
    ) {
      round.latestTaskEventIndex = messageIndex;
    }
  });
  return rounds;
}

function hasLaterConversationRound(
  messages: Message[],
  sessionKey: string,
  planMessageIndex: number,
  roundId: string,
): boolean {
  return messages.slice(planMessageIndex + 1).some((message) => (
    message.role !== "system"
    && message.round_id !== undefined
    && message.round_id !== roundId
    && isSameSessionMessage(message, sessionKey)
  ));
}

function findLatestTodoRound(
  rounds: Map<string, MutableTodoRound>,
): [string, MutableTodoRound] | null {
  let latest: [string, MutableTodoRound] | null = null;
  for (const entry of rounds) {
    const round = entry[1];
    if (
      round.latestTaskEventIndex >= 0 &&
      (!latest || round.latestTaskEventIndex > latest[1].latestTaskEventIndex)
    ) {
      latest = entry;
    }
  }
  return latest;
}

function buildTodoRoundProjection(
  messages: Message[],
  sessionKey: string,
  roundId: string,
  round: MutableTodoRound,
): TodoRoundProjection {
  const runtimeTasks = [...round.runtimeTasksById.values()];
  if (!round.plan) {
    return {
      completeRuntimeTasks: round.latestSummary?.isError === false,
      kind: "runtime",
      plan: [],
      runtimeTasks,
    };
  }

  const shouldHidePlan =
    round.plan.length === 0 ||
    round.latestSummary?.isError === true ||
    (!round.latestSummary &&
      hasLaterConversationRound(
        messages,
        sessionKey,
        round.planMessageIndex,
        roundId,
      ));
  return {
    completeRuntimeTasks: false,
    kind: shouldHidePlan ? "hidden" : "plan",
    plan: round.plan,
    runtimeTasks,
  };
}

export function projectConversationTodos(
  messages: Message[],
  sessionKey: string | null,
): TodoItem[] {
  return projectConversationTodoProjection(messages, sessionKey).todos;
}

export function projectConversationTodoProcesses(
  messages: Message[],
  sessionKey: string | null,
): ConversationTodoProcess[] {
  if (!sessionKey || messages.length === 0) {
    return [];
  }

  const messagesByAgent = new Map<string, {
    indexes: number[];
    messages: Message[];
  }>();
  messages.forEach((message, messageIndex) => {
    if (!isSameSessionMessage(message, sessionKey)) {
      return;
    }
    const agentId = message.agent_id.trim();
    if (!agentId) {
      return;
    }
    const group = messagesByAgent.get(agentId) ?? {
      indexes: [],
      messages: [],
    };
    group.indexes.push(messageIndex);
    group.messages.push(message);
    messagesByAgent.set(agentId, group);
  });

  const processes: ConversationTodoProcess[] = [];
  for (const [agentId, group] of messagesByAgent) {
    const projection = projectConversationTodoProjection(
      group.messages,
      sessionKey,
    );
    if (projection.todos.length === 0) {
      continue;
    }
    processes.push({
      agentId,
      latestTaskEventIndex:
        group.indexes[projection.latestTaskEventIndex] ?? -1,
      todos: projection.todos,
    });
  }
  return processes;
}

function projectConversationTodoProjection(
  messages: Message[],
  sessionKey: string | null,
): ConversationTodoProjection {
  if (!sessionKey || messages.length === 0) {
    return {
      latestTaskEventIndex: -1,
      todos: [],
    };
  }

  const taskListProjection = projectTaskListToolTodos(messages, sessionKey);
  if (taskListProjection.observed) {
    return {
      latestTaskEventIndex: taskListProjection.latestTaskEventIndex,
      todos: taskListProjection.todos,
    };
  }

  const roundIndex = buildTodoRoundIndex(messages, sessionKey);
  const activeRoundEntry = findLatestTodoRound(roundIndex);
  if (!activeRoundEntry) {
    return {
      latestTaskEventIndex: -1,
      todos: [],
    };
  }

  const [roundId, activeRound] = activeRoundEntry;
  const projection = buildTodoRoundProjection(
    messages,
    sessionKey,
    roundId,
    activeRound,
  );
  return {
    latestTaskEventIndex: activeRound.latestTaskEventIndex,
    todos: TODO_ROUND_PROJECTORS[projection.kind](projection),
  };
}

export function areTodoListsEqual(left: TodoItem[], right: TodoItem[]): boolean {
  return left === right || (
    left.length === right.length
    && left.every((todo, index) => {
      const other = right[index];
      return Boolean(
        other
        && todo.content === other.content
        && todo.status === other.status
        && todo.active_form === other.active_form,
      );
    })
  );
}

export function areTodoProcessListsEqual(
  left: ConversationTodoProcess[],
  right: ConversationTodoProcess[],
): boolean {
  return left === right || (
    left.length === right.length
    && left.every((process, index) => {
      const other = right[index];
      return Boolean(
        other
        && process.agentId === other.agentId
        && process.latestTaskEventIndex === other.latestTaskEventIndex
        && areTodoListsEqual(process.todos, other.todos),
      );
    })
  );
}
