import type { SkillImportDialogMode } from "../controller/skill-marketplace-controller";

export interface GitSkillImportDraft {
  branch: string;
  path: string;
  url: string;
}

export const EMPTY_GIT_SKILL_IMPORT_DRAFT: GitSkillImportDraft = {
  branch: "",
  path: "",
  url: "",
};

export const SKILL_IMPORT_MODES: Array<{
  key: SkillImportDialogMode;
  label: string;
}> = [
  { key: "local", label: "本地 zip" },
  { key: "git", label: "Git 仓库" },
];

export function canSubmitGitSkillImport(
  mode: SkillImportDialogMode | null,
  importing: boolean,
  draft: GitSkillImportDraft,
): boolean {
  return mode === "git" && !importing && Boolean(draft.url.trim());
}
