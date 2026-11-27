import { describe, expect, it } from "vitest";

import type { SidebarConversationItem } from "../sidebar/sidebar-conversation-model";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import {
  HOME_LAUNCH_CATEGORIES,
  pickMixedPrompts,
  resolveHomeDayPeriod,
  selectActiveItems,
  selectUpcomingTasks,
  shouldShowSamples,
} from "./home-launch-model";

function item(id: string, patch: Partial<SidebarConversationItem> = {}): SidebarConversationItem {
  return {
    activityStatus: null, canDelete: true, id, isPinned: false, kind: "dm", lastActivityAt: 0,
    members: [], messageCount: 1, summary: "", timeLabel: "", title: id, ...patch,
  };
}

function task(id: string, patch: Partial<ScheduledTaskItem>): ScheduledTaskItem {
  return { job_id: id, enabled: true, running: false, next_run_at: null, ...patch } as ScheduledTaskItem;
}

describe("home launch model", () => {
  it("按小时划分时段", () => {
    expect(resolveHomeDayPeriod(new Date(2026, 0, 1, 8))).toBe("morning");
    expect(resolveHomeDayPeriod(new Date(2026, 0, 1, 15))).toBe("afternoon");
    expect(resolveHomeDayPeriod(new Date(2026, 0, 1, 20))).toBe("evening");
    expect(resolveHomeDayPeriod(new Date(2026, 0, 1, 2))).toBe("night");
  });

  it("进行中只取等你回复和执行中的会话，排除主智能体与空闲会话，最多四条", () => {
    const items = [
      item("main", { isPinned: true, activityStatus: "working" }),
      item("a"), item("b", { activityStatus: "working" }), item("c", { activityStatus: "working" }),
      item("d", { activityStatus: "working" }), item("e", { activityStatus: "waiting" }),
      item("f", { activityStatus: "working" }),
    ];
    expect(selectActiveItems(items).map((entry) => entry.id)).toEqual(["e", "b", "c", "d"]);
    expect(selectActiveItems([item("a"), item("b")])).toEqual([]);
  });

  it("非主智能体会话少于三个时展示示例", () => {
    expect(shouldShowSamples([item("main", { isPinned: true }), item("a"), item("b")])).toBe(true);
    expect(shouldShowSamples([item("a"), item("b"), item("c")])).toBe(false);
  });

  it("接下来的任务：运行中在前，其余按下次运行时间，跳过停用和删除中", () => {
    const tasks = [
      task("late", { next_run_at: 300 }),
      task("off", { enabled: false, next_run_at: 1 }),
      task("soon", { next_run_at: 100 }),
      task("run", { running: true }),
      task("gone", { deletion_state: "deleting", next_run_at: 2 }),
    ];
    expect(selectUpcomingTasks(tasks).map((entry) => entry.job_id)).toEqual(["run", "soon", "late"]);
  });
});

describe("pickMixedPrompts", () => {
  it("每个分类各取一条并打乱顺序", () => {
    const picked = pickMixedPrompts(HOME_LAUNCH_CATEGORIES, () => 0);
    expect(picked).toHaveLength(HOME_LAUNCH_CATEGORIES.length);
    const categories = picked.map((item) => item.id.split("-")[0]);
    expect(new Set(categories)).toEqual(new Set(HOME_LAUNCH_CATEGORIES.map((category) => category.id)));
  });
});
