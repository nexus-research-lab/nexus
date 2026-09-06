// INPUT: 编辑文本、光标、候选数据、搜索词与导航键。
// OUTPUT: Mention 匹配、完整插入、候选筛选与语义键盘动作。
// POS: Mention 纯模型；视口定位归公共 anchored overlay，不拥有浮层尺寸。
import { createUiSearchMatcher } from "@/shared/ui/form/search-query";

export type MentionTrigger = "@" | "#";

export interface MentionTargetItem {
  id: string;
  label: string;
  marker: string;
  subtitle?: string | null;
}

export interface MentionTextMatch {
  filter: string;
  startPosition: number;
  trigger: MentionTrigger;
}

export interface MentionTextInsertion {
  cursorPosition: number;
  value: string;
}

export type MentionKeyboardAction = "next" | "previous" | "select" | "close";
const KEYBOARD_ACTION_BY_KEY: Readonly<Record<string, MentionKeyboardAction>> = {
  ArrowDown: "next",
  ArrowUp: "previous",
  Enter: "select",
  Tab: "select",
  Escape: "close",
};

const MENTION_MATCH_PATTERN = /(?:^|\s)([@#])([^\s@#]*)$/;

export function findMentionTextMatch(
  value: string,
  cursorPosition: number,
  allowedTriggers: readonly MentionTrigger[],
): MentionTextMatch | null {
  const beforeCursor = value.slice(0, cursorPosition);
  const match = MENTION_MATCH_PATTERN.exec(beforeCursor);
  const trigger = match?.[1] as MentionTrigger | undefined;
  if (!match || !trigger || !allowedTriggers.includes(trigger)) {
    return null;
  }
  const filter = match[2] ?? "";
  return {
    filter,
    startPosition: beforeCursor.length - filter.length - 1,
    trigger,
  };
}

export function insertMentionTarget(
  value: string,
  cursorPosition: number,
  match: MentionTextMatch,
  label: string,
): MentionTextInsertion {
  const insertedValue = `${match.trigger}${label} `;
  return {
    cursorPosition: match.startPosition + insertedValue.length,
    value: `${value.slice(0, match.startPosition)}${insertedValue}${value.slice(cursorPosition)}`,
  };
}

export function filterMentionTargets(
  items: readonly MentionTargetItem[],
  filter: string,
): MentionTargetItem[] {
  const search = createUiSearchMatcher(filter);
  return items.filter((item) => search.matches([item.label, item.subtitle]));
}

export function getMentionKeyboardAction(key: string): MentionKeyboardAction | null {
  return KEYBOARD_ACTION_BY_KEY[key] ?? null;
}

export function isMentionNavigationKey(key: string): boolean {
  return key in KEYBOARD_ACTION_BY_KEY;
}
