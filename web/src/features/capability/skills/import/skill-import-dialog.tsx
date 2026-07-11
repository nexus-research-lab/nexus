"use client";

import {
  type FormEvent,
  type RefObject,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import { Download, FileText, FolderUp, GitBranch, Info, Loader2, PackageCheck } from "lucide-react";

import { cn } from "@/lib/utils";
import { UiButton } from "@/shared/ui/button/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import { UiField, UiInput } from "@/shared/ui/form/form-control";

import type { SkillImportDialogMode } from "../controller/skill-marketplace-controller";
import roomCollaborationMechanismMarkdown from "../../../../../../docs/specs/room-collaboration-mechanism.md?raw";

interface SkillImportDialogProps {
  fileInputRef: RefObject<HTMLInputElement | null>;
  importing: boolean;
  mode: SkillImportDialogMode | null;
  onClose: () => void;
  onImportGit: (url: string, branch?: string, path?: string) => void;
  onSelectMode: (mode: SkillImportDialogMode) => void;
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

const ROOM_COLLABORATION_MECHANISM_FILE_NAME = "room-collaboration-mechanism.md";

function downloadRoomCollaborationMechanism() {
  const blob = new Blob([roomCollaborationMechanismMarkdown], {
    type: "text/markdown;charset=utf-8",
  });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = ROOM_COLLABORATION_MECHANISM_FILE_NAME;
  anchor.rel = "noopener";
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}

export function SkillImportDialog({
  fileInputRef,
  importing,
  mode,
  onClose,
  onImportGit,
  onSelectMode,
}: SkillImportDialogProps) {
  const [gitUrl, setGitUrl] = useState("");
  const [gitBranch, setGitBranch] = useState("");
  const [gitPath, setGitPath] = useState("");
  const gitUrlInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!mode) {
      setGitUrl("");
      setGitBranch("");
      setGitPath("");
    } else if (mode === "git") {
      gitUrlInputRef.current?.focus();
    }
  }, [mode]);

  const handleClose = useCallback(() => {
    if (importing) return;
    onClose();
  }, [importing, onClose]);

  if (!mode) return null;

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (importing) return;
    if (mode !== "git") return;
    onImportGit(gitUrl, gitBranch, gitPath);
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop className="z-[9999]" onClose={importing ? undefined : handleClose}>
        <UiDialogFormShell className="max-h-[86vh]" onSubmit={handleSubmit} size="xl">
          <UiDialogHeader
            icon={mode === "git" ? <GitBranch className="h-4 w-4" /> : <PackageCheck className="h-4 w-4" />}
            onClose={importing ? undefined : handleClose}
            subtitle="导入前请确认目录内包含合法的 SKILL.md，Room 技能需要显式声明 scope: room。"
            title="导入 Skill"
          />

          <UiDialogBody className="grid min-h-0 gap-5 lg:grid-cols-[minmax(0,1fr)_340px]" scrollable>
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
                    disabled={importing}
                    onClick={() => onSelectMode(item)}
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
                      aria-label="Git 仓库 URL"
                      disabled={importing}
                      onChange={(event) => setGitUrl(event.target.value)}
                      placeholder="https://github.com/owner/repo.git"
                      ref={gitUrlInputRef}
                      required
                      type="url"
                      value={gitUrl}
                    />
                  </UiField>
                  <div className="grid gap-3 sm:grid-cols-2">
                    <UiField
                      description="留空时使用仓库默认分支。"
                      label="Branch"
                    >
                      <UiInput
                        disabled={importing}
                        onChange={(event) => setGitBranch(event.target.value)}
                        placeholder="main"
                        value={gitBranch}
                      />
                    </UiField>
                    <UiField
                      description="Skill 不在仓库根目录时填写，例如 skills/werewolf-6p。"
                      label="子目录 Path"
                    >
                      <UiInput
                        disabled={importing}
                        onChange={(event) => setGitPath(event.target.value)}
                        placeholder="skills/room-playbook"
                        value={gitPath}
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
                        className="mt-4"
                        disabled={importing}
                        onClick={() => fileInputRef.current?.click()}
                        size="sm"
                        tone="primary"
                        variant="solid"
                      >
                        {importing ? <Loader2 className="h-4 w-4 animate-spin" /> : <FolderUp className="h-4 w-4" />}
                        {importing ? "导入中" : "选择 zip 文件"}
                      </UiButton>
                    </div>
                  </div>
                </div>
              )}
            </section>

            <aside className="space-y-3">
              <div className="rounded-[12px] border border-(--divider-subtle-color) px-4 py-3">
                <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                  <div className="flex items-center gap-2 text-sm font-semibold text-(--text-strong)">
                    <Info className="h-4 w-4 text-(--primary)" />
                    SKILL.md 规范
                  </div>
                  <UiButton
                    aria-label="下载 Room 协作机制 Markdown 文档"
                    className="shrink-0"
                    disabled={importing}
                    onClick={downloadRoomCollaborationMechanism}
                    size="xs"
                    tone="primary"
                    variant="surface"
                  >
                    <FileText className="h-3.5 w-3.5" />
                    room协作机制
                  </UiButton>
                </div>
                <ul className="space-y-1.5 text-xs leading-5 text-(--text-muted)">
                  <li>必须包含 `name`，推荐补齐 `title`、`description`、`tags`。</li>
                  <li>`scope: any` 可安装到 Agent；`scope: main` 只给主 Agent；`scope: room` 只给群聊。</li>
                  <li>编写 Room Skill 时，把“room协作机制”文档交给 agent 参考，先明确公开协作和私下协作的边界。</li>
                  <li>Room Skill 导入后在群聊管理弹窗的“群聊技能”里选择，不会安装到单个 Agent。</li>
                  <li>Git 导入会保存 URL、branch、path 和 commit，后续检查更新会按这些信息比对远端版本。</li>
                </ul>
              </div>

              <pre className="max-h-[260px] overflow-auto rounded-[12px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_92%,black_2%)] p-3 text-[11px] leading-5 text-(--text-default)">
                {SKILL_FRONTMATTER_EXAMPLE}
              </pre>
            </aside>
          </UiDialogBody>

          <UiDialogFooter className="gap-2">
            <UiButton disabled={importing} onClick={handleClose} size="sm" variant="surface">
              取消
            </UiButton>
            {mode === "git" ? (
              <UiButton
                disabled={importing || !gitUrl.trim()}
                size="sm"
                tone="primary"
                type="submit"
                variant="solid"
              >
                {importing ? <Loader2 className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
                {importing ? "导入中" : "导入 Git Skill"}
              </UiButton>
            ) : null}
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
