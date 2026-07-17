/**
 * INPUT: A background Stage window's app identity, title, icon, and tone.
 * OUTPUT: A compact, non-interactive visual proxy for Stage Manager layout.
 * POS: Background-window presentation; mounted app content remains owned by OperationStageWindow.
 */
import type { LucideIcon } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

export function StageManagerWindowPreview({
  appLabel,
  icon: Icon,
  title,
  tone,
}: {
  appLabel: string;
  icon: LucideIcon;
  title: string;
  tone: "default" | "terminal";
}) {
  const skin = stage_manager_preview_skin(appLabel, tone);
  return (
    <div className={cn(
      "flex h-full min-h-0 flex-col overflow-hidden rounded-[12px] border shadow-[inset_0_1px_0_rgba(255,255,255,0.62)] transition group-hover:brightness-[1.03]",
      skin.frame,
    )}>
      <div className={cn("flex h-5 shrink-0 items-center gap-1.5 border-b px-2", skin.titlebar)}>
        <span className="h-1.5 w-1.5 rounded-full bg-[rgba(223,93,98,0.72)]" />
        <span className="h-1.5 w-1.5 rounded-full bg-[rgba(223,157,46,0.72)]" />
        <span className="h-1.5 w-1.5 rounded-full bg-[rgba(47,184,132,0.72)]" />
        <span className="ml-auto flex min-w-0 items-center gap-1 text-[7px] font-black">
          <Icon className="h-2.5 w-2.5 shrink-0" />
          <span className="truncate">{appLabel}</span>
        </span>
      </div>
      <div className="min-h-0 flex-1 p-2">
        <p className={cn("truncate text-[9px] font-black leading-3", skin.title)}>{title}</p>
        <div className={cn("mt-1.5 space-y-1", skin.body)}>
          {skin.lines.map((line, index) => (
            <span
              className={cn("block h-1.5 rounded-full", line)}
              key={`${appLabel}-${index}`}
            />
          ))}
        </div>
      </div>
      <div className={cn("h-3 shrink-0 border-t px-2 py-0.5", skin.status)}>
        <span className="block h-1.5 w-8 rounded-full bg-current opacity-30" />
      </div>
    </div>
  );
}

function stage_manager_preview_skin(app_label: string, tone: "default" | "terminal") {
  if (tone === "terminal" || app_label === "终端") {
    return {
      body: "bg-[#070c12]",
      frame: "border-white/12 bg-[#0a1118] text-[#8de0ad]",
      lines: ["w-10 bg-[#8de0ad]/35", "w-16 bg-white/18", "w-12 bg-[#8de0ad]/28"],
      status: "border-white/8 bg-[#070c12] text-[#8de0ad]",
      title: "text-[#d8e8e2]",
      titlebar: "border-white/8 bg-white/[0.04] text-[#8aa09b]",
    };
  }
  if (app_label === "Navi") {
    return {
      body: "bg-white/60",
      frame: "border-white/70 bg-[#eef5fb] text-(--text-soft)",
      lines: ["w-full bg-[rgba(91,114,255,0.16)]", "w-3/4 bg-[rgba(47,184,132,0.14)]", "w-1/2 bg-[rgba(117,131,149,0.14)]"],
      status: "border-white/54 bg-white/50 text-(--text-soft)",
      title: "text-(--text-strong)",
      titlebar: "border-white/60 bg-white/64 text-(--text-soft)",
    };
  }
  if (app_label === "Editor") {
    return {
      body: "bg-[#101820]",
      frame: "border-white/14 bg-[#0c141c] text-[#8aa0ad]",
      lines: ["w-14 bg-[#8de0ad]/25", "w-full bg-white/12", "w-2/3 bg-white/10"],
      status: "border-white/8 bg-[#0a1118] text-[#8de0ad]",
      title: "text-[#dce8ee]",
      titlebar: "border-white/8 bg-[#151f29] text-[#8aa0ad]",
    };
  }
  return {
    body: "bg-white/54",
    frame: "border-white/68 bg-white/58 text-(--text-soft)",
    lines: ["w-full bg-[rgba(91,114,255,0.14)]", "w-4/5 bg-[rgba(117,131,149,0.14)]", "w-1/2 bg-[rgba(47,184,132,0.14)]"],
    status: "border-white/58 bg-white/46 text-(--text-soft)",
    title: "text-(--text-strong)",
    titlebar: "border-white/58 bg-white/58 text-(--text-soft)",
  };
}
