import type {
  SkillInfo,
  SkillSourceType,
} from "@/types/capability/skill";

import type { SkillUpdateCheckNotice } from "../controller/skill-update-check-model";

export type SkillCatalogIcon = "lock" | "puzzle";

export interface SkillCardModel {
  description: string;
  icon: SkillCatalogIcon;
  iconClassName: string | null;
  showDelete: boolean;
  showUpdate: boolean;
  sourceLabel: string;
  stateLabel: string;
  stateTone: "default" | "success" | "warning";
  title: string;
  usageLabel: string | null;
  visibleTags: string[];
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

interface SkillStatePresentation {
  icon: SkillCatalogIcon;
  iconClassName: string | null;
  label: string;
  tone: SkillCardModel["stateTone"];
}

interface SkillStateRule extends SkillStatePresentation {
  matches: (skill: SkillInfo) => boolean;
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

const SKILL_SOURCE_LABEL: Readonly<Record<SkillSourceType, string>> = {
  builtin: "内置推荐",
  external: "外部导入",
  system: "系统内置",
  workspace: "Agent 本地",
};

function getSkillSourceLabel(skill: SkillInfo): string {
  if (skill.source_type === "builtin") {
    if (skill.source_kind === "nexus_platform") return "Nexus 平台库";
    if (skill.source_kind === "user_global") return "用户全局 Skill";
  }
  if (skill.origin_kind === "marketplace") return "第三方市场";
  if (skill.origin_kind === "user_import") return "用户导入";
  return SKILL_SOURCE_LABEL[skill.source_type];
}

const DEFAULT_SKILL_STATE: SkillStatePresentation = {
  icon: "puzzle",
  iconClassName: null,
  label: "全局可用",
  tone: "default",
};

const SKILL_STATE_RULES: readonly SkillStateRule[] = [
  {
    icon: "lock",
    iconClassName: "text-(--warning)",
    label: "系统托管",
    matches: (skill) => skill.locked,
    tone: "warning",
  },
  {
    icon: "puzzle",
    iconClassName: "text-(--status-info-soft-text)",
    label: "Agent 本地",
    matches: (skill) => skill.storage_scope === "agent_workspace"
      || skill.source_type === "workspace",
    tone: "success",
  },
  {
    icon: "puzzle",
    iconClassName: "text-(--status-info-soft-text)",
    label: "用户库",
    matches: (skill) => skill.source_type === "external",
    tone: "success",
  },
];

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
  const state = SKILL_STATE_RULES.find((rule) => rule.matches(skill))
    ?? DEFAULT_SKILL_STATE;
  return {
    description: description || "暂无描述",
    icon: state.icon,
    iconClassName: state.iconClassName,
    showDelete: skill.deletable,
    showUpdate: skill.has_update,
    sourceLabel: getSkillSourceLabel(skill),
    stateLabel: state.label,
    stateTone: state.tone,
    title: skill.title || skill.name,
    usageLabel: skill.scope === "room"
      ? "在 Room 设置中启用"
      : skill.enabled_agent_count
      ? `已用于 ${skill.enabled_agent_count} 个 Agent`
      : "尚未启用",
    visibleTags: skill.tags.slice(0, 2),
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
