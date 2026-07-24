import { cn } from "@/shared/ui/class-name";

import {
  ACTIVE_TAB_MIN_WIDTH,
  INACTIVE_TAB_MIN_WIDTH,
} from "./conversation-tabs-model";

const TAB_BASE_CLASS_NAME =
  "group relative inline-flex h-9 flex-none snap-start items-center overflow-hidden rounded-[10px] border border-transparent text-[11px] font-medium transition-[width,background-color,border-color,color,box-shadow] duration-[145ms] ease-[cubic-bezier(0.25,0.1,0.25,1)]";
const TAB_SEPARATOR_CLASS_NAME =
  "before:pointer-events-none before:absolute before:bottom-1.5 before:left-0 before:top-1.5 before:w-px before:bg-[color:color-mix(in_srgb,var(--divider-subtle-color)_78%,transparent)] before:transition-opacity before:content-[''] hover:before:opacity-0";
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
    indicatorClassName: "bg-(--primary) shadow-[0_0_0_2px_color-mix(in_srgb,var(--primary)_10%,transparent)]",
    minWidth: ACTIVE_TAB_MIN_WIDTH,
    rootClassName: "z-10 border-(--surface-interactive-active-border) bg-(--surface-interactive-active-background) font-semibold text-(--text-strong) shadow-(--surface-interactive-active-shadow)",
  },
  inactive: {
    closeClassName: "opacity-0 group-hover:opacity-100",
    indicatorClassName: "border border-[color:color-mix(in_srgb,var(--icon-muted)_72%,transparent)] bg-transparent group-hover:border-(--icon-default) group-hover:bg-[color:color-mix(in_srgb,var(--icon-default)_28%,transparent)]",
    minWidth: INACTIVE_TAB_MIN_WIDTH,
    rootClassName: "bg-transparent text-(--text-soft) shadow-none hover:border-[color:color-mix(in_srgb,var(--divider-subtle-color)_58%,transparent)] hover:bg-(--surface-interactive-hover-background) hover:text-(--text-default)",
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
  showSeparator,
  tabWidth,
  title,
}: {
  canClose: boolean;
  externalSessionLabel: string | null;
  isActive: boolean;
  showSeparator: boolean;
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
      showSeparator && TAB_SEPARATOR_CLASS_NAME,
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
