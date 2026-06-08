import {
  ArrowRight,
  CheckCircle2,
  Clock3,
  FileText,
  FolderOpen,
  ListChecks,
  PackageCheck,
  RotateCcw,
  Search,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

import { cn } from "@/lib/utils";

import type { StageHandoffSummary } from "../operation-desktop-types";
import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
  OperationEvidence,
} from "../operation-types";
import { display_stage_event_title } from "../operation-stage-labels";
import { resolve_operation_tool_profile } from "../operation-tool-catalog";
import { build_operation_event_io_summary } from "../operation-event-io";
import {
  collect_manifest_artifacts,
  format_manifest_duration,
  icon_for_manifest_artifact,
  PHASE_LABEL,
} from "./run-manifest-data";

export function HandoffSurface({
  event,
  evidence,
  handoff_summary,
  on_focus_event,
  related_events,
  snapshot,
}: {
  event: NexusOperationEvent;
  evidence: OperationEvidence[];
  handoff_summary?: StageHandoffSummary;
  on_focus_event?: (event: NexusOperationEvent) => void;
  related_events: NexusOperationEvent[];
  snapshot: NexusOperationSnapshot | null;
}) {
  const source_events = related_events.length ? related_events : [event];
  const tool_events = source_events.filter((item) => item.surface !== "conversation");
  const completed_count = tool_events.filter((item) => item.phase === "done").length;
  const artifacts = collect_manifest_artifacts(event, source_events, snapshot, evidence);
  const duration = format_manifest_duration(tool_events.length ? tool_events : source_events);
  const resume_prompt = handoff_summary?.resume_prompt ?? event.summary ?? "本轮执行已经归档，可以继续打开产物或回看工具现场。";
  const primary_artifact = artifacts[0] ?? {
    id: "summary",
    label: "执行摘要",
    value: event.target ?? event.title,
    type: "status" as const,
  };
  const PrimaryArtifactIcon = icon_for_manifest_artifact(primary_artifact.type, primary_artifact.value);
  const primary_related_event = source_events.find((item) => item.target === primary_artifact.value)
    ?? tool_events.find((item) => item.target === primary_artifact.value)
    ?? null;
  const replay_event = tool_events.at(-1) ?? source_events.at(-1) ?? event;

  return (
    <div className="grid h-full min-h-[320px] grid-cols-[214px_minmax(0,1fr)] overflow-hidden bg-[#f5f7fa] text-(--text-default) max-md:grid-cols-1">
      <aside className="soft-scrollbar min-h-0 overflow-auto border-r border-(--divider-subtle-color) bg-[#edf1f6]/92 p-2.5 max-md:hidden">
        <div className="flex items-center gap-2 rounded-[11px] bg-white/64 px-2.5 py-2 shadow-[inset_0_1px_0_rgba(255,255,255,0.72)]">
          <span className="grid h-8 w-8 place-items-center rounded-[10px] bg-[rgba(47,184,132,0.12)] text-[color:var(--success)]">
            <PackageCheck className="h-4 w-4" />
          </span>
          <div className="min-w-0">
            <p className="truncate text-[12px] font-black text-(--text-strong)">交付包</p>
            <p className="truncate text-[10px] text-(--text-soft)">round {event.round_id}</p>
          </div>
        </div>

        <div className="mt-2 grid gap-1">
          <FinderSidebarRow active icon={FolderOpen} label="产物" value={`${artifacts.length || 1} 项`} />
          <FinderSidebarRow icon={ListChecks} label="执行记录" value={`${completed_count}/${tool_events.length || 1}`} />
          <FinderSidebarRow icon={Clock3} label="耗时" value={duration} />
        </div>

        <div className="mt-3 px-1 text-[9px] font-black uppercase tracking-[0.12em] text-(--text-soft)">文件</div>
        <div className="mt-1.5 space-y-1">
          {[primary_artifact, ...artifacts.filter((artifact) => artifact.id !== primary_artifact.id)].slice(0, 7).map((artifact) => {
            const Icon = icon_for_manifest_artifact(artifact.type, artifact.value);
            const related_event = source_events.find((item) => item.target === artifact.value);
            return (
              <button
                className="flex w-full min-w-0 items-center gap-2 rounded-[8px] px-2 py-1.5 text-left transition hover:bg-white/66 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(47,184,132,0.32)]"
                disabled={!related_event || !on_focus_event}
                key={artifact.id}
                onClick={() => related_event && on_focus_event?.(related_event)}
                title={artifact.value}
                type="button"
              >
                <span className="grid h-7 w-7 shrink-0 place-items-center rounded-[9px] bg-white/72 text-(--icon-default)">
                  <Icon className="h-3.5 w-3.5" />
                </span>
                <span className="min-w-0">
                  <span className="block truncate text-[10.5px] font-black text-(--text-strong)">{artifact.value}</span>
                  <span className="block truncate text-[9.5px] text-(--text-soft)">{artifact.value}</span>
                </span>
              </button>
            );
          })}
        </div>
      </aside>

      <section className="flex min-w-0 flex-col bg-white/82">
        <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 border-b border-(--divider-subtle-color) bg-white/74 px-4 py-2.5">
          <div className="flex min-w-0 items-center gap-2">
            <Search className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
            <span className="truncate rounded-[8px] bg-[#eef2f7] px-2.5 py-1 text-[10.5px] font-semibold text-(--text-soft)">
              {primary_artifact.value}
            </span>
          </div>
          <span className="inline-flex h-7 items-center gap-1.5 rounded-full bg-[rgba(47,184,132,0.10)] px-2.5 text-[10px] font-black text-[color:var(--success)]">
            <CheckCircle2 className="h-3.5 w-3.5" />
            已完成
          </span>
        </div>

        <div className="soft-scrollbar min-h-0 flex-1 overflow-auto bg-[linear-gradient(180deg,#fbfcfe_0%,#f4f7fb_100%)] p-3">
          <div className="mx-auto max-w-[760px]">
            <div className="grid grid-cols-[104px_minmax(0,1fr)] gap-3 rounded-[14px] border border-(--divider-subtle-color) bg-white/78 p-3 shadow-[0_16px_44px_rgba(18,28,42,0.075)] max-md:grid-cols-1">
              <div className="grid place-items-center rounded-[13px] bg-[#eef3f8] p-4">
                <span className="grid h-16 w-16 place-items-center rounded-[18px] bg-white text-[color:var(--primary)] shadow-[0_14px_30px_rgba(18,28,42,0.08)]">
                  <PrimaryArtifactIcon className="h-8 w-8" />
                </span>
              </div>
              <div className="min-w-0">
                <p className="text-[10px] font-black uppercase tracking-[0.14em] text-(--text-soft)">Quick Look</p>
                <h2 className="mt-1 truncate text-[22px] font-black tracking-normal text-(--text-strong)">
                  {primary_artifact.value}
                </h2>
                <p className="mt-1.5 line-clamp-2 text-[12px] font-semibold leading-5 text-(--text-soft)">
                  {resume_prompt}
                </p>
                <div className="mt-3 grid grid-cols-3 gap-2 text-[10.5px] max-md:grid-cols-1">
                  <QuickLookMetric label="状态" value="已归档" />
                  <QuickLookMetric label="工具" value={`${completed_count}/${tool_events.length || 1}`} />
                  <QuickLookMetric label="耗时" value={duration} />
                </div>
              </div>
            </div>

            <div className="mt-3 overflow-hidden rounded-[13px] border border-(--divider-subtle-color) bg-white/72">
              <div className="grid grid-cols-[minmax(0,1fr)_88px] border-b border-(--divider-subtle-color) bg-[#f3f6fa] px-3 py-2 text-[10px] font-black text-(--text-soft)">
                <span>执行记录</span>
                <span className="text-right">状态</span>
              </div>
            {tool_events.slice(-5).map((item, index) => {
              const profile = resolve_operation_tool_profile(item.tool_name, item.kind, item.surface);
              const io_summary = build_operation_event_io_summary(item);
              return (
                <button
                  className={cn(
                    "grid w-full grid-cols-[28px_minmax(0,1fr)_88px] items-center gap-3 border-b border-(--divider-subtle-color) px-3 py-2 text-left transition last:border-b-0",
                    on_focus_event && "hover:bg-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.32)]",
                  )}
                  key={item.id}
                  onClick={() => on_focus_event?.(item)}
                  type="button"
                >
                  <span className="grid h-7 w-7 place-items-center rounded-[9px] bg-[rgba(47,184,132,0.10)] text-[color:var(--success)]">
                    {index + 1}
                  </span>
                  <span className="min-w-0">
                    <span className="block truncate text-[12px] font-black text-(--text-strong)">
                      {display_stage_event_title(item, profile.action_label)}
                    </span>
                    <span className="mt-0.5 block truncate text-[10.5px] text-(--text-soft)">
                      {io_summary.output_label || io_summary.input_detail || item.summary || item.target || profile.action_label}
                    </span>
                  </span>
                  <span className="justify-self-end rounded-full bg-[rgba(47,184,132,0.10)] px-2 py-1 text-[10px] font-black text-[color:var(--success)]">
                    {PHASE_LABEL[item.phase]}
                  </span>
                </button>
              );
            })}
            </div>
          </div>
        </div>

        <div className="grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-2 border-t border-(--divider-subtle-color) bg-white/66 px-4 py-2 text-[10.5px] font-bold text-(--text-soft)">
          <span className="truncate">现场已收束，历史应用保留在左侧缩略片与 Dock 中。</span>
          <button
            className="inline-flex h-7 items-center gap-1.5 rounded-full border border-(--divider-subtle-color) bg-white/62 px-3 text-(--text-strong) transition hover:bg-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.32)] disabled:cursor-not-allowed disabled:opacity-50"
            disabled={!on_focus_event || !replay_event}
            onClick={() => on_focus_event?.(replay_event)}
            type="button"
          >
            <RotateCcw className="h-3.5 w-3.5" />
            回看工具
          </button>
          <button
            className="inline-flex h-7 items-center gap-1.5 rounded-full bg-[color:var(--primary)] px-3 text-white transition hover:brightness-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.34)] disabled:cursor-not-allowed disabled:opacity-50"
            disabled={!on_focus_event || !primary_related_event}
            onClick={() => primary_related_event && on_focus_event?.(primary_related_event)}
            type="button"
          >
            <FileText className="h-3.5 w-3.5" />
            打开产物
            <ArrowRight className="h-3.5 w-3.5" />
          </button>
        </div>
      </section>
    </div>
  );
}

function FinderSidebarRow({
  active = false,
  icon: Icon,
  label,
  value,
}: {
  active?: boolean;
  icon: LucideIcon;
  label: string;
  value: string;
}) {
  return (
    <div className={cn(
      "grid grid-cols-[18px_minmax(0,1fr)_auto] items-center gap-2 rounded-[8px] px-2 py-1.5 text-[10px]",
      active ? "bg-white/78 text-(--text-strong)" : "text-(--text-soft)",
    )}>
      <Icon className="h-3.5 w-3.5" />
      <span className="truncate font-black">{label}</span>
      <span className="font-semibold">{value}</span>
    </div>
  );
}

function QuickLookMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[10px] bg-[#f3f6fa] px-2.5 py-2">
      <p className="text-[9px] font-black text-(--text-soft)">{label}</p>
      <p className="mt-0.5 truncate font-black text-(--text-strong)">{value}</p>
    </div>
  );
}
