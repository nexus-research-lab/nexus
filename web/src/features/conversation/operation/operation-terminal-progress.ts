import type { TerminalOutputSnapshotContent } from "@/types/conversation/message/content";

import { redactProjectedTerminalValue } from "./operation-projection-preview";

export function buildTerminalProgressResultPreview(
  snapshot: TerminalOutputSnapshotContent | null | undefined,
): unknown | null {
  if (!isTerminalOutputSnapshot(snapshot) || snapshot.text === "") {
    return null;
  }
  const text = redactTerminalText(snapshot.text);
  if (text === "") {
    return null;
  }
  const timeoutMs = finiteNumber(snapshot.timeout_ms);
  const totalBytes = finiteNumber(snapshot.total_bytes);
  const totalLines = finiteNumber(snapshot.total_lines);
  const terminalOutput: TerminalOutputSnapshotContent = {
    kind: "snapshot",
    stream: "combined",
    text,
    ...(typeof snapshot.tail === "string"
      ? { tail: redactTerminalText(snapshot.tail) }
      : {}),
    ...(timeoutMs == null ? {} : { timeout_ms: timeoutMs }),
    ...(totalBytes == null ? {} : { total_bytes: totalBytes }),
    ...(totalLines == null ? {} : { total_lines: totalLines }),
  };
  return {
    content: text,
    is_error: false,
    terminal_output: terminalOutput,
  };
}

export function terminalProgressResultFromRuntimeDelta(delta: unknown): unknown | null {
  if (!delta || typeof delta !== "object" || Array.isArray(delta)) {
    return null;
  }
  const candidate = (delta as Record<string, unknown>).terminal_output;
  return isTerminalOutputSnapshot(candidate)
    ? buildTerminalProgressResultPreview(candidate)
    : null;
}

function isTerminalOutputSnapshot(value: unknown): value is TerminalOutputSnapshotContent {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return false;
  }
  const snapshot = value as Record<string, unknown>;
  return snapshot.kind === "snapshot"
    && snapshot.stream === "combined"
    && typeof snapshot.text === "string";
}

function finiteNumber(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function redactTerminalText(value: string): string {
  const redacted = redactProjectedTerminalValue(value);
  return typeof redacted === "string" ? redacted : value;
}
