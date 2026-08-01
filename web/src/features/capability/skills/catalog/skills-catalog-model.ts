import type { SkillInfo } from "@/types/capability/skill";

import type { SkillUpdateCheckNotice } from "../controller/skill-update-check-model";

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
  description = skill.description,
): SkillCardModel {
  return {
    description: description || "暂无描述",
    showDelete: skill.deletable,
    showUpdate: skill.has_update,
    title: skill.title || skill.name,
  };
}

export function buildSkillsUpdateModel(
  context: SkillUpdateContext,
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
    actionLabel: context.checkingUpdates ? "检查中" : "重新检查",
    badgeLabel: context.updateCount > 0
      ? `${context.updateCount} 个可更新`
      : null,
    showUpdates: context.updateCount > 0,
    status: context.checkingUpdates ? "checking" : noticeStatus ?? status,
    statusLabel: buildSkillUpdateStatusLabel(context),
    title: context.updateCount > 0 ? "可更新 Skill" : "更新检查",
  };
}

function buildSkillUpdateStatusLabel(context: SkillUpdateContext): string {
  if (context.checkingUpdates) {
    return "正在检查远端版本...";
  }
  if (context.checkUpdateNotice) {
    return context.checkUpdateNotice.message;
  }
  return `上次检查 ${formatCheckedTime(context.lastUpdateCheckedAt)}`;
}

function formatCheckedTime(value: number | null): string {
  if (!value) {
    return "尚未检查";
  }
  return new Date(value).toLocaleString("zh-CN", {
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    month: "2-digit",
  });
}
