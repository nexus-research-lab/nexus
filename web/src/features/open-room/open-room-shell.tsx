"use client";

import { ReactNode, useMemo, useState } from "react";
import { Activity, Bot, CheckCircle2, Megaphone, Send, Sparkles, Wrench } from "lucide-react";

import { cn, formatRelativeTime } from "@/lib/utils";
import { RoomActionEnvelope, RoomArtifactRecord, RoomEventRecord, RoomMemberRecord, RoomRuntimeView, RoomTaskRecord } from "@/types";

interface OpenRoomShellProps {
  view: RoomRuntimeView;
  is_loading: boolean;
  error: string | null;
  on_refresh: () => Promise<unknown>;
  on_start: () => Promise<unknown>;
  on_tick: () => Promise<unknown>;
  on_run_phase: () => Promise<unknown>;
  on_run_until_finished: () => Promise<unknown>;
  on_post_message: (params: {
    content: string;
    scope?: "broadcast" | "direct" | "group" | "system";
    sender_member_id?: string | null;
    target_member_ids?: string[];
    metadata?: Record<string, any>;
  }) => Promise<unknown>;
  on_post_action: (params: RoomActionEnvelope) => Promise<unknown>;
}

const STATUS_LABEL: Record<string, string> = {
  listening: "Listening",
  speaking: "Speaking",
  thinking: "Thinking",
  working: "Working",
  waiting: "Waiting",
  blocked: "Blocked",
  done: "Done",
};

const EVENT_TONE: Record<string, string> = {
  room_started: "border-sky-300/40 bg-sky-500/10",
  message: "border-white/60 bg-white/38",
  task_assigned: "border-amber-300/40 bg-amber-400/12",
  workspace_event: "border-emerald-300/40 bg-emerald-400/12",
  artifact_created: "border-fuchsia-300/40 bg-fuchsia-400/10",
  help_requested: "border-rose-300/40 bg-rose-400/12",
  member_blocked: "border-rose-300/40 bg-rose-400/12",
  task_closed: "border-slate-300/50 bg-slate-500/10",
};

