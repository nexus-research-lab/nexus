/**
 * INPUT: 当前导入模式、Git 草稿与本地文件入口。
 * OUTPUT: 文字分段选择及对应的单一导入表单。
 * POS: Skill 导入弹窗的主内容，不承载格式教程。
 */
import {
  type ComponentType,
  type RefObject,
} from "react";
import { FolderUp, Loader2 } from "lucide-react";

import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { UiSegmentedControl } from "@/shared/ui/form/segmented-control";
import { UiPanel } from "@/shared/ui/panel";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

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

function SkillImportModeTabs({
  importing,
  mode,
  onSelectMode,
}: Pick<SkillImportSourceProps, "importing" | "mode" | "onSelectMode">) {
  const { t } = useI18n();
  return (
    <UiSegmentedControl
      density="compact"
      disabled={importing}
      onChange={onSelectMode}
      options={SKILL_IMPORT_MODES.map((option) => ({
        label: t(option.labelKey),
        value: option.key,
      }))}
      title={t("capability.skills_import_title")}
      value={mode}
    />
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
    ? <Loader2 className={getUiSpinnerClassName()} />
    : <FolderUp className="h-4 w-4" />;
}

function LocalSkillImportSource({
  fileInputRef,
  importing,
}: SourceViewProps) {
  const { t } = useI18n();
  return (
    <UiPanel padding="lg" radius="md" variant="dashed">
      <div className="min-w-0 text-center">
        <h3 className={getUiTypographyClassName({
          role: "sectionTitle",
          tone: "strong",
          weight: "medium",
        })}>
          {t("capability.skills_import_zip_title")}
        </h3>
        <p className={cn(
          "mt-1",
          getUiTypographyClassName({ role: "metadata", tone: "muted" }),
        )}>
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
    </UiPanel>
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
