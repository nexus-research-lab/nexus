import type { NexusOperationEvent } from "./operation-types";

export function isLowSignalStageLabel(value: string | null | undefined): value is string {
  if (!value) {
    return true;
  }
  const normalized = value.trim().toLowerCase();
  return (
    !normalized ||
    /^\d+\s+turns?$/.test(normalized) ||
    /^\d+\s+actions?$/.test(normalized) ||
    /^\d+\s+步$/.test(normalized) ||
    normalized.endsWith(" turns") ||
    normalized === "本轮执行收口" ||
    normalized === "当前目标"
  );
}

export function fallbackStageEventObjectLabel(
  event: NexusOperationEvent | null,
  surface_label?: string,
): string {
  if (!event) {
    return "等待第一个应用窗口";
  }
  if (event.kind === "round_summary" || event.surface === "summary") {
    return surface_label === "控制台" ? "控制台日志" : "本轮摘要";
  }
  return event.tool_name ?? `${surface_label ?? "Nexus"}窗口`;
}

export function fallbackStageEventTargetLabel(
  event: NexusOperationEvent,
  surface_label?: string,
): string {
  if (event.kind === "round_summary" || event.surface === "summary") {
    return surface_label === "控制台" ? "本轮日志" : "执行摘要";
  }
  return "等待应用输入";
}

export function displayStageEventTitle(
  event: NexusOperationEvent,
  surface_label?: string,
): string {
  const candidate = event.tool_name ?? event.title;
  if (event.kind === "round_summary" || isLowSignalStageLabel(candidate)) {
    return fallbackStageEventObjectLabel(event, surface_label);
  }
  return candidate;
}

export function displayStageEventTarget(
  event: NexusOperationEvent,
  surface_label?: string,
): string {
  const candidate = event.target ?? event.summary ?? event.title;
  if (event.kind === "round_summary" || isLowSignalStageLabel(candidate)) {
    return fallbackStageEventTargetLabel(event, surface_label);
  }
  return candidate;
}
