import {
  AlertTriangle,
  Bookmark,
  CheckCircle2,
  HardDrive,
  Filter,
  Search,
  Trash2,
} from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";

import type { StageHandoffSummary } from "../operation-desktop-types";
import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
  OperationEvidence,
} from "../operation-types";
import { buildOperationEventIoSummary } from "../operation-event-io";
import {
  displayStageEventTarget,
  displayStageEventTitle,
} from "../operation-stage-labels";
import { resolveOperationToolProfile } from "../operation-tool-catalog";
import { ACTION_ICON } from "./operation-action-style";
import {
  consoleEventLevel,
  consoleEventSubsystem,
} from "./run-manifest-console";
import {
  collectManifestArtifacts,
  extractManifestEventOutput,
  extractManifestResultText,
  formatManifestDuration,
  iconForManifestArtifact,
  PHASE_LABEL,
} from "./run-manifest-data";
import { collectManifestLogSources } from "./run-manifest-sources";

export function RunManifestSurface({
  event,
  evidence,
  handoffSummary,
  onFocusEvent,
  relatedEvents,
  snapshot,
}: {
  event: NexusOperationEvent;
  evidence: OperationEvidence[];
  handoffSummary?: StageHandoffSummary;
  onFocusEvent?: (event: NexusOperationEvent) => void;
  relatedEvents: NexusOperationEvent[];
  snapshot: NexusOperationSnapshot | null;
}) {
  const source_events = relatedEvents.length ? relatedEvents : [event];
  const events = source_events.filter(should_show_manifest_event);
  const manifest_events = events.length ? events : [event];
  const artifacts = collectManifestArtifacts(event, source_events, snapshot, evidence);
  const terminal_events = manifest_events.filter((item) => item.surface === "terminal");
  const failed_count = manifest_events.filter((item) => item.phase === "error" || item.phase === "cancelled").length;
  const completed_count = manifest_events.filter((item) => item.phase === "done").length;
  const duration = formatManifestDuration(manifest_events);
  const result_text = extractManifestResultText(event);
  const log_sources = collectManifestLogSources(manifest_events);

  return (
    <div className="flex h-full min-h-[330px] min-w-0 overflow-hidden bg-[#f6f8fb] text-(--text-default) max-md:flex-col">
      <aside className="soft-scrollbar flex w-[220px] shrink-0 flex-col overflow-auto border-r border-(--divider-subtle-color) bg-white/62 p-2.5 max-md:w-full max-md:border-b max-md:border-r-0">
        <div className="mb-2 flex items-center gap-2 px-1.5 py-1">
          <span className={cn(
            "grid h-7 w-7 shrink-0 place-items-center rounded-[9px]",
            failed_count ? "bg-[rgba(223,93,98,0.10)] text-[color:var(--destructive)]" : "bg-[rgba(47,184,132,0.10)] text-[color:var(--success)]",
          )}>
            {failed_count ? <AlertTriangle className="h-3.5 w-3.5" /> : <CheckCircle2 className="h-3.5 w-3.5" />}
          </span>
          <div className="min-w-0">
            <p className="truncate text-[11px] font-black text-(--text-strong)">Nexus Console</p>
            <p className="truncate text-[9.5px] text-(--text-soft)">这台 Mac · round {event.round_id}</p>
          </div>
        </div>

        <ManifestSourceList sources={log_sources} />

        <div className="mt-2 space-y-1 rounded-[10px] border border-white/52 bg-white/42 p-1.5 text-[10px]">
          <ManifestSidebarRow label="事件" value={manifest_events.length} />
          <ManifestSidebarRow label="完成" value={`${completed_count}/${manifest_events.length}`} />
          <ManifestSidebarRow label="命令" value={terminal_events.length} />
          <ManifestSidebarRow label="耗时" value={duration} />
        </div>

        <div className="mt-3 flex items-center gap-1.5 px-1 text-[9px] font-black uppercase tracking-[0.12em] text-(--text-soft)">
          <Bookmark className="h-3 w-3" />
          <span>书签</span>
        </div>
        <div className="mt-1.5 space-y-1.5">
            {(artifacts.length ? artifacts : [{
              id: "context-only",
              label: "上下文记录",
              value: event.target ?? event.title,
              type: "status" as const,
            }]).slice(0, 6).map((artifact) => {
              const Icon = iconForManifestArtifact(artifact.type, artifact.value);
              return (
                <div
                  className="flex min-w-0 items-center gap-2 rounded-[11px] border border-white/52 bg-white/42 px-2.5 py-2"
                  key={artifact.id}
                  title={artifact.value}
                >
                  <span className="grid h-6 w-6 shrink-0 place-items-center rounded-[9px] bg-white/62 text-(--icon-muted)">
                    <Icon className="h-3.5 w-3.5" />
                  </span>
                  <span className="min-w-0">
                    <span className="block truncate text-[10.5px] font-black text-(--text-strong)">
                      {artifact.label}
                    </span>
                    <span className="block truncate text-[9.5px] text-(--text-soft)">
                      {artifact.value}
                    </span>
                  </span>
                </div>
              );
            })}
        </div>
      </aside>

      <section className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-white/74">
        <div className="border-b border-(--divider-subtle-color) bg-white/70 px-3 py-2.5">
          <div className="mb-2 flex min-w-0 items-center gap-2">
            <ConsoleToolbarButton label="过滤">
              <Filter className="h-3.5 w-3.5" />
            </ConsoleToolbarButton>
            <div className="flex min-w-0 flex-1 items-center gap-1.5 rounded-[8px] border border-(--divider-subtle-color) bg-white/74 px-2 py-1 text-[10px] text-(--text-soft)">
              <Search className="h-3 w-3 shrink-0" />
              <span className="truncate">搜索所有消息</span>
            </div>
            <ConsoleToolbarButton label="清空显示">
              <Trash2 className="h-3.5 w-3.5" />
            </ConsoleToolbarButton>
          </div>
          <div className="flex min-w-0 items-center justify-between gap-3">
          <div className="min-w-0">
            <p className="truncate text-[12px] font-black text-(--text-strong)">所有消息 · Nexus</p>
            <p className="truncate text-[10px] text-(--text-soft)">
              {result_text || event.summary || handoffSummary?.resume_prompt || "执行日志已保留，可用于回放。"}
            </p>
          </div>
          <span className="shrink-0 font-mono text-[10px] text-(--text-soft)">{PHASE_LABEL[event.phase]}</span>
          </div>
        </div>
        <div className="soft-scrollbar min-h-0 flex-1 overflow-auto p-3">
          <div className="overflow-hidden rounded-[12px] border border-(--divider-subtle-color) bg-white/52">
            <div className="grid grid-cols-[76px_72px_92px_minmax(0,1fr)_72px] gap-2 border-b border-(--divider-subtle-color) bg-white/64 px-3 py-2 text-[9px] font-black uppercase tracking-[0.12em] text-(--text-soft) max-md:grid-cols-[70px_70px_minmax(0,1fr)_64px]">
              <span>时间</span>
              <span>级别</span>
              <span className="max-md:hidden">子系统</span>
              <span>消息</span>
              <span>状态</span>
            </div>
            {manifest_events.map((item, index) => {
              const profile = resolveOperationToolProfile(item.tool_name, item.kind, item.surface);
              const Icon = ACTION_ICON[profile.action];
              const can_focus_event = Boolean(onFocusEvent);
              const io_summary = buildOperationEventIoSummary(item);
              const input_label = io_summary.input_detail;
              const output_label = extractManifestEventOutput(item);
              const event_title = displayStageEventTitle(item, profile.action_label);
              const event_target = displayStageEventTarget(item, profile.action_label);
              const level_label = consoleEventLevel(item.phase);
              return (
                <button
                  aria-label={`查看执行步骤 ${index + 1}：${profile.action_label} ${event_title}`}
                  className={cn(
                    "grid w-full grid-cols-[76px_72px_92px_minmax(0,1fr)_72px] items-center gap-2 border-b border-(--divider-subtle-color) px-3 py-2 text-left transition last:border-b-0 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[rgba(91,114,255,0.36)] max-md:grid-cols-[70px_70px_minmax(0,1fr)_64px]",
                    can_focus_event && "cursor-pointer hover:bg-[rgba(91,114,255,0.045)]",
                    item.id === event.id
                      ? "bg-[rgba(91,114,255,0.08)]"
                      : "bg-transparent",
                  )}
                  key={item.id}
                  onClick={() => onFocusEvent?.(item)}
                  title={`${profile.action_label} · ${event_title}`}
                  type="button"
                >
                  <span className="truncate font-mono text-[10px] text-(--text-soft)">
                    {format_console_time(item.updated_at)}
                  </span>
                  <span className={cn(
                    "inline-flex min-w-0 items-center gap-1.5 rounded-[7px] px-1.5 py-1 text-[9.5px] font-black",
                    item.phase === "error" || item.phase === "cancelled"
                      ? "bg-[rgba(223,93,98,0.10)] text-[color:var(--destructive)]"
                      : item.phase === "done"
                        ? "bg-[rgba(47,184,132,0.10)] text-[color:var(--success)]"
                        : "bg-[rgba(91,114,255,0.08)] text-[color:var(--primary)]",
                  )}>
                    <Icon className="h-3 w-3 shrink-0" />
                    <span className="truncate">{level_label}</span>
                  </span>
                  <span className="truncate text-[10px] font-semibold text-(--text-muted) max-md:hidden">
                    {consoleEventSubsystem(item)}
                  </span>
                  <span className="min-w-0 text-[10.5px]">
                    <span className="block truncate font-black text-(--text-strong)">
                      {String(index + 1).padStart(2, "0")} {event_title}
                    </span>
                    <span className="mt-0.5 block truncate text-[10px] text-(--text-soft)">
                      {event_target}
                    </span>
                    {input_label || output_label ? (
                      <span className="mt-1 grid min-w-0 grid-cols-2 gap-1.5 max-sm:grid-cols-1">
                        {input_label ? (
                          <ManifestEventIOPill label="输入" value={input_label} />
                        ) : null}
                        {output_label ? (
                          <ManifestEventIOPill label="输出" value={output_label} />
                        ) : null}
                      </span>
                    ) : null}
                  </span>
                  <span className="shrink-0 truncate rounded-[7px] bg-white/58 px-1.5 py-1 text-center text-[9px] font-bold text-(--text-soft)">
                    {PHASE_LABEL[item.phase]}
                  </span>
                </button>
              );
            })}
          </div>
        </div>

        <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 border-t border-(--divider-subtle-color) bg-white/62 px-3 py-2 text-[10px] text-(--text-soft)">
          <span className="truncate">{manifest_status_detail({ completed_count, failed_count, total_count: manifest_events.length })}</span>
          <span className="shrink-0 font-mono">{manifest_events.length} 事件</span>
        </div>
      </section>
    </div>
  );
}