export function OpenRoomShell({
  view,
  is_loading,
  error,
  on_refresh,
  on_start,
  on_tick,
  on_run_phase,
  on_run_until_finished,
  on_post_message,
  on_post_action,
}: OpenRoomShellProps) {
  const [broadcast, set_broadcast] = useState("");
  const [messageScope, setMessageScope] = useState<"broadcast" | "direct">("broadcast");
  const [messageTargetId, setMessageTargetId] = useState("");
  const [taskTitle, set_taskTitle] = useState("");
  const [taskSummary, set_taskSummary] = useState("");
  const [taskAssigneeId, set_taskAssigneeId] = useState("");

  const agentMembers = useMemo(
    () => view.members.filter((member) => member.member_type === "agent" && member.member_agent_id),
    [view.members],
  );
  const timeline = useMemo(
    () => [...view.events].sort((left, right) => left.created_at.localeCompare(right.created_at)),
    [view.events],
  );
  const phase = String(view.room.runtime_state?.phase ?? "setup");

  const handleSendBroadcast = async () => {
    const content = broadcast.trim();
    if (!content) {
      return;
    }
    await on_post_message({
      content,
      scope: messageScope,
      target_member_ids: messageScope === "direct" && messageTargetId ? [messageTargetId] : [],
      metadata: { source: "user", room_mode: "open", directed: messageScope === "direct" },
    });
    set_broadcast("");
  };

  const handleAssignTask = async () => {
    if (!taskAssigneeId || !taskTitle.trim()) {
      return;
    }
    await on_post_action({
      action_type: "assign_task",
      target_member_id: taskAssigneeId,
      payload: {
        title: taskTitle.trim(),
        summary: taskSummary.trim(),
      },
    });
    set_taskTitle("");
    set_taskSummary("");
  };

  return (
    <div className="relative flex min-h-0 flex-1 flex-col px-4 py-4 sm:px-6 sm:py-6">
      <div className="pointer-events-none absolute left-[8%] top-[12%] h-40 w-40 rounded-full bg-[radial-gradient(circle,rgba(255,216,157,0.28),transparent_70%)] blur-3xl" />
      <div className="pointer-events-none absolute bottom-[8%] right-[10%] h-44 w-44 rounded-full bg-[radial-gradient(circle,rgba(137,196,255,0.22),transparent_72%)] blur-3xl" />
      <section className="panel-surface relative flex min-h-0 flex-1 flex-col overflow-hidden rounded-[34px] px-4 py-4 sm:px-6 sm:py-6">
        <header className="relative z-10 flex flex-col gap-4 border-b border-white/55 pb-4 lg:flex-row lg:items-end lg:justify-between">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-slate-700/52">
              <span className="neo-pill rounded-full px-3 py-1">Meeting Room</span>
              <span>{view.room.mode}</span>
              <span>{phase}</span>
            </div>
            <h1 className="mt-3 text-[30px] font-black tracking-[-0.05em] text-slate-950/92">
              {view.room.name || view.room.id}
            </h1>
            <p className="mt-2 max-w-3xl text-sm leading-6 text-slate-700/65">
              {view.room.goal || "这是一个会议室式多智能体协作频道。主智能体主持流程，成员在各自 workspace 工作，再把动作、结果和文件变化回流到房间时间线。"}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <button className="neo-pill rounded-full px-4 py-2 text-sm font-semibold text-slate-900" onClick={() => void on_refresh()} type="button">刷新</button>
            <button className="rounded-full bg-slate-950 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50" disabled={is_loading || view.room.runtime_status !== "created"} onClick={() => void on_start()} type="button">启动会议</button>
          </div>
        </header>

        {error ? (
          <div className="neo-card-flat mt-4 rounded-[22px] border border-rose-400/28 px-4 py-3 text-sm text-rose-900/80">
            {error}
          </div>
        ) : null}

        <div className="relative z-10 grid min-h-0 flex-1 gap-4 py-5 xl:grid-cols-[290px_minmax(0,1fr)_340px]">
          <aside className="neo-card min-h-0 rounded-[28px] p-4">
            <div className="flex items-center justify-between gap-3 border-b border-white/55 pb-3">
              <div>
                <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/48">Participants</p>
                <h2 className="mt-1 text-xl font-black tracking-[-0.04em] text-slate-950/92">席位与状态</h2>
              </div>
              <Bot className="h-5 w-5 text-slate-700/52" />
            </div>
            <div className="mt-4 space-y-3 overflow-y-auto pr-1 scrollbar-hide">
              {view.members.map((member) => (
                <ParticipantCard key={member.id} member={member} />
              ))}
            </div>
          </aside>

          <main className="neo-card flex min-h-0 flex-col rounded-[28px] p-4">
            <div className="flex items-center justify-between gap-3 border-b border-white/55 pb-3">
              <div>
                <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/48">Timeline</p>
                <h2 className="mt-1 text-[24px] font-black tracking-[-0.05em] text-slate-950/92">会议流</h2>
              </div>
              <div className="neo-card-flat rounded-full px-3 py-1.5 text-xs font-semibold uppercase tracking-[0.14em] text-slate-700/56">
                {timeline.length} events
              </div>
            </div>

            <div className="mt-4 min-h-0 flex-1 space-y-3 overflow-y-auto pr-1 scrollbar-hide">
              {timeline.map((event) => (
                <TimelineEvent key={event.id} event={event} members={view.members} />
              ))}
              {!timeline.length ? (
                <div className="flex min-h-[220px] items-center justify-center rounded-[24px] border border-dashed border-white/60 bg-white/20 px-6 text-center text-sm leading-6 text-slate-700/56">
                  会议室还没有开始流动。你可以先启动 room，或者直接广播一条消息把协作推起来。
                </div>
              ) : null}
            </div>

            <div className="mt-4 border-t border-white/55 pt-4">
              <div className="rounded-[24px] border border-white/60 bg-white/46 p-4">
                <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
                  <Megaphone className="h-4 w-4" />
                  <span>Broadcast</span>
                </div>
                <div className="mt-3 flex flex-wrap gap-2">
                  <select className="neo-inset rounded-full px-3 py-2 text-sm text-slate-900/86 outline-none" onChange={(event) => setMessageScope(event.target.value as "broadcast" | "direct")} value={messageScope}>
                    <option value="broadcast">全房间广播</option>
                    <option value="direct">@指定成员</option>
                  </select>
                  {messageScope === "direct" ? (
                    <select className="neo-inset rounded-full px-3 py-2 text-sm text-slate-900/86 outline-none" onChange={(event) => setMessageTargetId(event.target.value)} value={messageTargetId}>
                      <option value="">选择成员</option>
                      {agentMembers.map((member) => (
                        <option key={member.id} value={member.id}>{member.member_role || member.member_agent_id}</option>
                      ))}
                    </select>
                  ) : null}
                </div>
                <textarea className="neo-inset mt-3 min-h-[108px] w-full resize-y rounded-[18px] px-4 py-3 text-sm leading-6 text-slate-900/86 outline-none" onChange={(event) => set_broadcast(event.target.value)} placeholder="告诉整个会议室新的目标、上下文、补充资料，或者直接打断当前节奏……" value={broadcast} />
                <div className="mt-3 flex items-center justify-between gap-3">
                  <p className="text-xs leading-5 text-slate-700/56">
                    {messageScope === "direct"
                      ? "这条消息会定向送给选中的成员，同时保留在会议时间线里。"
                      : "这条消息会作为房间广播进入 timeline，所有成员都能看到。"}
                  </p>
                  <button className="inline-flex items-center gap-2 rounded-full bg-slate-950 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50" disabled={!broadcast.trim() || is_loading || (messageScope === "direct" && !messageTargetId)} onClick={() => void handleSendBroadcast()} type="button"><Send className="h-4 w-4" />发送</button>
                </div>
              </div>
            </div>
          </main>

          <aside className="neo-card min-h-0 rounded-[28px] p-4">
            <div className="flex items-center justify-between gap-3 border-b border-white/55 pb-3">
              <div>
                <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/48">Context / Control</p>
                <h2 className="mt-1 text-xl font-black tracking-[-0.04em] text-slate-950/92">编排面板</h2>
              </div>
              <Sparkles className="h-5 w-5 text-slate-700/52" />
            </div>

            <div className="mt-4 grid grid-cols-2 gap-2">
              <ControlButton disabled={is_loading || view.room.runtime_status === "created"} label="Tick" onClick={() => void on_tick()} />
              <ControlButton disabled={is_loading || view.room.runtime_status === "created"} label="Run Phase" onClick={() => void on_run_phase()} />
              <ControlButton disabled={is_loading || view.room.runtime_status === "created"} label="Run All" onClick={() => void on_run_until_finished()} />
              <div className="rounded-[18px] border border-white/60 bg-white/32 px-3 py-3 text-xs leading-5 text-slate-700/62">
                status: <span className="font-semibold text-slate-900/84">{view.room.runtime_status}</span>
              </div>
            </div>

            <section className="mt-4 rounded-[24px] border border-white/60 bg-white/34 p-4">
              <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
                <Wrench className="h-4 w-4" />
                <span>Assign Task</span>
              </div>
              <select className="neo-inset mt-3 w-full rounded-[16px] px-3 py-2 text-sm text-slate-900/86 outline-none" onChange={(event) => set_taskAssigneeId(event.target.value)} value={taskAssigneeId}>
                <option value="">选择成员</option>
                {agentMembers.map((member) => (
                  <option key={member.id} value={member.id}>{member.member_role || member.member_agent_id}</option>
                ))}
              </select>
              <input className="neo-inset mt-3 w-full rounded-[16px] px-3 py-2 text-sm text-slate-900/86 outline-none" onChange={(event) => set_taskTitle(event.target.value)} placeholder="任务标题" value={taskTitle} />
              <textarea className="neo-inset mt-3 min-h-[92px] w-full resize-y rounded-[16px] px-3 py-2.5 text-sm leading-6 text-slate-900/86 outline-none" onChange={(event) => set_taskSummary(event.target.value)} placeholder="任务说明、交付物、注意事项" value={taskSummary} />
              <button className="mt-3 inline-flex w-full items-center justify-center gap-2 rounded-full bg-slate-950 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50" disabled={!taskAssigneeId || !taskTitle.trim() || is_loading} onClick={() => void handleAssignTask()} type="button">
                <Send className="h-4 w-4" />
                分配到 workspace
              </button>
            </section>

            <section className="mt-4 space-y-3">
              <InfoList title="Tasks" items={view.tasks} renderItem={(task) => <TaskCard task={task} />} />
              <InfoList title="Artifacts" items={view.artifacts} renderItem={(artifact) => <ArtifactCard artifact={artifact} />} />
            </section>
          </aside>
        </div>
      </section>
    </div>
  );
}

