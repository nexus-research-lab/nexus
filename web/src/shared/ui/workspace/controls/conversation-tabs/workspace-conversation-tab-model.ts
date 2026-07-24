import { cn } from "@/shared/ui/class-name";

import {
  ACTIVE_TAB_MIN_WIDTH,
  INACTIVE_TAB_MIN_WIDTH,
} from "./conversation-tabs-model";

const TAB_BASE_CLASS_NAME =
  "workspace-surface-header-conversation-tab group relative inline-flex h-9 flex-none snap-start items-center rounded-[var(--workspace-session-tab-radius)] border border-transparent text-[12px] font-normal transition-[width,background-color,border-color,color] duration-[145ms] ease-[cubic-bezier(0.25,0.1,0.25,1)]";
const TAB_CLOSE_BASE_CLASS_NAME =
  "absolute right-1 top-1/2 flex h-6 w-6 shrink-0 -translate-y-1/2 items-center justify-center rounded-[7px] text-(--icon-muted) transition-[background-color,color,opacity] duration-(--motion-duration-fast) hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] hover:text-(--destructive) focus-visible:opacity-100";

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
    rootClassName: "workspace-surface-header-inactive-tab bg-transparent text-(--text-default) shadow-none hover:border-[color:color-mix(in_srgb,var(--divider-subtle-color)_72%,transparent)] hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
  },
} as const satisfies Record<
  "active" | "inactive",
  WorkspaceConversationTabStatePresentation
>;

interface WorkspaceConversationTabPresentation {
  ariaCurrent: "page" | undefined;
  closeClassName: string;
  indicatorClassName: string;
  rootClassName: string;
  showClose: boolean;
  showExternalSessionLabel: boolean;
  style: {
    minWidth: number;
    width: number;
  };
  title: string;
}

export function resolveWorkspaceConversationTabPresentation({
  canClose,
  externalSessionLabel,
  isActive,
  tabWidth,
  title,
}: {
  canClose: boolean;
  externalSessionLabel: string | null;
  isActive: boolean;
  tabWidth?: number;
  title: string;
}): WorkspaceConversationTabPresentation {
  const state = TAB_STATE_PRESENTATIONS[isActive ? "active" : "inactive"];
  return {
    ariaCurrent: isActive ? "page" : undefined,
    closeClassName: cn(TAB_CLOSE_BASE_CLASS_NAME, state.closeClassName),
    indicatorClassName: cn(
      "absolute left-2.5 top-1/2 h-1.5 w-1.5 -translate-y-1/2 rounded-full transition-[background-color,border-color] duration-(--motion-duration-fast)",
      state.indicatorClassName,
    ),
    rootClassName: cn(
      TAB_BASE_CLASS_NAME,
      state.rootClassName,
    ),
    showClose: canClose,
    showExternalSessionLabel: Boolean(externalSessionLabel),
    style: {
      minWidth: state.minWidth,
      width: tabWidth ?? state.minWidth,
    },
    title: externalSessionLabel
      ? `${title} · IM ${externalSessionLabel}`
      : title,
  };
}
