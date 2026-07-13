import type { OperationPhase } from "../operation-types";

export function activityCpuLoad(running_count: number, finished_count: number): {
  system: number;
  total: number;
  user: number;
} {
  const user = Math.min(72, running_count ? 18 + running_count * 14 : Math.max(3, finished_count * 2));
  const system = Math.min(24, running_count ? 7 + running_count * 3 : 2);
  return {
    system,
    total: Math.min(96, user + system),
    user,
  };
}

export function activityPidLabel(id: string): string {
  let hash = 0;
  for (let index = 0; index < id.length; index += 1) {
    hash = (hash * 33 + id.charCodeAt(index)) >>> 0;
  }
  return String(120 + (hash % 8800));
}

export function activityCpuLabel(phase: OperationPhase, index: number): string {
  if (phase === "running") {
    return `${(12 + index * 3.7).toFixed(1)}`;
  }
  if (phase === "waiting") {
    return "1.2";
  }
  if (phase === "done") {
    return "0.0";
  }
  if (phase === "error" || phase === "cancelled") {
    return "0.1";
  }
  return "0.0";
}
