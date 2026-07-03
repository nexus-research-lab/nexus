"use client";

import { useCallback, useMemo, useState } from "react";
import { AppRouteBuilders } from "@/app/router/route-paths";

import { ANIMATIONS } from "@/config/animation-assets";
import { useI18n } from "@/shared/i18n/i18n-context";
import { LottiePlayer } from "@/shared/ui/feedback/lottie-player";
import { cn } from "@/lib/utils";
import { useSidebarStore } from "@/store/sidebar";
import { queryLauncher } from "@/lib/api/launcher-api";
import { ensureDirectRoom, getRoomContexts } from "@/lib/api/room-api";
import {
  buildLauncherTour,
} from "@/features/launcher/launcher-tour";
import { usePageOnboardingTour } from "@/shared/ui/onboarding/use-page-onboarding-tour";
import {
  buildDecorativeTokens,
  buildLauncherMentionTargets,
  buildRecentLauncherEntries,
} from "./launcher-console-helpers";
import {
  LauncherConsoleProps,
  RecentLauncherEntry,
} from "./launcher-console-types";
import { LauncherHeroStage } from "./launcher-hero-stage";

export function LauncherConsole({
  agents,
  rooms,
  conversations,
  currentAgentId: currentAgentId,
  onOpenMainAgentDm: onOpenMainAgentDm,
  onOpenRoute: onOpenRoute,
  onSelectAgent: onSelectAgent,
}: LauncherConsoleProps) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const [isQueryLoading, setIsQueryLoading] = useState(false);
  const setActivePanelItem = useSidebarStore((s) => s.set_active_panel_item);
  const launcherTour = useMemo(() => buildLauncherTour(t), [t]);
  usePageOnboardingTour({
    tour: launcherTour,
    enabled: true,
    autoStartDelayMs: 260,
  });
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

  const handleOpenRecentEntry = useCallback(
    (entry: RecentLauncherEntry) => {
      void (async () => {
        try {
          if (entry.conversation_id) {
            if (!entry.room_id) {
              return;
            }
            setActivePanelItem(entry.room_id);
            onOpenRoute(
              AppRouteBuilders.roomConversation(entry.room_id, entry.conversation_id),
            );
            return;
          }

          if (entry.type === "dm" && entry.agent_id) {
            onSelectAgent(entry.agent_id);
            const context = await ensureDirectRoom(entry.agent_id);
            setActivePanelItem(context.room.id);
            onOpenRoute(
              AppRouteBuilders.roomConversation(context.room.id, context.conversation.id),
            );
            return;
          }

          if (!entry.room_id) {
            return;
          }

          const contexts = await getRoomContexts(entry.room_id);
          if (contexts.length > 0) {
            setActivePanelItem(entry.room_id);
            onOpenRoute(
              AppRouteBuilders.roomConversation(entry.room_id, contexts[0].conversation.id),
            );
          }
        } catch (error) {
          console.error("Failed to open recent entry:", error);
        }
      })();
    },
    [onOpenRoute, onSelectAgent, setActivePanelItem],
  );

  const handleSubmit = useCallback(
    async (nextQuery?: string) => {
      const trimmed = (nextQuery ?? query).trim();
      if (!trimmed || isQueryLoading) {
        return;
      }

      setIsQueryLoading(true);
      try {
        const action = await queryLauncher({ query: trimmed });

        switch (action.action_type) {
          case "open_agent_dm": {
            onSelectAgent(action.target_id);
            const context = await ensureDirectRoom(action.target_id);
            if (context) {
              setActivePanelItem(context.room.id);
              const route = AppRouteBuilders.roomConversation(
                context.room.id,
                context.conversation.id,
              );
              const finalRoute = action.initial_message
                ? `${route}?initial=${encodeURIComponent(action.initial_message)}`
                : route;
              onOpenRoute(finalRoute);
            }
            break;
          }
          case "open_app": {
            onOpenMainAgentDm(action.initial_message || trimmed);
            break;
          }
          case "open_room": {
            const contexts = await getRoomContexts(action.target_id);
            if (contexts.length > 0) {
              setActivePanelItem(action.target_id);
              const route = AppRouteBuilders.roomConversation(
                action.target_id,
                contexts[0].conversation.id,
              );
              const finalRoute = action.initial_message
                ? `${route}?initial=${encodeURIComponent(action.initial_message)}`
                : route;
              onOpenRoute(finalRoute);
            }
            break;
          }
        }
      } catch (error) {
        console.error("Launcher query failed:", error);
      } finally {
        setIsQueryLoading(false);
      }
    },
    [
      query,
      isQueryLoading,
      onOpenMainAgentDm,
      onOpenRoute,
      onSelectAgent,
      setActivePanelItem,
    ],
  );

  const handleEnterHome = useCallback(() => {
    onOpenRoute(AppRouteBuilders.home());
  }, [onOpenRoute]);

  const handleInputChange = useCallback((value: string) => {
    setQuery(value);
  }, []);

  const handlePrimaryAction = useCallback(
    (submittedInput: string) => {
      const trimmedQuery = submittedInput.trim();
      if (!trimmedQuery || isQueryLoading) {
        return false;
      }

      setQuery("");
      void handleSubmit(trimmedQuery);
      return true;
    },
    [handleSubmit, isQueryLoading],
  );

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
            className="text-[32px] font-semibold text-foreground mb-3"
            style={{
              fontFamily: '"striper", var(--font-sans)',
              fontWeight: 400,
            }}
          >
            nexus
          </span>
        </div>
      </div>
      <div
        className={cn(
          "relative flex min-h-0 flex-1 items-center justify-center px-8",
          "pb-8 pt-6",
        )}
      >
        <LauncherHeroStage
          currentAgentId={currentAgentId}
          decorativeTokens={decorativeTokens}
          mentionTargets={mentionTargets}
          onEnterHome={handleEnterHome}
          onOpenMainAgentDm={onOpenMainAgentDm}
          onQueryChange={handleInputChange}
          onSelectAgent={onSelectAgent}
          onOpenRecentEntry={handleOpenRecentEntry}
          onSubmit={handlePrimaryAction}
          query={query}
          recentEntries={recentEntries}
          isQueryLoading={isQueryLoading}
        />
      </div>
    </section>
  );
}