function should_show_manifest_event(event: NexusOperationEvent): boolean {
  return !(event.surface === "conversation" && event.phase === "running");
}

function manifest_status_detail({
  completed_count,
  failed_count,
  total_count,
}: {
  completed_count: number;
  failed_count: number;
  total_count: number;
}): string {
  if (failed_count) {
    return `本轮有 ${failed_count} 个异常步骤，执行现场已保留。`;
  }
  if (completed_count >= total_count) {
    return "本轮工具轨迹、窗口现场和关键产物已经归档。";
  }
  return `已完成 ${completed_count}/${total_count} 个步骤。`;
}

function ConsoleToolbarButton({ children, label }: { children: ReactNode; label: string }) {
  return (
    <button
      aria-label={label}
      className="grid h-7 w-7 shrink-0 place-items-center rounded-[8px] border border-(--divider-subtle-color) bg-white/64 text-(--icon-muted) transition hover:bg-white hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.32)]"
      title={label}
      type="button"
    >
      {children}
    </button>
  );
}

function ManifestEventIOPill({ label, value }: { label: string; value: string }) {
  return (
    <span className="flex min-w-0 overflow-hidden rounded-[8px] border border-white/46 bg-white/38 px-1.5 py-1">
      <span className="mr-1 shrink-0 font-black text-(--text-soft)">{label}</span>
      <span className="truncate font-semibold text-(--text-muted)">{value}</span>
    </span>
  );
}

