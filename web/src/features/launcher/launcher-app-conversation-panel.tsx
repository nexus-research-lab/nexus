"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  AlertCircle,
  ArrowRight,
  Bot,
  Clock3,
  FolderKanban,
  LoaderCircle,
  MessageCircleMore,
  RotateCcw,
  Sparkles,
  Users,
  X,
} from "lucide-react";

import {
  HeroActionOrbShell,
  HeroActionPillShell,
  HeroInputShell,
  HeroSidePanelShell,
} from "@/features/launcher/launcher-glass-shell";
import { RoomConversationFeed } from "@/features/room-conversation/room-conversation-feed";
import { RoomScrollToLatestButton } from "@/features/room-conversation/room-scroll-to-latest-button";
import { cn, formatRelativeTime } from "@/lib/utils";
import { Agent } from "@/types/agent";
import { Message, UserMessage } from "@/types/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/permission";
import { WebSocketState } from "@/types/websocket";
import { ConversationWithOwner } from "@/types/launcher";

interface AppConversationAction {
  description: string;
  key: string;
  label: string;
  on_click: () => void;
}

interface LauncherAppConversationPanelProps {
  agents: Agent[];
  app_conversation_draft: string;
  app_conversation_messages: Message[];
  conversations_with_owners: ConversationWithOwner[];
  error: string | null;
  is_loading: boolean;
  ws_state: WebSocketState;
  on_create_room: () => void;
  on_clear_conversation: () => void;
  on_change_draft: (next_value: string) => void;
  on_close: () => void;
  on_delete_round: (round_id: string) => Promise<void>;
  on_open_agent_room: (agent_id: string) => void;
  on_open_conversation: (conversation_id: string, agent_id?: string) => void;
  on_open_contacts_page: () => void;
  on_permission_response: (payload: PermissionDecisionPayload) => boolean;
  on_regenerate_round: (round_id: string) => Promise<void>;
  on_stop_generation: () => void;
  on_submit: (next_prompt: string) => void;
  pending_permission: PendingPermission | null;
  suggested_room_title: string;
}

type NexusSurfaceTab = "about" | "chat" | "workspace";

const BOTTOM_THRESHOLD_PX = 80;

function group_messages_by_round(messages: Message[]): Map<string, Message[]> {
  const groups = new Map<string, Message[]>();

  for (const message of messages) {
    const round_id = message.round_id || message.message_id;
    const current_group = groups.get(round_id) ?? [];
    current_group.push(message);
    groups.set(round_id, current_group);
  }

  return groups;
}

