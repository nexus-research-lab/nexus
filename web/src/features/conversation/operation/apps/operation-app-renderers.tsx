/**
 * INPUT: A planned Stage window and app-level interaction callbacks.
 * OUTPUT: The concrete Agent OS app surface for that semantic window kind.
 * POS: Stage app router; app state and domain logic remain in focused components.
 */
import { cn } from "@/shared/ui/class-name";
import type { PermissionDecisionPayload } from "@/types/conversation/interaction/permission";

import type { StageWindowState } from "../operation-desktop-types";
import type {
  NexusOperationEvent,
} from "../operation-types";
import type { OperationToolProfile } from "../operation-tool-catalog";
import {
  buildOperationInputRows,
  extractOperationInputValue,
  PHASE_LABELS,
  resolveOperationToolProfile,
} from "../operation-tool-catalog";
import {
  buildEditorPreviewLines,
  getPreviewLines,
} from "../operation-preview";
import { ACTION_ICON, ACTION_TONE_CLASS } from "./operation-action-style";
import { appSurfaceForWindowKind } from "./operation-app-surface-policy";
import { TaskAppSurface } from "./task-app-surface";
import { BrowserSurface } from "./browser-surface";
import { DocumentPreview } from "./document-preview-surface";
import { resolveFilePreviewValue } from "./file-preview-value";
import { OperationReviewPanel, PermissionCheckpointPanel } from "./operation-review-panels";
import { HandoffSurface } from "./handoff-surface";
import { ImageInspectionSurface } from "./image-inspection-surface";
import { RunManifestSurface } from "./run-manifest-surface";
import { TerminalSession } from "./terminal-session";
import { StageWorkspaceFilePreview } from "./stage-workspace-file-preview";
import { WorkspaceFinder } from "./workspace-finder-surface";

export function StageWindowContent({
  window,
  onFocusEvent,
  onOpenWorkspaceFile,
  onPermissionResponse,
}: {
  window: StageWindowState;
  onFocusEvent?: (event: NexusOperationEvent) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
}) {
  const { event, snapshot } = window.payload;
  const profile = resolveOperationToolProfile(event.tool_name, event.kind, event.surface);

  if (window.kind === "finder") {
    const workspace_items = window.payload.workspace_items ?? [];
    return (
      <div className="flex h-full min-h-[240px] flex-col">
        <WorkspaceFinder
          activePath={window.payload.target ?? event.target}
          event={event}
          items={workspace_items}
          onOpenFile={onOpenWorkspaceFile}
        />
      </div>
    );
  }

  if (window.kind === "terminal") {
    return (
      <TerminalSession
        event={event}
        relatedEvents={window.payload.related_events ?? []}
      />
    );
  }

  if (window.kind === "browser") {
    const query = window.payload.query ?? event.target ?? "web";
    return (
      <div className="flex h-full min-h-[280px] min-w-0 max-w-full flex-col">
        <BrowserSurface
          event={event}
          preview={window.payload.srcdoc ?? window.payload.preview}
          query={query}
          relatedEvents={window.payload.related_events ?? []}
          target={window.payload.target ?? event.target}
        />
      </div>
    );
  }

  if (window.kind === "tasks") {
    return (
      <TaskAppSurface
        event={event}
        onFocusEvent={onFocusEvent}
        relatedEvents={window.payload.related_events ?? []}
      />
    );
  }

  if (window.kind === "run_manifest") {
    return (
      <RunManifestSurface
        event={event}
        evidence={window.payload.evidence ?? []}
        handoffSummary={window.payload.handoff_summary}
        onFocusEvent={onFocusEvent}
        relatedEvents={window.payload.related_events ?? []}
        snapshot={snapshot}
      />
    );
  }

  if (window.kind === "handoff") {
    return (
      <HandoffSurface
        event={event}
        evidence={window.payload.evidence ?? []}
        handoffSummary={window.payload.handoff_summary}
        onFocusEvent={onFocusEvent}
        relatedEvents={window.payload.related_events ?? []}
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
          onPermissionResponse={onPermissionResponse}
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

  if (
    window.kind === "image_viewer" &&
    !window.payload.workspace_preview &&
    window.payload.image_source &&
    window.payload.image_source_kind
  ) {
    return (
      <ImageInspectionSurface
        event={event}
        preview={window.payload.preview}
        source={window.payload.image_source}
        sourceKind={window.payload.image_source_kind}
      />
    );
  }

  if (appSurfaceForWindowKind(window.kind) === "document") {
    const workspace_target = window.payload.target ?? window.target ?? event.target;
    if (window.payload.workspace_preview && workspace_target) {
      return (
        <StageWorkspaceFilePreview
          agentId={event.agent_id}
          diffStats={window.payload.diff_stats}
          event={event}
          initialContent={read_initial_workspace_content(event, window.payload.preview)}
          path={workspace_target}
          relatedEvents={window.payload.related_events ?? []}
          sourceView={window.kind === "code_editor"}
        />
      );
    }
    return (
      <DocumentPreview
        diffStats={window.payload.diff_stats}
        fallbackLines={buildEditorPreviewLines(event, getPreviewLines(window.payload.preview, 12))}
        operationEvent={event}
        summary={window.payload.summary ?? event.summary ?? event.title}
        target={window.payload.target ?? window.target ?? event.target}
        value={resolveFilePreviewValue(event, window.payload.preview)}
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
          diffStats={window.payload.diff_stats}
          fallbackLines={buildEditorPreviewLines(event, getPreviewLines(window.payload.preview, 12))}
          operationEvent={event}
          summary={window.payload.summary ?? event.summary ?? event.title}
          target={window.payload.target ?? window.target ?? event.target}
          value={resolveFilePreviewValue(event, window.payload.preview)}
        />
      </div>
    </div>
  );
}

function read_initial_workspace_content(
  event: NexusOperationEvent,
  preview: unknown,
): string | null {
  const value = resolveFilePreviewValue(event, preview);
  return typeof value === "string" ? value : null;
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
  const primary = extractOperationInputValue(event.input_preview, profile.target_keys);
  const rows = buildOperationInputRows(event.input_preview, profile.target_keys, 3);
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
