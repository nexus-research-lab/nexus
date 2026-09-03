import { ArrowRight } from "lucide-react";

import { LAUNCHER_TOUR_ANCHORS } from "@/features/onboarding/tours/launcher-tour";
import { useI18n } from "@/shared/i18n/i18n-context";
import { FadeSlideIn } from "@/shared/ui/feedback/animated-hero-text";
import { UiTooltip } from "@/shared/ui/overlay/tooltip";

import type { RecentLauncherEntry } from "../console/launcher-console-types";
import {
  type LauncherRecentEntryPresentation,
  buildLauncherRecentEntryPresentation,
  getLauncherHandoffDelay,
} from "./launcher-recent-entry-model";

interface LauncherRecentEntriesProps {
  handoffLabel: string;
  initialPrompt: string;
  onHandoff: (initialPrompt?: string) => void;
  onOpen: (entry: RecentLauncherEntry) => void;
  recentEntries: RecentLauncherEntry[];
}

function LauncherRecentEntryChip({
  onOpen,
  presentation,
}: {
  onOpen: (entry: RecentLauncherEntry) => void;
  presentation: LauncherRecentEntryPresentation;
}) {
  const entryButton = (
    <button
      aria-label={presentation.ariaLabel}
      className="inline-flex items-center gap-1 rounded-full px-2 py-1.5 text-sm font-medium transition-colors duration-150 ease-out hover:text-(--launcher-handoff-hover-color)"
      onClick={(event) => {
        event.stopPropagation();
        onOpen(presentation.entry);
      }}
      style={presentation.chipStyle}
      type="button"
    >
      {presentation.markerStyle ? (
        <span className="h-4 w-4 rounded-full" style={presentation.markerStyle} />
      ) : null}
      {presentation.chipLabel}
    </button>
  );

  return (
    <FadeSlideIn
      delayMs={presentation.delayMs}
      durationMs={360}
      style={{ display: "inline-flex" }}
      yOffset={6}
    >
      <div className="inline-flex">
        {presentation.tooltipLabel
          ? (
              <UiTooltip label={presentation.tooltipLabel} placement="bottom">
                {entryButton}
              </UiTooltip>
            )
          : entryButton}
      </div>
    </FadeSlideIn>
  );
}

export function LauncherRecentEntries({
  handoffLabel,
  initialPrompt,
  onHandoff,
  onOpen,
  recentEntries,
}: LauncherRecentEntriesProps) {
  const { t } = useI18n();
  const typeLabels = {
    dm: t("launcher.recent_dm"),
    room: t("launcher.recent_room"),
  };
  const presentations = recentEntries.map((entry, index) =>
    buildLauncherRecentEntryPresentation(entry, index, typeLabels));
  return (
    <div
      data-tour-anchor={LAUNCHER_TOUR_ANCHORS.recent}
      className="mx-auto mt-4 flex w-full max-w-[420px] flex-wrap items-center justify-center gap-1"
    >
      {presentations.map((presentation) => (
        <LauncherRecentEntryChip
          key={presentation.entry.key}
          onOpen={onOpen}
          presentation={presentation}
        />
      ))}
      <FadeSlideIn
        delayMs={getLauncherHandoffDelay(recentEntries.length)}
        durationMs={360}
        style={{ display: "inline-flex" }}
        yOffset={6}
      >
        <button
          data-tour-anchor={LAUNCHER_TOUR_ANCHORS.handoff}
          className="px-1 text-sm font-medium transition-colors duration-150 ease-out hover:text-(--launcher-handoff-hover-color)"
          onClick={() => onHandoff(initialPrompt)}
          style={{ color: "var(--launcher-handoff-color)" }}
          type="button"
        >
          <span className="inline-flex items-center gap-1.5">
            {handoffLabel}
            <ArrowRight className="h-3.5 w-3.5" />
          </span>
        </button>
      </FadeSlideIn>
    </div>
  );
}
