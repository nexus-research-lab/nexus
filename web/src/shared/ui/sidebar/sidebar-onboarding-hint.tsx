"use client";

import type { LucideIcon } from "lucide-react";
import { Compass, Hash, Puzzle, Users2, X } from "lucide-react";
import { useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { useOnboardingTour } from "@/shared/ui/onboarding/tour-provider";
import { SIDEBAR_NAVIGATION_TOUR_ID } from "@/shared/ui/sidebar/sidebar-navigation-tour";

export const SIDEBAR_ONBOARDING_HINT_DISMISSED_KEY =
  "nexus:sidebar-onboarding-dismissed";

function read_sidebar_hint_dismissed(): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  return window.localStorage.getItem(SIDEBAR_ONBOARDING_HINT_DISMISSED_KEY) === "true";
}

export function SidebarOnboardingHint() {
  const { t } = useI18n();
  const { active_tour_id, has_completed_tour, start_tour } = useOnboardingTour();
  const [dismissed, set_dismissed] = useState(() => {
    if (typeof window === "undefined") {
      return true;
    }
    return read_sidebar_hint_dismissed();
  });
  const is_completed = has_completed_tour(SIDEBAR_NAVIGATION_TOUR_ID);
  const is_tour_running = active_tour_id === SIDEBAR_NAVIGATION_TOUR_ID;

  const dismiss = () => {
    set_dismissed(true);
    if (typeof window !== "undefined") {
      window.localStorage.setItem(SIDEBAR_ONBOARDING_HINT_DISMISSED_KEY, "true");
    }
  };

  if (dismissed) {
    return null;
  }

  return (
    <div className="mb-2 rounded-[16px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_88%,white)] px-3 py-3 shadow-[0_10px_24px_color-mix(in_srgb,var(--shadow-color)_8%,transparent)]">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2 text-(--text-strong)">
            <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-[color:color-mix(in_srgb,var(--primary)_12%,transparent)] text-(--primary)">
              <Compass className="h-3.5 w-3.5" />
            </span>
            <p className="text-[12px] font-semibold">
              {is_completed
                ? t("sidebar.guide_completed_title")
                : t("sidebar.guide_title")}
            </p>
          </div>
        </div>

        <button
          aria-label={t("common.close")}
          className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-(--icon-muted) transition-[background,color] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
          onClick={dismiss}
          type="button"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      </div>

      <div className="mt-2.5 flex flex-col gap-1.5">
        <HintRow icon={Users2} text={t("sidebar.guide_agents")} />
        <HintRow icon={Hash} text={t("sidebar.guide_rooms")} />
        <HintRow icon={Puzzle} text={t("sidebar.guide_capabilities")} />
      </div>

      <div className="mt-3 flex justify-end">
        <button
          className="inline-flex shrink-0 items-center gap-1 rounded-full bg-(--primary) px-3 py-1.5 text-[12px] font-medium text-white transition-[transform,opacity] duration-(--motion-duration-fast) hover:-translate-y-[1px] hover:opacity-92 disabled:pointer-events-none disabled:opacity-(--disabled-opacity)"
          disabled={is_tour_running}
          onClick={() => start_tour(SIDEBAR_NAVIGATION_TOUR_ID)}
          type="button"
        >
          {is_completed
            ? t("sidebar.guide_action_restart")
            : t("sidebar.guide_action_start")}
        </button>
      </div>
    </div>
  );
}

function HintRow({ icon: Icon, text }: { icon: LucideIcon; text: string }) {
  return (
    <div className="flex items-start gap-2 rounded-[10px] bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_72%,transparent)] px-2 py-1.5">
      <Icon className="mt-0.5 h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
      <span className="text-[11px] leading-5 text-(--text-muted)">
        {text}
      </span>
    </div>
  );
}
