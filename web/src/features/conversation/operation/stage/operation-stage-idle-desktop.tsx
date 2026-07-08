import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

export function OperationStageIdleDesktop({
  header_action,
  presentation,
}: {
  header_action?: ReactNode;
  presentation: "panel" | "stage";
}) {
  const now = new Date();
  const time_label = new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
  }).format(now);
  const second_label = new Intl.DateTimeFormat("zh-CN", {
    second: "2-digit",
  }).format(now);

  return (
    <div
      aria-label="Nexus idle desktop"
      className={cn(
        "operation-stage-frame relative h-full min-h-[520px] flex-1 overflow-hidden rounded-[22px] border border-white/60 bg-[linear-gradient(135deg,#eef3f5_0%,#e7ece8_48%,#dfe8ea_100%)] p-4 outline-none shadow-[inset_0_1px_0_rgba(255,255,255,0.74)]",
        presentation === "stage" && "min-h-[620px]",
      )}
      data-stage-experience-phase="idle"
    >
      <div className="operation-desktop-wallpaper pointer-events-none absolute inset-0" data-surface="conversation" />
      <div className="operation-desktop-shadow" />
      <div className="absolute inset-x-4 top-2 z-20 flex h-9 items-center justify-between rounded-[14px] border border-white/64 bg-[rgba(255,255,255,0.54)] px-3 text-[11px] font-semibold text-(--text-strong) shadow-[0_12px_30px_rgba(18,28,42,0.08),inset_0_1px_0_rgba(255,255,255,0.72)] backdrop-blur-2xl max-md:hidden">
        <div className="flex min-w-0 items-center gap-3">
          <span className="grid h-6 w-6 shrink-0 place-items-center rounded-[8px] bg-[rgba(20,28,38,0.88)] font-mono text-[9px] font-black text-white shadow-[0_8px_18px_rgba(18,28,42,0.16)]">
            NX
          </span>
          <span className="font-black">Nexus OS</span>
          <span className="h-4 w-px bg-[rgba(117,131,149,0.28)]" />
          <span className="truncate font-black">桌面待命</span>
        </div>
        <div className="flex items-center gap-2 text-(--text-soft)">
          <span className="rounded-full border border-white/66 bg-white/44 px-2 py-1 text-[9px] font-bold">
            等待工具调用
          </span>
          <span className="font-mono text-[10px] text-(--text-strong)">{time_label}</span>
          {header_action ? <div className="ml-1">{header_action}</div> : null}
        </div>
      </div>

      <div className="relative z-10 flex h-full min-h-[520px] flex-col justify-end">
        <div className="mx-auto mb-[13vh] w-full max-w-[1180px] px-6">
          <div className="select-none font-mono text-[clamp(70px,15vw,230px)] font-black leading-none tracking-normal text-[rgba(91,114,255,0.34)] [text-shadow:0_18px_70px_rgba(91,114,255,0.18)]">
            nexus
          </div>
          <div className="mt-8 flex items-end gap-3 text-(--text-strong)">
            <span className="font-mono text-[64px] font-light leading-none tracking-normal max-md:text-[44px]">
              {time_label}
            </span>
            <span className="pb-1 font-mono text-[24px] font-light text-(--text-soft) max-md:text-[18px]">
              :{second_label}
            </span>
          </div>
          <div className="mt-5 max-w-[520px] rounded-[18px] border border-white/62 bg-white/38 px-4 py-3 text-[12px] font-semibold text-(--text-soft) shadow-[0_18px_54px_rgba(18,28,42,0.08),inset_0_1px_0_rgba(255,255,255,0.72)] backdrop-blur-xl">
            Nexus 桌面已就绪。第一个工具调用出现时，会打开对应应用窗口并进入执行现场。
          </div>
        </div>
      </div>
    </div>
  );
}
