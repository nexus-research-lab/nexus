"use client";

import { Check, Download, Lock, Power, Puzzle, RefreshCw, Trash2 } from "lucide-react";

import { cn } from "@/lib/utils";
import { SkillInfo } from "@/types/skill";

interface SkillsCardProps {
  skill: SkillInfo;
  busy?: boolean;
  on_select: () => void;
  on_install?: () => void;
  on_update?: () => void;
  on_delete?: () => void;
  on_toggle_global_enabled?: () => void;
}

/** Skill 卡片 — 清晰的三段式布局 */
export function SkillsCard({
  skill,
  busy = false,
  on_select,
  on_install,
  on_update,
  on_delete,
  on_toggle_global_enabled,
}: SkillsCardProps) {
  const {
    title,
    description,
    installed,
    locked,
    tags,
    source_type,
    has_update,
    global_enabled,
    deletable,
  } = skill;

  const source_label =
    source_type === "system" ? "系统" : source_type === "builtin" ? "内置" : "外部";

  return (
    <article
      className={cn(
        "group relative cursor-pointer workspace-card flex flex-col rounded-[22px] px-5 py-4 transition-all hover:border-white/36 hover:bg-white/40",
        !global_enabled && installed && "opacity-60",
      )}
      onClick={on_select}
    >
      {/* 右上角操作区 — 悬停显示 */}
      <div
        className="absolute right-3 top-3 flex items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100"
        onClick={(e) => e.stopPropagation()}
      >
        {installed && has_update && (
          <button
            className="flex h-7 w-7 items-center justify-center rounded-full text-sky-500 transition-colors hover:bg-sky-50"
            disabled={busy}
            onClick={on_update}
            title="更新"
            type="button"
          >
            <RefreshCw className="h-3.5 w-3.5" />
          </button>
        )}
        {installed && deletable && (
          <button
            className="flex h-7 w-7 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-rose-50 hover:text-rose-500"
            disabled={busy}
            onClick={on_delete}
            title="从资源池删除"
            type="button"
          >
            <Trash2 className="h-3.5 w-3.5" />
          </button>
        )}
      </div>

      {/* 头部：图标 + 名称 + 来源标签 */}
      <div className="flex items-center gap-3">
        <div
          className={cn(
            "flex h-10 w-10 shrink-0 items-center justify-center rounded-[14px] border",
            installed
              ? "border-emerald-200/60 bg-emerald-50/80 text-emerald-600"
              : locked
                ? "border-amber-200/60 bg-amber-50/80 text-amber-600"
                : "border-white/44 bg-white/64 text-slate-600",
          )}
        >
          {locked ? <Lock className="h-[18px] w-[18px]" /> : <Puzzle className="h-[18px] w-[18px]" />}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <p className="truncate text-[15px] font-bold tracking-[-0.02em] text-slate-950/90">
              {title}
            </p>
            <span className="shrink-0 rounded-full border border-white/40 bg-white/60 px-1.5 py-px text-[9px] font-semibold text-slate-500">
              {source_label}
            </span>
            {has_update && (
              <span className="shrink-0 rounded-full bg-sky-50 px-1.5 py-px text-[9px] font-semibold text-sky-600">
                可更新
              </span>
            )}
          </div>
        </div>
      </div>

      {/* 描述 */}
      <p className="mt-2.5 line-clamp-2 flex-1 text-[13px] leading-[1.6] text-slate-700/65">
        {description || "暂无描述"}
      </p>

      {/* 底部：标签 + 状态 */}
      <div className="mt-3 flex items-end justify-between gap-3">
        {/* 标签 */}
        <div className="flex min-w-0 flex-wrap gap-1">
          {tags.slice(0, 3).map((tag) => (
            <span
              key={tag}
              className="inline-flex rounded-full border border-white/30 bg-white/50 px-2 py-0.5 text-[10px] font-medium text-slate-500/80"
            >
              {tag}
            </span>
          ))}
        </div>

        {/* 状态/操作 */}
        <div
          className="flex shrink-0 items-center gap-1.5"
          onClick={(e) => e.stopPropagation()}
        >
          {locked ? (
            <span className="inline-flex items-center gap-1 rounded-full bg-amber-50/80 px-2.5 py-1 text-[10px] font-semibold text-amber-600">
              <Lock className="h-3 w-3" />
              内置
            </span>
          ) : installed ? (
            <>
              <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50/80 px-2.5 py-1 text-[10px] font-semibold text-emerald-600">
                <Check className="h-3 w-3" />
                已安装
              </span>
              <button
                className={cn(
                  "flex h-6 w-6 items-center justify-center rounded-full transition-colors",
                  global_enabled
                    ? "text-emerald-500 hover:bg-emerald-50"
                    : "text-slate-400 hover:bg-slate-100",
                )}
                disabled={busy}
                onClick={() => on_toggle_global_enabled?.()}
                title={global_enabled ? "全局停用" : "全局启用"}
                type="button"
              >
                <Power className="h-3 w-3" />
              </button>
            </>
          ) : (
            <button
              className="inline-flex items-center gap-1 rounded-full border border-white/40 bg-white/70 px-2.5 py-1 text-[10px] font-semibold text-slate-600 transition-all hover:bg-sky-50 hover:text-sky-600"
              disabled={busy}
              onClick={on_install}
              type="button"
            >
              <Download className="h-3 w-3" />
              安装
            </button>
          )}
        </div>
      </div>
    </article>
  );
}
