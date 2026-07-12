import type {
  SkillInfo,
  SkillSourceType,
} from "@/types/capability/skill";

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
  checkUpdateMessage: string | null;
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
  workspace: "工作区技能",
};

const DEFAULT_SKILL_STATE: SkillStatePresentation = {
  icon: "puzzle",
  iconClassName: null,
  label: "可安装",
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
    label: "工作区内",
    matches: (skill) => skill.source_type === "workspace",
    tone: "success",
  },
  {
    icon: "puzzle",
    iconClassName: "text-(--status-info-soft-text)",
    label: "已导入",
    matches: (skill) => skill.source_type === "external",
    tone: "success",
  },
  {
    icon: "puzzle",
    iconClassName: "text-(--success)",
    label: "已安装",
    matches: (skill) => skill.installed,
    tone: "success",
  },
];

const SKILL_UPDATE_STATUS_RULES: readonly SkillUpdateStatusRule[] = [
  {
    matches: ({ checkingUpdates }) => checkingUpdates,
    status: "checking",
  },
  {
    matches: ({ checkUpdateMessage }) => (
      checkUpdateMessage?.includes("无法检查") ?? false
    ),
    status: "failure",
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

export function buildSkillCardModel(skill: SkillInfo): SkillCardModel {
  const state = SKILL_STATE_RULES.find((rule) => rule.matches(skill))
    ?? DEFAULT_SKILL_STATE;
  return {
    description: skill.description || "暂无描述",
    icon: state.icon,
    iconClassName: state.iconClassName,
    showDelete: skill.deletable,
    showUpdate: skill.has_update,
    sourceLabel: SKILL_SOURCE_LABEL[skill.source_type],
    stateLabel: state.label,
    stateTone: state.tone,
    title: skill.title || skill.name,
    visibleTags: skill.tags.slice(0, 2),
  };
}

export function buildSkillsUpdateModel(
  context: SkillUpdateContext,
): SkillsUpdateModel | null {
  const shouldShow = context.checkingUpdates
    || Boolean(context.checkUpdateMessage)
    || context.updateCount > 0;
  if (!shouldShow) {
    return null;
  }
  const status = SKILL_UPDATE_STATUS_RULES.find((rule) => rule.matches(context))
    ?.status ?? "current";
  return {
    actionDisabled: context.checkingUpdates,
    actionLabel: context.checkingUpdates ? "检查中" : "重新检查",
    badgeLabel: context.updateCount > 0
      ? `${context.updateCount} 个可更新`
      : null,
    showUpdates: context.updateCount > 0,
    status,
    statusLabel: buildSkillUpdateStatusLabel(context),
    title: context.updateCount > 0 ? "可更新 Skill" : "更新检查",
  };
}

function buildSkillUpdateStatusLabel(context: SkillUpdateContext): string {
  if (context.checkingUpdates) {
    return "正在检查远端版本...";
  }
  if (context.checkUpdateMessage) {
    return context.checkUpdateMessage;
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
