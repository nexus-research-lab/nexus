/**
 * INPUT: 聊天目录控制器、共享目录降级状态与 Room 创建/删除命令。
 * OUTPUT: 可搜索、可恢复且保留 stale 会话的聊天侧栏。
 * POS: Home 聊天目录视图；不直接发起 bootstrap 请求。
 */
import { CircleAlert, MessageCirclePlus, MessageSquarePlus } from "lucide-react";
import { memo } from "react";

import { CreateRoomDialog } from "@/features/conversation/room/members/create-room-dialog";
import { HomeDirectoryRefreshErrorNotice } from "@/features/home/home-directory-refresh-error-notice";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { SidebarEmptyGuide } from "@/shared/ui/sidebar/sidebar-empty-guide";
import {
  SidebarSearchAction,
  SidebarSearchField,
} from "@/shared/ui/form/sidebar-search-field";
import { SIDEBAR_TOUR_ANCHORS } from "@/features/onboarding/tours/sidebar-navigation-tour";

import {
  ConversationRow,
  SidebarListLoadingRows,
} from "./sidebar-list-rows";
import { getRoomDeletionRecoveryPresentation } from "./room-deletion-recovery";
import { useChatSidebarController } from "./use-chat-sidebar-controller";

export const ChatSidebarPanelContent = memo(function ChatSidebarPanelContent() {
  const { t } = useI18n();
  const controller = useChatSidebarController({
    untitledRoomLabel: t("home.untitled_room"),
  });
  const deletionRecovery = controller.deletion.failure
    ? getRoomDeletionRecoveryPresentation(controller.deletion.failure)
    : null;
  const deletionFailure = deletionRecovery
    ? {
        impact: t(deletionRecovery.failure.impactKey),
        nextStep: t(deletionRecovery.failure.nextStepKey),
        title: t(deletionRecovery.failure.titleKey),
      }
    : undefined;

  return (
    <div
      className="flex min-h-0 flex-1 flex-col"
      data-tour-anchor={SIDEBAR_TOUR_ANCHORS.chat_list}
    >
      <SidebarSearchField
        action={(
          <SidebarSearchAction
            className="leading-none"
            onClick={controller.create.open}
            title={t("home.create_room")}
          >
            <MessageCirclePlus />
          </SidebarSearchAction>
        )}
        onChange={controller.list.setQuery}
        placeholder={t("sidebar.search_conversations")}
        value={controller.list.query}
      />

      {controller.list.isLoading ? (
        <SidebarListLoadingRows />
      ) : (
        <div className="flex min-h-0 flex-1 flex-col gap-0.5 px-2 pb-2 max-lg:gap-1 max-lg:px-3">
          {controller.list.hasError && controller.list.hasLoaded ? (
            <HomeDirectoryRefreshErrorNotice
              className="mb-1"
              onRetry={controller.list.retry}
            />
          ) : null}
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
          ) : controller.list.hasError && !controller.list.hasLoaded ? (
            <SidebarEmptyGuide
              actionLabel={t("sidebar.retry")}
              description={t("sidebar.directory_load_failed_description")}
              impact={t("sidebar.directory_load_failed_impact")}
              icon={CircleAlert}
              nextStep={t("sidebar.directory_load_failed_next_step")}
              onAction={controller.list.retry}
              title={t("sidebar.directory_load_failed")}
            />
          ) : (
            <SidebarEmptyGuide
              actionLabel={controller.list.query
                ? undefined
                : t("home.rooms_empty_action")}
              description={controller.list.query
                ? t("sidebar.no_matching_conversations_description")
                : t("home.rooms_empty_description")}
              icon={MessageSquarePlus}
              onAction={controller.list.query
                ? undefined
                : controller.create.open}
              title={controller.list.query
                ? t("sidebar.no_matching_conversations")
                : t("home.rooms_empty_title")}
            />
          )}
        </div>
      )}

      <ConfirmDialog
        busy={controller.deletion.action !== null}
        confirmText={deletionRecovery
          ? t(deletionRecovery.confirmTextKey)
          : t("common.delete")}
        failure={deletionFailure}
        isOpen={controller.deletion.target !== null}
        message={t("home.delete_message", {
          name: controller.deletion.target?.name ?? "",
        })}
        onCancel={controller.deletion.cancel}
        onConfirm={controller.deletion.confirm}
        title={t("home.delete_confirm")}
        variant={deletionRecovery?.variant ?? "danger"}
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
