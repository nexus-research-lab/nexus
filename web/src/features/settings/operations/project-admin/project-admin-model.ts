// INPUT: 项目目录读取结果、权限修改证据和当前语言。
// OUTPUT: 区分未应用、结果未知、已保存但刷新失败的项目管理反馈。
// POS: Project Admin 的纯失败展示模型；不执行刷新或重复 mutation。
import {
  getErrorMessage,
  projectMutationFailure,
} from "@/lib/error-message";
import type { TranslationKey } from "@/shared/i18n/messages";
import type { ProjectAccess, SharedProject } from "@/types/settings/project";

export interface ProjectFeedback {
  blocksMutation?: boolean;
  impact?: string;
  message: string;
  nextStep?: string;
  recoveryAction?: "refresh";
  title: string;
  tone: "success" | "error" | "warning";
}

export interface ProjectAdminViewModel {
  canManageMembers: boolean;
  feedback: ProjectFeedback | null;
  loading: boolean;
  memberDrafts: Record<string, string>;
  mutationsBlocked: boolean;
  newProjectId: string;
  pendingKey: string | null;
  projects: SharedProject[];
}

export type ProjectFeedbackEvent =
  | "create-invalid"
  | "create-failed"
  | "create-succeeded"
  | "grant-failed"
  | "grant-refresh-failed"
  | "grant-succeeded"
  | "load-failed";

export const PROJECT_ACCESS_VALUES: readonly ProjectAccess[] = [
  "read",
  "write",
  "none",
];

const FEEDBACK_COPY: Record<
  ProjectFeedbackEvent,
  {
    impact?: TranslationKey;
    message: TranslationKey;
    nextStep?: TranslationKey;
    recoveryAction?: ProjectFeedback["recoveryAction"];
    title: TranslationKey;
    tone: ProjectFeedback["tone"];
  }
> = {
  "create-invalid": {
    impact: "settings.projects.create_invalid_impact",
    message: "settings.projects.create_invalid_message",
    nextStep: "settings.projects.create_invalid_next_step",
    title: "settings.projects.create_invalid_title",
    tone: "error",
  },
  "create-failed": {
    message: "settings.projects.create_failed_message",
    title: "settings.projects.create_failed_title",
    tone: "error",
  },
  "create-succeeded": {
    message: "settings.projects.create_success_message",
    title: "settings.projects.create_success_title",
    tone: "success",
  },
  "grant-failed": {
    message: "settings.projects.grant_failed_message",
    title: "settings.projects.grant_failed_title",
    tone: "error",
  },
  "grant-refresh-failed": {
    impact: "settings.projects.grant_refresh_failed_impact",
    message: "settings.projects.grant_refresh_failed_message",
    nextStep: "settings.projects.grant_refresh_failed_next_step",
    recoveryAction: "refresh",
    title: "settings.projects.grant_refresh_failed_title",
    tone: "warning",
  },
  "grant-succeeded": {
    message: "settings.projects.grant_success_message",
    title: "settings.projects.grant_success_title",
    tone: "success",
  },
  "load-failed": {
    impact: "state.read_failure_impact",
    message: "settings.projects.load_failed_message",
    nextStep: "state.retry_next_step",
    recoveryAction: "refresh",
    title: "settings.projects.load_failed_title",
    tone: "error",
  },
};

const PROJECT_MUTATION_COPY = {
  create: {
    accepted: {
      impact: "settings.projects.create_accepted_impact",
      nextStep: "settings.projects.create_accepted_next_step",
      title: "settings.projects.create_accepted_title",
    },
    committed: {
      impact: "settings.projects.create_committed_impact",
      nextStep: "settings.projects.create_committed_next_step",
      title: "settings.projects.create_committed_title",
    },
    not_applied: {
      impact: "settings.projects.create_not_applied_impact",
      nextStep: "settings.projects.create_not_applied_next_step",
      title: "settings.projects.create_not_applied_title",
    },
    unknown: {
      impact: "settings.projects.create_unknown_impact",
      nextStep: "settings.projects.create_unknown_next_step",
      title: "settings.projects.create_unknown_title",
    },
  },
  grant: {
    accepted: {
      impact: "settings.projects.grant_accepted_impact",
      nextStep: "settings.projects.grant_accepted_next_step",
      title: "settings.projects.grant_accepted_title",
    },
    committed: {
      impact: "settings.projects.grant_committed_impact",
      nextStep: "settings.projects.grant_committed_next_step",
      title: "settings.projects.grant_committed_title",
    },
    not_applied: {
      impact: "settings.projects.grant_not_applied_impact",
      nextStep: "settings.projects.grant_not_applied_next_step",
      title: "settings.projects.grant_not_applied_title",
    },
    unknown: {
      impact: "settings.projects.grant_unknown_impact",
      nextStep: "settings.projects.grant_unknown_next_step",
      title: "settings.projects.grant_unknown_title",
    },
  },
} as const satisfies Record<
  "create" | "grant",
  Record<"accepted" | "committed" | "not_applied" | "unknown", {
    impact: TranslationKey;
    nextStep: TranslationKey;
    title: TranslationKey;
  }>
>;

export function buildProjectFeedback(
  translate: (key: TranslationKey) => string,
  event: ProjectFeedbackEvent,
  error?: unknown,
): ProjectFeedback {
  const copy = FEEDBACK_COPY[event];
  return {
    impact: copy.impact ? translate(copy.impact) : undefined,
    message: getErrorMessage(error, translate(copy.message)),
    nextStep: copy.nextStep ? translate(copy.nextStep) : undefined,
    recoveryAction: copy.recoveryAction,
    title: translate(copy.title),
    tone: copy.tone,
  };
}

export function buildProjectMutationFeedback(
  translate: (key: TranslationKey) => string,
  operation: "create" | "grant",
  error: unknown,
): ProjectFeedback {
  const fallback = translate(operation === "create"
    ? "settings.projects.create_failed_message"
    : "settings.projects.grant_failed_message");
  const failure = projectMutationFailure(error, fallback);
  const outcome = failure.effect === "accepted"
    || failure.effect === "committed"
    || failure.effect === "not_applied"
    ? failure.effect
    : "unknown";
  const notApplied = outcome === "not_applied";
  const copy = PROJECT_MUTATION_COPY[operation][outcome];
  return {
    blocksMutation: !notApplied,
    impact: translate(copy.impact),
    message: failure.message,
    nextStep: translate(copy.nextStep),
    recoveryAction: notApplied ? undefined : "refresh",
    title: translate(copy.title),
    tone: notApplied ? "error" : "warning",
  };
}

export function projectMemberEntries(project: SharedProject) {
  return Object.entries(project.members).sort(([left], [right]) =>
    left.localeCompare(right),
  );
}

export function projectMemberDraftKey(projectId: string): string {
  return `member:${projectId}`;
}
