import {
  CircleDot,
  Clock3,
  FileText,
  ListTree,
  Play,
  RadioTower,
  Settings2,
  Sparkles,
  Workflow,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

import { cn } from "@/lib/utils";

import {
  PHASE_LABELS,
  resolve_operation_tool_profile,
} from "../operation-tool-catalog";
import type { NexusOperationEvent } from "../operation-types";
import { ACTION_ICON, ACTION_TONE_CLASS } from "./operation-action-style";
import { build_nexus_tool_session_view } from "./nexus-tool-session";

export function NexusToolSurface({
  event,
  preview,
  related_events,
  target,
}: {
  event: NexusOperationEvent;
  preview: unknown;
  related_events: NexusOperationEvent[];
  target?: string | null;
}) {
  const profile = resolve_operation_tool_profile(event.tool_name, event.kind, event.surface);
  const ActionIcon = ACTION_ICON[profile.action];
  const session = build_nexus_tool_session_view({
    event,
    preview,
    related_events,
    target,
  });
  const output_lines = session.output_text.split("\n").filter(Boolean);
  const workflow_steps = build_workflow_steps(session);

  return (
    <div className="grid h-full min-h-[320px] min-w-0 grid-cols-[220px_minmax(0,1fr)] overflow-hidden bg-[#f4f6f8] text-(--text-default) max-md:grid-cols-1">
      <aside className="soft-scrollbar min-h-0 overflow-auto border-r border-(--divider-subtle-color) bg-[#eaf0f4]/88 p-3 max-md:hidden">
        <div className="rounded-[14px] border border-white/64 bg-white/64 p-3 shadow-[inset_0_1px_0_rgba(255,255,255,0.82)]">
          <div className="flex min-w-0 items-center gap-2.5">
            <span className="grid h-9 w-9 shrink-0 place-items-center rounded-[11px] border border-white/78 bg-[rgba(91,114,255,0.12)] text-[color:var(--primary)] shadow-[0_10px_24px_rgba(18,28,42,0.07)]">
              <Sparkles className="h-[18px] w-[18px]" />
            </span>
            <div className="min-w-0">
              <p className="truncate text-[13px] font-black text-(--text-strong)">快捷指令</p>
              <p className="truncate text-[10px] font-semibold text-(--text-soft)">{session.app_intent.group_label} · {profile.action_label}</p>
            </div>
          </div>
          <div className="mt-3 rounded-[10px] border border-(--divider-subtle-color) bg-white/58 px-2.5 py-2">
            <p className="truncate text-[10px] font-black text-(--text-strong)" title={session.tool_name}>{session.tool_name}</p>
            <p className="mt-1 line-clamp-2 text-[10px] font-semibold leading-4 text-(--text-soft)" title={session.display_target}>{session.display_target}</p>
          </div>
        </div>

        <div className="mt-3 px-1 text-[9px] font-black uppercase tracking-[0.12em] text-(--text-soft)">运行信息</div>
        <div className="mt-1.5 space-y-1">
          {session.sidebar_items.map((item, index) => (
            <SidebarMetric
              icon={sidebar_icon_for_index(index)}
              key={item.key}
              label={item.label}
              value={item.value}
            />
          ))}
        </div>

        <div className="mt-4 px-1 text-[9px] font-black uppercase tracking-[0.12em] text-(--text-soft)">最近记录</div>
        <div className="mt-1.5 space-y-1">
          {session.timeline.slice(-4).map((item, index) => (
            <div
              className="flex min-w-0 items-center gap-2 rounded-[9px] bg-white/50 px-2 py-1.5 text-[10px]"
              key={item.id}
              title={`${item.label} · ${item.phase_label}`}
            >
              <span className="grid h-5 w-5 shrink-0 place-items-center rounded-full bg-[rgba(47,184,132,0.12)] text-[9px] font-black text-[color:var(--success)]">
                {index + 1}
              </span>
              <span className="min-w-0">
                <span className="block truncate font-black text-(--text-strong)">{item.label}</span>
                <span className="block truncate font-semibold text-(--text-soft)">{item.phase_label}</span>
              </span>
            </div>
          ))}
        </div>
      </aside>

      <section className="flex min-h-0 min-w-0 flex-col">
        <header className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-3 border-b border-(--divider-subtle-color) bg-white/80 px-5 py-3">
          <div className="flex min-w-0 items-center gap-2.5">
            <span className={`inline-flex h-9 shrink-0 items-center gap-1.5 rounded-[11px] border px-2.5 text-[11px] font-black ${ACTION_TONE_CLASS[profile.action]}`}>
              <ActionIcon className="h-4 w-4" />
              {profile.action_label}
            </span>
            <div className="min-w-0">
              <h3 className="truncate text-[15px] font-black tracking-normal text-(--text-strong)" title={session.tool_name}>
                {session.tool_name}
              </h3>
              <p className="mt-0.5 truncate text-[10px] font-semibold text-(--text-soft)" title={session.display_target}>
                {session.app_intent.app_label} · {session.display_target}
              </p>
            </div>
          </div>
          <StatusPill phase={event.phase} />
        </header>

        <div className="soft-scrollbar min-h-0 flex-1 overflow-auto bg-[linear-gradient(180deg,#fbfcfd_0%,#eef3f7_100%)] px-6 py-5">
          <section className="mx-auto flex max-w-[960px] flex-col gap-3">
            <div className="rounded-[18px] border border-(--divider-subtle-color) bg-white/78 p-4 shadow-[0_22px_58px_rgba(18,28,42,0.08)]">
              <div className="grid min-w-0 gap-3 lg:grid-cols-[minmax(0,1fr)_240px]">
                <PaneTitle icon={Workflow} title={session.app_intent.detail_label} subtitle={event.summary ?? session.display_target} />
                <div className="grid grid-cols-3 gap-1.5">
                  <RunStat label="输入" value={`${session.input_rows.length || 1} 项`} />
                  <RunStat label="动作" value={`${workflow_steps.length} 步`} />
                  <RunStat label="输出" value={`${output_lines.length || 1} 行`} />
                </div>
              </div>
            </div>

            <div className="grid min-w-0 gap-3 lg:grid-cols-[minmax(0,1fr)_280px]">
              <div className="rounded-[16px] border border-(--divider-subtle-color) bg-white/72 p-3 shadow-[0_16px_44px_rgba(18,28,42,0.06)]">
                <div className="mb-3 flex items-center justify-between gap-3">
                  <PaneTitle icon={ListTree} title="输入参数" subtitle="本次调用传入的字段" />
                  <span className="rounded-full bg-[#eef2f7] px-2.5 py-1 text-[10px] font-black text-(--text-soft)">
                    {workflow_steps.length} 步
                  </span>
                </div>
                {workflow_steps.map((step, index) => (
                  <ShortcutActionRow
                    action_icon={ActionIcon}
                    is_last={index === workflow_steps.length - 1}
                    index={index + 1}
                    key={step.id}
                    label={step.label}
                    phase={PHASE_LABELS[event.phase]}
                    tone={step.tone}
                    value={step.value}
                  />
                ))}
              </div>

              <div className="rounded-[16px] border border-(--divider-subtle-color) bg-white/68 p-3 shadow-[0_16px_44px_rgba(18,28,42,0.055)]">
                <PaneTitle icon={Settings2} title="运行摘要" subtitle={PHASE_LABELS[event.phase]} />
                <div className="mt-3 space-y-2">
                  <SummaryRow label="目标应用" value={session.app_intent.app_label} />
                  <SummaryRow label="动作类型" value={session.app_intent.detail_label} />
                  <SummaryRow label="目标" value={session.display_target} />
                </div>
              </div>
            </div>

            <div className="overflow-hidden rounded-[16px] border border-[#1c2630]/12 bg-[#101821] shadow-[0_18px_46px_rgba(18,28,42,0.16)]">
              <div className="flex items-center justify-between gap-3 border-b border-white/10 px-3.5 py-2.5">
                <PaneTitle compact icon={RadioTower} title="执行结果" subtitle={`${output_lines.length || 1} 行输出`} />
                <span className={cn(
                  "h-2 w-2 shrink-0 rounded-full",
                  event.phase === "running" ? "animate-pulse bg-[#8de0ad]" : "bg-[#8de0ad]",
                )} />
              </div>
              <pre className="soft-scrollbar max-h-[220px] overflow-auto whitespace-pre-wrap break-words px-3.5 py-3 font-mono text-[11px] leading-5 text-[#dbe7ee]">
                {session.output_text}
              </pre>
            </div>
          </section>
        </div>
      </section>
    </div>
  );
}

function StatusPill({ phase }: { phase: NexusOperationEvent["phase"] }) {
  return (
    <span className={cn(
      "inline-flex h-8 shrink-0 items-center gap-1.5 rounded-full border px-3 text-[10px] font-black",
      phase === "running"
        ? "border-[rgba(91,114,255,0.22)] bg-[rgba(91,114,255,0.12)] text-[color:var(--primary)]"
        : "border-(--divider-subtle-color) bg-white/72 text-(--text-soft)",
    )}>
      <Play className={cn("h-3.5 w-3.5", phase === "running" && "animate-pulse")} />
      {PHASE_LABELS[phase]}
    </span>
  );
}

function RunStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 border-l border-(--divider-subtle-color) px-2.5 py-1">
      <p className="truncate text-[9px] font-black text-(--text-soft)">{label}</p>
      <p className="mt-0.5 truncate text-[12px] font-black text-(--text-strong)" title={value}>{value}</p>
    </div>
  );
}

function SummaryRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 border-t border-(--divider-subtle-color) pt-2 first:border-t-0 first:pt-0">
      <p className="truncate text-[9px] font-black text-(--text-soft)">{label}</p>
      <p className="mt-0.5 line-clamp-2 break-words text-[10.5px] font-bold leading-4 text-(--text-strong)" title={value}>{value}</p>
    </div>
  );
}

function SidebarMetric({
  icon: Icon,
  label,
  value,
}: {
  icon: LucideIcon;
  label: string;
  value: string;
}) {
  return (
    <div className="grid min-w-0 grid-cols-[22px_minmax(0,1fr)] items-center gap-2 rounded-[10px] bg-white/52 px-2 py-2">
      <span className="grid h-5 w-5 place-items-center rounded-[7px] bg-white/58 text-(--icon-muted)">
        <Icon className="h-3.5 w-3.5" />
      </span>
      <span className="min-w-0">
        <span className="block truncate text-[9px] font-black text-(--text-soft)">{label}</span>
        <span className="block truncate text-[10px] font-bold text-(--text-strong)" title={value}>{value}</span>
      </span>
    </div>
  );
}

function sidebar_icon_for_index(index: number): LucideIcon {
  const icons = [Settings2, FileText, CircleDot, Clock3, ListTree];
  return icons[index] ?? ListTree;
}

