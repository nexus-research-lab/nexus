import { formatOperationTime as formatOperationTime } from "../operation-preview";
import {
  isTerminalCommandEvent,
  isTerminalControlEvent,
  readTerminalCommandSessionId,
  readTerminalControlSessionId,
} from "../operation-terminal-session-events";
import type { NexusOperationEvent, OperationPhase } from "../operation-types";
import {
  parseTerminalResult,
  readTerminalResultString,
} from "./terminal-result-model";
import type {
  TerminalOutputRow,
  TerminalResultView,
} from "./terminal-result-model";

export interface TerminalControlEvent {
  durationLabel: string | null;
  id: string;
  phase: OperationPhase;
  resultRows: TerminalOutputRow[];
  statusLabel: string;
  targetLabel: string | null;
}

export interface TerminalEntry {
  command: string | null;
  controls: TerminalControlEvent[];
  cwdLabel: string | null;
  durationLabel: string | null;
  id: string;
  phase: OperationPhase;
  result: TerminalResultView;
  shellId: string | null;
  startedLabel: string;
  statusLabel: string;
  statusTone: "success" | "error" | "running" | "muted";
  toolUseId: string | null;
}

export interface TerminalSessionView {
  entries: TerminalEntry[];
  hasActiveProcess: boolean;
}

export function buildTerminalSession({
  event,
  relatedEvents,
}: {
  event: NexusOperationEvent;
  relatedEvents: NexusOperationEvent[];
}): TerminalSessionView {
  const terminalEvents = dedupeTerminalEvents(
    (relatedEvents.length ? relatedEvents : [event])
      .filter(isTerminalEvent)
      .sort(compareTerminalEvents),
  );
  const commandEntries = terminalEvents
    .filter(isTerminalCommandEvent)
    .map(buildCommandEntry);
  const controls = terminalEvents
    .filter(isTerminalControlEvent)
    .map(buildControlEvent);
  const orphanControls: TerminalControlEvent[] = [];

  for (const control of controls) {
    const targetIndex = findControlTargetIndex(commandEntries, control, terminalEvents);
    if (targetIndex < 0) {
      orphanControls.push(control);
      continue;
    }
    commandEntries[targetIndex].controls.push(control);
    applyControlState(commandEntries[targetIndex], control);
  }

  const entries = [
    ...commandEntries,
    ...orphanControls.map((control) => buildOrphanControlEntry(control, terminalEvents)),
  ].sort((left, right) => compareTerminalEntries(left, right, terminalEvents));

  return {
    entries,
    hasActiveProcess: entries.some((entry) => (
      entry.phase === "queued" || entry.phase === "running" || entry.phase === "waiting"
    )),
  };
}

export function readTerminalCommand(event: NexusOperationEvent): string | null {
  return readInputString(event.input_preview, ["command", "cmd"])
    ?? readEventTarget(event, "Bash");
}

export function terminalCwdLabel(event: NexusOperationEvent): string | null {
  const cwd = readInputString(event.input_preview, ["cwd", "working_directory", "workdir"])
    ?? readTerminalResultString(event.result_preview, ["cwd", "working_directory", "workdir"]);
  return cwd ? compactTerminalPath(cwd) : null;
}

function buildCommandEntry(event: NexusOperationEvent): TerminalEntry {
  const result = parseTerminalResult(event.result_preview);
  const shellId = readTerminalCommandSessionId(event);
  const backgroundProcess = isBackgroundProcess(event, shellId, result.exitCode);
  const phase = backgroundProcess
    ? "running"
    : event.phase;
  const measuredDuration = formatTerminalDuration(readTerminalDurationMs(event));
  const durationLabel = measuredDuration
    ? backgroundProcess ? `启动 ${measuredDuration}` : measuredDuration
    : isFinalPhase(phase) ? "耗时未知" : null;
  return {
    command: readTerminalCommand(event),
    controls: [],
    cwdLabel: terminalCwdLabel(event),
    durationLabel,
    id: event.id,
    phase,
    result,
    shellId,
    startedLabel: formatOperationTime(event.started_at ?? event.updated_at),
    statusLabel: phase === "running" && event.phase === "done" ? "后台运行中" : terminalStatusLabel(phase, result.exitCode),
    statusTone: terminalStatusTone(phase, result.exitCode),
    toolUseId: event.tool_use_id ?? null,
  };
}

