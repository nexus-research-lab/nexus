import { asUnknownRecord } from "@/lib/unknown-value";
import type { EventMessage } from "@/types/generated/protocol";

/** WebSocket 边界只接纳完整信封，具体 data 由事件所有者继续解码。 */
export function parseEventMessage(value: unknown): EventMessage | null {
  const record = asUnknownRecord(value);
  if (
    !record
    || typeof record.event_type !== "string"
    || typeof record.protocol_version !== "number"
    || typeof record.timestamp !== "number"
    || !asUnknownRecord(record.data)
  ) {
    return null;
  }
  return record as unknown as EventMessage;
}
