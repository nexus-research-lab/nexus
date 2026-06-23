export const SCHEDULED_TASKS_MUTATED_EVENT = "nexus:scheduled-tasks-mutated";

export function notify_scheduled_tasks_mutated(agent_id: string): void {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent(SCHEDULED_TASKS_MUTATED_EVENT, { detail: { agent_id } }));
}
