import { cn } from "@/lib/utils";
import type { PermissionDecisionPayload } from "@/types/conversation/permission";

import type { StageWindowState } from "../operation-desktop-types";
import type {
  NexusOperationEvent,
} from "../operation-types";
import type { OperationToolProfile } from "../operation-tool-catalog";
import {
  build_operation_input_rows,
  extract_operation_input_value,
  PHASE_LABELS,
  resolve_operation_tool_profile,
} from "../operation-tool-catalog";
import {
  build_editor_preview_lines,
  format_operation_time,
  get_preview_lines,
} from "../operation-preview";
import { ACTION_ICON, ACTION_TONE_CLASS } from "./operation-action-style";
import { app_surface_for_window_kind } from "./operation-app-surface-policy";
import { ActivityMonitorSurface } from "./activity-monitor-surface";
import { BrowserSurface } from "./browser-surface";
import { DocumentPreview } from "./document-preview-surface";
import { resolve_file_preview_value } from "./file-preview-value";
import { OperationReviewPanel, PermissionCheckpointPanel } from "./operation-review-panels";
import { HandoffSurface } from "./handoff-surface";
import { RunManifestSurface } from "./run-manifest-surface";
import { TerminalSession } from "./terminal-session";
import { WorkspaceFinder } from "./workspace-finder-surface";

