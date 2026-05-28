"use client";

import { type FormEvent, useCallback, useEffect, useState } from "react";
import { Download, FolderUp, GitBranch, Info, PackageCheck } from "lucide-react";

import { cn } from "@/lib/utils";
import { UiButton } from "@/shared/ui/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import { UiField, UiInput } from "@/shared/ui/form-control";

import type { SkillImportDialogMode, SkillMarketplaceController } from "./skills-view-model";

interface SkillImportDialogProps {
  ctrl: SkillMarketplaceController;
}

const SKILL_FRONTMATTER_EXAMPLE = `---
name: room-playbook
title: 群聊协作规则
description: 群聊中的协作流程和成员行为约束
scope: room
tags: [room, workflow]
---

# 群聊协作规则`;

const MODE_LABELS: Record<SkillImportDialogMode, string> = {
  local: "本地 zip",
  git: "Git 仓库",
};

export function SkillImportDialog({ ctrl }: SkillImportDialogProps) {
  const mode = ctrl.import_dialog_mode;
  const set_import_dialog_mode = ctrl.set_import_dialog_mode;
  const [git_url, set_git_url] = useState("");
  const [git_branch, set_git_branch] = useState("");
  const [git_path, set_git_path] = useState("");

  useEffect(() => {
    if (!mode) {
      set_git_url("");
      set_git_branch("");
      set_git_path("");
    }
  }, [mode]);

  const handle_close = useCallback(() => {
    set_import_dialog_mode(null);
  }, [set_import_dialog_mode]);

  if (!mode) return null;

  const handle_submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (mode !== "git") return;
    void ctrl.handle_git_import(git_url, git_branch, git_path);
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop class_name="z-[9999]" on_close={handle_close}>
        <UiDialogFormShell class_name="max-h-[86vh]" onSubmit={handle_submit} size="xl">
          <UiDialogHeader
            icon={mode === "git" ? <GitBranch className="h-4 w-4" /> : <PackageCheck className="h-4 w-4" />}
            on_close={handle_close}
            subtitle="导入前请确认目录内包含合法的 SKILL.md，Room 技能需要显式声明 scope: room。"
            title="导入 Skill"
          />

          <UiDialogBody class_name="grid min-h-0 gap-5 lg:grid-cols-[minmax(0,1fr)_340px]" scrollable>
            <section className="space-y-4">
              <div className="inline-flex rounded-[12px] border border-(--divider-subtle-color) p-1">
                {(["local", "git"] as SkillImportDialogMode[]).map((item) => (
                  <button
                    key={item}
                    className={cn(
                      "inline-flex min-h-8 items-center gap-1.5 rounded-[9px] px-3 text-xs font-semibold transition-[background,color]",
                      mode === item
                        ? "bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-(--primary)"
                        : "text-(--text-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                    )}
                    onClick={() => ctrl.set_import_dialog_mode(item)}
                    type="button"
                  >
                    {item === "git" ? <GitBranch className="h-3.5 w-3.5" /> : <FolderUp className="h-3.5 w-3.5" />}
                    {MODE_LABELS[item]}
                  </button>
                ))}
              </div>

              {mode === "git" ? (
                <div className="space-y-4">
                  <UiField
                    description="必须是 https:// 地址；仓库根目录或指定子目录内需要有 SKILL.md。"
                    label="Git 仓库 URL"
                  >
                    <UiInput
                      autoFocus
                      onChange={(event) => set_git_url(event.target.value)}
                      placeholder="https://github.com/owner/repo.git"
                      required
                      type="url"
                      value={git_url}
                    />
                  </UiField>
                  <div className="grid gap-3 sm:grid-cols-2">
                    <UiField
                      description="留空时使用仓库默认分支。"
                      label="Branch"
                    >
                      <UiInput
                        onChange={(event) => set_git_branch(event.target.value)}
                        placeholder="main"
                        value={git_branch}
                      />
                    </UiField>
                    <UiField
                      description="Skill 不在仓库根目录时填写，例如 skills/werewolf-6p。"
                      label="子目录 Path"
                    >
                      <UiInput
                        onChange={(event) => set_git_path(event.target.value)}
                        placeholder="skills/room-playbook"
                        value={git_path}
                      />
                    </UiField>
                  </div>
                </div>
              ) : (
                <div className="rounded-[12px] border border-(--divider-subtle-color) px-4 py-4">
                  <div className="flex items-start gap-3">
                    <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[10px] bg-[color:color-mix(in_srgb,var(--primary)_9%,transparent)] text-(--primary)">
                      <FolderUp className="h-4 w-4" />
                    </div>
                    <div className="min-w-0">
                      <h3 className="text-sm font-semibold text-(--text-strong)">上传 zip 包</h3>
                      <p className="mt-1 text-xs leading-5 text-(--text-muted)">
                        zip 内可以直接放一个 Skill 目录，也可以包含多层目录；系统会查找最靠近根部的 SKILL.md。
                      </p>
                      <UiButton
                        class_name="mt-4"
                        onClick={() => ctrl.file_input_ref.current?.click()}
                        size="sm"
                        tone="primary"
                        variant="solid"
                      >
                        <FolderUp className="h-4 w-4" />
                        选择 zip 文件
                      </UiButton>
                    </div>
                  </div>
                </div>
              )}
            </section>

            <aside className="space-y-3">
              <div className="rounded-[12px] border border-(--divider-subtle-color) px-4 py-3">
                <div className="mb-2 flex items-center gap-2 text-sm font-semibold text-(--text-strong)">
                  <Info className="h-4 w-4 text-(--primary)" />
                  SKILL.md 规范
                </div>
                <ul className="space-y-1.5 text-xs leading-5 text-(--text-muted)">
                  <li>必须包含 `name`，推荐补齐 `title`、`description`、`tags`。</li>
                  <li>`scope: any` 可安装到 Agent；`scope: main` 只给主 Agent；`scope: room` 只给群聊。</li>
                  <li>Room Skill 导入后在群聊管理弹窗的“群聊技能”里选择，不会安装到单个 Agent。</li>
                  <li>Git 导入会保存 URL、branch、path 和 commit，后续“更新技能库”会按这些信息重新拉取。</li>
                </ul>
              </div>

              <pre className="max-h-[260px] overflow-auto rounded-[12px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_92%,black_2%)] p-3 text-[11px] leading-5 text-(--text-default)">
                {SKILL_FRONTMATTER_EXAMPLE}
              </pre>
            </aside>
          </UiDialogBody>

          <UiDialogFooter class_name="gap-2">
            <UiButton onClick={handle_close} size="sm" variant="surface">
              取消
            </UiButton>
            {mode === "git" ? (
              <UiButton size="sm" tone="primary" type="submit" variant="solid">
                <Download className="h-4 w-4" />
                导入 Git Skill
              </UiButton>
            ) : null}
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
