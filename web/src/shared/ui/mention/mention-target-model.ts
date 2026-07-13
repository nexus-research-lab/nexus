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
export type MentionPlacement = "above" | "below" | "auto";

interface MentionPopoverAnchor {
  bottom: number;
  left: number;
  top: number;
  width: number;
}

interface MentionPopoverLayout {
  left: number;
  minWidth: number;
  top: number;
}

const KEYBOARD_ACTION_BY_KEY: Readonly<Record<string, MentionKeyboardAction>> = {
  ArrowDown: "next",
  ArrowUp: "previous",
  Enter: "select",
  Tab: "select",
  Escape: "close",
};

const MENTION_MATCH_PATTERN = /(?:^|\s)([@#])([^\s@#]*)$/;
const POPOVER_GAP = 6;
const POPOVER_MAX_HEIGHT = 192;

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
  const normalizedFilter = filter.trim().toLowerCase();
  if (!normalizedFilter) {
    return [...items];
  }
  return items.filter((item) =>
    item.label.toLowerCase().includes(normalizedFilter)
    || item.subtitle?.toLowerCase().includes(normalizedFilter));
}

export function getMentionKeyboardAction(key: string): MentionKeyboardAction | null {
  return KEYBOARD_ACTION_BY_KEY[key] ?? null;
}

export function isMentionNavigationKey(key: string): boolean {
  return key in KEYBOARD_ACTION_BY_KEY;
}

export function getMentionPopoverLayout(
  anchor: MentionPopoverAnchor,
  itemCount: number,
  placement: MentionPlacement,
): MentionPopoverLayout {
  const estimatedHeight = Math.min(itemCount * 52 + 8, POPOVER_MAX_HEIGHT);
  const canPlaceAbove = anchor.top - POPOVER_GAP - estimatedHeight >= 12;
  const placeBelow = placement === "below" || (placement === "auto" && !canPlaceAbove);
  return {
    left: anchor.left,
    minWidth: Math.max(anchor.width, 200),
    top: placeBelow
      ? anchor.bottom + POPOVER_GAP
      : anchor.top - POPOVER_GAP - estimatedHeight,
  };
}
