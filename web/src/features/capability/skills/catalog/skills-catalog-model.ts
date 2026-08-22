import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { SkillInfo } from "@/types/capability/skill";

import {
  formatSkillUpdateCheckNotice,
  type SkillUpdateCheckNotice,
} from "../controller/skill-update-check-model";

type SkillCatalogLocalization = Pick<I18nContextValue, "locale" | "t">;

export interface SkillCardModel {
  description: string;
  showDelete: boolean;
  showUpdate: boolean;
  title: string;
}

export type SkillUpdateStatus = "checking" | "current" | "failure" | "updates";

export interface SkillsUpdateModel {
  actionDisabled: boolean;
  actionLabel: string;
  badgeLabel: string | null;
  showUpdates: boolean;
  status: SkillUpdateStatus;
  statusLabel: string;
  title: string;
}

interface SkillUpdateContext {
  checkingUpdates: boolean;
  checkUpdateNotice: SkillUpdateCheckNotice | null;
  lastUpdateCheckedAt: number | null;
  updateCount: number;
}

interface SkillUpdateStatusRule {
  matches: (context: SkillUpdateContext) => boolean;
  status: SkillUpdateStatus;
}

const SKILL_UPDATE_STATUS_RULES: readonly SkillUpdateStatusRule[] = [
  {
    matches: ({ checkingUpdates }) => checkingUpdates,
    status: "checking",
  },
  {
    matches: ({ checkUpdateNotice }) => checkUpdateNotice !== null,
    status: "current",
  },
  {
    matches: ({ updateCount }) => updateCount > 0,
    status: "updates",
  },
  {
    matches: () => true,
    status: "current",
  },
];

export function buildSkillCardModel(
  skill: SkillInfo,
  description: string,
): SkillCardModel {
  return {
    description,
    showDelete: skill.deletable,
    showUpdate: skill.has_update,
    title: skill.title || skill.name,
  };
}

export function buildSkillsUpdateModel(
  context: SkillUpdateContext,
  localization: SkillCatalogLocalization,
): SkillsUpdateModel | null {
  const shouldShow = context.checkingUpdates
    || context.checkUpdateNotice !== null
    || context.updateCount > 0;
  if (!shouldShow) {
    return null;
  }
  const status = SKILL_UPDATE_STATUS_RULES.find((rule) => rule.matches(context))
    ?.status ?? "current";
  const noticeStatus = context.checkUpdateNotice?.status;
  return {
    actionDisabled: context.checkingUpdates,
    actionLabel: context.checkingUpdates
      ? localization.t("capability.skills_checking")
      : localization.t("capability.skills_recheck"),
    badgeLabel: context.updateCount > 0
      ? localization.t("capability.skills_updates_count", {
        count: context.updateCount,
      })
      : null,
    showUpdates: context.updateCount > 0,
    status: context.checkingUpdates ? "checking" : noticeStatus ?? status,
    statusLabel: buildSkillUpdateStatusLabel(context, localization),
    title: context.updateCount > 0
      ? localization.t("capability.skills_updates_title")
      : localization.t("capability.skills_update_check_title"),
  };
}

function buildSkillUpdateStatusLabel(
  context: SkillUpdateContext,
  localization: SkillCatalogLocalization,
): string {
  if (context.checkingUpdates) {
    return localization.t("capability.skills_checking_remote");
  }
  if (context.checkUpdateNotice) {
    return formatSkillUpdateCheckNotice(
      context.checkUpdateNotice,
      localization.t,
    );
  }
  return localization.t("capability.skills_last_checked", {
    time: formatCheckedTime(context.lastUpdateCheckedAt, localization),
  });
}

function formatCheckedTime(
  value: number | null,
  localization: SkillCatalogLocalization,
): string {
  if (!value) {
    return localization.t("capability.skills_never_checked");
  }
  return new Date(value).toLocaleString(
    localization.locale === "zh" ? "zh-CN" : "en-US",
    {
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      month: "2-digit",
    },
  );
}
