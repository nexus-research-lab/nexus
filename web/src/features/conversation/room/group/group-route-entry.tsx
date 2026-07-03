import { ArrowRight, MessageSquare, Sparkles, Users } from "lucide-react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceActionBar, WorkspaceActionCard } from "@/shared/ui/workspace/controls/workspace-action-bar";
import { Agent } from "@/types/agent/agent";
import { RoomConversationView } from "@/types/conversation/conversation";

const METRIC_ROW_CLASS_NAME = "flex items-center justify-between gap-3 border-b border-(--divider-subtle-color) py-3 last:border-b-0";

interface GroupRouteEntryProps {
  roomId?: string;
  conversationId?: string;
  agents: Agent[];
  conversations: RoomConversationView[];
}

export function GroupRouteEntry({
  roomId: roomId,
  conversationId: conversationId,
  agents,
  conversations,
}: GroupRouteEntryProps) {
  const { t } = useI18n();
  const navigate = useNavigate();
  const roomAgent = agents[0] ?? null;
  const recentRoomConversations = conversations
    .filter((conversation): conversation is RoomConversationView & { conversation_id: string } => (
      conversation.room_id === roomId && Boolean(conversation.conversation_id)
    ))
    .sort((left, right) => right.last_activity_at - left.last_activity_at)
    .slice(0, 4);

  return (
    <div className="grid flex-1 gap-4 xl:grid-cols-[0.92fr_1.08fr]">
      <section className="surface-radius-xl border border-(--divider-subtle-color) px-6 py-6">
        <h2 className="mt-2 text-[28px] font-black tracking-[-0.04em] text-(--text-strong)">
          {roomAgent ? roomAgent.name : t("room.route_empty_title")}
        </h2>

        <WorkspaceActionBar variant="cards">
          <WorkspaceActionCard
            description={t("room.route_back_launcher_description")}
            icon={<MessageSquare className="h-5 w-5 text-(--icon-strong)" />}
            onClick={() => navigate(AppRouteBuilders.launcher())}
            title={t("room.route_back_launcher")}
          />
          <WorkspaceActionCard
            description={t("room.route_browse_agents_description")}
            icon={<Users className="h-5 w-5 text-(--icon-strong)" />}
            onClick={() => navigate(AppRouteBuilders.contacts())}
            title={t("room.route_browse_agents")}
          />
          <WorkspaceActionCard
            icon={<Sparkles className="h-5 w-5 text-(--icon-strong)" />}
            onClick={() => navigate(AppRouteBuilders.launcher())}
            title={t("room.route_handoff")}
          />
        </WorkspaceActionBar>
      </section>

      <aside className="surface-radius-xl border border-(--divider-subtle-color) px-6 py-6">
        <div className="mt-4">
          <div className={METRIC_ROW_CLASS_NAME}>
            <p className="text-[11px] uppercase tracking-[0.12em] text-(--text-soft)">Room</p>
            <p className="text-sm font-semibold text-(--text-strong)">{roomId ?? "-"}</p>
          </div>

          <div className={METRIC_ROW_CLASS_NAME}>
            <p className="text-[11px] uppercase tracking-[0.12em] text-(--text-soft)">
              {t("room.route_conversation")}
            </p>
            <p className="text-sm font-semibold text-(--text-strong)">{conversationId ?? "-"}</p>
          </div>

          {recentRoomConversations.length ? (
            <div className="pt-3">
              <p className="text-[11px] uppercase tracking-[0.12em] text-(--text-soft)">{t("room.route_recent")}</p>
              <div className="mt-3 divide-y divide-(--divider-subtle-color)">
                {recentRoomConversations.map((conversation) => (
                  <button
                    key={conversation.conversation_id}
                    className="flex w-full items-center justify-between gap-3 py-3 text-left transition hover:text-(--text-strong)"
                    onClick={() =>
                      navigate(
                        AppRouteBuilders.roomConversation(
                          conversation.room_id ?? roomId ?? "",
                          conversation.conversation_id,
                        ),
                      )
                    }
                    type="button"
                  >
                    <span className="truncate text-sm text-(--text-default)">
                      {conversation.title || t("room.untitled_conversation")}
                    </span>
                    <span className="inline-flex h-8 w-8 items-center justify-center rounded-full text-(--icon-default) transition-colors hover:text-(--icon-strong)">
                      <ArrowRight className="h-4 w-4 shrink-0 text-(--icon-default)" />
                    </span>
                  </button>
                ))}
              </div>
            </div>
          ) : null}
        </div>
      </aside>
    </div>
  );
}