function ParticipantCard({ member }: { member: RoomMemberRecord }) {
  return (
    <article className="rounded-[22px] border border-white/60 bg-white/34 p-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="truncate text-sm font-semibold text-slate-950/90">{member.member_role || member.member_agent_id || "User"}</h3>
          <p className="mt-1 text-xs uppercase tracking-[0.14em] text-slate-700/48">{member.member_source} · {member.member_type}</p>
        </div>
        <div className={cn("rounded-full px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em]", member.member_status === "blocked" ? "bg-rose-500/14 text-rose-700" : member.member_status === "done" ? "bg-emerald-500/14 text-emerald-700" : "bg-slate-900/8 text-slate-700")}>
          {STATUS_LABEL[member.member_status] || member.member_status}
        </div>
      </div>
      <p className="mt-3 text-xs leading-5 text-slate-700/58">{member.workspace_binding ? "workspace 已接入，会把文件更新回流到房间。" : "仅参与会议流，不绑定独立 workspace。"}</p>
    </article>
  );
}

function TimelineEvent({ event, members }: { event: RoomEventRecord; members: RoomMemberRecord[] }) {
  const actor = members.find((member) => member.id === event.actor_member_id);
  return (
    <article className={cn("rounded-[24px] border px-4 py-4", EVENT_TONE[event.event_type] ?? "border-white/60 bg-white/38")}>
      <div className="flex items-start gap-3">
        <div className="rounded-full bg-white/78 p-2"><Activity className="h-4 w-4 text-slate-900/78" /></div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-700/48">
            <span>{event.event_type.replace(/_/g, " ")}</span>
            <span>{formatRelativeTime(new Date(event.created_at).getTime())}</span>
            {actor?.member_role || actor?.member_agent_id ? <span>{actor.member_role || actor.member_agent_id}</span> : null}
          </div>
          <h3 className="mt-2 text-sm font-semibold text-slate-950/90">{event.title || "Room event"}</h3>
          <p className="mt-2 text-sm leading-6 text-slate-700/66">{event.body || "系统没有提供更多说明。"}</p>
        </div>
      </div>
    </article>
  );
}

