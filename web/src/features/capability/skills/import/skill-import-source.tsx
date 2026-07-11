import {
  type ComponentType,
  type RefObject,
} from "react";
import { FolderUp, GitBranch, Loader2 } from "lucide-react";

import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiField, UiInput } from "@/shared/ui/form/form-control";

import type { SkillImportDialogMode } from "../controller/skill-marketplace-controller";
import {
  SKILL_IMPORT_MODES,
  type GitSkillImportDraft,
} from "./skill-import-dialog-model";

interface SkillImportSourceProps {
  draft: GitSkillImportDraft;
  fileInputRef: RefObject<HTMLInputElement | null>;
  gitUrlInputRef: RefObject<HTMLInputElement | null>;
  importing: boolean;
  mode: SkillImportDialogMode;
  onSelectMode: (mode: SkillImportDialogMode) => void;
  setDraftField: <Key extends keyof GitSkillImportDraft>(
    key: Key,
    value: GitSkillImportDraft[Key],
  ) => void;
}

interface SourceViewProps extends Omit<
  SkillImportSourceProps,
  "mode" | "onSelectMode"
> {}

const MODE_ICONS: Record<SkillImportDialogMode, ComponentType<{ className?: string }>> = {
  git: GitBranch,
  local: FolderUp,
};

function SkillImportModeTabs({
  importing,
  mode,
  onSelectMode,
}: Pick<SkillImportSourceProps, "importing" | "mode" | "onSelectMode">) {
  return (
    <div className="inline-flex rounded-[12px] border border-(--divider-subtle-color) p-1">
      {SKILL_IMPORT_MODES.map((option) => {
        const Icon = MODE_ICONS[option.key];
        const isActive = mode === option.key;
        return (
          <button
            className={cn(
              "inline-flex min-h-8 items-center gap-1.5 rounded-[9px] px-3 text-xs font-semibold transition-[background,color]",
              isActive
                ? "bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-(--primary)"
                : "text-(--text-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
            )}
            disabled={importing}
            key={option.key}
            onClick={() => onSelectMode(option.key)}
            type="button"
          >
            <Icon className="h-3.5 w-3.5" />
            {option.label}
          </button>
        );
      })}
    </div>
  );
}

function GitSkillImportSource({
  draft,
  gitUrlInputRef,
  importing,
  setDraftField,
}: SourceViewProps) {
  return (
    <div className="space-y-4">
      <UiField
        description="必须是 https:// 地址；仓库根目录或指定子目录内需要有 SKILL.md。"
        label="Git 仓库 URL"
      >
        <UiInput
          aria-label="Git 仓库 URL"
          disabled={importing}
          onChange={(event) => setDraftField("url", event.target.value)}
          placeholder="https://github.com/owner/repo.git"
          ref={gitUrlInputRef}
          required
          type="url"
          value={draft.url}
        />
      </UiField>
      <div className="grid gap-3 sm:grid-cols-2">
        <UiField description="留空时使用仓库默认分支。" label="Branch">
          <UiInput
            disabled={importing}
            onChange={(event) => setDraftField("branch", event.target.value)}
            placeholder="main"
            value={draft.branch}
          />
        </UiField>
        <UiField
          description="Skill 不在仓库根目录时填写，例如 skills/werewolf-6p。"
          label="子目录 Path"
        >
          <UiInput
            disabled={importing}
            onChange={(event) => setDraftField("path", event.target.value)}
            placeholder="skills/room-playbook"
            value={draft.path}
          />
        </UiField>
      </div>
    </div>
  );
}

function ImportingIcon({ importing }: { importing: boolean }) {
  return importing
    ? <Loader2 className="h-4 w-4 animate-spin" />
    : <FolderUp className="h-4 w-4" />;
}

function LocalSkillImportSource({
  fileInputRef,
  importing,
}: SourceViewProps) {
  return (
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
            <ImportingIcon importing={importing} />
            {importing ? "导入中" : "选择 zip 文件"}
          </UiButton>
        </div>
      </div>
    </div>
  );
}

const SOURCE_VIEWS: Record<
  SkillImportDialogMode,
  ComponentType<SourceViewProps>
> = {
  git: GitSkillImportSource,
  local: LocalSkillImportSource,
};

export function SkillImportSource({
  mode,
  onSelectMode,
  ...props
}: SkillImportSourceProps) {
  const Source = SOURCE_VIEWS[mode];
  return (
    <section className="space-y-4">
      <SkillImportModeTabs
        importing={props.importing}
        mode={mode}
        onSelectMode={onSelectMode}
      />
      <Source {...props} />
    </section>
  );
}
