// INPUT: 当前时间、侧栏同源的聊天投影与定时任务目录。
// OUTPUT: 首页的时段、继续区条目、接下来的定时任务、新手示例与是否展示示例。
// POS: Home 启动台纯投影；不访问网络、路由或 DOM。

import type { TranslationKey } from "@/shared/i18n/messages";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import type { SidebarConversationItem } from "../sidebar/sidebar-conversation-model";

export type HomeDayPeriod = "morning" | "afternoon" | "evening" | "night";

export interface HomeLaunchSample {
  id: "room" | "brief" | "repo" | "doc";
  titleKey: TranslationKey;
  descriptionKey: TranslationKey;
  promptKey: TranslationKey;
}

/** 继续区最多展示的会话数：两列两行，超出部分留给侧栏。 */
const CONTINUE_LIMIT = 4;
const UPCOMING_LIMIT = 3;
/** 非主智能体会话少于该数量时视为新用户，示例卡占据首屏。 */
const SAMPLE_VISIBLE_BELOW = 3;

export const HOME_LAUNCH_SAMPLES: readonly HomeLaunchSample[] = [
  {
    id: "room",
    titleKey: "home.launch.sample_room_title",
    descriptionKey: "home.launch.sample_room_desc",
    promptKey: "home.launch.sample_room_prompt",
  },
  {
    id: "brief",
    titleKey: "home.launch.sample_brief_title",
    descriptionKey: "home.launch.sample_brief_desc",
    promptKey: "home.launch.sample_brief_prompt",
  },
  {
    id: "repo",
    titleKey: "home.launch.sample_repo_title",
    descriptionKey: "home.launch.sample_repo_desc",
    promptKey: "home.launch.sample_repo_prompt",
  },
  {
    id: "doc",
    titleKey: "home.launch.sample_doc_title",
    descriptionKey: "home.launch.sample_doc_desc",
    promptKey: "home.launch.sample_doc_prompt",
  },
];

/** 时段与侧栏星空同一划分口径：5–11 早上，11–18 下午，18–23 晚上，其余为深夜。 */
export function resolveHomeDayPeriod(date: Date): HomeDayPeriod {
  const hour = date.getHours();
  if (hour >= 5 && hour < 11) {
    return "morning";
  }
  if (hour >= 11 && hour < 18) {
    return "afternoon";
  }
  if (hour >= 18 && hour < 23) {
    return "evening";
  }
  return "night";
}

/**
 * 继续区优先展示需要用户介入的会话，其次是执行中的，最后按最近活动排序。
 * 主智能体会话不进入继续区：首页输入框本身就是它的入口。
 */
export function selectContinueItems(
  items: readonly SidebarConversationItem[],
): SidebarConversationItem[] {
  return items
    .filter((item) => !item.isPinned)
    .map((item, index) => ({ item, index }))
    .sort((left, right) => {
      const rankDelta = activityRank(left.item) - activityRank(right.item);
      return rankDelta !== 0 ? rankDelta : left.index - right.index;
    })
    .slice(0, CONTINUE_LIMIT)
    .map(({ item }) => item);
}

export function shouldShowSamples(items: readonly SidebarConversationItem[]): boolean {
  return items.filter((item) => !item.isPinned).length < SAMPLE_VISIBLE_BELOW;
}

/** 接下来的定时任务：正在运行的在前，其余按下次运行时间升序；停用、删除中的任务不展示。 */
export function selectUpcomingTasks(
  tasks: readonly ScheduledTaskItem[],
): ScheduledTaskItem[] {
  return tasks
    .filter((task) => task.enabled && !task.deletion_state && (task.running || task.next_run_at !== null))
    .sort((left, right) => {
      if (left.running !== right.running) {
        return left.running ? -1 : 1;
      }
      return (left.next_run_at ?? Infinity) - (right.next_run_at ?? Infinity);
    })
    .slice(0, UPCOMING_LIMIT);
}

function activityRank(item: SidebarConversationItem): number {
  if (item.activityStatus === "waiting") {
    return 0;
  }
  return item.activityStatus === "working" ? 1 : 2;
}
