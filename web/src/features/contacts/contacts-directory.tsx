import { ArrowRight, Bot, Sparkles, Users } from "lucide-react";
import { useMemo } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";

interface ContactsDirectoryProps {
  agents: Agent[];
  conversations: Conversation[];
  selected_agent_id?: string;
}

function formatModelName(agent: Agent): string {
  return agent.options.model || "inherit";
}

export function ContactsDirectory({
  agents,
  conversations,
  selected_agent_id,
}: ContactsDirectoryProps) {
  const navigate = useNavigate();

  const conversations_by_agent = useMemo(() => {
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
    return grouped;
  }, [conversations]);

  const selected_agent =
    agents.find((agent) => agent.agent_id === selected_agent_id) ?? agents[0] ?? null;
  const selected_agent_conversations = selected_agent
    ? [...(conversations_by_agent.get(selected_agent.agent_id) ?? [])].sort(
        (left, right) => right.last_activity_at - left.last_activity_at,
      )
    : [];

  return (
    <div className="grid flex-1 gap-4 xl:grid-cols-[0.92fr_1.08fr]">
      <section className="workspace-card flex flex-col rounded-[30px] px-6 py-6">
        <div className="flex items-start justify-between gap-4">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/46">
              Contacts
            </p>
            <h2 className="mt-2 text-[28px] font-black tracking-[-0.04em] text-slate-950/90">
              选择成员
            </h2>
          </div>

          <div className="workspace-chip rounded-full px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-slate-700/56">
            {agents.length} Members
          </div>
        </div>

        <div className="mt-6 grid gap-3 md:grid-cols-2">
          {agents.length ? (
            agents.map((agent) => {
              const room_conversations = conversations_by_agent.get(agent.agent_id) ?? [];

              return (
                <button
                  key={agent.agent_id}
                  className="workspace-card rounded-[24px] px-4 py-4 text-left transition hover:bg-white/20"
                  onClick={() => navigate(AppRouteBuilders.contact_profile(agent.agent_id))}
                  type="button"
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="workspace-chip flex h-10 w-10 items-center justify-center rounded-full text-slate-900/80">
                      <Bot className="h-4 w-4" />
                    </div>
                    <span className="text-[11px] uppercase tracking-[0.12em] text-slate-700/46">
                      {agent.status || "active"}
                    </span>
                  </div>
                  <p className="mt-4 text-sm font-semibold text-slate-950/86">{agent.name}</p>
                  <p className="mt-1 text-xs text-slate-700/56">{formatModelName(agent)}</p>
                  <div className="mt-4 flex items-center justify-between text-xs text-slate-700/56">
                    <span>{room_conversations.length} 条对话</span>
                    <span className="inline-flex items-center gap-1">
                      查看资料
                      <ArrowRight className="h-3.5 w-3.5" />
                    </span>
                  </div>
                </button>
              );
            })
          ) : (
            <div className="workspace-card rounded-[24px] px-4 py-4 text-sm leading-6 text-slate-700/60">
              当前还没有可浏览的成员。先从首页创建第一个成员。
            </div>
          )}
        </div>
      </section>

      <aside className="workspace-card flex flex-col rounded-[30px] px-6 py-6">
        {selected_agent ? (
          <>
            <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/46">
              <Users className="h-4 w-4" />
              成员资料
            </div>
            <div className="mt-4 workspace-card rounded-[26px] px-5 py-5">
              <p className="text-xl font-semibold text-slate-950/88">{selected_agent.name}</p>
              <p className="mt-2 text-sm text-slate-700/58">{formatModelName(selected_agent)}</p>
              <div className="mt-4 grid grid-cols-2 gap-3 text-sm">
                <div>
                  <p className="text-[11px] uppercase tracking-[0.12em] text-slate-700/46">
                    最近对话
                  </p>
                  <p className="mt-1 font-semibold text-slate-950/84">
                    {selected_agent_conversations.length}
                  </p>
                </div>
                <div>
                  <p className="text-[11px] uppercase tracking-[0.12em] text-slate-700/46">
                    技能状态
                  </p>
                  <p className="mt-1 font-semibold text-slate-950/84">
                    {selected_agent.options.skills_enabled ? "已启用" : "未启用"}
                  </p>
                </div>
              </div>
            </div>

            <div className="mt-4 grid gap-3">
              <button
                className="workspace-card rounded-[24px] px-4 py-4 text-left transition hover:bg-white/20"
                onClick={() => navigate(AppRouteBuilders.room(selected_agent.agent_id))}
                type="button"
              >
                <p className="text-sm font-semibold text-slate-950/86">发起 1v1 协作</p>
              </button>

              <button
                className="workspace-card rounded-[24px] px-4 py-4 text-left transition hover:bg-white/20"
                onClick={() => navigate(AppRouteBuilders.launcher_app(`帮我组织和 ${selected_agent.name} 的协作`))}
                type="button"
              >
                <p className="flex items-center gap-2 text-sm font-semibold text-slate-950/86">
                  <Sparkles className="h-4 w-4" />
                  交给Nexus
                </p>
              </button>
            </div>
          </>
        ) : (
          <div className="mt-4 workspace-card rounded-[24px] px-4 py-4 text-sm leading-6 text-slate-700/60">
            当前还没有成员资料可展示。先创建一个成员，Contacts 才会成为真正的成员网络页。
          </div>
        )}
      </aside>
    </div>
  );
}
