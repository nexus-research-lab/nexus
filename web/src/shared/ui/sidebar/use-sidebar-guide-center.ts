import {
  Compass,
  MessageSquare,
  Rocket,
  Wrench,
} from "lucide-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { SKILLS_TOUR_ID } from "@/features/capability/skills/skills-tour";
import {
  DM_CONVERSATION_TOUR_ID,
  ROOM_CONVERSATION_TOUR_ID,
  ROOM_EMPTY_CONVERSATION_TOUR_ID,
} from "@/features/conversation/room/room-tour";
import { LAUNCHER_TOUR_ID } from "@/features/launcher/launcher-tour";
import { getLauncherBootstrapApi } from "@/lib/api/launcher-api";
import { resolveDirectRoomNavigationTarget } from "@/lib/conversation/direct-room-navigation";
import { useI18n } from "@/shared/i18n/i18n-context";
import { OnboardingGuideCenterItem } from "@/shared/ui/onboarding/onboarding-guide-center";
import { setRequestedTourId } from "@/shared/ui/onboarding/tour-state";
import { useOnboardingTour } from "@/shared/ui/onboarding/use-onboarding-tour";
import {
  buildSidebarNavigationTour,
  SIDEBAR_NAVIGATION_TOUR_ID,
} from "@/shared/ui/sidebar/sidebar-navigation-tour";
import { SIDEBAR_CAPABILITY_ITEM_IDS } from "@/store/sidebar";

interface UseSidebarGuideCenterInput {
  default_agent_id: string;
  set_active_panel_item: (itemId: string | null) => void;
}