function ControlButton({ label, disabled, onClick }: { label: string; disabled?: boolean; onClick: () => void }) {
  return <button className="rounded-[18px] border border-white/60 bg-white/40 px-3 py-3 text-sm font-semibold text-slate-900 transition hover:translate-y-[-1px] disabled:opacity-50" disabled={disabled} onClick={onClick} type="button">{label}</button>;
}

function InfoList<T>({ title, items, renderItem }: { title: string; items: T[]; renderItem: (item: T) => ReactNode }) {
  return (
    <section className="rounded-[24px] border border-white/60 bg-white/34 p-4">
      <div className="flex items-center justify-between gap-3">
        <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">{title}</p>
        <span className="text-xs text-slate-700/56">{items.length}</span>
      </div>
      <div className="mt-3 space-y-2">{items.length ? items.map(renderItem) : <p className="text-sm leading-6 text-slate-700/56">还没有内容。</p>}</div>
    </section>
  );
}

function TaskCard({ task }: { task: RoomTaskRecord }) {
  return <div className="rounded-[18px] border border-white/60 bg-white/40 px-3 py-3"><p className="text-sm font-semibold text-slate-950/88">{task.title}</p><p className="mt-1 text-xs uppercase tracking-[0.14em] text-slate-700/48">{task.status}</p><p className="mt-2 text-sm leading-6 text-slate-700/64">{task.summary || "暂无任务说明"}</p></div>;
}

function ArtifactCard({ artifact }: { artifact: RoomArtifactRecord }) {
  return <div className="rounded-[18px] border border-white/60 bg-white/40 px-3 py-3"><div className="flex items-center gap-2"><CheckCircle2 className="h-4 w-4 text-emerald-600" /><p className="text-sm font-semibold text-slate-950/88">{artifact.title}</p></div><p className="mt-1 text-xs uppercase tracking-[0.14em] text-slate-700/48">{artifact.kind}</p><p className="mt-2 text-sm leading-6 text-slate-700/64">{artifact.summary || artifact.workspace_ref || "暂无摘要"}</p></div>;
}
