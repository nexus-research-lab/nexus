import { ArrowRight, Bot, Clock3, MessageCircleMore, Sparkles, Users } from "lucide-react";
import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { HOME_WORKSPACE_OBJECT_LIST_WIDTH_CLASS } from "@/lib/home-layout";
import { cn, formatRelativeTime } from "@/lib/utils";
import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";

interface ContactsDirectoryProps {
  agents: Agent[];
  conversations: Conversation[];
  on_open_direct_room: (agent_id: string) => void;
  selected_agent_id?: string;
}

function formatModelName(agent: Agent): string {
  return agent.options.model || "inherit";
}

export function ContactsDirectory({
  agents,
  conversations,
  on_open_direct_room,
  selected_agent_id,
}: ContactsDirectoryProps) {
  const navigate = useNavigate();
  const [active_tab, set_active_tab] = useState<"about" | "history">("about");

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
    <div className="flex min-h-0 flex-1 overflow-hidden">
      <aside className={cn(
        "hidden min-h-0 shrink-0 border-r border-white/18 bg-white/8 lg:flex lg:flex-col",
        HOME_WORKSPACE_OBJECT_LIST_WIDTH_CLASS,
      )}>
        <div className="px-4 pb-4 pt-4">
          <p className="text-[11px] font-semibold uppercase tracking-[0.22em] text-slate-700/44">
            Contacts
          </p>
          <p className="mt-1 text-[20px] font-black tracking-[-0.04em] text-slate-950/90">
            成员网络
          </p>
          <p className="mt-1 text-[12px] text-slate-700/54">
            {agents.length} 个成员
          </p>
        </div>

        <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-2 pb-4">
          <div className="space-y-1.5">
            {agents.map((agent) => {
              const room_conversations = conversations_by_agent.get(agent.agent_id) ?? [];
              const is_active = selected_agent?.agent_id === agent.agent_id;

              return (
                <button
                  key={agent.agent_id}
                  className={cn(
                    "group flex w-full items-start gap-3 rounded-[18px] px-3 py-3 text-left transition-all duration-300",
                    is_active
                      ? "border border-white/28 bg-white/20 shadow-[0_14px_24px_rgba(111,126,162,0.08)]"
                      : "border border-transparent hover:bg-white/12",
                  )}
                  onClick={() => navigate(AppRouteBuilders.contact_profile(agent.agent_id))}
                  type="button"
                >
                  <div className="workspace-chip mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-slate-900/76">
                    <Bot className="h-4 w-4" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-semibold text-slate-950/86">
                      {agent.name}
                    </p>
                    <p className="mt-1 text-[12px] text-slate-700/56">
                      {formatModelName(agent)}
                    </p>
                    <div className="mt-2 flex items-center gap-2 text-[11px] text-slate-700/48">
                      <Clock3 className="h-3.5 w-3.5" />
                      <span>{room_conversations.length} 条历史协作</span>
                    </div>
                  </div>
                </button>
              );
            })}

            {!agents.length ? (
              <div className="workspace-card rounded-[22px] px-4 py-4 text-sm leading-6 text-slate-700/60">
                当前还没有可浏览的成员。先从首页创建第一个成员。
              </div>
            ) : null}
          </div>
        </div>
      </aside>

      <section className="flex min-h-0 min-w-0 flex-1 flex-col">
        {selected_agent ? (
          <>
            <div className="border-b workspace-divider px-6 py-4 xl:px-8">
              <div className="flex min-w-0 items-center justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <div className="truncate text-[22px] font-black tracking-[-0.04em] text-slate-950/90">
                    {selected_agent.name}
                  </div>
                  <div className="mt-1 flex items-center gap-2 text-[12px] text-slate-700/52">
                    <Users className="h-3.5 w-3.5 shrink-0" />
                    <span className="truncate">
                      {selected_agent.options.skills_enabled ? "技能已启用" : "技能未启用"} · {formatModelName(selected_agent)}
                    </span>
                  </div>
                </div>

                <button
                  className="workspace-chip inline-flex items-center gap-2 rounded-full px-4 py-2 text-xs font-semibold text-slate-900/82"
                  onClick={() => on_open_direct_room(selected_agent.agent_id)}
                  type="button"
                >
                  发起 1v1 协作
                  <ArrowRight className="h-3.5 w-3.5" />
                </button>
              </div>

              <div className="mt-4 flex items-center gap-1">
                <button
                  className={`inline-flex items-center gap-2 rounded-full px-3 py-1.5 text-[12px] font-semibold transition-all ${
                    active_tab === "about"
                      ? "bg-white/22 text-slate-950 shadow-[0_10px_20px_rgba(111,126,162,0.08)]"
                      : "text-slate-700/56 hover:bg-white/12 hover:text-slate-950"
                  }`}
                  onClick={() => set_active_tab("about")}
                  type="button"
                >
                  <Sparkles className="h-3.5 w-3.5" />
                  About
                </button>
                <button
                  className={`inline-flex items-center gap-2 rounded-full px-3 py-1.5 text-[12px] font-semibold transition-all ${
                    active_tab === "history"
                      ? "bg-white/22 text-slate-950 shadow-[0_10px_20px_rgba(111,126,162,0.08)]"
                      : "text-slate-700/56 hover:bg-white/12 hover:text-slate-950"
                  }`}
                  onClick={() => set_active_tab("history")}
                  type="button"
                >
                  <MessageCircleMore className="h-3.5 w-3.5" />
                  History
                </button>
              </div>
            </div>

            <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-4 py-4 sm:px-6 xl:px-8">
              {active_tab === "about" ? (
                <div className="workspace-card max-w-3xl rounded-[26px] px-5 py-5">
                  <p className="text-xl font-semibold text-slate-950/88">{selected_agent.name}</p>
                  <p className="mt-2 text-sm text-slate-700/58">{formatModelName(selected_agent)}</p>
                  <div className="mt-5 grid gap-3 sm:grid-cols-2">
                    <div className="rounded-[18px] border border-white/18 bg-white/10 px-4 py-4">
                      <p className="text-[11px] uppercase tracking-[0.12em] text-slate-700/46">
                        历史协作
                      </p>
                      <p className="mt-2 text-sm font-semibold text-slate-950/84">
                        {selected_agent_conversations.length} 条
                      </p>
                    </div>
                    <div className="rounded-[18px] border border-white/18 bg-white/10 px-4 py-4">
                      <p className="text-[11px] uppercase tracking-[0.12em] text-slate-700/46">
                        技能状态
                      </p>
                      <p className="mt-2 text-sm font-semibold text-slate-950/84">
                        {selected_agent.options.skills_enabled ? "已启用" : "未启用"}
                      </p>
                    </div>
                  </div>
                </div>
              ) : (
                <div className="max-w-3xl space-y-2">
                  {selected_agent_conversations.length ? (
                    selected_agent_conversations.map((conversation) => (
                      <button
                        key={conversation.session_key}
                        className="flex w-full items-start gap-4 rounded-[20px] border border-white/16 bg-white/8 px-4 py-4 text-left transition-all duration-300 hover:bg-white/12"
                        onClick={() => on_open_direct_room(selected_agent.agent_id)}
                        type="button"
                      >
                        <div className="workspace-chip flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-slate-900/76">
                          <MessageCircleMore className="h-4 w-4" />
                        </div>
                        <div className="min-w-0 flex-1">
                          <p className="truncate text-sm font-semibold text-slate-950/86">
                            {conversation.title?.trim() || "未命名对话"}
                          </p>
                          <div className="mt-2 flex items-center gap-2 text-[12px] text-slate-700/52">
                            <Clock3 className="h-3.5 w-3.5" />
                            <span>{formatRelativeTime(conversation.last_activity_at)}</span>
                            <span>·</span>
                            <span>{conversation.message_count ?? 0} 条消息</span>
                          </div>
                        </div>
                      </button>
                    ))
                  ) : (
                    <div className="workspace-card rounded-[24px] px-4 py-4 text-sm leading-6 text-slate-700/60">
                      这个成员还没有可回看的历史协作。
                    </div>
                  )}
                </div>
              )}
            </div>
          </>
        ) : (
          <div className="flex min-h-0 flex-1 items-center justify-center px-6 py-6">
            <div className="workspace-card max-w-xl rounded-[24px] px-5 py-5 text-sm leading-6 text-slate-700/60">
              当前还没有成员资料可展示。先创建一个成员，Contacts 才会成为真正的成员网络页。
            </div>
          </div>
        )}
      </section>
    </div>
  );
}
