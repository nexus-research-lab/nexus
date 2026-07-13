import type { NexusOperationEvent } from "./operation-types";

export function buildOperationTerminalLines(events: NexusOperationEvent[]): string[] {
  return events.flatMap((event) => {
    const command = readTerminalCommand(event);
    const result_lines = terminal_result_lines(event).filter((line) => !terminal_line_matches_command(line, command));
    if (event.kind === "command_stop" || event.tool_name === "KillShell") {
      return result_lines;
    }
    return [
      `$ ${command}`,
      ...result_lines,
    ];
  }).slice(-80);
}

export function readTerminalCommand(event: NexusOperationEvent): string {
  return readInputString(event.input_preview, ["command", "description"])
    ?? event.target
    ?? event.tool_name
    ?? "";
}

function terminal_result_lines(event: NexusOperationEvent): string[] {
  const result_lines = extract_terminal_lines(event.result_preview).slice(0, 24);
  if (result_lines.length > 0) {
    return result_lines;
  }
  if (event.summary) {
    return split_terminal_text(event.summary).slice(0, 8);
  }
  return [];
}

function extract_terminal_lines(value: unknown): string[] {
  if (value == null) {
    return [];
  }
  if (typeof value === "string") {
    return split_terminal_text(value);
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return [String(value)];
  }
  if (Array.isArray(value)) {
    return value.flatMap((item) => extract_terminal_lines(item));
  }
  if (typeof value === "object") {
    const record = value as Record<string, unknown>;
    const preferred_keys = ["stdout", "stderr", "output", "text", "content", "result", "message", "error"] as const;
    const lines = preferred_keys.flatMap((key) => extract_terminal_lines(record[key]));
    if (lines.length > 0) {
      return lines;
    }
    return split_terminal_text(safeJsonStringify(value));
  }
  return [String(value)];
}

function split_terminal_text(value: string): string[] {
  const normalized = value.replace(/\r\n/g, "\n").replace(/\r/g, "\n");
  if (!normalized.trim()) {
    return [];
  }
  return normalized.split("\n").map((line) => line.trimEnd());
}

function terminal_line_matches_command(line: string, command: string): boolean {
  return line.replace(/^\s*[$>]\s?/, "").trim() === command.trim();
}

function readInputString(
  input: Record<string, unknown> | null | undefined,
  keys: string[],
): string | null {
  if (!input) {
    return null;
  }
  for (const key of keys) {
    const value = input[key];
    if (typeof value === "string" && value.trim()) {
      return value.trim();
    }
  }
  return null;
}

function safeJsonStringify(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}
