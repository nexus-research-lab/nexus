// INPUT: 不同调度状态的任务和双语翻译函数。
// OUTPUT: 列名本地化不改变分列、排序或输入事实。
// POS: 看板纯投影回归；不执行调度或命令。
import { expect, it } from "vitest";
import { MESSAGES } from "@/shared/i18n/messages";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import { getScheduledTaskCardPresentation, buildScheduledTaskBoard } from "./scheduled-task-board-model";

function task(job_id: string, overrides: Partial<ScheduledTaskItem> = {}): ScheduledTaskItem {
  return {
    job_id, agent_id: "agent", configuration_version: 1, name: job_id, instruction: "Work",
    delivery: { mode: "none" }, enabled: true, expires_at: null, failure_streak: 0,
    last_error: null, last_run_at: null, next_run_at: null, running: false, running_started_at: null,
    schedule: { kind: "every", interval_seconds: 60 }, session_target: { kind: "main" },
    source: { kind: "user_page" }, ...overrides,
  };
}

it("translates titles while preserving column priority, schedule order and source data", () => {
  const items = [
    task("later", { next_run_at: 200 }), task("running", { running: true }),
    task("earlier", { next_run_at: 100 }), task("stopped", { enabled: false }),
    task("deleting", { running: true, deletion_state: "deleting" }),
  ];
  const before = structuredClone(items);
  const en = buildScheduledTaskBoard(items, (key) => MESSAGES.en[key]);
  const zh = buildScheduledTaskBoard(items, (key) => MESSAGES.zh[key]);
  expect(en.map((column) => column.title)).toEqual(["Running", "Scheduled", "Needs attention", "Stopped"]);
  expect(zh.map((column) => column.title)).toEqual(["执行中", "已计划", "需处理", "已停止"]);
  const identities = (columns: typeof en) => columns.map((column) => [column.id, column.items.map((item) => item.job_id)]);
  expect(identities(en)).toEqual([
    ["running", ["running"]], ["scheduled", ["earlier", "later"]],
    ["attention", ["deleting"]], ["stopped", ["stopped"]],
  ]);
  expect(identities(zh)).toEqual(identities(en));
  expect(items).toEqual(before);
});


it("localizes every card state without changing command eligibility or task data", () => {
  const overrides: Partial<ScheduledTaskItem>[] = [
    {}, { running: true }, { enabled: false },
    { enabled: false, last_run_at: 100, last_run_status: "succeeded" },
    { failure_streak: 2, last_error: "Provider failed" },
    { deletion_state: "deleting" }, { deletion_state: "review_required" },
    { session_binding_state: "rebind_required", session_binding_issues: ["execution", "delivery"] },
    ...["awaiting_approval", "awaiting_input", "awaiting_reauth", "denied", "ready_to_retry"].map((permission_state) => ({ permission_state })),
  ];
  const translate = (locale: "zh" | "en"): I18nContextValue["t"] => (key, params) =>
    Object.entries(params ?? {}).reduce((value, [name, param]) => value.replaceAll(`{${name}}`, String(param)), MESSAGES[locale][key]);
  const pending = { isDeleting: false, isPermissionPending: false, isRunning: false, isToggling: false };
  for (const override of overrides) {
    const input = task("task", override);
    const before = structuredClone(input);
    const en = getScheduledTaskCardPresentation(input, pending, translate("en"), "en");
    const zh = getScheduledTaskCardPresentation(input, pending, translate("zh"), "zh");
    expect(JSON.stringify(en)).not.toMatch(/[\u4e00-\u9fff]/);
    expect(en.columnId).toBe(zh.columnId);
    expect(en.runAction.disabled).toBe(zh.runAction.disabled);
    expect(en.toggleAction.disabled).toBe(zh.toggleAction.disabled);
    expect(input).toEqual(before);
  }
});
