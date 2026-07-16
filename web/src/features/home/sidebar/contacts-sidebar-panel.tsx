import { UserPlus, Users2 } from "lucide-react";
import { memo, useCallback, useMemo, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { buildChatNotificationTargetKey } from "@/features/home/notifications/chat-notification-target";
import { resolveDirectRoomNavigationTarget } from "@/features/navigation/direct-room/direct-room-navigation";
import { useI18n } from "@/shared/i18n/i18n-context";
import { SidebarEmptyGuide } from "@/shared/ui/sidebar/sidebar-empty-guide";
import { SIDEBAR_TOUR_ANCHORS } from "@/features/onboarding/tours/sidebar-navigation-tour";
import { useAgentStore } from "@/store/agent";
import { useSidebarStore } from "@/store/sidebar";

import { normalizeSidebarQuery } from "./sidebar-conversation-model";
import { useSidebarDirectory } from "./sidebar-directory";
import {
  ContactRow,
  SidebarListLoadingRows,
  SidebarSearchField,
} from "./sidebar-list-rows";

export const ContactsSidebarPanelContent = memo(function ContactsSidebarPanelContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const location = useLocation();
  const setActiveItem = useSidebarStore((state) => state.set_active_panel_item);
  const clearTargetNotifications = useSidebarStore(
    (state) => state.clear_chat_notifications_for_target,
  );
  const agentRuntimeStatuses = useAgentStore((state) => state.agent_runtime_statuses);
  const { agents, isLoading } = useSidebarDirectory();
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

  const openContacts = useCallback(() => {
    setActiveItem(null);
    if (location.pathname !== AppRouteBuilders.contacts() || location.search) {
      navigate(AppRouteBuilders.contacts());
    }
  }, [location.pathname, location.search, navigate, setActiveItem]);

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
          <button
            className="flex h-9 w-9 items-center justify-center rounded-[12px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_76%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_70%,transparent)] text-(--icon-muted) transition-[background,color] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
            onClick={openContacts}
            title={t("sidebar.manage_contacts")}
            type="button"
          >
            <UserPlus className="h-4 w-4" />
          </button>
        )}
        onChange={setQuery}
        placeholder={t("sidebar.search_contacts")}
        value={query}
      />

      {isLoading ? (
        <SidebarListLoadingRows />
      ) : (
        <div className="flex min-h-0 flex-1 flex-col gap-0.5 px-2 pb-2">
          {filteredAgents.length > 0 ? (
            filteredAgents.map((agent) => {
              const runningTaskCount = agentRuntimeStatuses[agent.id]?.running_task_count ?? 0;
              return (
                <ContactRow
                  agent={agent}
                  isActive={activeAgentId === agent.id}
                  isWorking={runningTaskCount > 0}
                  key={agent.id}
                  onChat={() => {
                    void openAgentDm(agent.id);
                  }}
                  onOpenDirectory={() => openAgentDetail(agent.id)}
                  runningTaskCount={runningTaskCount}
                />
              );
            })
          ) : (
            <SidebarEmptyGuide
              actionLabel={t("sidebar.manage_contacts")}
              description={t("sidebar.contacts_empty_description")}
              icon={Users2}
              onAction={openContacts}
              title={query ? t("sidebar.no_matching_contacts") : t("sidebar.no_contacts")}
            />
          )}
        </div>
      )}
    </div>
  );
});
