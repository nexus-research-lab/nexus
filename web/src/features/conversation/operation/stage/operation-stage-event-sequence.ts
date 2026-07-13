import type { NexusOperationEvent } from "../operation-types";

export function eventSequenceLabel(event: NexusOperationEvent, events: NexusOperationEvent[]): string {
  const index = events.findIndex((item) => item.id === event.id);
  if (index >= 0) {
    return `第 ${index + 1} 步`;
  }
  return "当前步";
}
