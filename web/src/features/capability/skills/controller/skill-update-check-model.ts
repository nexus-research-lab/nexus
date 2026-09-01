import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { SkillActionFailure } from "@/types/capability/skill";

export type SkillUpdateCheckNoticeStatus = "current" | "failure" | "updates";

export interface SkillUpdateCheckNotice {
  availableCount: number;
  failure: SkillUpdateCheckFailure | null;
  status: SkillUpdateCheckNoticeStatus;
}

interface SkillUpdateCheckFailure {
  additionalCount: number;
  skillName: string;
}

export function buildSkillUpdateCheckNotice(
  availableCount: number,
  failures: SkillActionFailure[],
  manual: boolean,
): SkillUpdateCheckNotice | null {
  const failure = buildFailure(failures);
  if (availableCount > 0) {
    return {
      availableCount,
      failure,
      status: "updates",
    };
  }
  if (failure) {
    return {
      availableCount,
      failure,
      status: "failure",
    };
  }
  return manual
    ? { availableCount, failure: null, status: "current" }
    : null;
}

function buildFailure(
  failures: SkillActionFailure[],
): SkillUpdateCheckFailure | null {
  if (failures.length === 0) {
    return null;
  }
  const failure = failures[0];
  return {
    additionalCount: Math.max(0, failures.length - 1),
    skillName: failure?.skill_name.trim() || "Skill",
  };
}

export function formatSkillUpdateCheckNotice(
  notice: SkillUpdateCheckNotice,
  t: I18nContextValue["t"],
): string {
  const failure = notice.failure
    ? formatFailure(notice.failure, t)
    : null;
  if (notice.availableCount > 0) {
    const updates = t("capability.skills_updates_found", {
      count: notice.availableCount,
    });
    return failure
      ? t("capability.skills_updates_found_with_failure", {
        failure,
        updates,
      })
      : updates;
  }
  return failure || t("capability.skills_up_to_date");
}

function formatFailure(
  failure: SkillUpdateCheckFailure,
  t: I18nContextValue["t"],
): string {
  const message = t("capability.skills_check_failed", { name: failure.skillName });
  return failure.additionalCount > 0
    ? t("capability.skills_check_failed_more", {
      count: failure.additionalCount,
      message,
    })
    : message;
}
