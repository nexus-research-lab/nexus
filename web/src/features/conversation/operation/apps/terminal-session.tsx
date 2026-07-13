import { useEffect, useMemo, useRef } from "react";
import { CircleStop, LoaderCircle } from "lucide-react";

import { cn } from "@/lib/utils";

import type { NexusOperationEvent } from "../operation-types";
import {
  buildTerminalSession,
} from "./terminal-session-model";
import type {
  TerminalControlEvent,
  TerminalEntry,
} from "./terminal-session-model";
import type { TerminalOutputRow } from "./terminal-result-model";

export function TerminalSession({
  event,
  relatedEvents,
}: {
  event: NexusOperationEvent;
  relatedEvents: NexusOperationEvent[];
}) {
  const viewportRef = useRef<HTMLDivElement>(null);
  const session = useMemo(() => buildTerminalSession({
    event,
    relatedEvents,
  }), [event, relatedEvents]);
  const transcriptVersion = session.entries.map((entry) => (
    `${entry.id}:${entry.phase}:${entry.result.rows.map((row) => `${row.stream}:${row.text}`).join("\n")}:${entry.controls.map((control) => `${control.id}:${control.phase}:${control.resultRows.map((row) => row.text).join("\n")}`).join("|")}`
  )).join("|");

  useEffect(() => {
    const viewport = viewportRef.current;
    if (viewport) {
      viewport.scrollTop = viewport.scrollHeight;
    }
  }, [transcriptVersion]);

  return (
    <div
      aria-label="终端会话"
      aria-live="polite"
      className="soft-scrollbar h-full min-h-[220px] min-w-0 overflow-auto bg-[#2b3948] px-3 py-3 font-mono text-[12px] leading-[1.65] text-[#9de6d7] sm:px-4 sm:py-3.5 sm:text-[13px]"
      ref={viewportRef}
      role="log"
    >
      <div className="w-full min-w-0">
        {session.entries.map((entry, index) => (
          <TerminalProcess
            entry={entry}
            key={entry.id}
            separated={index > 0}
          />
        ))}
        {!session.hasActiveProcess ? <TerminalPrompt cwdLabel={lastKnownCwd(session.entries)} /> : null}
      </div>
    </div>
  );
}

function TerminalProcess({
  entry,
  separated,
}: {
  entry: TerminalEntry;
  separated: boolean;
}) {
  return (
    <section className={cn("min-w-0", separated && "mt-3 border-t border-white/8 pt-3")}>
      {entry.command ? (
        <TerminalCommand command={entry.command} cwdLabel={entry.cwdLabel} />
      ) : null}

      {entry.result.rows.map((row) => (
        <TerminalOutput key={row.id} row={row} />
      ))}

      {entry.controls.map((control) => (
        <TerminalControl control={control} key={control.id} />
      ))}

      <TerminalStatus entry={entry} />
    </section>
  );
}

function TerminalPrompt({ cwdLabel }: { cwdLabel: string | null }) {
  return (
    <div className="mt-1 flex min-w-0 items-start gap-2">
      <TerminalCwd cwdLabel={cwdLabel} />
      <span className="shrink-0 select-none text-[#ff5ec7]">❯</span>
      <span className="operation-terminal-caret mt-[5px] shrink-0 bg-[#c9f7ef]" />
    </div>
  );
}

function TerminalCommand({
  command,
  cwdLabel,
}: {
  command: string;
  cwdLabel: string | null;
}) {
  return (
    <div className="flex min-w-0 items-start gap-2 text-[#dce8f1]">
      <TerminalCwd cwdLabel={cwdLabel} />
      <span className="shrink-0 select-none text-[#ff5ec7]">❯</span>
      <span className="min-w-0 whitespace-pre-wrap break-words [overflow-wrap:anywhere]">{command}</span>
    </div>
  );
}

function TerminalOutput({ row }: { row: TerminalOutputRow }) {
  if (row.text === "") {
    return <div aria-hidden="true" className="h-[1.65em]" />;
  }
  return (
    <div className="min-w-0">
      <span className={cn(
        "min-w-0 whitespace-pre-wrap break-words [overflow-wrap:anywhere]",
        row.stream === "stderr" ? "text-[#ff9b9b]" : row.stream === "stdout" ? "text-[#a7eadc]" : "text-[#b8c7d4]",
      )}>
        {row.text}
      </span>
    </div>
  );
}

function TerminalControl({ control }: { control: TerminalControlEvent }) {
  const isActive = control.phase === "queued" || control.phase === "running" || control.phase === "waiting";
  return (
    <div className="my-1.5 min-w-0 border-l-2 border-[#ffcc66]/55 pl-2.5 text-[#d8c79e]">
      <div className="flex min-w-0 items-start gap-1.5">
        {isActive
          ? <LoaderCircle className="mt-[3px] h-3.5 w-3.5 shrink-0 animate-spin" />
          : <CircleStop className="mt-[3px] h-3.5 w-3.5 shrink-0" />}
        <span className="min-w-0 break-words">
          KillShell · {control.statusLabel} · {control.targetLabel ?? "目标未知"}
          {control.durationLabel ? ` · ${control.durationLabel}` : ""}
        </span>
      </div>
      {control.resultRows.map((row) => (
        <TerminalOutput key={`${control.id}:${row.id}`} row={row} />
      ))}
    </div>
  );
}

function TerminalCwd({ cwdLabel }: { cwdLabel: string | null }) {
  return (
    <span className={cn(
      "shrink-0 select-none",
      cwdLabel ? "text-[#65c7f7]" : "text-[#7f90a3]",
    )}>
      {cwdLabel ?? "cwd ?"}
    </span>
  );
}

function TerminalStatus({ entry }: { entry: TerminalEntry }) {
  const isActive = entry.phase === "queued" || entry.phase === "running" || entry.phase === "waiting";
  const details = [entry.statusLabel, entry.durationLabel].filter(Boolean).join(" · ");
  return (
    <div className={cn(
      "mt-0.5 flex min-w-0 items-start gap-2 text-[#7f90a3]",
      entry.statusTone === "error" && "text-[#ff9b9b]",
      entry.statusTone === "success" && "text-[#91d6b5]",
      isActive && "text-[#d8c79e]",
    )}>
      <span className="shrink-0 select-none">#</span>
      <span className="min-w-0 break-words">{details}</span>
      {isActive ? <span className="operation-terminal-caret mt-[5px] shrink-0 bg-current" /> : null}
    </div>
  );
}

function lastKnownCwd(entries: TerminalEntry[]): string | null {
  for (let index = entries.length - 1; index >= 0; index -= 1) {
    if (entries[index].cwdLabel) {
      return entries[index].cwdLabel;
    }
  }
  return null;
}
