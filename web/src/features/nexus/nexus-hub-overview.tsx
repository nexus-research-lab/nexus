import { Link2, Network, PlusCircle, Sparkles, Users } from "lucide-react";
import { useMemo } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";

interface NexusHubOverviewProps {
  agents: Agent[];
  conversations: Conversation[];
  conversation_id?: string;
}

interface RecentRoomItem {
  agent_id: string;
  agent_name: string;
  latest_conversation: Conversation;
  conversation_count: number;
}

function formatRelativeConversationCount(conversation_count: number): string {
  if (conversation_count <= 1) {
    return "1 条对话";
  }
  return `${conversation_count} 条对话`;
}

export function NexusHubOverview({
  agents,
  conversations,
  conversation_id,
}: NexusHubOverviewProps) {
  const navigate = useNavigate();

  const recent_rooms = useMemo<RecentRoomItem[]>(() => {
    const grouped = new Map<string, Conversation[]>();

    conversations.forEach((conversation) => {
      const agent_id = conversation.agent_id;
      if (!agent_id) {
        return;
      }
      const current_group = grouped.get(agent_id) ?? [];
      current_group.push(conversation);
      grouped.set(agent_id, current_group);
    });

    return agents
      .map((agent) => {
        const room_conversations = grouped.get(agent.agent_id) ?? [];
        if (room_conversations.length === 0) {
          return null;
        }

        const latest_conversation = [...room_conversations].sort(
          (left, right) => right.last_activity_at - left.last_activity_at,
        )[0];

        return {
          agent_id: agent.agent_id,
          agent_name: agent.name,
          latest_conversation,
          conversation_count: room_conversations.length,
        };
      })
      .filter((item): item is RecentRoomItem => item !== null)
      .sort(
        (left, right) =>
          right.latest_conversation.last_activity_at - left.latest_conversation.last_activity_at,
      )
      .slice(0, 5);
  }, [agents, conversations]);

  return (
    <div className="grid flex-1 gap-4 xl:grid-cols-[1.2fr_0.8fr]">
      <section className="workspace-card flex min-h-[440px] flex-col rounded-[30px] px-6 py-6">
        <div className="flex items-start justify-between gap-4">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/46">
              System Orchestration
            </p>
            <h2 className="mt-2 text-[28px] font-black tracking-[-0.04em] text-slate-950/90">
              Nexus 负责组织成员、编排协作、生成 room
            </h2>
            <p className="mt-3 max-w-2xl text-sm leading-6 text-slate-700/62">
              这里不是某个成员的聊天页，而是系统级协作入口。更合适的动作包括创建协作空间、整理成员关系、决定谁加入 room，以及恢复一条长期的 Nexus 对话。
            </p>
          </div>

          <div className="workspace-chip rounded-full px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-slate-700/56">
            {conversation_id ? `Conversation ${conversation_id}` : "Nexus Entry"}
          </div>
        </div>

        <div className="mt-6 grid gap-3 md:grid-cols-3">
          <button
            className="workspace-card group rounded-[24px] px-4 py-4 text-left transition hover:bg-white/20"
            onClick={() => navigate(AppRouteBuilders.contacts())}
            type="button"
          >
            <Users className="h-5 w-5 text-slate-900/76" />
            <p className="mt-3 text-sm font-semibold text-slate-950/86">挑选成员</p>
            <p className="mt-1 text-xs leading-5 text-slate-700/58">
              先浏览联系人网络，决定由哪些成员加入新 room。
            </p>
          </button>

          <button
            className="workspace-card group rounded-[24px] px-4 py-4 text-left transition hover:bg-white/20"
            onClick={() => navigate(AppRouteBuilders.launcher())}
            type="button"
          >
            <PlusCircle className="h-5 w-5 text-slate-900/76" />
            <p className="mt-3 text-sm font-semibold text-slate-950/86">从首页启动</p>
            <p className="mt-1 text-xs leading-5 text-slate-700/58">
              回到 launcher，通过统一入口开始一次新的协作。
            </p>
          </button>

          <div className="workspace-card rounded-[24px] px-4 py-4">
            <Sparkles className="h-5 w-5 text-slate-900/76" />
            <p className="mt-3 text-sm font-semibold text-slate-950/86">系统动作建议</p>
            <p className="mt-1 text-xs leading-5 text-slate-700/58">
              创建 room、邀请成员、整理上下文，都是这里应承载的事情。
            </p>
          </div>
        </div>

        <div className="mt-6 grid gap-4 md:grid-cols-[0.92fr_1.08fr]">
          <div className="workspace-card rounded-[26px] px-5 py-5">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-slate-700/50">
              <Network className="h-4 w-4" />
              Nexus 应该做什么
            </div>
            <ul className="mt-4 space-y-3 text-sm leading-6 text-slate-700/64">
              <li>创建一个新的协作空间，并决定它属于哪类任务。</li>
              <li>从联系人网络里挑选成员，构成 1v1 或多人 room。</li>
              <li>恢复历史 Nexus conversation，继续系统级组织工作。</li>
            </ul>
          </div>

          <div className="workspace-card rounded-[26px] px-5 py-5">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-slate-700/50">
              <Link2 className="h-4 w-4" />
              当前编排视角
            </div>
            <dl className="mt-4 grid gap-3 sm:grid-cols-3">
              <div>
                <dt className="text-[11px] uppercase tracking-[0.12em] text-slate-700/46">
                  可用成员
                </dt>
                <dd className="mt-1 text-xl font-semibold text-slate-950/86">{agents.length}</dd>
              </div>
              <div>
                <dt className="text-[11px] uppercase tracking-[0.12em] text-slate-700/46">
                  活跃 room
                </dt>
                <dd className="mt-1 text-xl font-semibold text-slate-950/86">{recent_rooms.length}</dd>
              </div>
              <div>
                <dt className="text-[11px] uppercase tracking-[0.12em] text-slate-700/46">
                  Nexus 对话
                </dt>
                <dd className="mt-1 text-xl font-semibold text-slate-950/86">
                  {conversation_id ? "1" : "0"}
                </dd>
              </div>
            </dl>
          </div>
        </div>
      </section>

      <aside className="workspace-card flex flex-col rounded-[30px] px-6 py-6">
        <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/46">
          Rooms
        </p>
        <p className="mt-2 text-sm leading-6 text-slate-700/60">
          从系统层面继续最近活跃的协作空间，而不是在 room 页里承担全局编排。
        </p>

        <div className="mt-5 space-y-3">
          {recent_rooms.length ? (
            recent_rooms.map((room) => (
              <button
                key={room.agent_id}
                className="workspace-card w-full rounded-[24px] px-4 py-4 text-left transition hover:bg-white/20"
                onClick={() =>
                  navigate(
                    AppRouteBuilders.room_conversation(
                      room.agent_id,
                      room.latest_conversation.session_key,
                    ),
                  )
                }
                type="button"
              >
                <div className="flex items-center justify-between gap-3">
                  <p className="truncate text-sm font-semibold text-slate-950/86">
                    {room.agent_name}
                  </p>
                  <span className="text-[11px] text-slate-700/48">
                    {formatRelativeConversationCount(room.conversation_count)}
                  </span>
                </div>
                <p className="mt-2 truncate text-xs text-slate-700/58">
                  {room.latest_conversation.title || "未命名对话"}
                </p>
              </button>
            ))
          ) : (
            <div className="workspace-card rounded-[24px] px-4 py-4 text-sm leading-6 text-slate-700/60">
              当前还没有可恢复的 room。你可以先去联系人页挑选成员，或回到 launcher 开始第一条协作。
            </div>
          )}
        </div>
      </aside>
    </div>
  );
}
