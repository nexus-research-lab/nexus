import type { UiBadgeTone } from "@/shared/ui/badge-styles";

export type MemoryLayerFilter = "all" | "agent" | "dm_session" | "room";

export function memory_layer_key(scope?: string): MemoryLayerFilter {
  if (!scope) {
    return "agent";
  }
  if (scope.startsWith("dm_session:")) {
    return "dm_session";
  }
  if (scope.startsWith("room_shared:") || scope.startsWith("room_agent_session:")) {
    return "room";
  }
  return "agent";
}

export function memory_layer_label(scope?: string): string {
  const key = memory_layer_key(scope);
  switch (key) {
  case "dm_session":
    return "DM";
  case "room":
    return "Room";
  default:
    return "Agent";
  }
}

export function memory_scope_label(scope?: string): string {
  if (!scope) {
    return "Agent";
  }
  if (scope.startsWith("user:")) {
    return "User";
  }
  return memory_layer_label(scope);
}

export function format_memory_score(score: number): string {
  return `score ${score.toFixed(2)}`;
}

export function format_memory_time(value?: string): string {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function memory_status_tone(status: string): UiBadgeTone {
  switch (status) {
  case "promoted":
  case "active":
  case "auto":
    return "success";
  case "candidate":
    return "warning";
  case "ignored":
  case "deleted":
    return "idle";
  default:
    return "default";
  }
}