export function LauncherAppConversationPanel({
                                               agents,
                                               app_conversation_draft,
                                               app_conversation_messages,
                                               conversations_with_owners,
                                               error,
                                               is_loading,
                                               ws_state,
                                               on_create_room,
                                               on_clear_conversation,
                                               on_change_draft,
                                               on_close,
                                               on_delete_round,
                                               on_open_agent_room,
                                               on_open_conversation,
                                               on_open_contacts_page,
                                               on_permission_response,
                                               on_regenerate_round,
                                               on_stop_generation,
                                               on_submit,
                                               pending_permission,
                                               suggested_room_title,
                                             }: LauncherAppConversationPanelProps) {
  const scroll_ref = useRef<HTMLDivElement>(null);
  const bottom_anchor_ref = useRef<HTMLDivElement>(null);
  const textarea_ref = useRef<HTMLTextAreaElement>(null);
  const should_follow_latest_ref = useRef(true);
  const [show_scroll_to_bottom, set_show_scroll_to_bottom] = useState(false);
  const [active_tab, set_active_tab] = useState<NexusSurfaceTab>("chat");

  const latest_user_message = [...app_conversation_messages]
    .reverse()
    .find((message): message is UserMessage => message.role === "user");
  const latest_user_prompt = latest_user_message?.content ?? "";
  const recent_room = conversations_with_owners[0] ?? null;
  const message_groups = useMemo(
    () => group_messages_by_round(app_conversation_messages),
    [app_conversation_messages],
  );
  const round_ids = useMemo(() => Array.from(message_groups.keys()), [message_groups]);

  const suggested_actions = useMemo(() => {
    const actions: AppConversationAction[] = [];
    const prompt = latest_user_prompt.trim().toLowerCase();
    const matched_agent = agents.find((agent) =>
      prompt && agent.name.toLowerCase().includes(prompt),
    ) ?? agents.find((agent) =>
      prompt && prompt.includes(agent.name.toLowerCase()),
    ) ?? null;

    if ((prompt.includes("恢复") || prompt.includes("继续") || prompt.includes("最近")) && recent_room) {
      actions.push({
        key: `room-${recent_room.conversation.session_key}`,
        label: "打开最近协作",
        description: `${recent_room.owner?.name ?? "未知成员"} · ${recent_room.conversation.title || "未命名对话"}`,
        on_click: () => on_open_conversation(
          recent_room.conversation.session_key,
          recent_room.conversation.agent_id,
        ),
      });
    }

    if (matched_agent) {
      actions.push({
        key: `agent-${matched_agent.agent_id}`,
        label: `和 ${matched_agent.name} 开始协作`,
        description: "直接进入对应 room。",
        on_click: () => on_open_agent_room(matched_agent.agent_id),
      });
    }

    if (prompt.includes("成员") || prompt.includes("联系人") || prompt.includes("邀请")) {
      actions.push({
        key: "contacts",
        label: "打开 Contacts",
        description: "先选成员。",
        on_click: on_open_contacts_page,
      });
    }

    if (prompt.includes("创建") || prompt.includes("新建") || prompt.includes("开始")) {
      actions.push({
        key: "create-room",
        label: "创建新协作",
        description: `预填标题：${suggested_room_title}`,
        on_click: on_create_room,
      });
    }

    if (!actions.length && recent_room) {
      actions.push({
        key: `fallback-room-${recent_room.conversation.session_key}`,
        label: "回到最近 room",
        description: `${recent_room.owner?.name ?? "未知成员"} · ${recent_room.conversation.title || "未命名对话"}`,
        on_click: () => on_open_conversation(
          recent_room.conversation.session_key,
          recent_room.conversation.agent_id,
        ),
      });
    }

    if (!actions.length && agents[0]) {
      actions.push({
        key: `fallback-agent-${agents[0].agent_id}`,
        label: `和 ${agents[0].name} 开始 1v1`,
        description: "直接进入对应 room。",
        on_click: () => on_open_agent_room(agents[0].agent_id),
      });
    }

    if (!actions.some((action) => action.key === "contacts")) {
      actions.push({
        key: "contacts-fallback",
        label: "浏览成员网络",
        description: "去 Contacts 看当前有哪些成员。",
        on_click: on_open_contacts_page,
      });
    }

    return actions.slice(0, 3);
  }, [
    agents,
    latest_user_prompt,
    on_create_room,
    on_open_agent_room,
    on_open_contacts_page,
    on_open_conversation,
    recent_room,
    suggested_room_title,
  ]);

  const connection_meta = useMemo(() => {
    if (is_loading) {
      return {
        label: "回复中",
        tone_class_name: "text-emerald-900/78",
      };
    }

    if (ws_state === "connected") {
      return {
        label: "已连接",
        tone_class_name: "text-slate-700/48",
      };
    }

    if (ws_state === "connecting") {
      return {
        label: "连接中",
        tone_class_name: "text-slate-700/48",
      };
    }

    return {
      label: "正在重连",
      tone_class_name: "text-amber-900/68",
    };
  }, [is_loading, ws_state]);

  const can_send_message = ws_state === "connected" && !is_loading && app_conversation_draft.trim().length > 0;

  const update_follow_state = useCallback(() => {
    const container = scroll_ref.current;
    if (!container) {
      return;
    }

    const distance_to_bottom = container.scrollHeight - container.scrollTop - container.clientHeight;
    const is_near_bottom = distance_to_bottom <= BOTTOM_THRESHOLD_PX;
    should_follow_latest_ref.current = is_near_bottom;
    set_show_scroll_to_bottom(!is_near_bottom);
  }, []);

  const scroll_to_bottom = useCallback((behavior: ScrollBehavior = "smooth") => {
    const container = scroll_ref.current;
    if (!container) {
      return;
    }

    should_follow_latest_ref.current = true;
    set_show_scroll_to_bottom(false);
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        container.scrollTo({
          top: container.scrollHeight,
          behavior,
        });
        bottom_anchor_ref.current?.scrollIntoView({
          behavior,
          block: "end",
        });
      });
    });
  }, []);

  useEffect(() => {
    if (!should_follow_latest_ref.current) {
      update_follow_state();
      return;
    }

    scroll_to_bottom(is_loading ? "auto" : "smooth");
  }, [app_conversation_messages, is_loading, scroll_to_bottom, update_follow_state]);

  useEffect(() => {
    const textarea = textarea_ref.current;
    if (!textarea) {
      return;
    }

    textarea.style.height = "0px";
    textarea.style.height = `${Math.min(textarea.scrollHeight, 144)}px`;
  }, [app_conversation_draft]);

  const handle_submit = useCallback(() => {
    if (!can_send_message) {
      return;
    }
    should_follow_latest_ref.current = true;
    set_show_scroll_to_bottom(false);
    on_submit(app_conversation_draft);
  }, [app_conversation_draft, can_send_message, on_submit]);

  const tab_items = useMemo(() => ([
    {
      key: "chat" as const,
      label: "Chat",
      icon: MessageCircleMore,
    },
    {
      key: "workspace" as const,
      label: "Workspace",
      icon: FolderKanban,
    },
    {
      key: "about" as const,
      label: "About",
      icon: Sparkles,
    },
  ]), []);

  const workspace_rooms = useMemo(
    () => conversations_with_owners.slice(0, 6),
    [conversations_with_owners],
  );

  const workspace_agents = useMemo(
    () => agents.slice(0, 6),
    [agents],
  );

  return (
    <HeroSidePanelShell class_name="h-full min-h-[620px] w-full max-w-[420px]">
      <div className="flex h-full min-h-0 flex-col">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <HeroActionPillShell class_name="w-fit">
              <span
                className="inline-flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-slate-800/72">
                <span className="h-3 w-3 rounded-full bg-[#7fe3a8]"/>
                Nexus
              </span>
              <span className={`text-[11px] font-medium ${connection_meta.tone_class_name}`}>
                {connection_meta.label}
              </span>
            </HeroActionPillShell>
          </div>

          <div className="flex shrink-0 items-center gap-2">
            <button
              aria-label="清空 App 对话"
              className="transition-transform duration-300 hover:-translate-y-0.5"
              onClick={on_clear_conversation}
              type="button"
            >
              <HeroActionOrbShell class_name="h-[46px] w-[46px]">
                <RotateCcw className="h-4 w-4 text-slate-900/76"/>
              </HeroActionOrbShell>
            </button>
            <button
              aria-label="关闭 App 对话"
              className="transition-transform duration-300 hover:-translate-y-0.5"
              onClick={on_close}
              type="button"
            >
              <HeroActionOrbShell class_name="h-[54px] w-[54px]">
                <X className="h-4 w-4 text-slate-900/76"/>
              </HeroActionOrbShell>
            </button>
          </div>
        </div>

        <div className="border-b border-white/10 px-3 pb-3 pt-4">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="truncate text-[22px] font-black tracking-[-0.04em] text-slate-950/90">
                Nexus
              </p>
              <p className="mt-1 flex items-center gap-2 text-[12px] text-slate-700/52">
                <Bot className="h-3.5 w-3.5 shrink-0" />
                <span className="truncate">
                  系统级协作入口 · 统一调度 agent、room 和共享资源
                </span>
              </p>
            </div>
          </div>

          <div className="mt-4 flex items-center gap-1">
            {tab_items.map((tab_item) => {
              const Icon = tab_item.icon;
              const is_active = active_tab === tab_item.key;

              return (
                <button
                  key={tab_item.key}
                  className={cn(
                    "inline-flex items-center gap-2 rounded-full px-3 py-1.5 text-[12px] font-semibold transition-all",
                    is_active
                      ? "bg-white/22 text-slate-950 shadow-[0_10px_20px_rgba(111,126,162,0.08)]"
                      : "text-slate-700/56 hover:bg-white/12 hover:text-slate-950",
                  )}
                  onClick={() => set_active_tab(tab_item.key)}
                  type="button"
                >
                  <Icon className="h-3.5 w-3.5" />
                  {tab_item.label}
                </button>
              );
            })}
          </div>
        </div>

        <div
          className="relative mt-2 flex min-h-0 flex-1 flex-col overflow-hidden rounded-[28px] bg-[rgba(255,255,255,0.05)] shadow-[inset_0_0_0_1px_rgba(255,255,255,0.1)]">
          {error ? (
            <div
              className="mx-3 mt-3 flex items-start gap-2 rounded-[20px] bg-[rgba(255,120,120,0.12)] px-3 py-2 text-xs leading-5 text-red-900/84 shadow-[inset_0_0_0_1px_rgba(255,120,120,0.14)]">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0"/>
              <span>{error}</span>
            </div>
          ) : null}

          {active_tab === "chat" ? (
            <>
              <div
                ref={scroll_ref}
                className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-1 pb-2 pt-3"
                onScroll={update_follow_state}
              >
                {app_conversation_messages.length ? (
                  <RoomConversationFeed
                    bottom_anchor_ref={bottom_anchor_ref}
                    compact
                    current_agent_name="Nexus"
                    is_last_round_pending_permission={pending_permission}
                    is_loading={is_loading}
                    is_mobile_layout
                    message_groups={message_groups}
                    on_delete_round={on_delete_round}
                    on_permission_response={on_permission_response}
                    on_regenerate_round={on_regenerate_round}
                    round_ids={round_ids}
                  />
                ) : (
                  <div className="flex h-full min-h-[320px] flex-col justify-between gap-4 px-1 pb-1 pt-2">
                    <div>
                      <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/44">
                        Ready
                      </p>
                      <p className="mt-3 text-base font-semibold text-slate-950/84">
                        告诉 Nexus，你接下来要推进什么。
                      </p>
                      <p className="mt-2 text-sm leading-6 text-slate-700/58">
                        它会帮你恢复协作、找成员，或者先把新的协作入口搭起来。
                      </p>
                    </div>

                    <div className="grid gap-3">
                      <button
                        className="rounded-[22px] bg-white/8 px-4 py-4 text-left shadow-[inset_0_0_0_1px_rgba(255,255,255,0.1)] transition hover:bg-white/14"
                        onClick={() => {
                          if (!recent_room) {
                            return;
                          }
                          on_open_conversation(recent_room.conversation.session_key, recent_room.conversation.agent_id);
                        }}
                        type="button"
                      >
                        <p className="text-sm font-semibold text-slate-950/84">恢复最近协作</p>
                        <p className="mt-1 text-xs leading-5 text-slate-700/58">
                          {recent_room
                            ? `${recent_room.owner?.name ?? "最近成员"} · ${recent_room.conversation.title || "未命名对话"}`
                            : "当前还没有最近协作可恢复。"}
                        </p>
                      </button>

                      <button
                        className="rounded-[22px] bg-white/8 px-4 py-4 text-left shadow-[inset_0_0_0_1px_rgba(255,255,255,0.1)] transition hover:bg-white/14"
                        onClick={on_create_room}
                        type="button"
                      >
                        <p className="text-sm font-semibold text-slate-950/84">创建新协作</p>
                        <p className="mt-1 text-xs leading-5 text-slate-700/58">
                          预填标题：{suggested_room_title}
                        </p>
                      </button>

                      <button
                        className="rounded-[22px] bg-white/8 px-4 py-4 text-left shadow-[inset_0_0_0_1px_rgba(255,255,255,0.1)] transition hover:bg-white/14"
                        onClick={on_open_contacts_page}
                        type="button"
                      >
                        <p className="text-sm font-semibold text-slate-950/84">去 Contacts 选择成员</p>
                        <p className="mt-1 text-xs leading-5 text-slate-700/58">
                          先选成员，再继续。
                        </p>
                      </button>
                    </div>
                  </div>
                )}
              </div>

              {show_scroll_to_bottom ? (
                <RoomScrollToLatestButton
                  is_loading={is_loading}
                  is_mobile_layout={true}
                  on_click={() => scroll_to_bottom("smooth")}
                />
              ) : null}
            </>
          ) : null}

          {active_tab === "workspace" ? (
            <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-4 py-4">
              <div className="space-y-4">
                <div className="rounded-[22px] border border-white/16 bg-white/8 px-4 py-4">
                  <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/46">
                    Recent Rooms
                  </p>
                  <div className="mt-3 space-y-2">
                    {workspace_rooms.length ? workspace_rooms.map(({ conversation, owner }) => (
                      <button
                        key={conversation.session_key}
                        className="flex w-full items-start gap-3 rounded-[18px] border border-transparent bg-white/6 px-3 py-3 text-left transition hover:bg-white/12"
                        onClick={() => on_open_conversation(conversation.session_key, conversation.agent_id)}
                        type="button"
                      >
                        <div className="workspace-chip mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-slate-900/76">
                          <MessageCircleMore className="h-3.5 w-3.5" />
                        </div>
                        <div className="min-w-0 flex-1">
                          <p className="truncate text-sm font-semibold text-slate-950/86">
                            {conversation.title?.trim() || "未命名协作"}
                          </p>
                          <p className="mt-1 text-[12px] text-slate-700/54">
                            {owner?.name ?? "未知成员"} · {formatRelativeTime(conversation.last_activity_at)}
                          </p>
                        </div>
                      </button>
                    )) : (
                      <p className="text-sm leading-6 text-slate-700/58">
                        当前还没有可恢复的协作。
                      </p>
                    )}
                  </div>
                </div>

                <div className="rounded-[22px] border border-white/16 bg-white/8 px-4 py-4">
                  <div className="flex items-center justify-between gap-3">
                    <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/46">
                      Available Agents
                    </p>
                    <span className="text-[11px] text-slate-700/46">{workspace_agents.length} 个成员</span>
                  </div>
                  <div className="mt-3 space-y-2">
                    {workspace_agents.map((agent) => (
                      <button
                        key={agent.agent_id}
                        className="flex w-full items-center gap-3 rounded-[18px] border border-transparent bg-white/6 px-3 py-3 text-left transition hover:bg-white/12"
                        onClick={() => on_open_agent_room(agent.agent_id)}
                        type="button"
                      >
                        <div className="workspace-chip flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-slate-900/76">
                          <Users className="h-3.5 w-3.5" />
                        </div>
                        <div className="min-w-0 flex-1">
                          <p className="truncate text-sm font-semibold text-slate-950/86">
                            {agent.name}
                          </p>
                          <p className="mt-1 text-[12px] text-slate-700/54">
                            进入 1v1 协作
                          </p>
                        </div>
                      </button>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          ) : null}

          {active_tab === "about" ? (
            <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-4 py-4">
              <div className="space-y-4">
                <div className="rounded-[22px] border border-white/16 bg-white/8 px-4 py-4">
                  <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/46">
                    Role
                  </p>
                  <p className="mt-3 text-base font-semibold text-slate-950/86">
                    Nexus 是系统级 command center
                  </p>
                  <p className="mt-2 text-sm leading-6 text-slate-700/58">
                    负责唤起协作、恢复最近工作、组织成员，并把你带到合适的 DM 或 Room。
                  </p>
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="rounded-[20px] border border-white/16 bg-white/8 px-4 py-4">
                    <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-700/46">
                      Members
                    </p>
                    <p className="mt-2 text-sm font-semibold text-slate-950/84">
                      {agents.length} 个可协作成员
                    </p>
                  </div>
                  <div className="rounded-[20px] border border-white/16 bg-white/8 px-4 py-4">
                    <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-700/46">
                      Recent Activity
                    </p>
                    <p className="mt-2 text-sm font-semibold text-slate-950/84">
                      {conversations_with_owners.length} 条最近协作
                    </p>
                  </div>
                </div>

                <div className="rounded-[22px] border border-white/16 bg-white/8 px-4 py-4">
                  <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/46">
                    Quick Actions
                  </p>
                  <div className="mt-3 grid gap-2">
                    {suggested_actions.map((action) => (
                      <button
                        key={action.key}
                        className="flex items-center justify-between gap-3 rounded-[18px] bg-white/6 px-3 py-3 text-left transition hover:bg-white/12"
                        onClick={action.on_click}
                        type="button"
                      >
                        <div className="min-w-0">
                          <p className="truncate text-sm font-semibold text-slate-950/84">
                            {action.label}
                          </p>
                          <p className="mt-1 truncate text-[12px] text-slate-700/54">
                            {action.description}
                          </p>
                        </div>
                        <ArrowRight className="h-3.5 w-3.5 shrink-0 text-slate-700/54" />
                      </button>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          ) : null}
        </div>

        <div className="mt-4">
          {active_tab === "chat" && app_conversation_messages.length && suggested_actions.length ? (
            <div className="flex flex-wrap gap-1 px-4 pb-1">
              {suggested_actions.map((action) => (
                <button
                  key={action.key}
                  className="inline-flex max-w-full items-center gap-1.5 rounded-full bg-white/10 px-3 py-1.5 text-[10px] font-medium w-25 truncate text-slate-800/78 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.08)] transition hover:bg-white/16"
                  onClick={action.on_click}
                  type="button"
                >
                  <span className="truncate">{action.label}</span>
                  <ArrowRight className="h-2.5 w-2.5 shrink-0"/>
                </button>
              ))}
            </div>
          ) : null}
          <HeroInputShell class_name={cn("w-full", active_tab !== "chat" && "opacity-70")}>
            <div className="flex min-w-0 flex-col gap-3">
              <div className="flex min-w-0 items-end gap-3">
                <textarea
                  ref={textarea_ref}
                  className="max-h-36 min-h-[28px] flex-1 resize-none overflow-y-auto bg-transparent text-sm leading-6 text-slate-900/84 outline-none placeholder:text-slate-700/42"
                  onChange={(event) => on_change_draft(event.target.value)}
                  onKeyDown={(event) => {
                    if (active_tab !== "chat") {
                      return;
                    }
                    if (event.key === "Escape" && is_loading) {
                      event.preventDefault();
                      on_stop_generation();
                      return;
                    }
                    if (event.key !== "Enter" || event.shiftKey) {
                      return;
                    }

                    event.preventDefault();
                    handle_submit();
                  }}
                  disabled={active_tab !== "chat"}
                  placeholder="告诉Nexus 你要推进什么..."
                  rows={1}
                  value={app_conversation_draft}
                />
                <button
                  className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-white/84 text-slate-900 shadow-[0_10px_20px_rgba(255,255,255,0.16)] transition-transform duration-300 hover:-translate-y-0.5 disabled:cursor-not-allowed disabled:opacity-45 disabled:hover:translate-y-0"
                  disabled={active_tab !== "chat" || (!is_loading && !can_send_message)}
                  onClick={active_tab !== "chat" ? undefined : is_loading ? on_stop_generation : handle_submit}
                  type="button"
                >
                  {is_loading ? (
                    <RotateCcw className="h-4 w-4"/>
                  ) : ws_state === "connected" ? (
                    <ArrowRight className="h-4 w-4"/>
                  ) : (
                    <LoaderCircle className="h-4 w-4 animate-spin"/>
                  )}
                </button>
              </div>
            </div>
          </HeroInputShell>
          <div className="flex items-center justify-between px-8 pb-1 gap-2 text-[11px] text-slate-700/44">
            <span>
              {active_tab === "chat"
                ? ws_state === "connected"
                  ? "Enter 发送，Shift + Enter 换行"
                  : "正在建立主对话连接..."
                : "切回 Chat 后继续和 Nexus 对话"}
            </span>
            <span>{app_conversation_messages.length ? `${app_conversation_messages.length} 条消息` : ""}</span>
          </div>
        </div>
      </div>
    </HeroSidePanelShell>
  );
}