function build_workflow_steps(session: ReturnType<typeof build_nexus_tool_session_view>) {
  const input_steps = session.input_rows.length
    ? session.input_rows.slice(0, 4).map((row) => ({
      id: `input:${row.key}`,
      label: row.label,
      tone: "input" as const,
      value: row.value,
    }))
    : [{
      id: "input:target",
      label: "目标",
      tone: "input" as const,
      value: session.display_target,
    }];

  return [
    ...input_steps,
    {
      id: "action:tool",
      label: session.tool_name,
      tone: "action" as const,
      value: session.app_intent.detail_label,
    },
  ];
}

function ShortcutActionRow({
  action_icon: ActionIcon,
  index,
  is_last,
  label,
  phase,
  tone,
  value,
}: {
  action_icon: LucideIcon;
  index: number;
  is_last: boolean;
  label: string;
  phase: string;
  tone: "input" | "action";
  value: string;
}) {
  return (
    <div className="relative grid min-w-0 grid-cols-[38px_minmax(0,1fr)] gap-3">
      <div className="relative flex justify-center">
        {!is_last ? <span className="absolute bottom-[-10px] top-9 w-px bg-[rgba(145,157,173,0.28)]" /> : null}
        <span className={cn(
          "relative z-10 grid h-9 w-9 place-items-center rounded-[12px] text-[12px] font-black shadow-[inset_0_1px_0_rgba(255,255,255,0.72)]",
          tone === "action"
            ? "bg-[rgba(91,114,255,0.14)] text-[color:var(--primary)]"
            : "bg-[rgba(47,184,132,0.12)] text-[color:var(--success)]",
        )}>
          {tone === "action" ? <ActionIcon className="h-4 w-4" /> : index}
        </span>
      </div>
      <div className="mb-2 min-w-0 rounded-[12px] border border-(--divider-subtle-color) bg-white/82 px-3 py-2 shadow-[0_10px_26px_rgba(18,28,42,0.045)]">
        <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
          <p className="truncate text-[12px] font-black text-(--text-strong)">{label}</p>
          <span className={cn(
            "rounded-full px-2 py-0.5 text-[9px] font-black",
            tone === "action" ? "bg-[rgba(91,114,255,0.11)] text-[color:var(--primary)]" : "bg-[#eef2f7] text-(--text-soft)",
          )}>
            {tone === "action" ? phase : "输入"}
          </span>
        </div>
        <p className="mt-1 break-words font-mono text-[10.5px] leading-5 text-(--text-soft)">{value}</p>
      </div>
    </div>
  );
}

function PaneTitle({
  compact = false,
  icon: Icon,
  subtitle,
  title,
}: {
  compact?: boolean;
  icon: LucideIcon;
  subtitle: string;
  title: string;
}) {
  return (
    <div className="flex min-w-0 items-center gap-2">
      <span className={cn(
        "grid shrink-0 place-items-center rounded-[8px] border border-(--divider-subtle-color) bg-white/70 text-(--icon-default)",
        compact ? "h-6 w-6" : "h-7 w-7",
      )}>
        <Icon className="h-3.5 w-3.5" />
      </span>
      <div className="min-w-0">
        <p className={cn("truncate font-black", compact ? "text-[10px] text-[#dbe7ee]" : "text-[11px] text-(--text-strong)")}>{title}</p>
        <p className={cn("truncate text-[10px]", compact ? "text-[#8aa0ad]" : "text-(--text-soft)")}>{subtitle}</p>
      </div>
    </div>
  );
}
