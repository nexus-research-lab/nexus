/**
 * INPUT: A terminal round event, preserved desktop events, and real artifacts.
 * OUTPUT: A compact completion sheet that returns users to the existing app scene.
 * POS: Final Stage handoff UI; execution history remains in the global path control.
 */
import {
  ArrowRight,
  Ban,
  CheckCircle2,
  CircleAlert,
  FileText,
  RotateCcw,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

import type { StageHandoffSummary } from "../operation-desktop-types";
import { operationWorkspaceTargetsMatch } from "../operation-file-documents";
import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
  OperationEvidence,
} from "../operation-types";
import {
  collectManifestArtifacts,
  formatManifestDuration,
  iconForManifestArtifact,
} from "./run-manifest-data";

export function HandoffSurface({
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
  const tool_events = collect_handoff_tool_events(source_events);
  const artifacts = collectManifestArtifacts(event, source_events, snapshot, evidence);
  const preferred_artifact = handoffSummary?.primary_artifact ?? null;
  const primary_artifact = artifacts.find((artifact) => (
    preferred_artifact && targets_match(artifact.value, preferred_artifact)
  )) ?? artifacts[0] ?? {
    id: "summary",
    label: "执行摘要",
    value: preferred_artifact ?? event.target ?? event.title,
    type: "status" as const,
  };
  const outcome = resolve_handoff_outcome(event);
  const primary_event = find_artifact_event(source_events, primary_artifact.value);
  const replay_event = [...tool_events].reverse().find((item) => item.phase !== "queued") ?? null;
  const detail = handoffSummary?.status_detail
    ?? event.summary
    ?? outcome.fallback_detail;
  const resume_prompt = handoffSummary?.resume_prompt
    ?? (primary_artifact.value ? `继续查看 ${primary_artifact.value}` : "继续查看本轮执行现场");
  const duration = formatManifestDuration(tool_events.length ? tool_events : source_events);
  const visible_artifacts = [
    primary_artifact,
    ...artifacts.filter((artifact) => artifact.id !== primary_artifact.id),
  ].slice(0, 4);

  return (
    <div className="flex h-full min-h-[280px] min-w-0 flex-col overflow-hidden bg-[linear-gradient(180deg,#fbfcfe_0%,#f2f5f9_100%)] text-(--text-default)">
      <main className="soft-scrollbar min-h-0 flex-1 overflow-auto px-4 py-4">
        <div className="flex min-w-0 items-start gap-3">
          <span className={cn(
            "grid h-11 w-11 shrink-0 place-items-center rounded-[13px] shadow-[0_12px_28px_rgba(18,28,42,0.10)]",
            outcome.icon_class_name,
          )}>
            <outcome.Icon className="h-5 w-5" />
          </span>
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <h2 className="text-[17px] font-black tracking-normal text-(--text-strong)">{outcome.title}</h2>
              <span className={cn(
                "inline-flex h-5 items-center rounded-full px-2 text-[9px] font-black",
                outcome.badge_class_name,
              )}>
                {handoffSummary?.status_label ?? outcome.status_label}
              </span>
            </div>
            <p className="mt-1.5 text-[11px] font-semibold leading-5 text-(--text-soft)">{detail}</p>
          </div>
        </div>

        <div className="mt-4 border-y border-(--divider-subtle-color) bg-white/56">
          {visible_artifacts.map((artifact, index) => {
            const ArtifactIcon = iconForManifestArtifact(artifact.type, artifact.value);
            const related_event = find_artifact_event(source_events, artifact.value);
            return (
              <button
                className="flex w-full min-w-0 items-center gap-3 border-b border-(--divider-subtle-color) px-3 py-2.5 text-left transition last:border-b-0 hover:bg-white/76 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[rgba(91,114,255,0.30)] disabled:cursor-default"
                disabled={!related_event || !onFocusEvent}
                key={artifact.id}
                onClick={() => related_event && onFocusEvent?.(related_event)}
                type="button"
              >
                <span className={cn(
                  "grid h-8 w-8 shrink-0 place-items-center rounded-[9px]",
                  index === 0
                    ? "bg-[rgba(91,114,255,0.11)] text-[color:var(--primary)]"
                    : "bg-[#e9edf3] text-(--icon-default)",
                )}>
                  <ArtifactIcon className="h-4 w-4" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block break-all text-[11px] font-black leading-4 text-(--text-strong)">{artifact.value}</span>
                  <span className="mt-0.5 block text-[9.5px] font-semibold text-(--text-soft)">{artifact.label}</span>
                </span>
                {related_event && onFocusEvent ? <ArrowRight className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" /> : null}
              </button>
            );
          })}
        </div>

        <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-[9.5px] font-semibold text-(--text-soft)">
          <span>{tool_events.length} 个工具步骤</span>
          {duration ? <span>{duration}</span> : null}
          <span>{resume_prompt}</span>
        </div>
      </main>

      <footer className="flex min-w-0 items-center justify-end gap-2 border-t border-(--divider-subtle-color) bg-white/66 px-4 py-2.5">
        <button
          className="inline-flex h-7 items-center gap-1.5 rounded-[7px] border border-(--divider-subtle-color) bg-white/72 px-3 text-[10px] font-black text-(--text-strong) transition hover:bg-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.30)] disabled:cursor-not-allowed disabled:opacity-45"
          disabled={!onFocusEvent || !replay_event}
          onClick={() => replay_event && onFocusEvent?.(replay_event)}
          type="button"
        >
          <RotateCcw className="h-3.5 w-3.5" />
          回看最后一步
        </button>
        <button
          className="inline-flex h-7 items-center gap-1.5 rounded-[7px] bg-[color:var(--primary)] px-3 text-[10px] font-black text-white transition hover:brightness-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.34)] disabled:cursor-not-allowed disabled:opacity-45"
          disabled={!onFocusEvent || !primary_event}
          onClick={() => primary_event && onFocusEvent?.(primary_event)}
          type="button"
        >
          <FileText className="h-3.5 w-3.5" />
          打开产物
        </button>
      </footer>
    </div>
  );
}

function resolve_handoff_outcome(event: NexusOperationEvent): {
  Icon: LucideIcon;
  badge_class_name: string;
  fallback_detail: string;
  icon_class_name: string;
  status_label: string;
  title: string;
} {
  if (event.phase === "error") {
    return {
      Icon: CircleAlert,
      badge_class_name: "bg-[rgba(223,93,98,0.10)] text-[color:var(--destructive)]",
      fallback_detail: "本轮执行遇到错误，应用现场和最后一步输入已经保留。",
      icon_class_name: "bg-[rgba(223,93,98,0.13)] text-[color:var(--destructive)]",
      status_label: "需要处理",
      title: "执行未完成",
    };
  }
  if (event.phase === "cancelled") {
    return {
      Icon: Ban,
      badge_class_name: "bg-[rgba(223,157,46,0.12)] text-[color:var(--warning)]",
      fallback_detail: "本轮执行已经中断，现有产物和应用现场保持可查看。",
      icon_class_name: "bg-[rgba(223,157,46,0.14)] text-[color:var(--warning)]",
      status_label: "已中断",
      title: "执行已停止",
    };
  }
  return {
    Icon: CheckCircle2,
    badge_class_name: "bg-[rgba(47,184,132,0.10)] text-[color:var(--success)]",
    fallback_detail: "本轮工作已经完成，产物和执行现场仍保留在桌面。",
    icon_class_name: "bg-[rgba(47,184,132,0.14)] text-[color:var(--success)]",
    status_label: "可继续",
    title: "本轮已完成",
  };
}

function find_artifact_event(
  events: NexusOperationEvent[],
  target: string,
): NexusOperationEvent | null {
  return [...events].reverse().find((event) => (
    Boolean(event.target && targets_match(event.target, target))
    || (event.evidence ?? []).some((item) => Boolean(item.value && targets_match(item.value, target)))
  )) ?? null;
}

function targets_match(left: string, right: string): boolean {
  return left === right || (
    looks_like_path(left)
    && looks_like_path(right)
    && operationWorkspaceTargetsMatch(left, right)
  );
}

function collect_handoff_tool_events(events: NexusOperationEvent[]): NexusOperationEvent[] {
  const deduped = new Map<string, NexusOperationEvent>();
  for (const event of events) {
    if (event.surface === "conversation" || event.kind === "round_summary") {
      continue;
    }
    const identity = event.tool_use_id ? `tool:${event.tool_use_id}` : `event:${event.id}`;
    if (!deduped.has(identity)) {
      deduped.set(identity, event);
    }
  }
  return [...deduped.values()];
}

function looks_like_path(value: string): boolean {
  return value.includes("/") || value.includes("\\") || /\.[a-z0-9]{1,12}$/i.test(value);
}