function ManifestSourceList({ sources }: { sources: ReturnType<typeof collectManifestLogSources> }) {
  if (!sources.length) {
    return null;
  }

  return (
    <section className="rounded-[10px] border border-white/52 bg-white/42 p-1.5">
      <div className="mb-1 flex items-center justify-between gap-2 px-1 text-[9px] font-black uppercase tracking-[0.12em] text-(--text-soft)">
        <span>日志源</span>
        <span>{sources.length}</span>
      </div>
      <div className="space-y-1">
        {sources.map((source, index) => (
          <button
            aria-label={`查看日志源 ${source.label}`}
            className={cn(
              "flex w-full min-w-0 items-center gap-2 rounded-[8px] px-2 py-1.5 text-left transition hover:bg-white/62 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.32)]",
              index === 0 && "bg-[rgba(91,114,255,0.09)] text-[color:var(--primary)]",
            )}
            key={source.id}
            type="button"
          >
            <span className="grid h-6 w-6 shrink-0 place-items-center rounded-[8px] bg-white/58 text-(--icon-muted)">
              <HardDrive className="h-3.5 w-3.5" />
            </span>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-[10px] font-black text-(--text-strong)">
                {source.label}
              </span>
              <span className="block truncate text-[9px] text-(--text-soft)">
                {source.detail}
              </span>
            </span>
            <span className="shrink-0 rounded-[7px] bg-white/58 px-1.5 py-0.5 font-mono text-[9px] font-black text-(--text-soft)">
              {source.count}
            </span>
          </button>
        ))}
      </div>
    </section>
  );
}

function format_console_time(timestamp: number): string {
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(new Date(timestamp < 1_000_000_000_000 ? timestamp * 1000 : timestamp));
}

function ManifestSidebarRow({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="flex min-w-0 items-center justify-between gap-2 rounded-[8px] px-2 py-1.5">
      <span className="truncate font-semibold text-(--text-soft)">{label}</span>
      <span className="shrink-0 font-mono font-black text-(--text-strong)">{value}</span>
    </div>
  );
}
