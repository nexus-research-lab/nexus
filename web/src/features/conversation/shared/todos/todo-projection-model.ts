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

interface MutableTodoRound {
  plan: TodoItem[] | null;
  planMessageIndex: number;
  runtimeTasksById: Map<string, TodoItem>;
  latestTaskEventIndex: number;
  latestSummary: {isError: boolean} | null;
}

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

export function projectConversationTodos(
  messages: Message[],
  sessionKey: string | null,
): TodoItem[] {
  if (!sessionKey || messages.length === 0) {
    return [];
  }

  const roundIndex = buildTodoRoundIndex(messages, sessionKey);
  const activeRoundEntry = [...roundIndex.entries()]
    .filter(([, round]) => round.latestTaskEventIndex >= 0)
    .sort((left, right) => (
      right[1].latestTaskEventIndex - left[1].latestTaskEventIndex
    ))[0];
  if (!activeRoundEntry) {
    return [];
  }

  const [roundId, activeRound] = activeRoundEntry;
  const runtimeTasks = [...activeRound.runtimeTasksById.values()];
  if (!activeRound.plan) {
    return completeOrphanRuntimeTasks(
      runtimeTasks,
      Boolean(activeRound.latestSummary && !activeRound.latestSummary.isError),
    );
  }
  if (activeRound.plan.length === 0 || activeRound.latestSummary?.isError) {
    return [];
  }
  if (
    !activeRound.latestSummary
    && hasLaterConversationRound(
      messages,
      sessionKey,
      activeRound.planMessageIndex,
      roundId,
    )
  ) {
    return [];
  }
  return mergeTodoPlanWithRuntimeTasks(activeRound.plan, runtimeTasks);
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
