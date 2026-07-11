"use client";

import { useCallback } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import {
  DM_CONVERSATION_TOUR_ID,
} from "@/features/onboarding/tours/conversation-tour";
import { LAUNCHER_TOUR_ID } from "@/features/onboarding/tours/launcher-tour";
import { SIDEBAR_NAVIGATION_TOUR_ID } from "@/features/onboarding/tours/sidebar-navigation-tour";
import { SKILLS_TOUR_ID } from "@/features/onboarding/tours/skills-tour";
import { getLauncherBootstrapApi } from "@/lib/api/launcher-api";
import { resolveDirectRoomNavigationTarget } from "@/features/navigation/direct-room/direct-room-navigation";
import { setRequestedTourId } from "@/shared/ui/onboarding/tour-state";
import { SIDEBAR_CAPABILITY_ITEM_IDS } from "@/store/sidebar";

import {
  type RoomTourNavigationTarget,
  resolveRoomTourNavigationTarget,
} from "./guide-center-model";

interface UseGuideCenterNavigationOptions {
  closeGuideCenter: () => void;
  defaultAgentId: string;
  isDmTourRegistered: boolean;
  isSkillsTourRegistered: boolean;
  registeredRoomTourId: RoomTourNavigationTarget["tourId"] | null;
  setActivePanelItem: (itemId: string | null) => void;
  startTourFromCenter: (tourId: string) => void;
}

interface PageTourRequest {
  route: string;
  sidebarItemId?: string | null;
  tourId: string;
}

export function useGuideCenterNavigation({
  closeGuideCenter,
  defaultAgentId,
  isDmTourRegistered,
  isSkillsTourRegistered,
  registeredRoomTourId,
  setActivePanelItem,
  startTourFromCenter,
}: UseGuideCenterNavigationOptions) {
  const navigate = useNavigate();

  const requestPageTour = useCallback((request: PageTourRequest) => {
    setRequestedTourId(request.tourId);
    closeGuideCenter();
    if (request.sidebarItemId) {
      setActivePanelItem(request.sidebarItemId);
    }
    navigate(request.route);
  }, [closeGuideCenter, navigate, setActivePanelItem]);

  const openLauncherTour = useCallback(() => {
    requestPageTour({
      route: AppRouteBuilders.launcher(),
      tourId: LAUNCHER_TOUR_ID,
    });
  }, [requestPageTour]);

  const openSidebarTour = useCallback(() => {
    startTourFromCenter(SIDEBAR_NAVIGATION_TOUR_ID);
  }, [startTourFromCenter]);

  const openDmTour = useCallback(async () => {
    if (isDmTourRegistered) {
      startTourFromCenter(DM_CONVERSATION_TOUR_ID);
      return;
    }
    closeGuideCenter();
    if (!defaultAgentId) {
      navigate(AppRouteBuilders.contacts());
      return;
    }
    try {
      const target = await resolveDirectRoomNavigationTarget(defaultAgentId);
      setRequestedTourId(DM_CONVERSATION_TOUR_ID);
      setActivePanelItem(target.context.room.id);
      navigate(target.route);
    } catch (error) {
      console.error("[GuideCenter] 打开 DM 引导失败:", error);
      navigate(AppRouteBuilders.contacts());
    }
  }, [
    closeGuideCenter,
    defaultAgentId,
    isDmTourRegistered,
    navigate,
    setActivePanelItem,
    startTourFromCenter,
  ]);

  const openRoomTour = useCallback(async () => {
    if (registeredRoomTourId) {
      startTourFromCenter(registeredRoomTourId);
      return;
    }
    closeGuideCenter();
    try {
      const target = resolveRoomTourNavigationTarget(
        await getLauncherBootstrapApi(),
      );
      if (!target) {
        navigate(AppRouteBuilders.home());
        return;
      }
      setRequestedTourId(target.tourId);
      setActivePanelItem(target.roomId);
      navigate(target.conversationId
        ? AppRouteBuilders.roomConversation(target.roomId, target.conversationId)
        : AppRouteBuilders.room(target.roomId));
    } catch (error) {
      console.error("[GuideCenter] 打开 Room 引导失败:", error);
      navigate(AppRouteBuilders.home());
    }
  }, [
    closeGuideCenter,
    navigate,
    registeredRoomTourId,
    setActivePanelItem,
    startTourFromCenter,
  ]);

  const openSkillsTour = useCallback(() => {
    if (isSkillsTourRegistered) {
      startTourFromCenter(SKILLS_TOUR_ID);
      return;
    }
    requestPageTour({
      route: AppRouteBuilders.skills(),
      sidebarItemId: SIDEBAR_CAPABILITY_ITEM_IDS.skills,
      tourId: SKILLS_TOUR_ID,
    });
  }, [isSkillsTourRegistered, requestPageTour, startTourFromCenter]);

  return {
    openDmTour,
    openLauncherTour,
    openRoomTour,
    openSidebarTour,
    openSkillsTour,
  };
}
