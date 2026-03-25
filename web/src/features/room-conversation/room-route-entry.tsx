import { ArrowRight, MessageSquare, Sparkles, Users } from "lucide-react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";

interface RoomRouteEntryProps {
  room_id?: string;
  conversation_id?: string;
  agents: Agent[];
  conversations: Conversation[];
}

export function RoomRouteEntry({
  room_id,
  conversation_id,
  agents,
  conversations,
}: RoomRouteEntryProps) {
  const navigate = useNavigate();
  const room_agent = agents.find((agent) => agent.agent_id === room_id) ?? null;
  const recent_room_conversations = conversations
    .filter((conversation) => conversation.agent_id === room_id)
    .sort((left, right) => right.last_activity_at - left.last_activity_at)
    .slice(0, 4);

  return (
    <div className="grid flex-1 gap-4 xl:grid-cols-[0.92fr_1.08fr]">
      <section className="workspace-card rounded-[30px] px-6 py-6">
        <h2 className="mt-2 text-[28px] font-black tracking-[-0.04em] text-slate-950/90">
          {room_agent ? room_agent.name : "这个协作空间还没有对应成员"}
        </h2>

        <div className="mt-6 grid gap-3 md:grid-cols-3">
          <button
            className="workspace-card rounded-[24px] px-4 py-4 text-left transition hover:bg-white/20"
            onClick={() => navigate(AppRouteBuilders.launcher())}
            type="button"
          >
            <MessageSquare className="h-5 w-5 text-slate-900/78" />
            <p className="mt-3 text-sm font-semibold text-slate-950/86">回到 launcher</p>
            <p className="mt-1 text-xs leading-5 text-slate-700/58">
              从首页继续最近对话，或重新开始一次协作。
            </p>
          </button>

          <button
            className="workspace-card rounded-[24px] px-4 py-4 text-left transition hover:bg-white/20"
            onClick={() => navigate(AppRouteBuilders.contacts())}
            type="button"
          >
            <Users className="h-5 w-5 text-slate-900/78" />
            <p className="mt-3 text-sm font-semibold text-slate-950/86">浏览成员</p>
            <p className="mt-1 text-xs leading-5 text-slate-700/58">
              去 Contacts 选择合适成员，重新发起 1v1 或多人协作。
            </p>
          </button>

          <button
            className="workspace-card rounded-[24px] px-4 py-4 text-left transition hover:bg-white/20"
            onClick={() => navigate(AppRouteBuilders.launcher_app("帮我整理一个新的协作 room"))}
            type="button"
          >
            <Sparkles className="h-5 w-5 text-slate-900/78" />
            <p className="mt-3 text-sm font-semibold text-slate-950/86">交给Nexus</p>
          </button>
        </div>
      </section>

      <aside className="workspace-card rounded-[30px] px-6 py-6">
        <div className="mt-4 grid gap-3">
          <div className="workspace-card rounded-[24px] px-4 py-4">
            <p className="text-[11px] uppercase tracking-[0.12em] text-slate-700/46">Room</p>
            <p className="mt-1 text-sm font-semibold text-slate-950/86">{room_id ?? "-"}</p>
          </div>

          <div className="workspace-card rounded-[24px] px-4 py-4">
            <p className="text-[11px] uppercase tracking-[0.12em] text-slate-700/46">
              Conversation
            </p>
            <p className="mt-1 text-sm font-semibold text-slate-950/86">{conversation_id ?? "-"}</p>
          </div>

          {recent_room_conversations.length ? (
            <div className="workspace-card rounded-[24px] px-4 py-4">
              <p className="text-[11px] uppercase tracking-[0.12em] text-slate-700/46">Recent</p>
              <div className="mt-3 space-y-2">
                {recent_room_conversations.map((conversation) => (
                  <button
                    key={conversation.session_key}
                    className="flex w-full items-center justify-between gap-3 rounded-2xl bg-white/10 px-3 py-3 text-left transition hover:bg-white/16"
                    onClick={() =>
                      navigate(
                        AppRouteBuilders.room_conversation(
                          conversation.agent_id ?? room_id ?? "",
                          conversation.session_key,
                        ),
                      )
                    }
                    type="button"
                  >
                    <span className="truncate text-sm text-slate-900/84">
                      {conversation.title || "未命名对话"}
                    </span>
                    <ArrowRight className="h-4 w-4 shrink-0 text-slate-700/52" />
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
