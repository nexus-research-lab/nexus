/**
 * INPUT: Agent 目录、共享目录降级状态、当前路由与联系人导航命令。
 * OUTPUT: 可搜索、可恢复且保留 stale Agent 数据的联系人侧栏。
 * POS: Home 联系人目录视图；不直接发起 bootstrap 请求。
 */
import { CircleAlert, CirclePlus, Users2 } from "lucide-react";
import { memo, useCallback, useMemo, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { buildChatNotificationTargetKey } from "@/features/home/notifications/chat-notification-target";
import { HomeDirectoryRefreshErrorNotice } from "@/features/home/home-directory-refresh-error-notice";
import { resolveDirectRoomNavigationTarget } from "@/features/navigation/direct-room/direct-room-navigation";
import { useI18n } from "@/shared/i18n/i18n-context";
import { SidebarEmptyGuide } from "@/shared/ui/sidebar/sidebar-empty-guide";
import {
  SidebarSearchAction,
  SidebarSearchField,
} from "@/shared/ui/form/sidebar-search-field";
import { SIDEBAR_TOUR_ANCHORS } from "@/features/onboarding/tours/sidebar-navigation-tour";
import { useSidebarStore } from "@/store/sidebar";

import { normalizeSidebarQuery } from "./sidebar-conversation-model";
import { useSidebarDirectory } from "./sidebar-directory";
import {
  ContactRow,
  SidebarListLoadingRows,
} from "./sidebar-list-rows";

export const ContactsSidebarPanelContent = memo(function ContactsSidebarPanelContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const location = useLocation();
  const setActiveItem = useSidebarStore((state) => state.set_active_panel_item);
  const clearTargetNotifications = useSidebarStore(
    (state) => state.clear_chat_notifications_for_target,
  );
  const {
    agents,
    hasError,
    hasLoaded,
    isLoading,
    refreshDirectory,
  } = useSidebarDirectory();
  const [query, setQuery] = useState("");
  const activeAgentId = location.pathname === AppRouteBuilders.contacts()
    ? new URLSearchParams(location.search).get("agent")
    : null;
  const filteredAgents = useMemo(() => {
    const normalizedQuery = normalizeSidebarQuery(query);
    return normalizedQuery
      ? agents.filter((agent) => agent.name.toLowerCase().includes(normalizedQuery))
      : agents;
  }, [agents, query]);

  const openContactsDirectory = useCallback(() => {
    setActiveItem(null);
    const target = location.pathname === AppRouteBuilders.contacts()
      ? AppRouteBuilders.contactsManage()
      : AppRouteBuilders.contacts();
    if (`${location.pathname}${location.search}` !== target) {
      navigate(target);
    }
  }, [location.pathname, location.search, navigate, setActiveItem]);

  const openAgentCreation = useCallback(() => {
    setActiveItem(null);
    navigate(AppRouteBuilders.contactsCreate());
  }, [navigate, setActiveItem]);

  const openAgentDetail = useCallback((agentId: string) => {
    setActiveItem(agentId);
    navigate(AppRouteBuilders.contactAgent(agentId));
  }, [navigate, setActiveItem]);

  const openAgentDm = useCallback(async (agentId: string) => {
    const target = await resolveDirectRoomNavigationTarget(agentId);
    clearTargetNotifications(buildChatNotificationTargetKey({
      conversation_id: target.context.conversation.id,
      room_id: target.context.room.id,
    }));
    setActiveItem(target.context.room.id);
    navigate(target.route);
  }, [clearTargetNotifications, navigate, setActiveItem]);

  return (
    <div
      className="flex min-h-0 flex-1 flex-col"
      data-tour-anchor={SIDEBAR_TOUR_ANCHORS.contacts_list}
    >
      <SidebarSearchField
        action={(
          <SidebarSearchAction
            onClick={openAgentCreation}
            title={t("contacts.new_agent")}
          >
            <CirclePlus />
          </SidebarSearchAction>
        )}
        onChange={setQuery}
        placeholder={t("sidebar.search_contacts")}
        value={query}
      />

      {isLoading ? (
        <SidebarListLoadingRows />
      ) : (
        <div className="flex min-h-0 flex-1 flex-col gap-0.5 px-2 pb-2 max-lg:gap-1 max-lg:px-3">
          {hasError && hasLoaded ? (
            <HomeDirectoryRefreshErrorNotice
              className="mb-1"
              onRetry={refreshDirectory}
            />
          ) : null}
          {filteredAgents.length > 0 ? (
            filteredAgents.map((agent) => (
              <ContactRow
                agent={agent}
                isActive={activeAgentId === agent.id}
                key={agent.id}
                onChat={() => {
                  void openAgentDm(agent.id);
                }}
                onOpenDirectory={() => openAgentDetail(agent.id)}
              />
            ))
          ) : hasError && !hasLoaded ? (
            <SidebarEmptyGuide
              actionLabel={t("sidebar.retry")}
              description={t("sidebar.directory_load_failed_description")}
              impact={t("sidebar.directory_load_failed_impact")}
              icon={CircleAlert}
              onAction={refreshDirectory}
              title={t("sidebar.directory_load_failed")}
            />
          ) : (
            <SidebarEmptyGuide
              actionLabel={t("sidebar.manage_contacts")}
              description={t("sidebar.contacts_empty_description")}
              icon={Users2}
              onAction={openContactsDirectory}
              title={query ? t("sidebar.no_matching_contacts") : t("sidebar.no_contacts")}
            />
          )}
        </div>
      )}
    </div>
  );
});
