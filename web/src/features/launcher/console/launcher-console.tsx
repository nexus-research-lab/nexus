"use client";

import { useMemo } from "react";

import { ANIMATIONS } from "@/config/animation-assets";
import { buildLauncherTour } from "@/features/onboarding/tours/launcher-tour";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { LottiePlayer } from "@/shared/ui/feedback/lottie-player";
import { usePageOnboardingTour } from "@/shared/ui/onboarding/use-page-onboarding-tour";

import { LauncherHeroStage } from "../hero/launcher-hero-stage";
import {
  buildDecorativeTokens,
  buildLauncherMentionTargets,
  buildRecentLauncherEntries,
} from "./launcher-console-helpers";
import type { LauncherConsoleProps } from "./launcher-console-types";
import { useLauncherConsoleController } from "./use-launcher-console-controller";

export function LauncherConsole({
  agents,
  conversations,
  currentAgentId,
  onOpenMainAgentDm,
  onOpenRoute,
  onSelectAgent,
  rooms,
}: LauncherConsoleProps) {
  const { t } = useI18n();
  const controller = useLauncherConsoleController({
    onOpenMainAgentDm,
    onOpenRoute,
    onSelectAgent,
  });
  const launcherTour = useMemo(() => buildLauncherTour(t), [t]);
  const decorativeTokens = useMemo(
    () => buildDecorativeTokens(agents, rooms),
    [agents, rooms],
  );
  const mentionTargets = useMemo(
    () => buildLauncherMentionTargets(agents, rooms),
    [agents, rooms],
  );
  const recentEntries = useMemo(
    () => buildRecentLauncherEntries(conversations),
    [conversations],
  );
  usePageOnboardingTour({
    autoStartDelayMs: 260,
    enabled: true,
    tour: launcherTour,
  });

  return (
    <section className="relative flex h-full min-h-0 flex-1 flex-col overflow-hidden">
      <div className="pointer-events-none absolute left-3 top-3 z-20 sm:left-5 sm:top-4">
        <div className="relative flex items-center gap-1 px-1 py-1">
          <LottiePlayer
            className="pointer-events-none absolute left-10 -top-4 h-12 w-12 opacity-[0.72] sm:left-3 sm:-top-15 sm:h-30 sm:w-30"
            inlineStyle={undefined}
            src={ANIMATIONS.BOM}
          />
          <img alt="" className="h-9 w-9 sm:h-10 sm:w-10" src="/logo.webp" />
          <span
            className="mb-3 text-[32px] font-semibold text-foreground"
            style={{
              fontFamily: '"striper", var(--font-sans)',
              fontWeight: 400,
            }}
          >
            nexus
          </span>
        </div>
      </div>
      <div className={cn(
        "relative flex min-h-0 flex-1 items-center justify-center px-8",
        "pb-8 pt-6",
      )}>
        <LauncherHeroStage
          currentAgentId={currentAgentId}
          decorativeTokens={decorativeTokens}
          isQueryLoading={controller.state.isQueryLoading}
          mentionTargets={mentionTargets}
          onEnterHome={controller.actions.enterHome}
          onOpenMainAgentDm={onOpenMainAgentDm}
          onOpenRecentEntry={controller.actions.openRecentEntry}
          onQueryChange={controller.actions.updateQuery}
          onSelectAgent={onSelectAgent}
          onSubmit={controller.actions.submitQuery}
          query={controller.state.query}
          recentEntries={recentEntries}
        />
      </div>
    </section>
  );
}
