/**
 * INPUT: 当前会话消息、session 与 Room Agent 身份。
 * OUTPUT: 单会话任务列表、按 Agent 隔离的 legacy 进程，以及按 Agent round 精确隔离的节点局部 Task run。
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

export interface ConversationTaskRun {
  agentId: string;
  agentRoundId: string;
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

function todoWriteString(
  todo: Record<string, unknown>,
  keys: string[],
): string | null {
  for (const key of keys) {
    const value = todo[key];
    if (typeof value === "string" && value.trim()) {
      return value.trim();
    }
  }
  return null;
}

function todoWriteStatus(value: unknown): TodoItem["status"] | null {
  switch (typeof value === "string" ? value.trim() : "") {
    case "completed":
      return "completed";
    case "in_progress":
      return "in_progress";
    case "pending":
      return "pending";
    default:
      return null;
  }
}

function todoWriteItem(value: unknown): TodoItem | null {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return null;
  }
  const todo = value as Record<string, unknown>;
  // 早期 transcript 使用 task；外部载荷必须在进入 UI 前统一为 TodoItem。
  const content = todoWriteString(todo, ["content", "task"]);
  const status = todoWriteStatus(todo.status);
  if (!content || !status) {
    return null;
  }
  const activeForm = todoWriteString(todo, ["activeForm", "active_form"]);
  return {
    content,
    status,
    ...(activeForm ? {active_form: activeForm} : {}),
  };
}

function todoWritePlan(value: unknown): TodoItem[] | null {
  if (!Array.isArray(value)) {
    return null;
  }
  return value.flatMap((item) => {
    const todo = todoWriteItem(item);
    return todo ? [todo] : [];
  });
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
    ) {
      const nextPlan = todoWritePlan(block.input?.todos);
      if (nextPlan) {
        plan = nextPlan;
      }
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
    hasLaterConversationRound(
      messages,
      sessionKey,
      round.planMessageIndex,
      roundId,
    );
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

/**
 * 分组投影的跨帧缓存。消息对象不可变，组内消息逐个同一引用时上次投影仍然成立；
 * 流式帧只重算正在变化的 Agent / Agent round，其余组直接复用。投影依赖 session
 * 过滤，缓存记录所属 sessionKey，换会话时整体失效。
 */
export interface TodoGroupProjectionCache {
  groups: Map<string, TodoGroupCacheEntry>;
  sessionKey: string | null;
}

interface TodoGroupCacheEntry {
  messages: Message[];
  projection: ConversationTodoProjection;
}

interface TodoMessageGroup {
  indexes: number[];
  messages: Message[];
}

export function createTodoGroupProjectionCache(): TodoGroupProjectionCache {
  return { groups: new Map(), sessionKey: null };
}

function hasSameMessages(left: Message[], right: Message[]): boolean {
  return left.length === right.length
    && left.every((message, index) => message === right[index]);
}

/** 按 key 分组后逐组投影；只保留本次仍存在的组，消失的组随之释放。 */
function projectTodoGroups(
  groups: Map<string, TodoMessageGroup>,
  sessionKey: string,
  cache: TodoGroupProjectionCache,
): Map<string, ConversationTodoProjection> {
  const previous = cache.sessionKey === sessionKey ? cache.groups : null;
  const next = new Map<string, TodoGroupCacheEntry>();
  const projections = new Map<string, ConversationTodoProjection>();
  for (const [key, group] of groups) {
    const prior = previous?.get(key);
    const entry = prior && hasSameMessages(prior.messages, group.messages)
      ? prior
      : {
        messages: group.messages,
        projection: projectConversationTodoProjection(group.messages, sessionKey),
      };
    next.set(key, entry);
    projections.set(key, entry.projection);
  }
  cache.groups = next;
  cache.sessionKey = sessionKey;
  return projections;
}

function groupSessionMessages(
  messages: Message[],
  sessionKey: string,
  resolveKey: (message: Message) => string | null,
): Map<string, TodoMessageGroup> {
  const groups = new Map<string, TodoMessageGroup>();
  messages.forEach((message, messageIndex) => {
    if (!isSameSessionMessage(message, sessionKey)) {
      return;
    }
    const key = resolveKey(message);
    if (!key) {
      return;
    }
    const group = groups.get(key) ?? { indexes: [], messages: [] };
    group.indexes.push(messageIndex);
    group.messages.push(message);
    groups.set(key, group);
  });
  return groups;
}

function agentGroupKey(message: Message): string | null {
  return message.agent_id.trim() || null;
}

// \u0000 不会出现在 ID 中，保证 agent 与 agent round 拼接后仍可唯一拆分。
const AGENT_ROUND_KEY_SEPARATOR = "\u0000";

function agentRoundGroupKey(message: Message): string | null {
  const agentId = message.agent_id.trim();
  const agentRoundId = message.agent_round_id?.trim() ?? "";
  return agentId && agentRoundId
    ? `${agentId}${AGENT_ROUND_KEY_SEPARATOR}${agentRoundId}`
    : null;
}

export function projectConversationTodoProcesses(
  messages: Message[],
  sessionKey: string | null,
  cache: TodoGroupProjectionCache = createTodoGroupProjectionCache(),
): ConversationTodoProcess[] {
  if (!sessionKey || messages.length === 0) {
    cache.groups = new Map();
    cache.sessionKey = sessionKey;
    return [];
  }

  const groups = groupSessionMessages(messages, sessionKey, agentGroupKey);
  const projections = projectTodoGroups(groups, sessionKey, cache);
  const processes: ConversationTodoProcess[] = [];
  for (const [agentId, group] of groups) {
    const projection = projections.get(agentId);
    if (!projection || projection.todos.length === 0) {
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

export function projectConversationTaskRuns(
  messages: Message[],
  sessionKey: string | null,
  cache: TodoGroupProjectionCache = createTodoGroupProjectionCache(),
): ConversationTaskRun[] {
  if (!sessionKey || messages.length === 0) {
    cache.groups = new Map();
    cache.sessionKey = sessionKey;
    return [];
  }

  const groups = groupSessionMessages(messages, sessionKey, agentRoundGroupKey);
  const projections = projectTodoGroups(groups, sessionKey, cache);
  const runs: ConversationTaskRun[] = [];
  for (const [key, group] of groups) {
    const projection = projections.get(key);
    if (!projection || projection.todos.length === 0) {
      continue;
    }
    const [agentId, agentRoundId] = key.split(AGENT_ROUND_KEY_SEPARATOR);
    runs.push({
      agentId,
      agentRoundId,
      latestTaskEventIndex:
        group.indexes[projection.latestTaskEventIndex] ?? -1,
      todos: projection.todos,
    });
  }
  return runs.sort((left, right) => (
    left.latestTaskEventIndex - right.latestTaskEventIndex
    || left.agentId.localeCompare(right.agentId)
    || left.agentRoundId.localeCompare(right.agentRoundId)
  ));
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

export function areTaskRunListsEqual(
  left: ConversationTaskRun[],
  right: ConversationTaskRun[],
): boolean {
  return left === right || (
    left.length === right.length
    && left.every((run, index) => {
      const other = right[index];
      return Boolean(
        other
        && run.agentId === other.agentId
        && run.agentRoundId === other.agentRoundId
        && run.latestTaskEventIndex === other.latestTaskEventIndex
        && areTodoListsEqual(run.todos, other.todos),
      );
    })
  );
}
