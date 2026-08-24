/**
 * INPUT: 单标签活动、固定、关闭和外部 Session 状态。
 * OUTPUT: 内容留白、动作可见性与稳定宽度/状态样式投影。
 * POS: Workspace 单会话标签的唯一样式模型。
 */
import { cn } from "@/shared/ui/class-name";
import { getUiTabDismissClassName } from "@/shared/ui/navigation/tabs-styles";

import {
  ACTIVE_TAB_MIN_WIDTH,
  INACTIVE_TAB_MIN_WIDTH,
} from "./conversation-tabs-model";

const TAB_BASE_CLASS_NAME =
  "workspace-surface-header-conversation-tab group relative inline-flex h-8 flex-none snap-start items-center rounded-[var(--workspace-session-tab-radius)] border border-transparent text-compact font-normal transition-[width,background-color,border-color,color] duration-[145ms] ease-[cubic-bezier(0.25,0.1,0.25,1)]";
interface WorkspaceConversationTabStatePresentation {
  closeClassName: string;
  indicatorClassName: string;
  minWidth: number;
  rootClassName: string;
}

const TAB_STATE_PRESENTATIONS = {
  active: {
    closeClassName: "opacity-80 hover:opacity-100",
    indicatorClassName: "bg-(--primary)",
    minWidth: ACTIVE_TAB_MIN_WIDTH,
    rootClassName: "workspace-surface-header-active-tab z-10 font-medium text-(--text-strong)",
  },
  inactive: {
    closeClassName: "opacity-0 group-hover:opacity-100",
    indicatorClassName: "border border-[color:color-mix(in_srgb,var(--icon-muted)_72%,transparent)] bg-transparent group-hover:border-(--icon-default) group-hover:bg-[color:color-mix(in_srgb,var(--icon-default)_28%,transparent)]",
    minWidth: INACTIVE_TAB_MIN_WIDTH,
    rootClassName: "workspace-surface-header-inactive-tab bg-transparent text-(--text-default) shadow-none hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
  },
} as const satisfies Record<
  "active" | "inactive",
  WorkspaceConversationTabStatePresentation
>;

interface WorkspaceConversationTabPresentation {
  actionsClassName: string;
  ariaCurrent: "page" | undefined;
  closeClassName: string;
  contentClassName: string;
  indicatorClassName: string;
  pinClassName: string;
  rootClassName: string;
  showClose: boolean;
  showExternalSessionLabel: boolean;
  showPin: boolean;
  style: {
    minWidth: number;
    width: number;
  };
  title: string;
}

export function resolveWorkspaceConversationTabPresentation({
  canClose,
  canPin,
  externalSessionLabel,
  isActive,
  isPinned,
  tabWidth,
  title,
}: {
  canClose: boolean;
  canPin: boolean;
  externalSessionLabel: string | null;
  isActive: boolean;
  isPinned: boolean;
  tabWidth?: number;
  title: string;
}): WorkspaceConversationTabPresentation {
  const state = TAB_STATE_PRESENTATIONS[isActive ? "active" : "inactive"];
  return {
    actionsClassName: cn(
      "absolute right-1 top-1/2 flex -translate-y-1/2 items-center gap-px transition-opacity duration-(--motion-duration-fast)",
      isActive || isPinned
        ? "opacity-90"
        : "opacity-0 group-hover:opacity-100 group-focus-within:opacity-100",
    ),
    ariaCurrent: isActive ? "page" : undefined,
    closeClassName: getUiTabDismissClassName(cn(
      state.closeClassName,
    )),
    contentClassName: cn(
      "flex h-full w-full min-w-0 items-center justify-start pl-[22px] text-left",
      canClose && canPin ? "pr-[52px]" : canClose || canPin ? "pr-7" : "pr-2.5",
    ),
    indicatorClassName: cn(
      "absolute left-2.5 top-1/2 h-1.5 w-1.5 -translate-y-1/2 rounded-full transition-[background-color,border-color] duration-(--motion-duration-fast)",
      state.indicatorClassName,
    ),
    pinClassName: cn(
      "flex h-6 w-6 shrink-0 items-center justify-center radius-control-xs transition-[background-color,color,opacity] duration-(--motion-duration-fast) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_32%,transparent)]",
      isPinned
        ? "bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-(--primary) hover:bg-[color:color-mix(in_srgb,var(--primary)_16%,transparent)]"
        : "text-(--icon-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
    ),
    rootClassName: cn(
      TAB_BASE_CLASS_NAME,
      state.rootClassName,
    ),
    showClose: canClose,
    showExternalSessionLabel: Boolean(externalSessionLabel),
    showPin: canPin,
    style: {
      minWidth: state.minWidth,
      width: tabWidth ?? state.minWidth,
    },
    title: externalSessionLabel
      ? `${title} · IM ${externalSessionLabel}`
      : title,
  };
}
