"use client";

import { Fragment, RefObject, useState } from "react";

import { RoomContextPanel } from "@/features/conversation-shared/context/room-context-panel";
import { EditorPanel } from "@/features/conversation-shared/context/editor-panel";
import { DmChatPanel } from "@/features/dm-conversation/dm-chat-panel";
import { DmConversationHeader } from "@/features/dm-conversation/dm-conversation-header";
import {
  HOME_CHAT_PANEL_CLASS,
} from "@/lib/home-layout";
import { cn } from "@/lib/utils";
import { WorkspaceCanvasShell } from "@/shared/ui/workspace/workspace-canvas-shell";
import { WorkspaceInspectorShell } from "@/shared/ui/workspace/workspace-inspector-shell";
import { Agent } from "@/types/agent";
import { Conversation, ConversationSnapshotPayload, RoomConversationView } from "@/types/conversation";
import { RoomSurfaceTabKey } from "@/types/room-surface";
import { TodoItem } from "@/types/todo";
import { UpdateRoomParams } from "@/types/room";

import { RoomChatPanel } from "./room-chat-panel";
import { RoomChatErrorBoundary } from "./room-chat-error-boundary";

const ChatBoundary = import.meta.env.DEV ? RoomChatErrorBoundary : Fragment;
import { RoomConversationHeader } from "./room-conversation-header";
import { RoomConversationHistoryView } from "./room-conversation-history-view";
import { RoomWorkspaceView } from "./room-workspace-view";
import { RoomAgentAboutView } from "./room-agent-about-view";
import { RoomThreadContextProvider, useRoomThread, useThreadPanelData } from "./thread/room-thread-context";
import { ThreadDetailPanel } from "./thread-detail-panel";

interface RoomWorkspaceLayoutProps {
  current_agent: Agent;
  current_agent_id: string | null;
  current_room_type: string;
  room_id: string | null;
  room_description: string;
  room_members: Agent[];
  available_room_agents: Agent[];
  current_room_title: string;
  current_room_conversation: RoomConversationView | null;
  current_agent_conversation: Conversation | null;
  current_agent_session_key: string | null;
  conversation_id: string | null;
  current_room_conversations: RoomConversationView[];
  active_workspace_path: string | null;
  active_surface_tab: RoomSurfaceTabKey;
  initial_draft?: string | null;
  is_editor_open: boolean;
  editor_width_percent: number;
  is_resizing_editor: boolean;
  is_conversation_busy: boolean;
  current_todos: TodoItem[];
  workspace_split_ref: RefObject<HTMLElement | null>;
  on_change_surface_tab: (tab: RoomSurfaceTabKey) => void;
  on_select_agent: (agent_id: string) => void;
  on_edit_agent: (agent_id: string) => void;
  on_create_conversation: (title?: string) => Promise<string | null>;
  on_select_conversation: (conversation_id: string) => void;
  on_delete_conversation: (conversation_id: string) => Promise<string | null>;
  on_add_room_member: (agent_id: string) => Promise<void>;
  on_remove_room_member: (agent_id: string) => Promise<void>;
  on_update_room: (room_id: string, params: UpdateRoomParams) => Promise<void>;
  on_delete_room: () => Promise<void>;
  on_update_conversation_title: (conversation_id: string, title: string) => Promise<void>;
  on_open_workspace_file: (path: string | null) => void;
  on_close_workspace_pane: () => void;
  on_start_editor_resize: () => void;
  on_loading_change: (is_loading: boolean) => void;
  on_todos_change: (todos: TodoItem[]) => void;
  on_conversation_snapshot_change: (snapshot: ConversationSnapshotPayload) => void;
  on_room_event?: (event_type: string, data: import("@/types/agent-conversation").RoomEventPayload) => void;
}

/**
 * Room 双栏工作区布局
 *
 * 左侧：ConversationArea（flex-1），包含 Header + 消息流 + 输入框
 * 右侧：可折叠 DetailPanel（成员列表等） / Thread 面板
 */
