import { useEffect, useState } from "react";
import {
  Apple,
  Battery,
  Code2,
  Compass,
  FolderOpen,
  Globe2,
  ImageIcon,
  ListChecks,
  PackageCheck,
  Search,
  TerminalSquare,
  Wifi,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

import { cn } from "@/lib/utils";

import type { StageWindowKind } from "./operation-desktop-types";
import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
} from "./operation-types";
import {
  build_stage_transition_style,
} from "./operation-stage-transition";
import type { StageTransitionIntent } from "./operation-stage-transition";

const IDLE_DOCK_APPS: Array<{
  Icon: LucideIcon;
  kind: StageWindowKind;
  label: string;
  skin: IdleDockIconSkin;
}> = [
  { Icon: FolderOpen, kind: "finder", label: "访达", skin: "finder" },
  { Icon: Globe2, kind: "browser", label: "Safari", skin: "safari" },
  { Icon: TerminalSquare, kind: "terminal", label: "终端", skin: "terminal" },
  { Icon: Code2, kind: "code_editor", label: "Code", skin: "code" },
  { Icon: PackageCheck, kind: "handoff", label: "交付台", skin: "handoff" },
  { Icon: ListChecks, kind: "run_manifest", label: "控制台", skin: "console" },
  { Icon: ImageIcon, kind: "image_viewer", label: "预览", skin: "preview" },
];

type IdleDockIconSkin = "code" | "console" | "finder" | "handoff" | "nexus" | "preview" | "safari" | "terminal";

export function EmptyStage({
  exiting = false,
  on_launch_app,
  subtitle,
  transition_intent = "summary",
}: {
  active_event?: NexusOperationEvent | null;
  exiting?: boolean;
  on_launch_app?: (app: { app_label: string; kind: StageWindowKind }) => void;
  previous_event?: NexusOperationEvent | null;
  round_event_count?: number;
  snapshot: NexusOperationSnapshot | null;
  subtitle: string;
  transition_intent?: StageTransitionIntent;
}) {
  const now = useStageClock();
  const time_label = format_stage_clock(now);
  const second_label = format_stage_seconds(now);
  const transition_style = build_stage_transition_style(transition_intent);

  return (
    <div className={cn(
      "relative h-full min-h-[300px] w-full flex-1 overflow-hidden bg-[linear-gradient(180deg,rgba(250,252,255,0.98),rgba(239,244,251,0.86))]",
      exiting && "pointer-events-none absolute inset-0 z-20 operation-idle-stage-exit",
    )}
    data-stage-experience-phase={exiting ? "awakening" : "idle"}
    style={exiting ? transition_style : undefined}
    >
      <div className="operation-idle-sky pointer-events-none absolute inset-0 bg-[radial-gradient(60%_48%_at_50%_43%,rgba(255,255,255,0.96),transparent_72%),radial-gradient(44%_30%_at_50%_62%,rgba(91,114,255,0.13),transparent_75%)]" />
      <div className="operation-idle-grid operation-stage-gridlines pointer-events-none absolute inset-0 opacity-[0.18]" />
      <div className="operation-idle-dotfield pointer-events-none absolute inset-0 opacity-[0.32] [background-image:radial-gradient(rgba(91,114,255,0.16)_1px,transparent_1px)] [background-size:34px_34px] [mask-image:linear-gradient(to_bottom,transparent,black_20%,black_78%,transparent)]" />

      <IdleMenuBar subtitle={subtitle} time_label={time_label} />

      <div className="operation-idle-clock pointer-events-none absolute bottom-8 left-8 z-10 flex items-end gap-2 max-sm:bottom-5 max-sm:left-5">
        <div className="font-mono text-[54px] font-semibold leading-none tracking-normal text-[rgba(32,43,58,0.88)] max-sm:text-[42px]">
          {time_label}
        </div>
        <div className="pb-1.5 font-mono text-[24px] font-semibold leading-none tracking-normal text-[rgba(32,43,58,0.28)] max-sm:text-[18px]">
          :{second_label}
        </div>
      </div>
      <IdleDock on_launch_app={on_launch_app} />

    </div>
  );
}

function IdleMenuBar({
  subtitle,
  time_label,
}: {
  subtitle: string;
  time_label: string;
}) {
  return (
    <div className="absolute inset-x-0 top-0 z-10 flex h-8 items-center justify-between border-b border-white/58 bg-white/50 px-4 text-[11px] font-semibold text-[rgba(32,43,58,0.86)] shadow-[0_1px_0_rgba(255,255,255,0.72),0_12px_34px_rgba(18,28,42,0.08)] backdrop-blur-2xl max-sm:px-3">
      <div className="flex min-w-0 items-center gap-3">
        <Apple className="h-3.5 w-3.5 shrink-0" />
        <span className="font-black">Nexus</span>
        <span className="hidden text-[rgba(75,88,108,0.72)] sm:inline">文件</span>
        <span className="hidden text-[rgba(75,88,108,0.72)] sm:inline">编辑</span>
        <span className="hidden text-[rgba(75,88,108,0.72)] md:inline">显示</span>
        <span className="hidden max-w-[180px] truncate text-[rgba(75,88,108,0.58)] lg:inline">
          {subtitle}
        </span>
      </div>
      <div className="flex shrink-0 items-center gap-3 text-[rgba(75,88,108,0.72)]">
        <Search className="hidden h-3.5 w-3.5 sm:block" />
        <Wifi className="h-3.5 w-3.5" />
        <Battery className="h-3.5 w-3.5" />
        <span className="font-mono text-[11px] text-[rgba(32,43,58,0.86)]">{time_label}</span>
      </div>
    </div>
  );
}

