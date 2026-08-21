"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { ANIMATIONS } from "@/config/animation-assets";
import { ProviderSetupDialog } from "@/features/onboarding/provider-setup/provider-setup-dialog";
import {
  buildLauncherTour,
  LAUNCHER_TOUR_ID,
} from "@/features/onboarding/tours/launcher-tour";
import { useProviderAvailability } from "@/hooks/capability/use-provider-availability";
import { useI18n } from "@/shared/i18n/i18n-context";
import { LottiePlayer } from "@/shared/ui/feedback/lottie-player";
import { setTourDismissed } from "@/shared/ui/onboarding/tour-state";
import { usePageOnboardingTour } from "@/shared/ui/onboarding/use-page-onboarding-tour";

import { LauncherHeroStage } from "../hero/launcher-hero-stage";
import {
  buildDecorativeTokens,
  buildLauncherMentionTargets,
  buildRecentLauncherEntries,
} from "./launcher-console-helpers";
import type { LauncherConsoleProps } from "./launcher-console-types";
import { useLauncherConsoleController } from "./use-launcher-console-controller";

const PROVIDER_SETUP_PROMPTED_SESSION_KEY = "nexus:onboarding:provider-setup-prompted:v1";

export function LauncherConsole({
  agents,
  conversations,
  currentAgentId,
  initialQuery,
  onOpenMainAgentDm,
  onOpenRoute,
  onSelectAgent,
  rooms,
}: LauncherConsoleProps) {
  const { t } = useI18n();
  const controller = useLauncherConsoleController({
    initialQuery,
    onOpenMainAgentDm,
    onOpenRoute,
    onSelectAgent,
  });
  const { hasAvailableProvider, isReady } = useProviderAvailability();
  const [setupOpen, setSetupOpen] = useState(false);
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
  const openProviderSetup = useCallback(() => {
    markProviderSetupPrompted();
    setTourDismissed(LAUNCHER_TOUR_ID, true);
    setSetupOpen(true);
  }, []);

  useEffect(() => {
    if (!isReady || hasAvailableProvider || setupOpen || providerSetupWasPrompted()) {
      return undefined;
    }
    const timeoutId = window.setTimeout(() => {
      openProviderSetup();
    }, 420);
    return () => {
      window.clearTimeout(timeoutId);
    };
  }, [hasAvailableProvider, isReady, openProviderSetup, setupOpen]);

  usePageOnboardingTour({
    autoStartDelayMs: 260,
    enabled: isReady && hasAvailableProvider && !setupOpen,
    tour: launcherTour,
  });

  const submitQuery = useCallback((input: string) => {
    if (isReady && !hasAvailableProvider) {
      controller.actions.updateQuery(input);
      openProviderSetup();
      return true;
    }
    return controller.actions.submitQuery(input);
  }, [controller.actions, hasAvailableProvider, isReady, openProviderSetup]);

  const openMainAgentDm = useCallback((initialPrompt?: string) => {
    if (isReady && !hasAvailableProvider) {
      if (initialPrompt) {
        controller.actions.updateQuery(initialPrompt);
      }
      openProviderSetup();
      return;
    }
    onOpenMainAgentDm(initialPrompt);
  }, [controller.actions, hasAvailableProvider, isReady, onOpenMainAgentDm, openProviderSetup]);

  return (
    <>
      <section className="relative flex h-full min-h-0 flex-1 flex-col overflow-hidden">
        <header
          className="relative z-20 h-28 shrink-0"
          data-desktop-window-drag-region
        >
          <LottiePlayer
            className="launcher-console-spotlights pointer-events-none absolute opacity-[0.68]"
            inlineStyle={undefined}
            src={ANIMATIONS.BOM}
          />
          <div className="pointer-events-none absolute bottom-2 left-3 flex items-center gap-1">
            <img alt="" className="h-10 w-10" src="/logo.webp" />
            <span
              className="launcher-console-wordmark mb-3 text-[32px] text-foreground"
              style={{
                fontFamily: '"Striper", var(--font-sans)',
              }}
            >
              nexus
            </span>
          </div>
        </header>
        <div className="relative min-h-0 flex-1 px-8 pb-6">
          <LauncherHeroStage
            currentAgentId={currentAgentId}
            decorativeTokens={decorativeTokens}
            isQueryLoading={controller.state.isQueryLoading}
            mentionTargets={mentionTargets}
            onEnterHome={controller.actions.enterHome}
            onOpenMainAgentDm={openMainAgentDm}
            onOpenRecentEntry={controller.actions.openRecentEntry}
            onQueryChange={controller.actions.updateQuery}
            onSelectAgent={onSelectAgent}
            onSubmit={submitQuery}
            query={controller.state.query}
            recentEntries={recentEntries}
          />
        </div>
      </section>
      <ProviderSetupDialog
        isOpen={setupOpen}
        onClose={() => setSetupOpen(false)}
        onStart={() => onOpenMainAgentDm(controller.state.query.trim() || undefined)}
      />
    </>
  );
}

function providerSetupWasPrompted(): boolean {
  try {
    return window.sessionStorage.getItem(PROVIDER_SETUP_PROMPTED_SESSION_KEY) === "1";
  } catch {
    return false;
  }
}

function markProviderSetupPrompted(): void {
  try {
    window.sessionStorage.setItem(PROVIDER_SETUP_PROMPTED_SESSION_KEY, "1");
  } catch {
    // 存储不可用时仍允许当前页面完成引导。
  }
}
