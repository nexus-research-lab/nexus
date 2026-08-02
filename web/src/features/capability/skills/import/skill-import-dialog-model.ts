import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";

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
  labelKey: TranslationKey;
}> = [
  { key: "local", labelKey: "capability.skills_import_mode_local" },
  { key: "git", labelKey: "capability.skills_import_mode_git" },
];

export function buildSkillFrontmatterExample(
  t: I18nContextValue["t"],
): string {
  const title = t("capability.skills_import_example_title");
  const description = t("capability.skills_import_example_description");
  return `---
name: room-playbook
title: ${title}
description: ${description}
scope: room
tags: [room, workflow]
---

# ${title}`;
}

export function canSubmitGitSkillImport(
  mode: SkillImportDialogMode | null,
  importing: boolean,
  draft: GitSkillImportDraft,
): boolean {
  return mode === "git" && !importing && Boolean(draft.url.trim());
}
