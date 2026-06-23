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
import { get_launcher_bootstrap_api } from "@/lib/api/launcher-api";
import { resolve_direct_room_navigation_target } from "@/lib/conversation/direct-room-navigation";
import { useI18n } from "@/shared/i18n/i18n-context";
import { OnboardingGuideCenterItem } from "@/shared/ui/onboarding/onboarding-guide-center";
import { set_requested_tour_id } from "@/shared/ui/onboarding/tour-state";
import { useOnboardingTour } from "@/shared/ui/onboarding/use-onboarding-tour";
import {
  build_sidebar_navigation_tour,
  SIDEBAR_NAVIGATION_TOUR_ID,
} from "@/shared/ui/sidebar/sidebar-navigation-tour";
import { SIDEBAR_CAPABILITY_ITEM_IDS } from "@/store/sidebar";

interface UseSidebarGuideCenterInput {
  default_agent_id: string;
  set_active_panel_item: (item_id: string | null) => void;
}

export function useSidebarGuideCenter({
  default_agent_id,
  set_active_panel_item,
}: UseSidebarGuideCenterInput) {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [is_guide_center_open, set_is_guide_center_open] = useState(false);
  const has_auto_started_tour_ref = useRef(false);
  const {
    active_tour_id,
    has_completed_tour,
    is_tour_registered,
    is_tour_state_ready,
    register_tour,
    reset_version,
    reset_all_tours,
    start_tour,
    unregister_tour,
  } = useOnboardingTour();
  const sidebar_navigation_tour = useMemo(
    () => build_sidebar_navigation_tour(t),
    [t],
  );
  const is_dm_tour_registered = is_tour_registered(DM_CONVERSATION_TOUR_ID);
  const registered_room_tour_id = useMemo(() => {
    if (is_tour_registered(ROOM_CONVERSATION_TOUR_ID)) {
      return ROOM_CONVERSATION_TOUR_ID;
    }
    if (is_tour_registered(ROOM_EMPTY_CONVERSATION_TOUR_ID)) {
      return ROOM_EMPTY_CONVERSATION_TOUR_ID;
    }
    return null;
  }, [is_tour_registered]);

  useEffect(() => {
    register_tour(sidebar_navigation_tour);
    return () => {
      unregister_tour(sidebar_navigation_tour.id);
    };
  }, [register_tour, sidebar_navigation_tour, unregister_tour]);

  useEffect(() => {
    if (has_auto_started_tour_ref.current) {
      return;
    }
    if (!is_tour_state_ready) {
      return;
    }
    if (active_tour_id) {
      return;
    }
    if (has_completed_tour(SIDEBAR_NAVIGATION_TOUR_ID)) {
      return;
    }
    has_auto_started_tour_ref.current = true;
    const timeout_id = window.setTimeout(() => {
      start_tour(SIDEBAR_NAVIGATION_TOUR_ID);
    }, 220);

    return () => {
      window.clearTimeout(timeout_id);
    };
  }, [active_tour_id, has_completed_tour, is_tour_state_ready, start_tour]);

  useEffect(() => {
    has_auto_started_tour_ref.current = false;
  }, [reset_version]);

  const open_guide_center = useCallback(() => {
    set_is_guide_center_open(true);
  }, []);

  const close_guide_center = useCallback(() => {
    set_is_guide_center_open(false);
  }, []);

  const handle_reset_guides = useCallback(() => {
    reset_all_tours();
    set_is_guide_center_open(false);
  }, [reset_all_tours]);

  const handle_start_tour_from_center = useCallback((tour_id: string) => {
    set_is_guide_center_open(false);
    window.setTimeout(() => {
      start_tour(tour_id);
    }, 0);
  }, [start_tour]);

  const handle_request_page_tour = useCallback((
    tour_id: string,
    route: string,
    sidebar_item_id?: string | null,
  ) => {
    set_requested_tour_id(tour_id);
    set_is_guide_center_open(false);
    if (sidebar_item_id) {
      set_active_panel_item(sidebar_item_id);
    }
    navigate(route);
  }, [navigate, set_active_panel_item]);

  const handle_open_dm_tour = useCallback(async () => {
    if (is_dm_tour_registered) {
      handle_start_tour_from_center(DM_CONVERSATION_TOUR_ID);
      return;
    }

    set_is_guide_center_open(false);
    if (!default_agent_id) {
      navigate(AppRouteBuilders.contacts());
      return;
    }

    try {
      const target = await resolve_direct_room_navigation_target(default_agent_id);
      set_requested_tour_id(DM_CONVERSATION_TOUR_ID);
      set_active_panel_item(target.context.room.id);
      navigate(target.route);
    } catch (error) {
      console.error("[SidebarWidePanel] 打开 DM 引导失败:", error);
      navigate(AppRouteBuilders.contacts());
    }
  }, [
    default_agent_id,
    handle_start_tour_from_center,
    is_dm_tour_registered,
    navigate,
    set_active_panel_item,
  ]);

  const handle_open_room_tour = useCallback(async () => {
    if (registered_room_tour_id) {
      handle_start_tour_from_center(registered_room_tour_id);
      return;
    }

    set_is_guide_center_open(false);

    try {
      const payload = await get_launcher_bootstrap_api();
      const target_room = payload.rooms.find((room) => room.room_type === "room");

      if (!target_room) {
        navigate(AppRouteBuilders.home());
        return;
      }

      const room_conversations = payload.conversations
        .filter((conversation) => conversation.room_id === target_room.id)
        .sort((left, right) =>
          new Date(right.last_activity).getTime() - new Date(left.last_activity).getTime()
        );

      set_active_panel_item(target_room.id);
      if (room_conversations.length > 0 && room_conversations[0].conversation_id) {
        set_requested_tour_id(ROOM_CONVERSATION_TOUR_ID);
        navigate(
          AppRouteBuilders.room_conversation(
            target_room.id,
            room_conversations[0].conversation_id,
          ),
        );
        return;
      }

      set_requested_tour_id(ROOM_EMPTY_CONVERSATION_TOUR_ID);
      navigate(AppRouteBuilders.room(target_room.id));
    } catch (error) {
      console.error("[SidebarWidePanel] 打开 Room 引导失败:", error);
      navigate(AppRouteBuilders.home());
    }
  }, [
    handle_start_tour_from_center,
    navigate,
    registered_room_tour_id,
    set_active_panel_item,
  ]);

  const guide_center_items = useMemo<OnboardingGuideCenterItem[]>(() => [
    {
      id: LAUNCHER_TOUR_ID,
      icon: Rocket,
      title: t("launcher.tour_intro_title"),
      description: t("launcher.tour_intro_description"),
      action_label: t("common.view_guide"),
      completed: has_completed_tour(LAUNCHER_TOUR_ID),
      on_action: () => handle_request_page_tour(
        LAUNCHER_TOUR_ID,
        AppRouteBuilders.launcher(),
      ),
    },
    {
      id: SIDEBAR_NAVIGATION_TOUR_ID,
      icon: Compass,
      title: t("sidebar.tour_intro_title"),
      description: t("sidebar.tour_intro_description"),
      action_label: t("common.view_guide"),
      completed: has_completed_tour(SIDEBAR_NAVIGATION_TOUR_ID),
      on_action: () => handle_start_tour_from_center(SIDEBAR_NAVIGATION_TOUR_ID),
    },
    {
      id: DM_CONVERSATION_TOUR_ID,
      icon: MessageSquare,
      title: t("room.tour_dm_intro_title"),
      description: t("room.tour_dm_intro_description"),
      action_label: t("common.view_guide"),
      completed: has_completed_tour(DM_CONVERSATION_TOUR_ID),
      on_action: () => {
        void handle_open_dm_tour();
      },
    },
    {
      id: ROOM_CONVERSATION_TOUR_ID,
      icon: MessageSquare,
      title: t("room.tour_group_intro_title"),
      description: t("room.tour_group_intro_description"),
      action_label: t("common.view_guide"),
      completed: has_completed_tour(ROOM_CONVERSATION_TOUR_ID)
        || has_completed_tour(ROOM_EMPTY_CONVERSATION_TOUR_ID),
      on_action: () => {
        void handle_open_room_tour();
      },
    },
    {
      id: SKILLS_TOUR_ID,
      icon: Wrench,
      title: t("capability.skills_tour_intro_title"),
      description: t("capability.skills_tour_intro_description"),
      action_label: t("common.view_guide"),
      completed: has_completed_tour(SKILLS_TOUR_ID),
      on_action: () => {
        if (is_tour_registered(SKILLS_TOUR_ID)) {
          handle_start_tour_from_center(SKILLS_TOUR_ID);
          return;
        }
        handle_request_page_tour(
          SKILLS_TOUR_ID,
          AppRouteBuilders.skills(),
          SIDEBAR_CAPABILITY_ITEM_IDS.skills,
        );
      },
    },
  ], [
    handle_open_dm_tour,
    handle_open_room_tour,
    handle_request_page_tour,
    handle_start_tour_from_center,
    has_completed_tour,
    is_tour_registered,
    t,
  ]);

  const guide_center_props = useMemo(() => ({
    close_label: t("common.close"),
    description: t("onboarding.guide_center_description"),
    is_open: is_guide_center_open,
    items: guide_center_items,
    on_close: close_guide_center,
    on_reset: handle_reset_guides,
    reset_label: t("common.reset_guides"),
    reviewed_label: t("common.reviewed"),
    title: t("common.guide_center"),
  }), [
    close_guide_center,
    guide_center_items,
    handle_reset_guides,
    is_guide_center_open,
    t,
  ]);

  return {
    guide_center_props,
    is_guide_center_open,
    open_guide_center,
  };
}
