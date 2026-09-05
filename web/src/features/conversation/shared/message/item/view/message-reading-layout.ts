// INPUT: Compact or expanded message presentation, independent of content and runtime state.
// OUTPUT: One reading scale and outer rhythm, with role-specific body/header geometry and collapse threshold.
// POS: Conversation reading layout owner; User/Assistant data models never return CSS.

import { CONVERSATION_ASSISTANT_FRAME_WIDTH_CLASS_NAME } from "../../../conversation-panel-styles";

const READING_LAYOUTS = {
  compact: { body: "text-base leading-6", section: "px-0" },
  expanded: { body: "text-md leading-7", section: "px-2 sm:px-3" },
} as const;

export function resolveUserMessageLayout(compact: boolean) {
  const reading = READING_LAYOUTS[compact ? "compact" : "expanded"];
  return {
    content: `${reading.body} ${compact ? "[&_.katex-display]:my-2" : "[&_.katex-display]:my-3"}`,
    header: compact ? "h-6" : "h-7",
    row: compact ? "" : "gap-3",
    section: reading.section,
  };
}

export function resolveAssistantMessageLayout(compact: boolean) {
  const reading = READING_LAYOUTS[compact ? "compact" : "expanded"];
  return {
    content: compact ? `pt-1 ${reading.body}` : `w-full max-w-full pt-3 ${reading.body}`,
    inner: compact ? "max-w-full" : CONVERSATION_ASSISTANT_FRAME_WIDTH_CLASS_NAME,
    section: reading.section,
    showMetadata: compact,
  };
}

export const USER_MESSAGE_COLLAPSED_HEIGHT = 220;

export function isUserMessageContentCollapsible(contentHeight: number): boolean {
  return contentHeight > USER_MESSAGE_COLLAPSED_HEIGHT;
}
