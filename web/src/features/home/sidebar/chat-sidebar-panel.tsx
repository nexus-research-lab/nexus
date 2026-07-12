import { MessageSquarePlus, Plus } from "lucide-react";
import { memo } from "react";

import { CreateRoomDialog } from "@/features/conversation/room/members/create-room-dialog";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { SidebarEmptyGuide } from "@/shared/ui/sidebar/sidebar-empty-guide";
import { SIDEBAR_TOUR_ANCHORS } from "@/features/onboarding/tours/sidebar-navigation-tour";

import {
  ConversationRow,
  SidebarListLoadingRows,
  SidebarSearchField,
} from "./sidebar-list-rows";
import { useChatSidebarController } from "./use-chat-sidebar-controller";

export const ChatSidebarPanelContent = memo(function ChatSidebarPanelContent() {
  const { t } = useI18n();
  const controller = useChatSidebarController({
    untitledRoomLabel: t("home.untitled_room"),
  });
  const emptyCopy = controller.directory.hasAgents
    ? {
      action: t("home.rooms_empty_action"),
      description: t("home.rooms_empty_description"),
      onAction: controller.create.open,
    }
    : {
      action: t("home.rooms_empty_no_agents_action"),
      description: t("home.rooms_empty_no_agents_description"),
      onAction: controller.navigation.openContacts,
    };

  return (
    <div
      className="flex min-h-0 flex-1 flex-col"
      data-tour-anchor={SIDEBAR_TOUR_ANCHORS.chat_list}
    >
      <SidebarSearchField
        action={(
          <button
            className="flex h-9 w-9 items-center justify-center rounded-[12px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_76%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_70%,transparent)] text-(--icon-muted) transition-[background,color,transform] duration-(--motion-duration-fast) hover:-translate-y-[1px] hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
            onClick={controller.create.open}
            title={t("home.create_room")}
            type="button"
          >
            <Plus className="h-4 w-4" />
          </button>
        )}
        onChange={controller.list.setQuery}
        placeholder={t("sidebar.search_conversations")}
        value={controller.list.query}
      />

      {controller.list.isLoading ? (
        <SidebarListLoadingRows />
      ) : (
        <div className="flex min-h-0 flex-1 flex-col gap-0.5 px-2 pb-2">
          {controller.list.items.length > 0 ? (
            controller.list.items.map((item) => (
              <ConversationRow
                isActive={controller.list.isItemActive(item)}
                item={item}
                key={item.id}
                onClick={() => controller.list.openConversation(item)}
                onDelete={item.canDelete ? () => controller.deletion.request(item) : undefined}
              />
            ))
          ) : (
            <SidebarEmptyGuide
              actionLabel={emptyCopy.action}
              description={emptyCopy.description}
              icon={MessageSquarePlus}
              onAction={emptyCopy.onAction}
              title={controller.list.query
                ? t("sidebar.no_matching_conversations")
                : t("home.rooms_empty_title")}
            />
          )}
        </div>
      )}

      <ConfirmDialog
        confirmText={t("common.delete")}
        isOpen={controller.deletion.target !== null}
        message={t("home.delete_message", {
          name: controller.deletion.target?.name ?? "",
        })}
        onCancel={controller.deletion.cancel}
        onConfirm={controller.deletion.confirm}
        title={t("home.delete_confirm")}
        variant="danger"
      />

      <CreateRoomDialog
        agents={controller.directory.agents.map((agent) => ({
          agent_id: agent.id,
          avatar: agent.avatar,
          name: agent.name,
        }))}
        isCreating={controller.create.isCreating}
        isOpen={controller.create.isOpen}
        onCancel={controller.create.cancel}
        onConfirm={(submission) => {
          void controller.create.submit(submission);
        }}
      />
    </div>
  );
});