export function RoomWorkspaceLayout(props: RoomWorkspaceLayoutProps) {
  const is_dm = props.current_room_type === "dm";

  // DM 不需要 Thread Context
  if (is_dm) {
    return <RoomWorkspaceLayoutInner {...props} />;
  }

  return (
    <RoomThreadContextProvider>
      <RoomWorkspaceLayoutInner {...props} />
    </RoomThreadContextProvider>
  );
}

function RoomWorkspaceLayoutInner({
  current_agent,
  current_agent_id,
  current_room_type,
  room_id,
  room_description,
  room_members,
  available_room_agents,
  current_room_title,
  current_room_conversation,
  current_agent_conversation,
  current_agent_session_key,
  conversation_id,
  current_room_conversations,
  active_workspace_path,
  active_surface_tab,
  initial_draft = null,
  is_editor_open,
  editor_width_percent,
  is_resizing_editor,
  is_conversation_busy,
  current_todos,
  workspace_split_ref,
  on_change_surface_tab,
  on_select_agent,
  on_edit_agent,
  on_create_conversation,
  on_select_conversation,
  on_delete_conversation,
  on_add_room_member,
  on_remove_room_member,
  on_update_room,
  on_delete_room,
  on_update_conversation_title,
  on_open_workspace_file,
  on_close_workspace_pane,
  on_start_editor_resize,
  on_loading_change,
  on_todos_change,
  on_conversation_snapshot_change,
  on_room_event,
}: RoomWorkspaceLayoutProps) {
  const [is_detail_panel_open, set_is_detail_panel_open] = useState(true);
  const is_dm = current_room_type === "dm";
  // 仅在 chat tab 且编辑器未打开时显示详情面板
  const show_detail_panel = is_detail_panel_open && !is_editor_open && active_surface_tab === "chat";

  const handle_open_workspace_file = (path: string) => on_open_workspace_file(path);

  return (
    <div className={cn("flex min-h-0 min-w-0 flex-1 gap-2")}>
      {/* 主内容区：编辑器 + 对话区 */}
      <section
        ref={workspace_split_ref}
        className={cn(
          "flex min-h-0 min-w-0 flex-1",
          is_editor_open && "gap-2",
          is_resizing_editor && "cursor-col-resize select-none",
        )}
      >
        <WorkspaceCanvasShell
          class_name={HOME_CHAT_PANEL_CLASS}
          is_joined_with_inspector={show_detail_panel}
        >
          <div className="flex min-h-0 min-w-0 flex-1 flex-col">
            {/* Header 常驻在 tab 内容上方，切 tab 不消失 */}
            {is_dm ? (
              <DmConversationHeader
                active_tab={active_surface_tab}
                conversation_count={current_room_conversations.length}
                current_agent_name={current_agent.name}
                is_detail_panel_open={is_detail_panel_open}
                is_loading={is_conversation_busy}
                on_change_tab={on_change_surface_tab}
                on_toggle_detail_panel={() => set_is_detail_panel_open((prev) => !prev)}
              />
            ) : (
              <RoomConversationHeader
                active_tab={active_surface_tab}
                conversations={current_room_conversations}
                conversation_id={conversation_id}
                current_room_title={current_room_title}
                is_detail_panel_open={is_detail_panel_open}
                is_loading={is_conversation_busy}
                on_change_tab={on_change_surface_tab}
                on_create_conversation={on_create_conversation}
                on_select_conversation={on_select_conversation}
                on_toggle_detail_panel={() => set_is_detail_panel_open((prev) => !prev)}
                room_members={room_members}
              />
            )}
            <div className="relative min-h-0 min-w-0 flex-1 overflow-hidden">
              {active_surface_tab === "chat" ? (
                is_dm ? (
                  <ChatBoundary>
                    <DmChatPanel
                      active_tab={active_surface_tab}
                      agent_id={current_agent.agent_id}
                      conversations={current_room_conversations}
                      current_agent_name={current_agent.name}
                      hide_header
                      initial_draft={initial_draft}
                      is_detail_panel_open={is_detail_panel_open}
                      on_change_tab={on_change_surface_tab}
                      on_conversation_snapshot_change={on_conversation_snapshot_change}
                      on_create_conversation={on_create_conversation}
                      on_loading_change={on_loading_change}
                      on_open_workspace_file={handle_open_workspace_file}
                      on_todos_change={on_todos_change}
                      on_toggle_detail_panel={() => set_is_detail_panel_open((prev) => !prev)}
                      session_key={current_agent_session_key}
                      session_title={current_agent_conversation?.title ?? null}
                    />
                  </ChatBoundary>
                ) : (
                  <ChatBoundary>
                    <RoomChatPanel
                      active_tab={active_surface_tab}
                      agent_id={current_agent.agent_id}
                      conversation_id={conversation_id}
                      conversations={current_room_conversations}
                      current_agent_name={current_agent.name}
                      current_room_title={current_room_title}
                      hide_header
                      initial_draft={initial_draft}
                      is_detail_panel_open={is_detail_panel_open}
                      on_change_tab={on_change_surface_tab}
                      on_conversation_snapshot_change={on_conversation_snapshot_change}
                      on_create_conversation={on_create_conversation}
                      on_loading_change={on_loading_change}
                      on_open_workspace_file={handle_open_workspace_file}
                      on_room_event={on_room_event}
                      on_select_conversation={on_select_conversation}
                      on_todos_change={on_todos_change}
                      on_toggle_detail_panel={() => set_is_detail_panel_open((prev) => !prev)}
                      room_id={room_id}
                      room_members={room_members}
                      session_title={current_room_conversation?.title ?? null}
                    />
                  </ChatBoundary>
                )
              ) : null}

              {active_surface_tab === "history" ? (
                <RoomConversationHistoryView
                  conversations={current_room_conversations}
                  conversation_id={conversation_id}
                  current_room_type={current_room_type}
                  on_create_conversation={on_create_conversation}
                  on_delete_conversation={on_delete_conversation}
                  on_select_conversation={on_select_conversation}
                  on_update_conversation_title={on_update_conversation_title}
                />
              ) : null}

              {active_surface_tab === "workspace" ? (
                <RoomWorkspaceView
                  active_workspace_path={active_workspace_path}
                  agent_id={current_agent.agent_id}
                  is_dm={is_dm}
                  room_members={room_members}
                  on_open_workspace_file={on_open_workspace_file}
                />
              ) : null}

              {active_surface_tab === "about" && is_dm ? (
                <RoomAgentAboutView agent={current_agent} />
              ) : null}
            </div>
          </div>
        </WorkspaceCanvasShell>

        <EditorPanel
          agent_id={current_agent.agent_id}
          class_name="hidden lg:flex"
          is_open={is_editor_open}
          on_close={on_close_workspace_pane}
          on_resize_start={on_start_editor_resize}
          path={active_workspace_path}
          width_percent={editor_width_percent}
        />
      </section>

      {/* 右侧可折叠详情面板 / Thread 面板 */}
      {
        is_dm ? (
          show_detail_panel ? (
            <WorkspaceInspectorShell>
              <RoomContextPanel
                active_conversation={current_room_conversation}
                agent={current_agent}
                available_room_agents={available_room_agents}
                current_agent_id={current_agent_id}
                current_room_type={current_room_type}
                room_id={room_id}
                room_name={current_room_title}
                room_description={room_description}
                is_conversation_busy={is_conversation_busy}
                on_add_room_member={on_add_room_member}
                on_edit_agent={on_edit_agent}
                on_remove_room_member={on_remove_room_member}
                on_update_room={on_update_room}
                on_delete_room={on_delete_room}
                on_select_agent={on_select_agent}
                room_conversations={current_room_conversations}
                room_members={room_members}
                todos={current_todos}
              />
            </WorkspaceInspectorShell>
          ) : null
        ) : (
          /* Room: 当 Thread 活跃或 detail panel 开启时显示右侧面板 */
          !is_editor_open && active_surface_tab === "chat" ? (
            <RoomInspectorSlot
              show_context_panel={is_detail_panel_open}
              current_agent={current_agent}
              current_agent_id={current_agent_id}
              current_room_type={current_room_type}
              room_id={room_id}
              room_description={room_description}
              current_room_title={current_room_title}
              current_room_conversation={current_room_conversation}
              current_room_conversations={current_room_conversations}
              available_room_agents={available_room_agents}
              room_members={room_members}
              is_conversation_busy={is_conversation_busy}
              current_todos={current_todos}
              on_add_room_member={on_add_room_member}
              on_edit_agent={on_edit_agent}
              on_remove_room_member={on_remove_room_member}
              on_update_room={on_update_room}
              on_delete_room={on_delete_room}
              on_select_agent={on_select_agent}
            />
          ) : null
        )
      }
    </div >
  );
}