export function StageWindowContent({
  window,
  on_focus_event,
  on_permission_response,
}: {
  window: StageWindowState;
  on_focus_event?: (event: NexusOperationEvent) => void;
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
}) {
  const { event, snapshot } = window.payload;
  const profile = resolve_operation_tool_profile(event.tool_name, event.kind, event.surface);

  if (window.kind === "finder") {
    const workspace_items = window.payload.workspace_items ?? [];
    return (
      <div className="flex h-full min-h-[240px] flex-col">
        <WorkspaceFinder
          active_path={window.payload.target ?? event.target}
          event={event}
          items={workspace_items}
        />
      </div>
    );
  }

  if (window.kind === "terminal") {
    const lines = window.payload.lines?.length
      ? window.payload.lines
      : [
        window.payload.command ? `$ ${window.payload.command}` : "$",
        ...get_preview_lines(event.result_preview ?? event.summary, 10),
      ];
    return (
      <TerminalSession
        command={window.payload.command ?? event.target ?? ""}
        event={event}
        lines={lines}
        related_events={window.payload.related_events ?? []}
      />
    );
  }

  if (window.kind === "browser") {
    const query = window.payload.query ?? event.target ?? "web";
    const lines = window.payload.lines ?? get_preview_lines(event.result_preview ?? event.summary, 7);
    return (
      <div className="flex h-full min-h-[280px] min-w-0 max-w-full flex-col">
        <BrowserSurface
          event={event}
          lines={lines}
          preview={window.payload.srcdoc ?? window.payload.preview}
          query={query}
          target={window.payload.target ?? event.target}
        />
      </div>
    );
  }

  if (window.kind === "task_board") {
    return (
      <ActivityMonitorSurface
        event={event}
        lines={window.payload.lines ?? []}
        on_focus_event={on_focus_event}
        snapshot={snapshot}
      />
    );
  }

  if (window.kind === "run_manifest") {
    return (
      <RunManifestSurface
        event={event}
        evidence={window.payload.evidence ?? []}
        handoff_summary={window.payload.handoff_summary}
        on_focus_event={on_focus_event}
        related_events={window.payload.related_events ?? []}
        snapshot={snapshot}
      />
    );
  }

  if (window.kind === "handoff") {
    return (
      <HandoffSurface
        event={event}
        evidence={window.payload.evidence ?? []}
        handoff_summary={window.payload.handoff_summary}
        on_focus_event={on_focus_event}
        related_events={window.payload.related_events ?? []}
        snapshot={snapshot}
      />
    );
  }

  if (window.kind === "evidence" || window.kind === "permission_wait") {
    if (window.kind === "permission_wait") {
      return (
        <PermissionCheckpointPanel
          compact={window.phase === "minimized"}
          event={event}
          evidence={window.payload.evidence}
          on_permission_response={on_permission_response}
          snapshot={snapshot}
        />
      );
    }
    return (
      <OperationReviewPanel
        compact={window.phase === "minimized"}
        event={event}
        evidence={window.payload.evidence}
        mode="evidence"
        snapshot={snapshot}
      />
    );
  }

  if (window.kind === "summary") {
    return (
      <div className="flex h-full min-h-0 flex-col gap-3">
        <ToolActionHeader event={event} profile={profile} target={event.target} />
        <div className="min-h-0 flex-1">
          <DocumentPreview
            summary={event.summary ?? event.target ?? "暂无摘要"}
            target="run-summary.md"
            value={window.payload.preview ?? event.result_preview ?? event.summary ?? event.target}
          />
        </div>
      </div>
    );
  }

  if (app_surface_for_window_kind(window.kind) === "document") {
    return (
      <DocumentPreview
        diff_stats={window.payload.diff_stats}
        fallback_lines={build_editor_preview_lines(event, get_preview_lines(window.payload.preview, 12))}
        operation_event={event}
        summary={window.payload.summary ?? event.summary ?? event.title}
        target={window.payload.target ?? window.target ?? event.target}
        value={resolve_file_preview_value(event, window.payload.preview)}
      />
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-3">
      <ToolActionHeader
        event={event}
        profile={profile}
        target={window.payload.target ?? window.target ?? event.target}
      />
      <div className="min-h-0 flex-1">
        <DocumentPreview
          diff_stats={window.payload.diff_stats}
          fallback_lines={build_editor_preview_lines(event, get_preview_lines(window.payload.preview, 12))}
          operation_event={event}
          summary={window.payload.summary ?? event.summary ?? event.title}
          target={window.payload.target ?? window.target ?? event.target}
          value={resolve_file_preview_value(event, window.payload.preview)}
        />
      </div>
    </div>
  );
}

function ToolActionHeader({
  event,
  profile,
  target,
  tone = "default",
}: {
  event: NexusOperationEvent;
  profile: OperationToolProfile;
  target?: string | null;
  tone?: "default" | "terminal";
}) {
  const Icon = ACTION_ICON[profile.action];
  const primary = extract_operation_input_value(event.input_preview, profile.target_keys);
  const rows = build_operation_input_rows(event.input_preview, profile.target_keys, 3);
  const display_target = primary?.value ?? target ?? event.target ?? event.summary ?? event.title;
  const is_terminal = tone === "terminal";

  return (
    <div className={cn(
      "min-w-0 max-w-full rounded-[13px] border p-3",
      is_terminal
        ? "border-white/10 bg-white/[0.035] text-[#d8e8e2]"
        : "border-(--divider-subtle-color) bg-white/72 text-(--text-default)",
    )}>
      <div className="flex min-w-0 items-center justify-between gap-3 max-md:flex-col max-md:items-start">
        <div className="flex min-w-0 max-w-full items-center gap-2">
          <span className={cn(
            "inline-flex h-7 shrink-0 items-center gap-1.5 rounded-full border px-2 text-[10px] font-black",
            is_terminal ? "border-white/12 bg-white/[0.04] text-[#8de0ad]" : ACTION_TONE_CLASS[profile.action],
          )}>
            <Icon className="h-3.5 w-3.5" />
            {profile.action_label}
          </span>
          <div className="min-w-0">
            <p className={cn(
              "truncate text-[12px] font-black tracking-[-0.02em]",
              is_terminal ? "text-[#e8f6f0]" : "text-(--text-strong)",
            )}>
              {profile.title}
            </p>
            <p className={cn(
              "mt-0.5 truncate text-[11px]",
              is_terminal ? "text-[#8aa09b]" : "text-(--text-soft)",
            )}>
              {display_target}
            </p>
          </div>
        </div>
        <span className={cn(
          "shrink-0 rounded-full px-2 py-1 text-[10px] font-semibold max-md:ml-[34px]",
          is_terminal ? "bg-white/[0.05] text-[#8de0ad]" : "bg-white/70 text-(--text-muted)",
        )}>
          {PHASE_LABELS[event.phase]}
        </span>
      </div>
      {rows.length > 1 ? (
        <div className="mt-2 grid grid-cols-1 gap-1.5 sm:grid-cols-2">
          {rows.slice(0, 2).map((row) => (
            <div
              className={cn(
                "min-w-0 overflow-hidden rounded-[9px] px-2 py-1.5 text-[10px]",
                is_terminal ? "bg-black/12 text-[#99b0aa]" : "bg-white/62 text-(--text-soft)",
              )}
              key={row.key}
            >
              <span className="font-semibold">{row.label}</span>
              <span className="ml-1 break-words">{row.value}</span>
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}