function buildControlEvent(event: NexusOperationEvent): TerminalControlEvent {
  const result = parseTerminalResult(event.result_preview);
  return {
    durationLabel: formatTerminalDuration(readTerminalDurationMs(event))
      ?? (isFinalPhase(event.phase) ? "耗时未知" : null),
    id: event.id,
    phase: event.phase,
    resultRows: result.rows,
    statusLabel: terminalControlStatusLabel(event.phase),
    targetLabel: readTerminalControlSessionId(event),
  };
}

function buildOrphanControlEntry(
  control: TerminalControlEvent,
  events: NexusOperationEvent[],
): TerminalEntry {
  const source = events.find((event) => event.id === control.id);
  return {
    command: null,
    controls: [control],
    cwdLabel: null,
    durationLabel: source
      ? formatTerminalDuration(readTerminalDurationMs(source)) ?? (isFinalPhase(source.phase) ? "耗时未知" : null)
      : "耗时未知",
    id: control.id,
    phase: control.phase,
    result: parseTerminalResult(null),
    shellId: control.targetLabel,
    startedLabel: source ? formatOperationTime(source.started_at ?? source.updated_at) : "",
    statusLabel: control.statusLabel,
    statusTone: terminalStatusTone(control.phase, null),
    toolUseId: source?.tool_use_id ?? null,
  };
}

function applyControlState(entry: TerminalEntry, control: TerminalControlEvent): void {
  if (control.phase === "running") {
    entry.phase = "running";
    entry.statusLabel = "终止中";
    entry.statusTone = "running";
    return;
  }
  if (control.phase === "done") {
    entry.phase = "cancelled";
    entry.statusLabel = "已终止";
    entry.statusTone = "muted";
    entry.durationLabel ??= "启动耗时未知";
  }
}

function findControlTargetIndex(
  entries: TerminalEntry[],
  control: TerminalControlEvent,
  events: NexusOperationEvent[],
): number {
  const exactShellIndex = control.targetLabel
    ? entries.findIndex((entry) => entry.shellId === control.targetLabel)
    : -1;
  if (exactShellIndex >= 0) {
    return exactShellIndex;
  }

  const controlEvent = events.find((event) => event.id === control.id);
  const controlCommand = controlEvent
    ? readInputString(controlEvent.input_preview, ["command", "cmd"])
    : null;
  if (controlCommand) {
    const commandIndex = entries.findIndex((entry) => entry.command === controlCommand);
    if (commandIndex >= 0) {
      return commandIndex;
    }
  }

  const controlTime = controlEvent?.started_at ?? controlEvent?.updated_at ?? Number.POSITIVE_INFINITY;
  for (let index = entries.length - 1; index >= 0; index -= 1) {
    const entryEvent = events.find((event) => event.id === entries[index].id);
    const entryTime = entryEvent?.started_at ?? entryEvent?.updated_at ?? 0;
    if (entryTime <= controlTime) {
      return index;
    }
  }
  return -1;
}

function readTerminalDurationMs(event: NexusOperationEvent): number | null {
  if (typeof event.duration_ms === "number" && Number.isFinite(event.duration_ms)) {
    return Math.max(0, event.duration_ms);
  }
  const raw = readTerminalResultString(event.result_preview, [
    "duration_ms",
    "durationMs",
    "elapsed_ms",
    "elapsedMs",
  ]);
  if (raw && /^\d+(?:\.\d+)?$/.test(raw)) {
    return Number(raw);
  }
  return null;
}

function terminalStatusLabel(phase: OperationPhase, exitCode: number | null): string {
  if (phase === "waiting") {
    return "等待确认";
  }
  if (phase === "queued") {
    return "排队中";
  }
  if (phase === "running") {
    return "运行中";
  }
  if (phase === "cancelled") {
    return exitCode == null ? "已中断 · 退出码未知" : `已中断 · 退出 ${exitCode}`;
  }
  if (phase === "error") {
    return exitCode == null ? "执行失败 · 退出码未知" : `退出 ${exitCode}`;
  }
  return exitCode == null ? "已完成 · 退出码未知" : `退出 ${exitCode}`;
}

