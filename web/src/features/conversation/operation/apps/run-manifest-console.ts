import type { NexusOperationEvent } from "../operation-types";

export function consoleEventLevel(phase: NexusOperationEvent["phase"]): "ERROR" | "INFO" | "NOTICE" {
  if (phase === "error" || phase === "cancelled") {
    return "ERROR";
  }
  if (phase === "waiting" || phase === "queued") {
    return "NOTICE";
  }
  return "INFO";
}

export function consoleEventSubsystem(event: NexusOperationEvent): string {
  if (event.surface === "terminal") {
    return "Terminal";
  }
  if (event.surface === "web") {
    return "Navi";
  }
  if (event.surface === "workspace") {
    return "Finder";
  }
  if (event.surface === "editor") {
    return "Code";
  }
  if (event.surface === "summary") {
    return "Console";
  }
  if (event.surface === "task") {
    return "Activity Monitor";
  }
  if (event.surface === "knowledge") {
    return "Preview";
  }
  return "Nexus";
}
