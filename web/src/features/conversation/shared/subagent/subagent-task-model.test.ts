// INPUT: Runtime status/capability aliases, observation clocks and readable directory snapshots.
// OUTPUT: Closed state handling, conservative freshness and distinct active/history/unknown groups.
// POS: Pure subagent projection regressions; no requests or UI rendering.

import { describe, expect, it } from "vitest";
import type { SubagentTask, SubagentTaskListResponse } from "@/types/conversation/subagent-task";
import { buildSubagentTaskListModel, filterSubagentTasksByHostAgent } from "./subagent-task-list-model";
import { canSendSubagentTaskMessage, getSubagentTaskStatus, isSubagentTaskActive, normalizeSubagentTaskListResponse, preferFreshSubagentTask, subagentTaskTimestamp } from "./subagent-task-model";

const NOW = 1_788_761_000_000;
const TASK: SubagentTask = {
  task_id: "task", status: "running", runtime_kind: "nxs", updated_at: NOW,
  capabilities: { observe: true, transcript: true, stop: true, send_message: true, resume: true },
};
const DATA: SubagentTaskListResponse = { runtime_kind: "nxs", capabilities: TASK.capabilities, items: [] };
const task = (status: string, updated_at = NOW): SubagentTask => ({ ...TASK, task_id: status, status, updated_at });

describe("Subagent observation projection", () => {
  it("preserves supported status aliases without treating an unknown state as queued or terminal", () => {
    const aliases = {
      pending: ["queued", "created", "pending"], running: ["running", "started", "in_progress", "in progress"],
      completed: ["completed", "complete", "success", "done", "finished"],
      stopped: ["stopped", "deleted", "cancelled", "canceled", "killed", "interrupted"], failed: ["failed", "error"],
    };
    for (const [status, values] of Object.entries(aliases)) for (const value of values) {
      expect(getSubagentTaskStatus(task(` ${value.toUpperCase()} `))).toBe(status);
      expect(isSubagentTaskActive(task(value))).toBe(status === "pending" || status === "running");
    }
    for (const value of ["", "future_state", "constructor", "__proto__", "toString"]) {
      expect(getSubagentTaskStatus(task(value))).toBe("unknown");
      expect(isSubagentTaskActive(task(value))).toBe(false);
      expect(canSendSubagentTaskMessage(task(value))).toBe(true);
    }
    expect(canSendSubagentTaskMessage(task(" DELETED "))).toBe(false);
  });

  it("normalizes documented runtimes and rejects truthy non-boolean capabilities", () => {
    const aliases = { nxs: ["nxs", "go", "go-native", "gonative"], claude: ["claude", "cc", "claude-code", "claudecode"], mixed: ["mixed"], unknown: ["", "future", "constructor", "__proto__"] };
    for (const [expected, values] of Object.entries(aliases)) for (const value of values) {
      // Historical wire snapshots may omit fields that the current type requires.
      const response = { ...DATA, runtime_kind: value, items: [{ ...TASK, runtime_kind: undefined }] } as unknown as SubagentTaskListResponse;
      const normalized = normalizeSubagentTaskListResponse(response);
      expect(normalized.runtime_kind).toBe(expected);
      expect(normalized.items[0].runtime_kind).toBe(expected);
    }
    const response = { ...DATA, items: [{ ...TASK, capabilities: { observe: null, stop: "false", resume: 1, send_message: false } }] } as unknown as SubagentTaskListResponse;
    const normalized = normalizeSubagentTaskListResponse(response).items[0];
    expect(normalized.capabilities).toEqual({ observe: true, transcript: true, stop: false, resume: false, send_message: false });
    expect(canSendSubagentTaskMessage(normalized)).toBe(false);
  });

  it("rejects non-finite and out-of-range clocks, falling back to a valid start time", () => {
    for (const value of [Infinity, NaN, -1, 0, 8_640_000_000_000_001]) {
      expect(subagentTaskTimestamp({ ...TASK, updated_at: value, started_at: NOW / 1000 })).toBe(NOW);
      expect(subagentTaskTimestamp({ ...TASK, updated_at: value })).toBe(0);
    }
    expect(subagentTaskTimestamp(task("running", NOW / 1000))).toBe(NOW);
    expect(subagentTaskTimestamp(TASK)).toBe(NOW);
  });

  it("uses timestamp order first and never interprets equal-clock unknown evidence as terminal", () => {
    const completed = task("completed"), running = task("running"), unknown = task("future");
    expect(preferFreshSubagentTask(completed, running)).toBe(completed);
    expect(preferFreshSubagentTask(running, completed)).toBe(completed);
    expect(preferFreshSubagentTask(running, unknown)).toBe(running);
    expect(preferFreshSubagentTask(unknown, running)).toBe(running);
    expect(preferFreshSubagentTask(completed, unknown)).toBe(completed);
    const newer = task("future", NOW + 1000);
    expect(preferFreshSubagentTask(completed, newer)).toBe(newer);
    expect(preferFreshSubagentTask(newer, completed)).toBe(newer);
  });
});

describe("Subagent task directory", () => {
  it("sorts separate active, historical and unknown groups without mutating input", () => {
    const tasks = [task("completed"), task("future"), task("running"), task("failed", NOW + 1000), task("queued", NOW + 1000), task("constructor", NOW + 1000)];
    const original = [...tasks];
    const result = buildSubagentTaskListModel({ data: DATA, hasError: false, isLoading: false, tasks });
    expect(result.activeTasks.map(t => t.task_id)).toEqual(["queued", "running"]);
    expect(result.historyTasks.map(t => t.task_id)).toEqual(["failed", "completed"]);
    expect(result.unknownTasks.map(t => t.task_id)).toEqual(["constructor", "future"]);
    expect(result.activeEmptyState).toBeNull();
    expect(tasks).toEqual(original);
  });

  it("shows a successful empty state only when absence is known", () => {
    const options = { data: null, hasError: false, isLoading: false, tasks: [] };
    expect(buildSubagentTaskListModel(options).activeEmptyState).toBe("empty");
    expect(buildSubagentTaskListModel({ ...options, isLoading: true }).activeEmptyState).toBe("loading");
    expect(buildSubagentTaskListModel({ ...options, hasError: true }).activeEmptyState).toBeNull();
    const cached = buildSubagentTaskListModel({ ...options, data: DATA, hasError: true, tasks: [TASK] });
    expect(cached.activeTasks).toEqual([TASK]);
    expect(cached.activeEmptyState).toBeNull();
  });

  it("retains explicit unsupported-runtime behavior instead of exposing stale tasks", () => {
    const result = buildSubagentTaskListModel({ data: { ...DATA, runtime_kind: "claude", capabilities: { ...DATA.capabilities, observe: false } }, hasError: false, isLoading: false, tasks: [TASK] });
    expect(result.supportNotice).toBe("claude");
    expect(result.activeTasks).toEqual([]);
    expect(result.activeEmptyState).toBeNull();
  });
});


it("keeps omitted caller filters separate from an explicitly unavailable caller", () => {
  const tasks = [{ ...TASK, host_agent_id: "host" }];
  expect(filterSubagentTasksByHostAgent(tasks)).toBe(tasks);
  expect(filterSubagentTasksByHostAgent(tasks, null)).toBe(tasks);
  expect(filterSubagentTasksByHostAgent(tasks, "")).toEqual([]);
  expect(filterSubagentTasksByHostAgent(tasks, " ")).toEqual([]);
  expect(filterSubagentTasksByHostAgent(tasks, " host ")).toEqual(tasks);
});
