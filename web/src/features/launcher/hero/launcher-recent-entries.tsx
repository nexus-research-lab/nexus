import { ArrowRight } from "lucide-react";

import { LAUNCHER_TOUR_ANCHORS } from "@/features/onboarding/tours/launcher-tour";
import { FadeSlideIn } from "@/shared/ui/feedback/animated-hero-text";

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
  return (
    <FadeSlideIn
      delayMs={presentation.delayMs}
      durationMs={360}
      style={{ display: "inline-flex" }}
      yOffset={6}
    >
      <div className="group relative inline-flex">
        {presentation.tooltipLabel ? (
          <div
            className="pointer-events-none absolute left-1/2 top-full z-20 mt-2 w-max max-w-[220px] -translate-x-1/2 translate-y-1 rounded-2xl px-3 py-2 text-center text-xs font-medium leading-5 opacity-0 shadow-[0_18px_42px_rgba(38,52,76,0.16)] transition duration-200 ease-out group-hover:translate-y-0 group-hover:opacity-100 group-focus-within:translate-y-0 group-focus-within:opacity-100"
            style={{
              background: "rgba(247, 249, 253, 0.96)",
              boxShadow:
                "0 18px 42px rgba(38, 52, 76, 0.16), inset 0 0 0 1px rgba(255, 255, 255, 0.52)",
              color: "rgba(39, 50, 74, 0.88)",
            }}
          >
            {presentation.tooltipLabel}
          </div>
        ) : null}
        <button
          aria-label={presentation.ariaLabel}
          className="inline-flex items-center gap-1 rounded-full px-2 py-1.5 text-xs font-medium transition-colors duration-150 ease-out hover:text-(--launcher-handoff-hover-color) sm:text-sm"
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
  const presentations = recentEntries.map(buildLauncherRecentEntryPresentation);
  return (
    <div
      data-tour-anchor={LAUNCHER_TOUR_ANCHORS.recent}
      className="mx-auto mt-3 flex w-full max-w-[326px] flex-wrap items-center justify-center gap-1 sm:mt-4 sm:max-w-[420px]"
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
          className="px-1 text-xs font-medium transition-colors duration-150 ease-out hover:text-(--launcher-handoff-hover-color) sm:text-sm"
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
