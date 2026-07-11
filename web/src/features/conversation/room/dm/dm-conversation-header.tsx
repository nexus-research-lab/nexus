"use client";

import { memo, useRef, useState } from "react";
import {
  Bot,
  Compass,
  FolderTree,
  History,
  Info,
  type LucideIcon,
  MoreHorizontal,
} from "lucide-react";

import { UiActionMenu } from "@/shared/ui/action-menu";
import { UiAgentAvatar } from "@/shared/ui/avatar";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceConversationTabs } from "@/shared/ui/workspace/controls/workspace-conversation-tabs";
import { RoomSurfaceTabKey } from "@/types/conversation/room-surface";
import { RoomConversationView } from "@/types/conversation/conversation";
import { useSidebarStore } from "@/store/sidebar";
import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";

interface DmConversationHeaderProps {
  conversationId: string | null;
  conversations: RoomConversationView[];
  currentAgentName: string | null;
  currentAgentAvatar?: string | null;
  activeTab: RoomSurfaceTabKey;
  onReplayTour?: () => void;
  onChangeTab: (tab: RoomSurfaceTabKey) => void;
  onCloseActiveTab: () => void;
  onSelectConversation: (conversationId: string) => void;
  onCloseConversation: (conversationId: string) => Promise<void>;
  onCreateConversation?: (title?: string) => Promise<string | null>;
}

const DmConversationHeaderView = memo(({
  conversationId: conversationId,
  conversations,
  currentAgentName: currentAgentName,
  currentAgentAvatar: currentAgentAvatar,
  activeTab: activeTab,
  onReplayTour: onReplayTour,
  onChangeTab: onChangeTab,
  onCloseActiveTab: onCloseActiveTab,
  onSelectConversation: onSelectConversation,
  onCloseConversation: onCloseConversation,
  onCreateConversation: onCreateConversation,
}: DmConversationHeaderProps) => {
  const { t } = useI18n();
  const widePanelCollapsed = useSidebarStore((state) => state.wide_panel_collapsed);
  const moreButtonRef = useRef<HTMLButtonElement>(null);
  const [isMoreOpen, setIsMoreOpen] = useState(false);
  const headerTitle = currentAgentName?.trim() || t("room.untitled_dm");
  const dmTabs: {
    key: RoomSurfaceTabKey;
    label: string;
    icon: LucideIcon;
    anchor?: string;
  }[] = [
    { key: "history", label: t("room.history"), icon: History, anchor: CONVERSATION_TOUR_ANCHORS.tab_history },
    { key: "workspace", label: t("room.workspace"), icon: FolderTree, anchor: CONVERSATION_TOUR_ANCHORS.tab_workspace },
    { key: "subagents", label: t("subagents.label"), icon: Bot },
    { key: "about", label: t("room.about"), icon: Info, anchor: CONVERSATION_TOUR_ANCHORS.tab_about },
  ];

  const conversationTabs = (
    <WorkspaceConversationTabs
      conversations={conversations}
      conversationId={conversationId}
      onCloseConversation={onCloseConversation}
      onCreateConversation={onCreateConversation}
      onSelectConversation={onSelectConversation}
      tourAnchor={CONVERSATION_TOUR_ANCHORS.session_switcher}
    />
  );

  return (
    <WorkspaceSurfaceHeader
      activeTab={activeTab}
      density="compact"
      leading={<UiAgentAvatar avatar={currentAgentAvatar} className="h-full w-full border-0 shadow-none" name={headerTitle} size="sm" />}
      onChangeTab={onChangeTab}
      onDismissActiveTab={onCloseActiveTab}
      dismissActiveTabLabel={t("common.close")}
      tabsLeading={conversationTabs}
      tabs={dmTabs}
      title={widePanelCollapsed ? headerTitle : undefined}
      trailing={onReplayTour ? (
        <div className="flex items-center">
          <button
            ref={moreButtonRef}
            aria-haspopup="menu"
            aria-label={t("common.more_actions")}
            className="inline-flex h-7 w-7 items-center justify-center rounded-full text-(--icon-default) transition-[background,color] hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
            onClick={() => setIsMoreOpen((prev) => !prev)}
            title={t("common.more_actions")}
            type="button"
          >
            <MoreHorizontal className="h-4 w-4" />
          </button>
          <UiActionMenu
            anchorRef={moreButtonRef}
            ariaLabel={t("common.more_actions")}
            isOpen={isMoreOpen}
            items={[
              {
                value: "guide",
                label: t("common.view_guide"),
                icon: <Compass className="h-4 w-4 text-(--icon-muted)" />,
              },
            ]}
            onClose={() => setIsMoreOpen(false)}
            onSelect={(value) => {
              if (value === "guide") {
                onReplayTour?.();
              }
            }}
          />
        </div>
      ) : undefined}
    />
  );
});

DmConversationHeaderView.displayName = "DmConversationHeaderView";

export function DmConversationHeader(props: DmConversationHeaderProps) {
  return <DmConversationHeaderView {...props} />;
}