// ─── Room Inspector Slot ──────────────────────────────────────────────────────
// 仅在 RoomThreadContextProvider 内部渲染，安全使用 useRoomThread

interface RoomInspectorSlotProps {
  show_context_panel: boolean;
  current_agent: Agent;
  current_agent_id: string | null;
  current_room_type: string;
  room_id: string | null;
  room_description: string;
  current_room_title: string;
  current_room_conversation: RoomConversationView | null;
  current_room_conversations: RoomConversationView[];
  available_room_agents: Agent[];
  room_members: Agent[];
  is_conversation_busy: boolean;
  current_todos: TodoItem[];
  on_add_room_member: (agent_id: string) => Promise<void>;
  on_edit_agent: (agent_id: string) => void;
  on_remove_room_member: (agent_id: string) => Promise<void>;
  on_update_room: (room_id: string, params: UpdateRoomParams) => Promise<void>;
  on_delete_room: () => Promise<void>;
  on_select_agent: (agent_id: string) => void;
}

function RoomInspectorSlot({
  show_context_panel,
  current_agent,
  current_agent_id,
  current_room_type,
  room_id,
  room_description,
  current_room_title,
  current_room_conversation,
  current_room_conversations,
  available_room_agents,
  room_members,
  is_conversation_busy,
  current_todos,
  on_add_room_member,
  on_edit_agent,
  on_remove_room_member,
  on_update_room,
  on_delete_room,
  on_select_agent,
}: RoomInspectorSlotProps) {
  const { active_thread, close_thread } = useRoomThread();
  const { thread_panel_data } = useThreadPanelData();
  const has_thread = !!(active_thread && thread_panel_data);

  // 既无 Thread 也无 Context 面板时隐藏
  if (!has_thread && !show_context_panel) return null;

  // Thread 面板打开时，替换 RoomContextPanel
  if (has_thread) {
    return (
      <WorkspaceInspectorShell class_name="!w-[360px] xl:!w-[clamp(360px,24vw,480px)] 2xl:!w-[clamp(380px,24vw,520px)]">
        <ThreadDetailPanel
          round_id={active_thread!.round_id}
          agent_id={active_thread!.agent_id}
          agent_name={thread_panel_data!.agent_name ?? active_thread!.agent_id}
          all_round_messages={thread_panel_data!.round_messages}
          on_close={close_thread}
          on_stop_message={thread_panel_data!.on_stop_message}
          on_open_workspace_file={thread_panel_data!.on_open_workspace_file}
          is_loading={thread_panel_data!.is_loading}
          layout="desktop"
        />
      </WorkspaceInspectorShell>
    );
  }

  return (
    <WorkspaceInspectorShell>
      <RoomContextPanel
        active_conversation={current_room_conversation}
        agent={current_agent}
        available_room_agents={available_room_agents}
        current_agent_id={current_agent_id}
        current_room_type={current_room_type}
        room_id={room_id}
        room_name={current_room_title}
        room_description={room_description}
        is_conversation_busy={is_conversation_busy}
        on_add_room_member={on_add_room_member}
        on_edit_agent={on_edit_agent}
        on_remove_room_member={on_remove_room_member}
        on_update_room={on_update_room}
        on_delete_room={on_delete_room}
        on_select_agent={on_select_agent}
        room_conversations={current_room_conversations}
        room_members={room_members}
        todos={current_todos}
      />
    </WorkspaceInspectorShell>
  );
}
