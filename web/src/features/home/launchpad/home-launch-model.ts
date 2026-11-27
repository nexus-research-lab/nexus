// INPUT: 当前时间、侧栏同源的聊天投影与定时任务目录。
// OUTPUT: 首页的时段、进行中的会话、接下来的定时任务、新手提示词分类与是否展示示例。
// POS: Home 启动台纯投影；不访问网络、路由或 DOM。

import type { TranslationKey } from "@/shared/i18n/messages";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import type { SidebarConversationItem } from "../sidebar/sidebar-conversation-model";

export type HomeDayPeriod = "morning" | "afternoon" | "evening" | "night";

export type HomeLaunchCategoryId = "collab" | "automate" | "code" | "write";
export type HomeLaunchPickerId = HomeLaunchCategoryId | "mix";

export interface HomeLaunchPrompt {
  id: string;
  /** 列表里展示的短标题。 */
  titleKey: TranslationKey;
  /** 悬停预览、点击填入输入框的完整提示词。 */
  promptKey: TranslationKey;
}

export interface HomeLaunchCategory {
  id: HomeLaunchCategoryId;
  labelKey: TranslationKey;
  prompts: readonly HomeLaunchPrompt[];
}

/** 进行中区最多展示的会话数：两列两行，完整列表留给侧栏。 */
const ACTIVE_LIMIT = 4;
const UPCOMING_LIMIT = 3;
/** 非主智能体会话少于该数量时视为新用户，首屏展示提示词分类。 */
const SAMPLE_VISIBLE_BELOW = 3;

/** 分类胶囊的顺序即展示顺序；「随便看看」不是独立分类，而是从各分类各取一条拼成。 */
export const HOME_LAUNCH_CATEGORIES: readonly HomeLaunchCategory[] = [
  {
    id: "collab",
    labelKey: "home.launch.category_collab",
    prompts: [
      prompt("collab", 1),
      prompt("collab", 2),
      prompt("collab", 3),
      prompt("collab", 4),
    ],
  },
  {
    id: "automate",
    labelKey: "home.launch.category_automate",
    prompts: [
      prompt("automate", 1),
      prompt("automate", 2),
      prompt("automate", 3),
      prompt("automate", 4),
    ],
  },
  {
    id: "code",
    labelKey: "home.launch.category_code",
    prompts: [
      prompt("code", 1),
      prompt("code", 2),
      prompt("code", 3),
      prompt("code", 4),
    ],
  },
  {
    id: "write",
    labelKey: "home.launch.category_write",
    prompts: [
      prompt("write", 1),
      prompt("write", 2),
      prompt("write", 3),
      prompt("write", 4),
    ],
  },
];

/** 「随便看看」：每个分类随机取一条，再打乱顺序；random 可注入，便于测试。 */
export function pickMixedPrompts(
  categories: readonly HomeLaunchCategory[] = HOME_LAUNCH_CATEGORIES,
  random: () => number = Math.random,
): HomeLaunchPrompt[] {
  const picked = categories
    .filter((category) => category.prompts.length > 0)
    .map((category) => category.prompts[Math.floor(random() * category.prompts.length)]!);
  // Fisher–Yates 洗牌，避免每次都按分类顺序排列。
  for (let index = picked.length - 1; index > 0; index -= 1) {
    const swap = Math.floor(random() * (index + 1));
    [picked[index], picked[swap]] = [picked[swap]!, picked[index]!];
  }
  return picked;
}

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

type PromptIndex = 1 | 2 | 3 | 4;

/** 文案 key 由分类与序号拼出；模板字面量类型让缺失的 key 在类型检查时报错。 */
function prompt(categoryId: HomeLaunchCategoryId, index: PromptIndex): HomeLaunchPrompt {
  const promptKey: `home.launch.prompt_${HomeLaunchCategoryId}_${PromptIndex}` = `home.launch.prompt_${categoryId}_${index}`;
  const titleKey: `home.launch.prompt_${HomeLaunchCategoryId}_${PromptIndex}_title` = `home.launch.prompt_${categoryId}_${index}_title`;
  return { id: `${categoryId}-${index}`, promptKey, titleKey };
}

function activityRank(item: SidebarConversationItem): number {
  return item.activityStatus === "waiting" ? 0 : 1;
}
