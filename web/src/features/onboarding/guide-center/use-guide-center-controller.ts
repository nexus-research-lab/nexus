"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import {
  DM_CONVERSATION_TOUR_ID,
  ROOM_CONVERSATION_TOUR_ID,
} from "@/features/onboarding/tours/conversation-tour";
import { LAUNCHER_TOUR_ID } from "@/features/onboarding/tours/launcher-tour";
import {
  buildSidebarNavigationTour,
  SIDEBAR_NAVIGATION_TOUR_ID,
} from "@/features/onboarding/tours/sidebar-navigation-tour";
import { SKILLS_TOUR_ID } from "@/features/onboarding/tours/skills-tour";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useOnboardingTour } from "@/shared/ui/onboarding/use-onboarding-tour";

import {
  buildGuideCenterItems,
  type GuideCenterActions,
  GUIDE_CENTER_ADVANCED_ITEM_IDS,
  resolveRegisteredRoomTourId,
} from "./guide-center-model";
import { useAutoStartSidebarTour } from "./use-auto-start-sidebar-tour";
import { useGuideCenterNavigation } from "./use-guide-center-navigation";

interface UseGuideCenterControllerInput {
  defaultAgentId: string;
  setActivePanelItem: (itemId: string | null) => void;
}

export function useGuideCenterController({
  defaultAgentId,
  setActivePanelItem,
}: UseGuideCenterControllerInput) {
  const { t } = useI18n();
  const [isGuideCenterOpen, setIsGuideCenterOpen] = useState(false);
  const {
    activeTourId,
    hasCompletedTour,
    isTourRegistered,
    isTourStateReady,
    registerTour,
    resetAllTours,
    resetVersion,
    startTour,
    unregisterTour,
  } = useOnboardingTour();
  const sidebarNavigationTour = useMemo(
    () => buildSidebarNavigationTour(t),
    [t],
  );

  useEffect(() => {
    registerTour(sidebarNavigationTour);
    return () => unregisterTour(sidebarNavigationTour.id);
  }, [registerTour, sidebarNavigationTour, unregisterTour]);

  useAutoStartSidebarTour({
    activeTourId,
    hasCompletedTour,
    isTourStateReady,
    resetVersion,
    startTour,
  });

  const openGuideCenter = useCallback(() => {
    setIsGuideCenterOpen(true);
  }, []);
  const closeGuideCenter = useCallback(() => {
    setIsGuideCenterOpen(false);
  }, []);
  const resetGuides = useCallback(() => {
    resetAllTours();
    closeGuideCenter();
  }, [closeGuideCenter, resetAllTours]);
  const startTourFromCenter = useCallback((tourId: string) => {
    closeGuideCenter();
    window.setTimeout(() => startTour(tourId), 0);
  }, [closeGuideCenter, startTour]);

  const registeredRoomTourId = resolveRegisteredRoomTourId(isTourRegistered);
  const {
    openAutomationsGuide,
    openConnectorsGuide,
    openDmTour,
    openGoalGuide,
    openLauncherTour,
    openProvidersGuide,
    openRoomTour,
    openSidebarTour,
    openSkillsTour,
    openSubagentsGuide,
    openThreadGuide,
    openWorkGraphGuide,
  } = useGuideCenterNavigation({
    closeGuideCenter,
    defaultAgentId,
    isDmTourRegistered: isTourRegistered(DM_CONVERSATION_TOUR_ID),
    isSkillsTourRegistered: isTourRegistered(SKILLS_TOUR_ID),
    registeredRoomTourId,
    setActivePanelItem,
    startTourFromCenter,
  });
  const actions = useMemo<GuideCenterActions>(() => ({
    [DM_CONVERSATION_TOUR_ID]: () => void openDmTour(),
    [GUIDE_CENTER_ADVANCED_ITEM_IDS.automations]: openAutomationsGuide,
    [GUIDE_CENTER_ADVANCED_ITEM_IDS.connectors]: openConnectorsGuide,
    [GUIDE_CENTER_ADVANCED_ITEM_IDS.goal]: () => void openGoalGuide(),
    [GUIDE_CENTER_ADVANCED_ITEM_IDS.providers]: openProvidersGuide,
    [GUIDE_CENTER_ADVANCED_ITEM_IDS.subagents]: () => void openSubagentsGuide(),
    [GUIDE_CENTER_ADVANCED_ITEM_IDS.thread]: () => void openThreadGuide(),
    [GUIDE_CENTER_ADVANCED_ITEM_IDS.workgraph]: openWorkGraphGuide,
    [LAUNCHER_TOUR_ID]: openLauncherTour,
    [ROOM_CONVERSATION_TOUR_ID]: () => void openRoomTour(),
    [SIDEBAR_NAVIGATION_TOUR_ID]: openSidebarTour,
    [SKILLS_TOUR_ID]: openSkillsTour,
  }), [
    openAutomationsGuide,
    openConnectorsGuide,
    openDmTour,
    openGoalGuide,
    openLauncherTour,
    openProvidersGuide,
    openRoomTour,
    openSidebarTour,
    openSkillsTour,
    openSubagentsGuide,
    openThreadGuide,
    openWorkGraphGuide,
  ]);
  const items = useMemo(
    () => buildGuideCenterItems(t, hasCompletedTour, actions),
    [actions, hasCompletedTour, t],
  );
  const guideCenterProps = useMemo(() => ({
    closeLabel: t("common.close"),
    isOpen: isGuideCenterOpen,
    items,
    onClose: closeGuideCenter,
    onReset: resetGuides,
    resetLabel: t("common.reset_guides"),
    reviewedLabel: t("common.reviewed"),
    sectionLabels: {
      advanced: t("guide_center.section_advanced"),
      basics: t("guide_center.section_basics"),
    },
    title: t("common.guide_center"),
  }), [closeGuideCenter, isGuideCenterOpen, items, resetGuides, t]);

  return {
    guideCenterProps,
    isGuideCenterOpen,
    openGuideCenter,
  };
}
