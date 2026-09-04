// INPUT: 最近 DM/Room 数据、主 Agent 交接文案与导航动作。
// OUTPUT: 使用共享 Button、Tooltip 与动效原语的 Launcher 最近入口。
// POS: Launcher Hero 最近入口编排层；不定义控件圆角、字号、颜色或阴影。

import { ArrowRight, Bot } from "lucide-react";

import { LAUNCHER_TOUR_ANCHORS } from "@/features/onboarding/tours/launcher-tour";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { FadeSlideIn } from "@/shared/ui/feedback/animated-hero-text";
import { UiTooltip } from "@/shared/ui/overlay/tooltip";

import type { RecentLauncherEntry } from "../console/launcher-console-types";
import {
  type LauncherRecentEntryModel,
  buildLauncherRecentEntryModel,
} from "./launcher-recent-entry-model";
import {
  getLauncherHandoffDelayMs,
  getLauncherRecentEntryDelayMs,
  LauncherRecentEntryLayout,
} from "./launcher-recent-entry-layout";

interface LauncherRecentEntriesProps {
  handoffLabel: string;
  initialPrompt: string;
  onHandoff: (initialPrompt?: string) => void;
  onOpen: (entry: RecentLauncherEntry) => void;
  recentEntries: RecentLauncherEntry[];
}

function LauncherRecentEntryChip({
  index,
  model,
  onOpen,
}: {
  index: number;
  model: LauncherRecentEntryModel;
  onOpen: (entry: RecentLauncherEntry) => void;
}) {
  const entryButton = (
    <UiButton
      aria-label={model.ariaLabel}
      data-launcher-recent-entry={model.entry.type}
      onClick={(event) => {
        event.stopPropagation();
        onOpen(model.entry);
      }}
      shape="pill"
      size="sm"
      variant="surface"
    >
      {model.entry.type === "dm" ? <Bot aria-hidden="true" className="h-3.5 w-3.5" /> : null}
      {model.chipLabel}
    </UiButton>
  );

  return (
    <FadeSlideIn
      delayMs={getLauncherRecentEntryDelayMs(index)}
      durationMs={360}
      style={{ display: "inline-flex" }}
      yOffset={6}
    >
      <div className="inline-flex">
        {model.tooltipLabel
          ? (
              <UiTooltip label={model.tooltipLabel} placement="bottom">
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
  const models = recentEntries.map((entry) =>
    buildLauncherRecentEntryModel(entry, typeLabels));
  return (
    <div
      data-tour-anchor={LAUNCHER_TOUR_ANCHORS.recent}
      className={LauncherRecentEntryLayout.listClassName}
    >
      {models.map((model, index) => (
        <LauncherRecentEntryChip
          index={index}
          key={model.entry.key}
          model={model}
          onOpen={onOpen}
        />
      ))}
      <FadeSlideIn
        delayMs={getLauncherHandoffDelayMs(recentEntries.length)}
        durationMs={360}
        style={{ display: "inline-flex" }}
        yOffset={6}
      >
        <UiButton
          data-tour-anchor={LAUNCHER_TOUR_ANCHORS.handoff}
          onClick={() => onHandoff(initialPrompt)}
          shape="pill"
          size="sm"
          tone="primary"
          variant="text"
        >
          <span className="inline-flex items-center gap-1.5">
            {handoffLabel}
            <ArrowRight className="h-3.5 w-3.5" />
          </span>
        </UiButton>
      </FadeSlideIn>
    </div>
  );
}
