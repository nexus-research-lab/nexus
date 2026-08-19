import {
  type ComponentType,
  type RefObject,
} from "react";
import { FolderUp, GitBranch, Loader2 } from "lucide-react";

import { UiButton } from "@/shared/ui/button/button";
import { useI18n } from "@/shared/i18n/i18n-context";
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
  const { t } = useI18n();
  return (
    <div className="inline-flex rounded-[10px] border border-(--divider-subtle-color) p-1">
      {SKILL_IMPORT_MODES.map((option) => {
        const Icon = MODE_ICONS[option.key];
        const isActive = mode === option.key;
        return (
          <button
            className={cn(
              "inline-flex min-h-8 items-center gap-1.5 radius-control-sm px-3 text-xs font-medium transition-[background,color]",
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
            {t(option.labelKey)}
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
  const { t } = useI18n();
  return (
    <div className="space-y-4">
      <UiField
        description={t("capability.skills_import_git_url_description")}
        htmlFor="skill-import-git-url"
        label={t("capability.skills_import_git_url")}
        required
      >
        <UiInput
          disabled={importing}
          id="skill-import-git-url"
          onChange={(event) => setDraftField("url", event.target.value)}
          placeholder="https://github.com/owner/repo.git"
          ref={gitUrlInputRef}
          required
          type="url"
          value={draft.url}
        />
      </UiField>
      <div className="grid gap-3 sm:grid-cols-2">
        <UiField
          description={t("capability.skills_import_git_branch_description")}
          htmlFor="skill-import-git-branch"
          label={t("capability.skills_import_git_branch")}
        >
          <UiInput
            disabled={importing}
            id="skill-import-git-branch"
            onChange={(event) => setDraftField("branch", event.target.value)}
            placeholder="main"
            value={draft.branch}
          />
        </UiField>
        <UiField
          description={t("capability.skills_import_git_path_description")}
          htmlFor="skill-import-git-path"
          label={t("capability.skills_import_git_path")}
        >
          <UiInput
            disabled={importing}
            id="skill-import-git-path"
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
  const { t } = useI18n();
  return (
    <div className="rounded-[10px] border border-(--divider-subtle-color) px-3 py-3">
      <div className="flex items-start gap-3">
        <div className="flex h-8 w-8 shrink-0 items-center justify-center radius-control-sm bg-[color:color-mix(in_srgb,var(--primary)_9%,transparent)] text-(--primary)">
          <FolderUp className="h-4 w-4" />
        </div>
        <div className="min-w-0">
          <h3 className="text-sm font-medium text-(--text-strong)">
            {t("capability.skills_import_zip_title")}
          </h3>
          <p className="mt-1 text-compact leading-5 text-(--text-muted)">
            {t("capability.skills_import_zip_description")}
          </p>
          <UiButton
            className="mt-3"
            disabled={importing}
            onClick={() => fileInputRef.current?.click()}
            size="sm"
            tone="primary"
            variant="solid"
          >
            <ImportingIcon importing={importing} />
            {importing
              ? t("capability.skills_importing")
              : t("capability.skills_import_choose_zip")}
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
