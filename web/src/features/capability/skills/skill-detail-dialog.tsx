/**
 * Skill 详情弹窗
 *
 * 展示完整 Skill 文档、来源、版本和管理操作。
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

import { deleteSkillApi, getSkillDetailApi, updateSingleSkillApi } from "@/lib/skill-api";
import { cn } from "@/lib/utils";
import {
  DIALOG_ICON_BUTTON_CLASS_NAME,
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_HEADER_LEADING_CLASS_NAME,
  DIALOG_TAG_CLASS_NAME,
  getDialogActionClassName,
  getDialogNoteClassName,
  getDialogNoteStyle,
} from "@/shared/ui/dialog/dialog-styles";
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

  const handle_delete = useCallback(async () => {
    if (!skill || !skill.deletable) return;
    try {
      set_acting(true);
      await deleteSkillApi(skill.name);
      await Promise.resolve(on_refresh());
      on_close();
    } catch (err) {
      set_error(err instanceof Error ? err.message : "删除 skill 失败");
    } finally {
      set_acting(false);
    }
  }, [on_close, on_refresh, skill]);

  if (!is_open) return null;

  return (
    <div
      className="dialog-backdrop"
      onClick={on_close}
    >
      <div
        className="dialog-shell radius-shell-xl flex h-[84vh] w-full max-w-5xl flex-col overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="dialog-header">
          <div className={cn(DIALOG_HEADER_LEADING_CLASS_NAME, "min-w-0 flex-1 items-center")}>
            <div className={cn(DIALOG_HEADER_ICON_CLASS_NAME, "h-14 w-14 rounded-[20px]")}>
              <Puzzle className="h-7 w-7 text-(--text-strong)" />
            </div>
            <div className="min-w-0">
              <h2 className="dialog-title truncate" data-size="hero">
                {loading ? "加载中..." : skill?.title ?? skill_name}
              </h2>
              <p className="dialog-subtitle">
                {skill?.description || "正在读取 Skill.md 文档"}
              </p>
            </div>
          </div>
          <button
            className={DIALOG_ICON_BUTTON_CLASS_NAME}
            aria-label="关闭"
            onClick={on_close}
            type="button"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="dialog-body dialog-body--scroll soft-scrollbar flex-1">
          {loading ? (
            <div className="flex min-h-80 items-center justify-center">
              <Loader2 className="h-7 w-7 animate-spin text-(--text-muted)" />
            </div>
          ) : skill ? (
            <>
              <div className="mb-5 flex flex-wrap gap-2">
                <span className={DIALOG_TAG_CLASS_NAME}>
                  {skill.category_name}
                </span>
                <span className={DIALOG_TAG_CLASS_NAME}>
                  {skill.source_type === "system" ? "系统内置" : skill.source_type === "builtin" ? "内置推荐" : "用户导入"}
                </span>
                <span className={DIALOG_TAG_CLASS_NAME}>
                  版本 {skill.version || "unknown"}
                </span>
                {skill.locked ? (
                  <span className={cn(DIALOG_TAG_CLASS_NAME, "text-amber-700")}>
                    <Shield className="h-3 w-3" />
                    系统锁定
                  </span>
                ) : null}
                {skill.tags.map((tag) => (
                  <span
                    key={tag}
                    className={DIALOG_TAG_CLASS_NAME}
                  >
                    <Tag className="h-3 w-3" />
                    {tag}
                  </span>
                ))}
              </div>

              {skill.recommendation ? (
                <div className={getDialogNoteClassName("default", "mb-5")} style={getDialogNoteStyle("default")}>
                  {skill.recommendation}
                </div>
              ) : null}

              {error ? (
                <div className={getDialogNoteClassName("danger", "mb-5")} style={getDialogNoteStyle("danger")}>
                  {error}
                </div>
              ) : null}

              <SkillMarkdown markdown={skill.readme_markdown} />
            </>
          ) : (
            <div className="flex min-h-80 items-center justify-center text-sm text-(--text-muted)">
              未找到该 Skill
            </div>
          )}
        </div>

        <div className="dialog-footer flex-wrap gap-2">
          {skill?.locked ? (
            <button
              className={cn(getDialogActionClassName("default"), "text-amber-700")}
              disabled
              type="button"
            >
              <Lock className="h-4 w-4" />
              系统级
            </button>
          ) : skill ? (
            <>
              {skill.source_type === "external" && skill.has_update ? (
                <button
                  className={getDialogActionClassName("primary")}
                  disabled={acting}
                  onClick={() => void handle_update()}
                  type="button"
                >
                  {acting ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                  更新技能库
                </button>
              ) : (
                <span className={cn(DIALOG_TAG_CLASS_NAME, "px-4 py-2 text-sm")}>
                  {skill.source_type === "external" ? (
                    <>
                      <Check className="h-4 w-4" />
                      已导入到技能库
                    </>
                  ) : (
                    <>
                      <Puzzle className="h-4 w-4" />
                      可在 Agent 中安装
                    </>
                  )}
                </span>
              )}
              {skill.deletable ? (
                <button
                  className={getDialogActionClassName("danger")}
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
