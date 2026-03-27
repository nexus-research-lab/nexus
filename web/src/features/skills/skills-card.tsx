"use client";

import { Lock, Puzzle } from "lucide-react";

import { WorkspaceStatusBadge } from "@/shared/ui/workspace-status-badge";

interface SkillsCardProps {
  name: string;
  description: string;
  installed: boolean;
  locked: boolean;
  is_selected: boolean;
  tags: string[];
  on_select: () => void;
}

export function SkillsCard({
  name,
  description,
  installed,
  locked,
  is_selected,
  tags,
  on_select,
}: SkillsCardProps) {
  return (
    <article
      className={`cursor-pointer rounded-[26px] border px-6 py-5 transition-all ${
        is_selected
          ? "workspace-card-strong border-sky-300/36 shadow-[0_18px_34px_rgba(102,112,145,0.14)]"
          : "workspace-card border-white/24 hover:border-white/30 hover:bg-white/34"
      }`}
      onClick={on_select}
    >
      <div className="flex items-start justify-between gap-4">
        <div className="flex min-w-0 items-center gap-4">
          <div className="workspace-chip flex h-14 w-14 shrink-0 items-center justify-center rounded-[16px] text-slate-900/82">
            {locked ? (
              <Lock className="h-5 w-5 text-slate-600/72" />
            ) : (
              <Puzzle className="h-5 w-5 text-slate-900/88" />
            )}
          </div>
          <div className="min-w-0">
            <p className="truncate text-[20px] font-bold tracking-[-0.03em] text-slate-950/92">
              {name}
            </p>
            {tags.length > 0 && (
              <div className="mt-1 flex flex-wrap gap-1.5">
                {tags.map((tag) => (
                  <span
                    key={tag}
                    className="inline-flex rounded-full bg-slate-100/80 px-2 py-0.5 text-[10px] font-semibold text-slate-500"
                  >
                    {tag}
                  </span>
                ))}
              </div>
            )}
          </div>
        </div>

        <WorkspaceStatusBadge
          label={locked ? "系统" : installed ? "已启用" : "未启用"}
          tone={installed ? "running" : "idle"}
        />
      </div>

      <p className="mt-4 line-clamp-2 min-h-[48px] text-[14px] leading-6 text-slate-700/78">
        {description || "暂无描述"}
      </p>
    </article>
  );
}
