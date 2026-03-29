"use client";

import { Check, Lock, Plus, Puzzle } from "lucide-react";

import { cn } from "@/lib/utils";

interface SkillsCardProps {
  /** 技能名称 */
  name: string;
  /** 技能描述 */
  description: string;
  /** 是否已安装（至少一个 agent 安装了） */
  installed: boolean;
  /** 是否系统级（不可卸载） */
  locked: boolean;
  /** 标签列表 */
  tags: string[];
  /** 点击卡片 → 导航到详情页 */
  on_select: () => void;
  /** 点击安装按钮 */
  on_install?: () => void;
}

/** Skill 卡片 — 居中布局，底部安装状态/操作 */
export function SkillsCard({
  name,
  description,
  installed,
  locked,
  tags,
  on_select,
  on_install,
}: SkillsCardProps) {
  return (
    <article
      className="workspace-card cursor-pointer rounded-[26px] border border-white/24 px-6 py-6 text-center transition-all hover:border-white/30 hover:bg-white/34"
      onClick={on_select}
    >
      {/* 居中图标 */}
      <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full border border-white/24 bg-white/16 backdrop-blur-sm">
        {locked ? (
          <Lock className="h-6 w-6 text-slate-600/72" />
        ) : (
          <Puzzle className="h-6 w-6 text-slate-900/88" />
        )}
      </div>

      {/* 名称 */}
      <p className="mt-4 truncate text-[18px] font-bold tracking-[-0.03em] text-slate-950/92">
        {name}
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
              className="inline-flex rounded-full border border-white/20 bg-white/16 px-2 py-0.5 text-[10px] font-semibold text-slate-500 backdrop-blur-sm"
            >
              {tag}
            </span>
          ))}
        </div>
      )}

      {/* 底部状态/操作 */}
      <div className="mt-5" onClick={(e) => e.stopPropagation()}>
        {locked ? (
          <span className="inline-flex items-center gap-1.5 rounded-full bg-amber-50 px-3 py-1.5 text-[11px] font-semibold text-amber-700">
            <Check className="h-3.5 w-3.5" />
            系统级 · 已安装
          </span>
        ) : installed ? (
          <span className="inline-flex items-center gap-1.5 rounded-full bg-emerald-50 px-3 py-1.5 text-[11px] font-semibold text-emerald-600">
            <Check className="h-3.5 w-3.5" />
            已安装
          </span>
        ) : (
          <button
            className={cn(
              "inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-[11px] font-semibold transition-all",
              "workspace-chip text-slate-600 hover:bg-sky-50 hover:text-sky-600",
            )}
            onClick={on_install}
            type="button"
          >
            <Plus className="h-3.5 w-3.5" />
            安装
          </button>
        )}
      </div>
    </article>
  );
}
