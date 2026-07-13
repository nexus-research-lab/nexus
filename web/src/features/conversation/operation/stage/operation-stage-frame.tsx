import type { KeyboardEventHandler, ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";

import type { NexusOperationEvent } from "../operation-types";
import type { StageNarrativeState } from "./operation-stage-model";

export function DynamicStageFrame({
  event,
  narrative,
  children,
  onKeyDownCapture,
}: {
  event: NexusOperationEvent;
  narrative: StageNarrativeState;
  children: ReactNode;
  onKeyDownCapture?: KeyboardEventHandler<HTMLDivElement>;
}) {
  return (
    <div
      aria-label="Nexus desktop stage"
      className={cn(
        "operation-stage-frame relative h-full min-h-[520px] flex-1 overflow-hidden rounded-[22px] border border-white/60 bg-[linear-gradient(135deg,#eef3f5_0%,#e7ece8_48%,#dfe8ea_100%)] p-4 text-[12px] leading-normal outline-none shadow-[inset_0_1px_0_rgba(255,255,255,0.74)] focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.36)] focus-visible:ring-offset-2 focus-visible:ring-offset-transparent max-md:w-full max-md:min-w-0 max-md:max-w-full max-md:overflow-y-auto max-md:overflow-x-hidden",
        `operation-stage-narrative-${narrative.phase}`,
      )}
      data-stage-experience-phase={narrative.phase}
      onKeyDownCapture={onKeyDownCapture}
      role="application"
    >
      <div className="operation-desktop-wallpaper pointer-events-none absolute inset-0" data-surface={event.surface} />
      <div className="operation-desktop-shadow" />
      <div className="operation-stage-desktop-plane relative h-full min-h-[520px] max-md:flex max-md:h-auto max-md:min-h-0 max-md:w-full max-md:min-w-0 max-md:max-w-full max-md:flex-col max-md:gap-3 max-md:overflow-x-hidden">
        {children}
      </div>
    </div>
  );
}
