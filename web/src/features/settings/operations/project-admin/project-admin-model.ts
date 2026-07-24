import type { TranslationKey } from "@/shared/i18n/messages";
import type { ProjectAccess, SharedProject } from "@/types/settings/project";

export interface ProjectFeedback {
  message: string;
  title: string;
  tone: "success" | "error";
}

export interface ProjectAdminViewModel {
  canManageMembers: boolean;
  feedback: ProjectFeedback | null;
  loading: boolean;
  memberDrafts: Record<string, string>;
  newProjectId: string;
  pendingKey: string | null;
  projects: SharedProject[];
}

export type ProjectFeedbackEvent =
  | "create-failed"
  | "create-succeeded"
  | "grant-failed"
  | "grant-succeeded"
  | "load-failed";

export const PROJECT_ACCESS_VALUES: readonly ProjectAccess[] = [
  "read",
  "write",
  "none",
];

const FEEDBACK_COPY: Record<
  ProjectFeedbackEvent,
  { message: TranslationKey; title: TranslationKey; tone: ProjectFeedback["tone"] }
> = {
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
  "grant-succeeded": {
    message: "settings.projects.grant_success_message",
    title: "settings.projects.grant_success_title",
    tone: "success",
  },
  "load-failed": {
    message: "settings.projects.load_failed_message",
    title: "settings.projects.load_failed_title",
    tone: "error",
  },
};

export function buildProjectFeedback(
  translate: (key: TranslationKey) => string,
  event: ProjectFeedbackEvent,
  error?: unknown,
): ProjectFeedback {
  const copy = FEEDBACK_COPY[event];
  return {
    message: error instanceof Error ? error.message : translate(copy.message),
    title: translate(copy.title),
    tone: copy.tone,
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