function terminalControlStatusLabel(phase: OperationPhase): string {
  if (phase === "waiting") {
    return "等待确认终止";
  }
  if (phase === "queued") {
    return "终止请求排队中";
  }
  if (phase === "running") {
    return "正在终止";
  }
  if (phase === "error") {
    return "终止失败";
  }
  if (phase === "cancelled") {
    return "终止请求已取消";
  }
  return "终止请求已完成";
}

function terminalStatusTone(
  phase: OperationPhase,
  exitCode: number | null,
): TerminalEntry["statusTone"] {
  if (phase === "running") {
    return "running";
  }
  if (phase === "error" || (exitCode != null && exitCode !== 0)) {
    return "error";
  }
  if (phase === "done") {
    return "success";
  }
  return "muted";
}

function formatTerminalDuration(durationMs: number | null): string | null {
  if (durationMs == null) {
    return null;
  }
  if (durationMs < 1000) {
    return `${Math.round(durationMs)}ms`;
  }
  const seconds = durationMs / 1000;
  return `${seconds.toFixed(seconds < 10 ? 1 : 0)}s`;
}

function isFinalPhase(phase: OperationPhase): boolean {
  return phase === "done" || phase === "error" || phase === "cancelled";
}

function isBackgroundProcess(
  event: NexusOperationEvent,
  shellId: string | null,
  exitCode: number | null,
): boolean {
  if (event.phase !== "done") {
    return false;
  }
  if (
    event.input_preview?.run_in_background === true
    || event.input_preview?.runInBackground === true
  ) {
    return true;
  }
  return Boolean(shellId && exitCode == null);
}

function readInputString(
  input: Record<string, unknown> | null | undefined,
  keys: readonly string[],
): string | null {
  if (!input) {
    return null;
  }
  for (const key of keys) {
    const value = input[key];
    if (typeof value === "string" && value.trim()) {
      return value.trim();
    }
    if (typeof value === "number" && Number.isFinite(value)) {
      return String(value);
    }
  }
  return null;
}

function readEventTarget(event: NexusOperationEvent, ignoredTarget: string): string | null {
  const target = event.target?.trim();
  return target && target !== ignoredTarget ? target : null;
}

function compactTerminalPath(path: string): string {
  const trimmed = path.trim().replace(/\/$/, "");
  const workspaceIndex = trimmed.lastIndexOf("/workspace");
  if (workspaceIndex >= 0) {
    return `~${trimmed.slice(workspaceIndex)}`;
  }
  const parts = trimmed.split("/").filter(Boolean);
  return parts.length > 3 ? `…/${parts.slice(-2).join("/")}` : trimmed;
}

function isTerminalEvent(event: NexusOperationEvent): boolean {
  return isTerminalCommandEvent(event) || isTerminalControlEvent(event);
}

function compareTerminalEvents(left: NexusOperationEvent, right: NexusOperationEvent): number {
  return (left.started_at ?? left.updated_at) - (right.started_at ?? right.updated_at);
}

function dedupeTerminalEvents(events: NexusOperationEvent[]): NexusOperationEvent[] {
  const byToolUse = new Map<string, NexusOperationEvent>();
  for (const event of events) {
    const key = event.tool_use_id
      ? `${event.kind}:${event.tool_use_id}`
      : event.id;
    const current = byToolUse.get(key);
    if (!current) {
      byToolUse.set(key, event);
      continue;
    }
    byToolUse.set(key, {
      ...current,
      ...event,
      duration_ms: event.duration_ms ?? current.duration_ms ?? null,
      input_preview: event.input_preview ?? current.input_preview,
      permission_request_id: event.permission_request_id ?? current.permission_request_id,
      result_preview: event.result_preview ?? current.result_preview,
      started_at: Math.min(
        current.started_at ?? current.updated_at,
        event.started_at ?? event.updated_at,
      ),
      updated_at: Math.max(current.updated_at, event.updated_at),
    });
  }
  return [...byToolUse.values()].sort(compareTerminalEvents);
}

function compareTerminalEntries(
  left: TerminalEntry,
  right: TerminalEntry,
  events: NexusOperationEvent[],
): number {
  const leftEvent = events.find((event) => event.id === left.id);
  const rightEvent = events.find((event) => event.id === right.id);
  return (leftEvent?.started_at ?? leftEvent?.updated_at ?? 0)
    - (rightEvent?.started_at ?? rightEvent?.updated_at ?? 0);
}
