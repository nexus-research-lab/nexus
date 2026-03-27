"use client";

import {
  Clock3,
  FolderTree,
  MessageSquareText,
  PencilLine,
  Sparkles,
  Trash2,
} from "lucide-react";

import { formatRelativeTime } from "@/lib/utils";
import { WorkspaceInspectorSection } from "@/shared/ui/workspace-inspector-section";
import { WorkspaceInspectorShell } from "@/shared/ui/workspace-inspector-shell";
import { WorkspacePillButton } from "@/shared/ui/workspace-pill-button";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace-status-badge";
import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";
import { ContactsSkillsSection } from "@/features/contacts/contacts-skills-section";

interface ContactsProfilePanelProps {
  agent: Agent | null;
  conversations: Conversation[];
  status_label: string;
  status_tone: "active" | "running" | "idle" | "default";
  on_delete_agent: (agent_id: string) => void;
  on_edit_agent: (agent_id: string) => void;
  on_open_room: (agent_id: string) => void;
}

export function ContactsProfilePanel({
  agent,
  conversations,
  status_label,
  status_tone,
  on_delete_agent,
  on_edit_agent,
  on_open_room,
}: ContactsProfilePanelProps) {
  if (!agent) {
    return (
      <WorkspaceInspectorShell>
        <div className="flex min-h-full items-center justify-center px-8 text-center text-sm leading-7 text-slate-700/56">
          选择一个成员后，这里会显示配置、最近活动和快捷操作。
        </div>
      </WorkspaceInspectorShell>
    );
  }

  const latest_conversation = conversations[0] ?? null;
  const recent_items = conversations.slice(0, 3);

  return (
    <WorkspaceInspectorShell>
      <section className="border-b workspace-divider px-5 py-5">
        <div className="workspace-card rounded-[24px] px-5 py-5 text-center">
          <div className="flex items-center justify-between gap-3">
            <p className="text-[13px] font-semibold text-slate-700/60">当前成员</p>
            <WorkspaceStatusBadge label={status_label} tone={status_tone} />
          </div>

          <div className="workspace-chip mx-auto mt-5 flex h-24 w-24 items-center justify-center rounded-[26px] text-slate-950/92 shadow-[0_14px_28px_rgba(102,112,145,0.12)]">
            <span className="text-[30px] font-black tracking-[-0.06em]">
              {agent.name.slice(0, 2).toUpperCase()}
            </span>
          </div>
          <p className="mt-4 text-[18px] font-bold text-slate-950/90">{agent.name}</p>
          <p className="mt-1.5 text-[12px] font-semibold uppercase tracking-[0.16em] text-emerald-600/88">
            {agent.options.model || "inherit"}
          </p>

          <div className="mt-4 flex flex-wrap items-center justify-center gap-2">
            <span className="workspace-chip rounded-full px-3 py-1 text-[11px] font-semibold text-slate-700/76">
              {agent.options.permission_mode || "default"}
            </span>
            <span className="workspace-chip rounded-full px-3 py-1 text-[11px] font-semibold text-slate-700/76">
              {agent.options.skills_enabled ? "skills on" : "skills off"}
            </span>
          </div>
        </div>
      </section>

      <WorkspaceInspectorSection title="Details">
        <div className="grid gap-3">
          <div className="workspace-card rounded-[18px] px-4 py-3">
            <div className="flex items-center justify-between gap-3">
              <span className="text-[12px] text-slate-700/58">历史协作</span>
              <span className="text-[13px] font-semibold text-slate-950/84">{conversations.length} 条</span>
            </div>
          </div>
          <div className="workspace-card rounded-[18px] px-4 py-3">
            <div className="flex items-center justify-between gap-3">
              <span className="text-[12px] text-slate-700/58">最近活跃</span>
              <span className="text-[13px] font-semibold text-slate-950/84">
                {latest_conversation ? formatRelativeTime(latest_conversation.last_activity_at) : "暂无"}
              </span>
            </div>
          </div>
        </div>
      </WorkspaceInspectorSection>

      <WorkspaceInspectorSection icon={FolderTree} title="Workspace">
        <div className="workspace-card rounded-[18px] px-4 py-4">
          <p className="break-all text-[13px] leading-6 text-slate-700/80">{agent.workspace_path}</p>
        </div>
      </WorkspaceInspectorSection>

      <WorkspaceInspectorSection icon={Clock3} title="Recent Activity">
        <div className="space-y-3">
          {recent_items.length ? (
            recent_items.map((conversation) => (
              <div
                key={conversation.session_key}
                className="workspace-card rounded-[18px] px-4 py-3"
              >
                <p className="truncate text-[13px] font-semibold text-slate-950/88">
                  {conversation.title?.trim() || "未命名协作"}
                </p>
                <div className="mt-1 flex items-center gap-2 text-[11px] text-slate-700/56">
                  <Clock3 className="h-3.5 w-3.5" />
                  <span>{formatRelativeTime(conversation.last_activity_at)}</span>
                </div>
              </div>
            ))
          ) : (
            <div className="workspace-card rounded-[18px] px-4 py-4 text-[13px] leading-6 text-slate-700/60">
              这个成员还没有历史协作。
            </div>
          )}
        </div>
      </WorkspaceInspectorSection>

      <WorkspaceInspectorSection icon={Sparkles} title="Summary">
        <div className="space-y-3">
          <div className="workspace-card rounded-[18px] px-4 py-3">
            <div className="flex items-center gap-3 text-[13px] text-slate-700/78">
              <Sparkles className="h-4 w-4 text-sky-500" />
              <span>
                {agent.options.skills_enabled
                  ? `技能已启用 (${agent.options.installed_skills?.length ?? 0} 个自定义)`
                  : "技能未启用"}
              </span>
            </div>
          </div>
          <div className="workspace-card rounded-[18px] px-4 py-3">
            <div className="flex items-center gap-3 text-[13px] text-slate-700/78">
              <Clock3 className="h-4 w-4 text-emerald-500" />
              <span>
                {latest_conversation
                  ? `最近更新于 ${formatRelativeTime(latest_conversation.last_activity_at)}`
                  : "暂无最近活动"}
              </span>
            </div>
          </div>
        </div>
      </WorkspaceInspectorSection>

      {agent.options.skills_enabled && (
        <ContactsSkillsSection agent_id={agent.agent_id} />
      )}

      <section className="border-t workspace-divider px-5 py-5">
        <div className="flex flex-col gap-3">
          <WorkspacePillButton onClick={() => on_open_room(agent.agent_id)} variant="success">
            <MessageSquareText className="h-4 w-4" />
            发起协作
          </WorkspacePillButton>
          <div className="grid grid-cols-2 gap-3">
            <WorkspacePillButton onClick={() => on_edit_agent(agent.agent_id)}>
              <PencilLine className="h-4 w-4" />
              编辑
            </WorkspacePillButton>
            <WorkspacePillButton onClick={() => on_delete_agent(agent.agent_id)} variant="danger">
              <Trash2 className="h-4 w-4" />
              删除
            </WorkspacePillButton>
          </div>
        </div>
      </section>
    </WorkspaceInspectorShell>
  );
}
