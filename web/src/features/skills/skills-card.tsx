"use client";

import { Check, Lock, Puzzle, RefreshCw, Trash2 } from "lucide-react";

import { cn } from "@/lib/utils";
import { SkillInfo } from "@/types/skill";

interface SkillsCardProps {
  skill: SkillInfo;
  busy?: boolean;
  on_select: () => void;
  on_update?: () => void;
  on_delete?: () => void;
  on_toggle_global_enabled?: () => void;
}

/** Skill 卡片 — 居中布局，底部安装状态/操作 */
export function SkillsCard({
  skill,
  busy = false,
  on_select,
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
    version,
    global_enabled,
    deletable,
  } = skill;

  return (
    <article
      className="workspace-card cursor-pointer rounded-[26px] border border-white/24 px-6 py-6 text-center transition-all hover:border-white/30 hover:bg-white/34"
      onClick={on_select}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex flex-wrap gap-1.5">
          <span className="inline-flex rounded-full border border-white/40 bg-white/60 px-2 py-0.5 text-[10px] font-semibold text-slate-500">
            {source_type === "system" ? "系统" : source_type === "builtin" ? "内置" : "外部"}
          </span>
          {has_update ? (
            <span className="inline-flex rounded-full bg-sky-50 px-2 py-0.5 text-[10px] font-semibold text-sky-600">
              可更新
            </span>
          ) : null}
          {!global_enabled ? (
            <span className="inline-flex rounded-full bg-slate-100 px-2 py-0.5 text-[10px] font-semibold text-slate-500">
              全局停用
            </span>
          ) : null}
        </div>
        {deletable ? (
          <button
            className="flex h-8 w-8 items-center justify-center rounded-full text-rose-400 transition-colors hover:bg-rose-50 hover:text-rose-500"
            disabled={busy}
            onClick={(e) => {
              e.stopPropagation();
              on_delete?.();
            }}
            type="button"
          >
            <Trash2 className="h-4 w-4" />
          </button>
        ) : <div className="h-8 w-8" />}
      </div>

      {/* 居中图标 */}
      <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full border border-white/44 bg-white/64">
        {locked ? (
          <Lock className="h-6 w-6 text-slate-600/72" />
        ) : (
          <Puzzle className="h-6 w-6 text-slate-900/88" />
        )}
      </div>

      {/* 名称 */}
      <p className="mt-4 truncate text-[18px] font-bold tracking-[-0.03em] text-slate-950/92">
        {title}
      </p>

      {/* 描述：1-2 行截断 */}
      <p className="mt-2 line-clamp-2 min-h-[40px] text-[13px] leading-5 text-slate-700/68">
        {description || "暂无描述"}
      </p>

      {/* 标签 */}
      {tags.length > 0 && (
        <div className="mt-3 flex flex-wrap justify-center gap-1.5">
          {tags.slice(0, 3).map((tag) => (
            <span
              key={tag}
              className="inline-flex rounded-full border border-white/40 bg-white/60 px-2 py-0.5 text-[10px] font-semibold text-slate-500"
            >
              {tag}
            </span>
          ))}
        </div>
      )}

      <p className="mt-3 text-[11px] text-slate-500/70">
        版本 {version || "unknown"}
      </p>

      {/* 底部状态/操作 */}
      <div className="mt-5 flex flex-wrap items-center justify-center gap-2" onClick={(e) => e.stopPropagation()}>
        {locked ? (
          <span className="inline-flex items-center gap-1.5 rounded-full bg-amber-50 px-3 py-1.5 text-[11px] font-semibold text-amber-700">
            <Check className="h-3.5 w-3.5" />
            系统级 · 内置
          </span>
        ) : installed ? (
          <>
            <span className="inline-flex items-center gap-1.5 rounded-full bg-emerald-50 px-3 py-1.5 text-[11px] font-semibold text-emerald-600">
              <Check className="h-3.5 w-3.5" />
              已安装
            </span>
            {has_update ? (
              <button
                className="inline-flex items-center gap-1.5 rounded-full bg-sky-50 px-3 py-1.5 text-[11px] font-semibold text-sky-600 transition-all hover:bg-sky-100"
                disabled={busy}
                onClick={on_update}
                type="button"
              >
                <RefreshCw className="h-3.5 w-3.5" />
                更新
              </button>
            ) : null}
            {!locked ? (
              <label className="relative inline-flex cursor-pointer items-center">
                <input
                  checked={global_enabled}
                  className="peer sr-only"
                  disabled={busy}
                  onChange={() => on_toggle_global_enabled?.()}
                  type="checkbox"
                />
                <div className="h-6 w-11 rounded-full bg-slate-200/80 peer peer-checked:bg-emerald-500 after:absolute after:left-[2px] after:top-0.5 after:h-5 after:w-5 after:rounded-full after:border after:border-white/60 after:bg-white after:shadow-sm after:transition-all after:content-[''] peer-checked:after:translate-x-full peer-checked:after:border-white" />
              </label>
            ) : null}
          </>
        ) : (
          <span className="inline-flex items-center gap-1.5 rounded-full bg-white/70 px-3 py-1.5 text-[11px] font-semibold text-slate-600">
            导入后可使用
          </span>
        )}
      </div>
    </article>
  );
}
