/**
 * INPUT: A pure Editor session projected from file tool events.
 * OUTPUT: Compact macOS-style activity chrome and an optional exact-change inspector.
 * POS: Operation Stage Editor app view; no tool payload interpretation belongs here.
 */
import { useState, type ComponentType } from "react";
import {
  Check,
  ChevronDown,
  CircleAlert,
  FilePenLine,
  FilePlus2,
  LoaderCircle,
  NotebookPen,
  RefreshCw,
  ScanText,
} from "lucide-react";

import { cn } from "@/shared/ui/class-name";

import type {
  EditorActionKind,
  EditorActionView,
  EditorChangeFragment,
  EditorSessionView,
} from "./editor-session-model";

interface IconProps {
  className?: string;
}

const ACTION_ICONS: Record<EditorActionKind, ComponentType<IconProps>> = {
  edit: FilePenLine,
  multi_edit: FilePenLine,
  notebook_edit: NotebookPen,
  preview: ScanText,
  read: ScanText,
  sync: RefreshCw,
  write: FilePlus2,
};

export function EditorActivityBar({ session }: { session: EditorSessionView }) {
  const [expanded_action_id, set_expanded_action_id] = useState<string | null>(null);
  const action = session.activeAction;
  const ActionIcon = ACTION_ICONS[action.kind];
  const is_expanded = expanded_action_id === action.id;
  const can_expand = session.changes.length > 0 || session.history.length > 1;

  return (
    <div className="shrink-0 border-b border-[#dfe4eb] bg-[#f5f7fa] text-[#283340]">
      <div className="flex min-h-9 min-w-0 items-center gap-2 px-3 py-1.5">
        <span className="grid h-5 w-5 shrink-0 place-items-center rounded-[5px] bg-white text-[#52677d] shadow-[0_0_0_1px_rgba(75,91,109,0.12)]">
          <ActionIcon className="h-3.5 w-3.5" />
        </span>
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <span className="truncate text-[10px] font-semibold">{action.label}</span>
          {session.detailLabel ? (
            <span className="truncate text-[9px] text-[#7c8794]">{session.detailLabel}</span>
          ) : null}
        </div>
        <EditorActionStatus action={action} />
        {can_expand ? (
          <button
            aria-expanded={is_expanded}
            className="grid h-6 w-6 shrink-0 place-items-center rounded-[6px] text-[#687585] transition-colors hover:bg-white hover:text-[#26313e]"
            onClick={() => set_expanded_action_id(is_expanded ? null : action.id)}
            title={is_expanded ? "收起更改" : "查看更改"}
            type="button"
          >
            <ChevronDown className={cn("h-3.5 w-3.5 transition-transform", is_expanded && "rotate-180")} />
          </button>
        ) : null}
      </div>
      {is_expanded ? (
        <EditorActivityInspector
          changes={session.changes}
          history={session.history}
        />
      ) : null}
    </div>
  );
}

function EditorActionStatus({ action }: { action: EditorActionView }) {
  const is_running = action.phase === "running" || action.phase === "queued";
  const is_error = action.phase === "error" || action.phase === "cancelled";
  const StatusIcon = is_running ? LoaderCircle : is_error ? CircleAlert : Check;
  return (
    <span className={cn(
      "flex shrink-0 items-center gap-1 text-[9px] font-medium",
      is_error ? "text-[#bc4d58]" : is_running ? "text-[#347b67]" : "text-[#74808d]",
    )}>
      <StatusIcon className={cn("h-3 w-3", is_running && "animate-spin")} />
      {action.statusLabel}
    </span>
  );
}

function EditorActivityInspector({
  changes,
  history,
}: {
  changes: EditorChangeFragment[];
  history: EditorActionView[];
}) {
  return (
    <div className="soft-scrollbar max-h-44 overflow-auto border-t border-[#e4e8ee] bg-white/92 px-3 py-2">
      {changes.length > 0 ? (
        <div className="space-y-2">
          {changes.map((change, index) => (
            <div className="overflow-hidden rounded-[6px] border border-[#e2e6eb]" key={change.id}>
              <div className="flex items-center justify-between bg-[#f7f8fa] px-2 py-1 text-[9px] text-[#737f8c]">
                <span>修改 {index + 1}</span>
                {change.replaceAll ? <span>全部替换</span> : null}
              </div>
              {change.before ? (
                <pre className="max-h-20 overflow-auto whitespace-pre-wrap break-words border-t border-[#f0dadd] bg-[#fff4f5] px-2 py-1.5 text-[9px] leading-4 text-[#81434a]">
                  <span className="select-none pr-2 text-[#c16a73]">-</span>{change.before}
                </pre>
              ) : null}
              {change.after ? (
                <pre className="max-h-20 overflow-auto whitespace-pre-wrap break-words border-t border-[#d7eee4] bg-[#f1faf6] px-2 py-1.5 text-[9px] leading-4 text-[#286548]">
                  <span className="select-none pr-2 text-[#38a876]">+</span>{change.after}
                </pre>
              ) : null}
            </div>
          ))}
        </div>
      ) : (
        <div className="space-y-1">
          {history.map((action) => (
            <div className="flex min-w-0 items-center gap-2 py-1 text-[9px]" key={action.id}>
              <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-[#8da0b4]" />
              <span className="min-w-0 flex-1 truncate text-[#34404d]">{action.label}</span>
              <span className="shrink-0 text-[#8a95a1]">{action.statusLabel}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
