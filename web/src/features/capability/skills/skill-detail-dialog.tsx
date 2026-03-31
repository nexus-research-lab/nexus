/**
 * Skill 详情弹窗
 *
 * 展示完整 Skill 文档、来源、版本和安装操作。
 */

"use client";

import { useCallback, useEffect, useState } from "react";
import {
  Check,
  Loader2,
  Lock,
  Puzzle,
  RefreshCw,
  Shield,
  Tag,
  Trash2,
  X,
} from "lucide-react";

import {
  deleteSkillFromPoolApi,
  getSkillDetailApi,
  installSkillToPoolApi,
  setSkillGlobalEnabledApi,
  updateSingleSkillApi,
} from "@/lib/skill-api";
import { SkillDetail } from "@/types/skill";

import { SkillMarkdown } from "./skill-markdown";

interface SkillDetailDialogProps {
  skill_name: string;
  is_open: boolean;
  on_close: () => void;
  on_refresh: () => Promise<void> | void;
}

export function SkillDetailDialog({
  skill_name,
  is_open,
  on_close,
  on_refresh,
}: SkillDetailDialogProps) {
  const [skill, set_skill] = useState<SkillDetail | null>(null);
  const [loading, set_loading] = useState(false);
  const [acting, set_acting] = useState(false);
  const [error, set_error] = useState<string | null>(null);

  const load_detail = useCallback(async () => {
    if (!is_open) return;
    try {
      set_loading(true);
      set_error(null);
      const detail = await getSkillDetailApi(skill_name);
      set_skill(detail);
    } catch (err) {
      set_error(err instanceof Error ? err.message : "加载 skill 详情失败");
    } finally {
      set_loading(false);
    }
  }, [is_open, skill_name]);

  useEffect(() => {
    void load_detail();
  }, [load_detail]);

  const handle_update = useCallback(async () => {
    if (!skill) return;
    try {
      set_acting(true);
      await updateSingleSkillApi(skill.name);
      await Promise.resolve(on_refresh());
      await load_detail();
    } catch (err) {
      set_error(err instanceof Error ? err.message : "更新 skill 失败");
    } finally {
      set_acting(false);
    }
  }, [load_detail, on_refresh, skill]);

  const handle_toggle_global_enabled = useCallback(async () => {
    if (!skill || skill.locked) return;
    try {
      set_acting(true);
      await setSkillGlobalEnabledApi(skill.name, !skill.global_enabled);
      await Promise.resolve(on_refresh());
      await load_detail();
    } catch (err) {
      set_error(err instanceof Error ? err.message : "切换全局启用状态失败");
    } finally {
      set_acting(false);
    }
  }, [load_detail, on_refresh, skill]);

  const handle_delete = useCallback(async () => {
    if (!skill || !skill.deletable) return;
    try {
      set_acting(true);
      await deleteSkillFromPoolApi(skill.name);
      await Promise.resolve(on_refresh());
      on_close();
    } catch (err) {
      set_error(err instanceof Error ? err.message : "删除 skill 失败");
    } finally {
      set_acting(false);
    }
  }, [load_detail, on_close, on_refresh, skill]);

  const handle_install = useCallback(async () => {
    if (!skill || skill.locked || skill.installed) return;
    try {
      set_acting(true);
      await installSkillToPoolApi(skill.name);
      await Promise.resolve(on_refresh());
      await load_detail();
    } catch (err) {
      set_error(err instanceof Error ? err.message : "安装 skill 失败");
    } finally {
      set_acting(false);
    }
  }, [load_detail, on_refresh, skill]);

  if (!is_open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm"
      onClick={on_close}
    >
      <div
        className="modal-dialog-surface radius-shell-xl flex h-[84vh] w-full max-w-5xl flex-col overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-start justify-between gap-4 border-b modal-divider px-6 py-5">
          <div className="flex min-w-0 items-center gap-4">
            <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-[20px] bg-white/70">
              <Puzzle className="h-7 w-7 text-slate-900/88" />
            </div>
            <div className="min-w-0">
              <h2 className="truncate text-[28px] font-black tracking-[-0.04em] text-slate-950/92">
                {loading ? "加载中..." : skill?.title ?? skill_name}
              </h2>
              <p className="mt-1 text-[14px] text-slate-700/70">
                {skill?.description || "正在读取 Skill.md 文档"}
              </p>
            </div>
          </div>
          <button
            aria-label="关闭"
            className="rounded-full p-2 text-slate-400 transition-colors hover:bg-white/60 hover:text-slate-700"
            onClick={on_close}
            type="button"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="soft-scrollbar flex-1 overflow-y-auto px-6 py-5">
          {loading ? (
            <div className="flex min-h-[320px] items-center justify-center">
              <Loader2 className="h-7 w-7 animate-spin text-slate-400" />
            </div>
          ) : skill ? (
            <>
              <div className="mb-5 flex flex-wrap gap-2">
                <span className="inline-flex rounded-full bg-white/70 px-3 py-1 text-[11px] font-semibold text-slate-600">
                  {skill.category_name}
                </span>
                <span className="inline-flex rounded-full bg-white/70 px-3 py-1 text-[11px] font-semibold text-slate-600">
                  {skill.source_type === "system" ? "系统内置" : skill.source_type === "builtin" ? "内置推荐" : "用户导入"}
                </span>
                <span className="inline-flex rounded-full bg-white/70 px-3 py-1 text-[11px] font-semibold text-slate-600">
                  版本 {skill.version || "unknown"}
                </span>
                {!skill.global_enabled ? (
                  <span className="inline-flex rounded-full bg-slate-100 px-3 py-1 text-[11px] font-semibold text-slate-500">
                    全局停用
                  </span>
                ) : null}
                {skill.locked ? (
                  <span className="inline-flex items-center gap-1 rounded-full bg-amber-50 px-3 py-1 text-[11px] font-semibold text-amber-700">
                    <Shield className="h-3 w-3" />
                    系统锁定
                  </span>
                ) : null}
                {skill.tags.map((tag) => (
                  <span
                    key={tag}
                    className="inline-flex items-center gap-1 rounded-full bg-slate-100/80 px-3 py-1 text-[11px] font-semibold text-slate-600"
                  >
                    <Tag className="h-3 w-3" />
                    {tag}
                  </span>
                ))}
              </div>

              {skill.recommendation ? (
                <div className="workspace-card mb-5 rounded-[22px] px-5 py-4 text-[13px] leading-6 text-slate-700/78">
                  {skill.recommendation}
                </div>
              ) : null}

              {error ? (
                <div className="mb-5 rounded-[18px] border border-rose-200/80 bg-rose-50/80 px-4 py-3 text-[13px] text-rose-600">
                  {error}
                </div>
              ) : null}

              <SkillMarkdown markdown={skill.readme_markdown} />
            </>
          ) : (
            <div className="flex min-h-[320px] items-center justify-center text-sm text-slate-500">
              未找到该 Skill
            </div>
          )}
        </div>

      <div className="flex flex-wrap items-center justify-end gap-2 border-t modal-divider px-6 py-4">
          {skill?.locked ? (
            <button
              className="inline-flex items-center gap-2 rounded-full bg-amber-50 px-4 py-2 text-sm font-semibold text-amber-700"
              disabled
              type="button"
            >
              <Lock className="h-4 w-4" />
              系统级
            </button>
          ) : skill ? (
            <>
              {!skill.installed ? (
                <button
                  className="inline-flex items-center gap-2 rounded-full bg-sky-500 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
                  disabled={acting}
                  onClick={() => void handle_install()}
                  type="button"
                >
                  {acting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Puzzle className="h-4 w-4" />}
                  安装到资源池
                </button>
              ) : skill.has_update ? (
                <button
                  className="inline-flex items-center gap-2 rounded-full bg-sky-500 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
                  disabled={acting}
                  onClick={() => void handle_update()}
                  type="button"
                >
                  {acting ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                  更新全局技能
                </button>
              ) : (
                <span className="inline-flex items-center gap-2 rounded-full bg-emerald-50 px-4 py-2 text-sm font-semibold text-emerald-600">
                  <Check className="h-4 w-4" />
                  已安装
                </span>
              )}
              {!skill.locked && skill.installed ? (
                <label className="relative inline-flex cursor-pointer items-center">
                  <input
                    checked={skill.global_enabled}
                    className="peer sr-only"
                    disabled={acting}
                    onChange={() => void handle_toggle_global_enabled()}
                    type="checkbox"
                  />
                  <div className="h-6 w-11 rounded-full bg-slate-200/80 peer peer-checked:bg-emerald-500 after:absolute after:left-[2px] after:top-0.5 after:h-5 after:w-5 after:rounded-full after:border after:border-white/60 after:bg-white after:shadow-sm after:transition-all after:content-[''] peer-checked:after:translate-x-full peer-checked:after:border-white" />
                </label>
              ) : null}
              {skill.installed && skill.deletable ? (
                <button
                  className="inline-flex items-center gap-2 rounded-full bg-rose-50 px-4 py-2 text-sm font-semibold text-rose-600 disabled:opacity-60"
                  disabled={acting}
                  onClick={() => void handle_delete()}
                  type="button"
                >
                  {acting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}
                  删除
                </button>
              ) : null}
            </>
          ) : null}
        </div>
      </div>
    </div>
  );
}
