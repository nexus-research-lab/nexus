// INPUT: 当前时间、侧栏同源的聊天投影与定时任务目录。
// OUTPUT: 首页的时段、进行中的会话、接下来的定时任务、新手示例与是否展示示例。
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

/** 进行中区最多展示的会话数；完整列表留给侧栏。 */
const ACTIVE_LIMIT = 3;
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
 * 首页只展示正在发生的会话：等你回复的在前，执行中的其次，同级保持侧栏的最近活动顺序。
 * 空闲会话已在侧栏按时间排好，首页不再重复；主智能体会话的入口就是首页输入框，也不进入。
 */
export function selectActiveItems(
  items: readonly SidebarConversationItem[],
): SidebarConversationItem[] {
  return items
    .filter((item) => !item.isPinned && item.activityStatus !== null)
    .map((item, index) => ({ item, index }))
    .sort((left, right) => {
      const rankDelta = activityRank(left.item) - activityRank(right.item);
      return rankDelta !== 0 ? rankDelta : left.index - right.index;
    })
    .slice(0, ACTIVE_LIMIT)
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
  return item.activityStatus === "waiting" ? 0 : 1;
}
