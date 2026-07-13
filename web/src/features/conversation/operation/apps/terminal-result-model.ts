const MAX_TERMINAL_ROWS = 600;

export type TerminalOutputStream = "stdout" | "stderr" | "output";

export interface TerminalOutputRow {
  id: string;
  stream: TerminalOutputStream;
  text: string;
}

export interface TerminalResultView {
  exitCode: number | null;
  rows: TerminalOutputRow[];
  stderr: string[];
  stdout: string[];
  output: string[];
}

interface PendingTerminalRow {
  stream: TerminalOutputStream;
  text: string;
}

export function parseTerminalResult(value: unknown): TerminalResultView {
  const pendingRows: PendingTerminalRow[] = [];
  appendTerminalValue(value, "output", pendingRows, new Set());
  const rows = pendingRows.slice(0, MAX_TERMINAL_ROWS).map((row, index) => ({
    id: `${row.stream}:${index}`,
    ...row,
  }));

  return {
    exitCode: readTerminalExitCode(value),
    rows,
    stderr: rows.filter((row) => row.stream === "stderr").map((row) => row.text),
    stdout: rows.filter((row) => row.stream === "stdout").map((row) => row.text),
    output: rows.filter((row) => row.stream === "output").map((row) => row.text),
  };
}

export function readTerminalExitCode(value: unknown): number | null {
  return readTerminalNumericField(value, [
    "exit_code",
    "exitCode",
    "exit_status",
    "exitStatus",
    "return_code",
    "returnCode",
  ]);
}

export function readTerminalResultString(value: unknown, keys: readonly string[]): string | null {
  if (!value || typeof value !== "object") {
    return null;
  }
  if (Array.isArray(value)) {
    for (const item of value) {
      const result = readTerminalResultString(item, keys);
      if (result) {
        return result;
      }
    }
    return null;
  }

  const record = value as Record<string, unknown>;
  for (const key of keys) {
    const candidate = terminalScalarString(record[key]);
    if (candidate) {
      return candidate;
    }
  }
  return readTerminalResultString(record.content, keys)
    ?? readTerminalResultString(record.result, keys)
    ?? readTerminalResultString(record.data, keys);
}

export function splitTerminalText(value: string): string[] {
  const normalized = stripTerminalControlSequences(value)
    .replace(/\r\n/g, "\n")
    .replace(/\r/g, "\n");
  if (!normalized.trim()) {
    return [];
  }
  const lines = normalized.split("\n").map((line) => line.trimEnd());
  while (lines.at(-1) === "") {
    lines.pop();
  }
  return lines;
}

function appendTerminalValue(
  value: unknown,
  stream: TerminalOutputStream,
  rows: PendingTerminalRow[],
  visited: Set<unknown>,
): void {
  if (value == null || rows.length >= MAX_TERMINAL_ROWS) {
    return;
  }
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
    for (const text of splitTerminalText(String(value))) {
      rows.push({ stream, text });
    }
    return;
  }
  if (typeof value !== "object" || visited.has(value)) {
    return;
  }
  visited.add(value);

  if (Array.isArray(value)) {
    for (const item of value) {
      appendTerminalValue(item, stream, rows, visited);
    }
    return;
  }

  const record = value as Record<string, unknown>;
  if (record.type === "text" && record.text != null) {
    appendTerminalValue(record.text, stream, rows, visited);
    return;
  }

  const hasExplicitStreams = ["stdout", "out", "stderr", "err"].some((key) => record[key] != null);
  if (hasExplicitStreams) {
    appendTerminalValue(record.stdout ?? record.out, "stdout", rows, visited);
    appendTerminalValue(record.stderr ?? record.err, "stderr", rows, visited);
  }

  const contentStream = record.is_error === true || record.subtype === "error" ? "stderr" : stream;
  const genericKeys = hasExplicitStreams
    ? ["output", "message", "text"]
    : ["content", "output", "result", "message", "text"];
  let appendedGeneric = false;
  for (const key of genericKeys) {
    if (record[key] == null) {
      continue;
    }
    appendTerminalValue(record[key], key === "content" ? contentStream : stream, rows, visited);
    appendedGeneric = true;
  }

  if (!hasExplicitStreams && !appendedGeneric && shouldRenderStructuredResult(record)) {
    appendTerminalValue(safeJsonStringify(record), stream, rows, visited);
  }
}

function readTerminalNumericField(value: unknown, keys: readonly string[]): number | null {
  if (!value || typeof value !== "object") {
    return null;
  }
  if (Array.isArray(value)) {
    for (const item of value) {
      const result = readTerminalNumericField(item, keys);
      if (result != null) {
        return result;
      }
    }
    return null;
  }
  const record = value as Record<string, unknown>;
  for (const key of keys) {
    const candidate = record[key];
    if (typeof candidate === "number" && Number.isFinite(candidate)) {
      return candidate;
    }
    if (typeof candidate === "string" && /^-?\d+$/.test(candidate.trim())) {
      return Number(candidate);
    }
  }
  return readTerminalNumericField(record.content, keys)
    ?? readTerminalNumericField(record.result, keys)
    ?? readTerminalNumericField(record.data, keys);
}

function terminalScalarString(value: unknown): string | null {
  if (typeof value === "string" && value.trim()) {
    return value.trim();
  }
  if (typeof value === "number" && Number.isFinite(value)) {
    return String(value);
  }
  return null;
}

function shouldRenderStructuredResult(record: Record<string, unknown>): boolean {
  const metadataKeys = new Set([
    "error_code",
    "exit_code",
    "exit_status",
    "is_error",
    "return_code",
    "type",
  ]);
  return Object.keys(record).some((key) => !metadataKeys.has(key));
}

function stripTerminalControlSequences(value: string): string {
  return value
    .replace(/\u001B\][^\u0007]*(?:\u0007|\u001B\\)/g, "")
    .replace(/\u001B(?:[@-_]|\[[0-?]*[ -/]*[@-~])/g, "")
    .replace(/[\u0000-\u0008\u000B\u000C\u000E-\u001F\u007F]/g, "");
}

function safeJsonStringify(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}