export function useSidebarGuideCenter({
  default_agent_id: defaultAgentId,
  set_active_panel_item: setActivePanelItem,
}: UseSidebarGuideCenterInput) {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [isGuideCenterOpen, setIsGuideCenterOpen] = useState(false);
  const hasAutoStartedTourRef = useRef(false);
  const {
    activeTourId: activeTourId,
    hasCompletedTour: hasCompletedTour,
    isTourRegistered: isTourRegistered,
    isTourStateReady: isTourStateReady,
    registerTour: registerTour,
    resetVersion: resetVersion,
    resetAllTours: resetAllTours,
    startTour: startTour,
    unregisterTour: unregisterTour,
  } = useOnboardingTour();
  const sidebarNavigationTour = useMemo(
    () => buildSidebarNavigationTour(t),
    [t],
  );
  const isDmTourRegistered = isTourRegistered(DM_CONVERSATION_TOUR_ID);
  const registeredRoomTourId = useMemo(() => {
    if (isTourRegistered(ROOM_CONVERSATION_TOUR_ID)) {
      return ROOM_CONVERSATION_TOUR_ID;
    }
    if (isTourRegistered(ROOM_EMPTY_CONVERSATION_TOUR_ID)) {
      return ROOM_EMPTY_CONVERSATION_TOUR_ID;
    }
    return null;
  }, [isTourRegistered]);

  useEffect(() => {
    registerTour(sidebarNavigationTour);
    return () => {
      unregisterTour(sidebarNavigationTour.id);
    };
  }, [registerTour, sidebarNavigationTour, unregisterTour]);

  useEffect(() => {
    if (hasAutoStartedTourRef.current) {
      return;
    }
    if (!isTourStateReady) {
      return;
    }
    if (activeTourId) {
      return;
    }
    if (hasCompletedTour(SIDEBAR_NAVIGATION_TOUR_ID)) {
      return;
    }
    hasAutoStartedTourRef.current = true;
    const timeoutId = window.setTimeout(() => {
      startTour(SIDEBAR_NAVIGATION_TOUR_ID);
    }, 220);

    return () => {
      window.clearTimeout(timeoutId);
    };
  }, [activeTourId, hasCompletedTour, isTourStateReady, startTour]);

  useEffect(() => {
    hasAutoStartedTourRef.current = false;
  }, [resetVersion]);

  const openGuideCenter = useCallback(() => {
    setIsGuideCenterOpen(true);
  }, []);

  const closeGuideCenter = useCallback(() => {
    setIsGuideCenterOpen(false);
  }, []);

  const handleResetGuides = useCallback(() => {
    resetAllTours();
    setIsGuideCenterOpen(false);
  }, [resetAllTours]);

  const handleStartTourFromCenter = useCallback((tourId: string) => {
    setIsGuideCenterOpen(false);
    window.setTimeout(() => {
      startTour(tourId);
    }, 0);
  }, [startTour]);

  const handleRequestPageTour = useCallback((
    tourId: string,
    route: string,
    sidebarItemId?: string | null,
  ) => {
    setRequestedTourId(tourId);
    setIsGuideCenterOpen(false);
    if (sidebarItemId) {
      setActivePanelItem(sidebarItemId);
    }
    navigate(route);
  }, [navigate, setActivePanelItem]);

  const handleOpenDmTour = useCallback(async () => {
    if (isDmTourRegistered) {
      handleStartTourFromCenter(DM_CONVERSATION_TOUR_ID);
      return;
    }

    setIsGuideCenterOpen(false);
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
      console.error("[SidebarWidePanel] 打开 DM 引导失败:", error);
      navigate(AppRouteBuilders.contacts());
    }
  }, [
    defaultAgentId,
    handleStartTourFromCenter,
    isDmTourRegistered,
    navigate,
    setActivePanelItem,
  ]);

  const handleOpenRoomTour = useCallback(async () => {
    if (registeredRoomTourId) {
      handleStartTourFromCenter(registeredRoomTourId);
      return;
    }

    setIsGuideCenterOpen(false);

    try {
      const payload = await getLauncherBootstrapApi();
      const targetRoom = payload.rooms.find((room) => room.room_type === "room");

      if (!targetRoom) {
        navigate(AppRouteBuilders.home());
        return;
      }

      const roomConversations = payload.conversations
        .filter((conversation) => conversation.room_id === targetRoom.id)
        .sort((left, right) =>
          new Date(right.last_activity).getTime() - new Date(left.last_activity).getTime()
        );

      setActivePanelItem(targetRoom.id);
      if (roomConversations.length > 0 && roomConversations[0].conversation_id) {
        setRequestedTourId(ROOM_CONVERSATION_TOUR_ID);
        navigate(
          AppRouteBuilders.roomConversation(
            targetRoom.id,
            roomConversations[0].conversation_id,
          ),
        );
        return;
      }

      setRequestedTourId(ROOM_EMPTY_CONVERSATION_TOUR_ID);
      navigate(AppRouteBuilders.room(targetRoom.id));
    } catch (error) {
      console.error("[SidebarWidePanel] 打开 Room 引导失败:", error);
      navigate(AppRouteBuilders.home());
    }
  }, [
    handleStartTourFromCenter,
    navigate,
    registeredRoomTourId,
    setActivePanelItem,
  ]);

  const guideCenterItems = useMemo<OnboardingGuideCenterItem[]>(() => [
    {
      id: LAUNCHER_TOUR_ID,
      icon: Rocket,
      title: t("launcher.tour_intro_title"),
      description: t("launcher.tour_intro_description"),
      actionLabel: t("common.view_guide"),
      completed: hasCompletedTour(LAUNCHER_TOUR_ID),
      onAction: () => handleRequestPageTour(
        LAUNCHER_TOUR_ID,
        AppRouteBuilders.launcher(),
      ),
    },
    {
      id: SIDEBAR_NAVIGATION_TOUR_ID,
      icon: Compass,
      title: t("sidebar.tour_intro_title"),
      description: t("sidebar.tour_intro_description"),
      actionLabel: t("common.view_guide"),
      completed: hasCompletedTour(SIDEBAR_NAVIGATION_TOUR_ID),
      onAction: () => handleStartTourFromCenter(SIDEBAR_NAVIGATION_TOUR_ID),
    },
    {
      id: DM_CONVERSATION_TOUR_ID,
      icon: MessageSquare,
      title: t("room.tour_dm_intro_title"),
      description: t("room.tour_dm_intro_description"),
      actionLabel: t("common.view_guide"),
      completed: hasCompletedTour(DM_CONVERSATION_TOUR_ID),
      onAction: () => {
        void handleOpenDmTour();
      },
    },
    {
      id: ROOM_CONVERSATION_TOUR_ID,
      icon: MessageSquare,
      title: t("room.tour_group_intro_title"),
      description: t("room.tour_group_intro_description"),
      actionLabel: t("common.view_guide"),
      completed: hasCompletedTour(ROOM_CONVERSATION_TOUR_ID)
        || hasCompletedTour(ROOM_EMPTY_CONVERSATION_TOUR_ID),
      onAction: () => {
        void handleOpenRoomTour();
      },
    },
    {
      id: SKILLS_TOUR_ID,
      icon: Wrench,
      title: t("capability.skills_tour_intro_title"),
      description: t("capability.skills_tour_intro_description"),
      actionLabel: t("common.view_guide"),
      completed: hasCompletedTour(SKILLS_TOUR_ID),
      onAction: () => {
        if (isTourRegistered(SKILLS_TOUR_ID)) {
          handleStartTourFromCenter(SKILLS_TOUR_ID);
          return;
        }
        handleRequestPageTour(
          SKILLS_TOUR_ID,
          AppRouteBuilders.skills(),
          SIDEBAR_CAPABILITY_ITEM_IDS.skills,
        );
      },
    },
  ], [
    handleOpenDmTour,
    handleOpenRoomTour,
    handleRequestPageTour,
    handleStartTourFromCenter,
    hasCompletedTour,
    isTourRegistered,
    t,
  ]);

  const guideCenterProps = useMemo(() => ({
    closeLabel: t("common.close"),
    description: t("onboarding.guide_center_description"),
    isOpen: isGuideCenterOpen,
    items: guideCenterItems,
    onClose: closeGuideCenter,
    onReset: handleResetGuides,
    resetLabel: t("common.reset_guides"),
    reviewedLabel: t("common.reviewed"),
    title: t("common.guide_center"),
  }), [
    closeGuideCenter,
    guideCenterItems,
    handleResetGuides,
    isGuideCenterOpen,
    t,
  ]);

  return {
    guide_center_props: guideCenterProps,
    is_guide_center_open: isGuideCenterOpen,
    open_guide_center: openGuideCenter,
  };
}
