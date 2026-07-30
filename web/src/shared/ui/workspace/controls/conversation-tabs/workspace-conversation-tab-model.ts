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
    closeClassName: getUiTabDismissClassName(cn(
      "absolute right-1 top-1/2 -translate-y-1/2",
      state.closeClassName,
    )),
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