function IdleDock({
  on_launch_app,
}: {
  on_launch_app?: (app: { app_label: string; kind: StageWindowKind }) => void;
}) {
  return (
    <div className="absolute inset-x-4 bottom-4 z-10 flex justify-center max-sm:bottom-3">
      <div className="operation-window-dock flex max-w-full items-end gap-2 overflow-x-auto rounded-[26px] border border-white/70 bg-white/58 px-2.5 py-2 shadow-[0_24px_60px_rgba(18,28,42,0.16),inset_0_1px_0_rgba(255,255,255,0.78)] backdrop-blur-2xl">
        <IdleDockIcon active Icon={Compass} label="Nexus" skin="nexus" />
        <div className="h-9 w-px shrink-0 bg-white/56" />
        {IDLE_DOCK_APPS.map((app) => (
          <IdleDockIcon
            Icon={app.Icon}
            key={app.label}
            label={app.label}
            onClick={on_launch_app ? () => on_launch_app({ app_label: app.label, kind: app.kind }) : undefined}
            skin={app.skin}
          />
        ))}
      </div>
    </div>
  );
}

function IdleDockIcon({
  active = false,
  Icon,
  label,
  onClick,
  skin,
}: {
  active?: boolean;
  Icon: LucideIcon;
  label: string;
  onClick?: () => void;
  skin: IdleDockIconSkin;
}) {
  return (
    <button
      aria-label={onClick ? `打开：${label}` : label}
      className={cn(
        "group relative grid h-[44px] w-[44px] shrink-0 place-items-center rounded-[18px] border transition duration-200 ease-out hover:-translate-y-1 hover:scale-105 focus-visible:-translate-y-1 focus-visible:scale-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.42)]",
        active
          ? "border-[rgba(91,114,255,0.30)] bg-[rgba(91,114,255,0.13)] shadow-[0_16px_32px_rgba(91,114,255,0.18)]"
          : "border-transparent bg-white/20 text-[rgba(75,88,108,0.74)] hover:bg-white/44",
      )}
      onClick={onClick}
      title={label}
      type="button"
    >
      <span className={cn(
        "relative grid h-[34px] w-[34px] place-items-center rounded-[14px] border shadow-[inset_0_1px_0_rgba(255,255,255,0.62),0_8px_18px_rgba(18,28,42,0.10)]",
        idle_dock_icon_skin(skin),
      )}>
        <Icon className="h-[18px] w-[18px]" />
      </span>
      {active ? (
        <span className="absolute -bottom-2 left-1/2 h-1.5 w-5 -translate-x-1/2 rounded-full bg-[rgba(91,114,255,0.86)]" />
      ) : null}
      <span className="pointer-events-none absolute bottom-[calc(100%+10px)] left-1/2 hidden -translate-x-1/2 whitespace-nowrap rounded-[10px] border border-white/70 bg-[rgba(20,28,38,0.82)] px-2.5 py-1.5 text-[10px] font-semibold text-white shadow-[0_12px_30px_rgba(18,28,42,0.22)] backdrop-blur-xl group-hover:block group-focus-visible:block">
        {label}
      </span>
    </button>
  );
}

function idle_dock_icon_skin(skin: IdleDockIconSkin): string {
  if (skin === "finder") {
    return "border-[rgba(72,152,224,0.42)] bg-[linear-gradient(135deg,#5ac8fa_0%,#e8f5ff_48%,#ffffff_49%,#7dd3fc_100%)] text-[#14517a]";
  }
  if (skin === "safari") {
    return "border-[rgba(72,152,224,0.36)] bg-[radial-gradient(circle_at_50%_50%,#ffffff_0_24%,#5ac8fa_25%_52%,#2f6dff_53%_70%,#f45b69_71%_100%)] text-white";
  }
  if (skin === "terminal") {
    return "border-[rgba(141,224,173,0.32)] bg-[linear-gradient(135deg,#111827,#05080d)] text-[#8de0ad]";
  }
  if (skin === "code") {
    return "border-[rgba(91,114,255,0.36)] bg-[linear-gradient(135deg,#243b74,#4f6fff)] text-white";
  }
  if (skin === "console") {
    return "border-[rgba(117,131,149,0.30)] bg-[linear-gradient(135deg,#f8fafc,#cbd5e1)] text-[#334155]";
  }
  if (skin === "handoff") {
    return "border-[rgba(47,184,132,0.34)] bg-[linear-gradient(135deg,#dffbf0,#f8fffc)] text-[#17734d]";
  }
  if (skin === "preview") {
    return "border-[rgba(72,152,224,0.32)] bg-[linear-gradient(135deg,#e0f2fe,#ffffff_56%,#dbeafe)] text-[#2563eb]";
  }
  return "border-[rgba(91,114,255,0.28)] bg-[linear-gradient(135deg,rgba(91,114,255,0.18),rgba(255,255,255,0.74),rgba(79,162,159,0.14))] text-[rgba(32,43,58,0.92)]";
}

function useStageClock(): Date {
  const [now, set_now] = useState(() => new Date());

  useEffect(() => {
    const timer = window.setInterval(() => set_now(new Date()), 1000);
    return () => window.clearInterval(timer);
  }, []);

  return now;
}

function format_stage_clock(value: Date): string {
  return `${pad_clock_value(value.getHours())}:${pad_clock_value(value.getMinutes())}`;
}

function format_stage_seconds(value: Date): string {
  return pad_clock_value(value.getSeconds());
}

function pad_clock_value(value: number): string {
  return String(value).padStart(2, "0");
}
