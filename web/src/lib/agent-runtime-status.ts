import { asUnknownRecord, readNumber, readString } from "@/lib/unknown-value";
import type { AgentRuntimeStatus } from "@/types/agent/agent";

const AGENT_RUNTIME_STATUSES = new Set<AgentRuntimeStatus["status"]>([
  "idle",
  "running",
]);

export function parseAgentRuntimeStatus(value: unknown): AgentRuntimeStatus | null {
  const record = asUnknownRecord(value);
  if (!record) {
    return null;
  }
  const agentId = readString(record, "agent_id");
  const runningTaskCount = readNumber(record, "running_task_count");
  const status = readString(record, "status") as AgentRuntimeStatus["status"] | null;
  if (
    !agentId
    || runningTaskCount === null
    || !status
    || !AGENT_RUNTIME_STATUSES.has(status)
  ) {
    return null;
  }
  return {
    agent_id: agentId,
    running_task_count: runningTaskCount,
    status,
  };
}
