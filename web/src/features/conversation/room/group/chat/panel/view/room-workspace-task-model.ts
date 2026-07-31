/**
 * INPUT: Room 成员、按 Agent 隔离的进程集合与用户选择。
 * OUTPUT: 保持成员目录顺序、优先用户选择并降级到最近进程的稳定选择结果。
 * POS: Room Workspace Task Agent 切换器的纯选择模型。
 */
import type { ConversationTodoProcess } from "@/features/conversation/shared/todos/todo-projection-model";
import type { Agent } from "@/types/agent/agent";

export function resolveRoomTaskSelection(
  processes: ConversationTodoProcess[],
  roomMembers: Agent[],
  selectedAgentId: string | null,
): {
  member: Agent;
  members: Agent[];
  process: ConversationTodoProcess;
} | null {
  const processByAgentId = new Map(
    processes
      .filter((process) => process.todos.length > 0)
      .map((process) => [process.agentId, process]),
  );
  const members = roomMembers.filter((member) => (
    processByAgentId.has(member.agent_id)
  ));
  if (members.length === 0) {
    return null;
  }

  const selectedProcess = selectedAgentId
    ? processByAgentId.get(selectedAgentId)
    : null;
  const process = selectedProcess ?? members.reduce<ConversationTodoProcess>(
    (latest, member) => {
      const candidate = processByAgentId.get(member.agent_id);
      return candidate
        && candidate.latestTaskEventIndex > latest.latestTaskEventIndex
        ? candidate
        : latest;
    },
    processByAgentId.get(members[0].agent_id)!,
  );
  const member = members.find((candidate) => (
    candidate.agent_id === process.agentId
  ));
  return member ? { member, members, process } : null;
}
