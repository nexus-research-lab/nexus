"use client";

import {
  FolderKanban,
  Loader2,
  Plus,
  RefreshCw,
  ShieldCheck,
  UserPlus,
} from "lucide-react";
import { type FormEvent, useMemo } from "react";

import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
} from "@/features/settings/shared/settings-panel-ui";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiButtonClassName } from "@/shared/ui/button/button-styles";
import { completeFeedbackBanner } from "@/shared/ui/feedback/feedback-banner-contract";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import type { ProjectAccess, SharedProject } from "@/types/settings/project";

import {
  PROJECT_ACCESS_VALUES,
  projectMemberDraftKey,
  projectMemberEntries,
  type ProjectAdminViewModel,
} from "./project-admin-model";
import { useProjectAdmin } from "./use-project-admin";

const INPUT_CLASS_NAME =
  "dialog-input h-9 w-full radius-control-md px-3 text-sm text-(--text-strong) outline-none disabled:opacity-(--disabled-opacity)";
const PRIMARY_BUTTON_CLASS_NAME = getUiButtonClassName({
  size: "sm",
  tone: "primary",
  variant: "solid",
});
const SECONDARY_BUTTON_CLASS_NAME = getUiButtonClassName({
  size: "sm",
  variant: "surface",
});

interface ProjectCardProps {
  model: ProjectAdminViewModel;
  project: SharedProject;
  onAddMember: (projectId: string) => Promise<void>;
  onChangeMemberDraft: (projectId: string, value: string) => void;
  onUpdateMember: (
    projectId: string,
    ownerUserId: string,
    access: ProjectAccess,
  ) => Promise<boolean>;
}

function ProjectCard({
  model,
  project,
  onAddMember,
  onChangeMemberDraft,
  onUpdateMember,
}: ProjectCardProps) {
  const { t } = useI18n();
  const memberEntries = projectMemberEntries(project);
  const memberDraft = model.memberDrafts[projectMemberDraftKey(project.project_id)] ?? "";
  const disabled = model.pendingKey !== null || model.mutationsBlocked;
  const accessOptions = useMemo(() => PROJECT_ACCESS_VALUES.map((access) => ({
    label: t(`settings.projects.access_${access}`),
    value: access,
  })), [t]);

  const handleAddMember = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void onAddMember(project.project_id);
  };

  return (
    <article className={SETTINGS_CARD_CLASS_NAME}>
      <div className="grid gap-3 border-b border-(--divider-subtle-color) px-4 py-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-start">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <FolderKanban className="h-4 w-4 shrink-0 text-primary" />
            <p className="truncate text-base font-semibold text-(--text-strong)">
              {project.project_id}
            </p>
          </div>
          <p className="mt-1 break-all text-xs leading-5 text-(--text-soft)">
            {t("settings.projects.root")}: {project.root}
          </p>
        </div>
        <span className="w-fit rounded-full border border-(--divider-subtle-color) px-2 py-0.5 text-2xs font-semibold text-(--text-muted)">
          {t("settings.projects.generation")}: {project.generation}
        </span>
      </div>

      <div className="px-4 py-3">
        <div className="flex items-center gap-2">
          <ShieldCheck className="h-3.5 w-3.5 text-(--text-soft)" />
          <p className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
            {t("settings.projects.members")}
          </p>
        </div>

        {memberEntries.length === 0 ? (
          <p className="py-5 text-center text-compact text-(--text-soft)">
            {t("settings.projects.members_empty")}
          </p>
        ) : (
          <div className="mt-3 divide-y divide-(--divider-subtle-color)">
            {memberEntries.map(([ownerUserId, access]) => {
              const pending =
                model.pendingKey === `member:${project.project_id}:${ownerUserId}`;
              return (
                <div
                  key={ownerUserId}
                  className="grid gap-2 py-2.5 sm:grid-cols-[minmax(0,1fr)_140px] sm:items-center"
                >
                  <span className="min-w-0 truncate text-compact font-medium text-(--text-default)">
                    {ownerUserId}
                  </span>
                  {model.canManageMembers ? (
                    <UiSelectMenu
                      ariaLabel={`${t("settings.projects.access")}: ${ownerUserId}`}
                      disabled={disabled}
                      onChange={(value) => void onUpdateMember(
                        project.project_id,
                        ownerUserId,
                        value as ProjectAccess,
                      )}
                      options={accessOptions}
                      size="xs"
                      value={access}
                    />
                  ) : (
                    <span className="text-xs font-semibold text-(--text-soft) sm:text-right">
                      {pending
                        ? t("settings.projects.updating")
                        : t(`settings.projects.access_${access}`)}
                    </span>
                  )}
                </div>
              );
            })}
          </div>
        )}

        {model.canManageMembers ? (
          <form
            className="mt-3 grid gap-2 border-t border-(--divider-subtle-color) pt-3 sm:grid-cols-[minmax(0,1fr)_auto]"
            onSubmit={handleAddMember}
          >
            <label className="min-w-0">
              <span className="sr-only">
                {t("settings.projects.member_owner_id")}
              </span>
              <input
                className={INPUT_CLASS_NAME}
                disabled={disabled}
                onChange={(event) =>
                  onChangeMemberDraft(project.project_id, event.target.value)}
                placeholder={t("settings.projects.member_owner_placeholder")}
                value={memberDraft}
              />
            </label>
            <button
              className={PRIMARY_BUTTON_CLASS_NAME}
              disabled={disabled || memberDraft.trim() === ""}
              type="submit"
            >
              {model.pendingKey === `member:${project.project_id}:${memberDraft.trim()}` ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <UserPlus className="h-3.5 w-3.5" />
              )}
              {t("settings.projects.add_member")}
            </button>
          </form>
        ) : (
          <p className="mt-3 border-t border-(--divider-subtle-color) pt-3 text-xs leading-5 text-(--text-soft)">
            {t("settings.projects.read_only_hint")}
          </p>
        )}
      </div>
    </article>
  );
}

export function ProjectAdminPanel() {
  const { status } = useAuth();
  const { t } = useI18n();
  const controller = useProjectAdmin({
    canManageMembers: status?.role === "admin",
  });
  const { viewModel } = controller;
  const disabled = viewModel.loading
    || viewModel.pendingKey !== null
    || viewModel.mutationsBlocked;
  const refreshDisabled = viewModel.loading || viewModel.pendingKey !== null;

  const handleCreateProject = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void controller.createProject();
  };

  return (
    <>
      <div className="grid gap-4">
        <section className={SETTINGS_CARD_CLASS_NAME}>
          <form
            className="grid gap-3 px-4 py-3 sm:grid-cols-[minmax(0,1fr)_auto_auto] sm:items-end"
            onSubmit={handleCreateProject}
          >
            <label className="grid min-w-0 gap-1.5">
              <span className="text-xs font-semibold text-(--text-muted)">
                {t("settings.projects.create_label")}
              </span>
              <input
                className={INPUT_CLASS_NAME}
                disabled={disabled}
                onChange={(event) => controller.setNewProjectId(event.target.value)}
                placeholder={t("settings.projects.create_placeholder")}
                value={viewModel.newProjectId}
              />
            </label>
            <button
              className={PRIMARY_BUTTON_CLASS_NAME}
              disabled={disabled || viewModel.newProjectId.trim() === ""}
              type="submit"
            >
              {viewModel.pendingKey === "create-project" ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <Plus className="h-3.5 w-3.5" />
              )}
              {t("settings.projects.create")}
            </button>
            <button
              className={SECONDARY_BUTTON_CLASS_NAME}
              disabled={refreshDisabled}
              onClick={() => void controller.refreshProjects()}
              type="button"
            >
              {viewModel.loading ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <RefreshCw className="h-3.5 w-3.5" />
              )}
              {t("settings.projects.refresh")}
            </button>
          </form>
        </section>

        {viewModel.loading ? (
          <div className="flex items-center justify-center gap-2 px-4 py-10 text-compact text-(--text-soft)">
            <Loader2 className="h-4 w-4 animate-spin" />
            {t("settings.projects.loading")}
          </div>
        ) : viewModel.projects.length === 0 ? (
          <div className="px-4 py-10 text-center text-compact text-(--text-soft)">
            {t("settings.projects.empty")}
          </div>
        ) : (
          <div className="grid gap-3">
            {viewModel.projects.map((project) => (
              <ProjectCard
                key={project.project_id}
                model={viewModel}
                onAddMember={controller.addMember}
                onChangeMemberDraft={controller.changeMemberDraft}
                onUpdateMember={controller.updateMember}
                project={project}
              />
            ))}
          </div>
        )}
      </div>

      <FeedbackBannerViewport
        item={viewModel.feedback
          ? completeFeedbackBanner(
            {
              action: viewModel.feedback.recoveryAction === "refresh"
                ? {
                    label: t("state.retry"),
                    onClick: () => {
                      void controller.refreshProjects();
                    },
                  }
                : undefined,
              impact: viewModel.feedback.impact,
              message: viewModel.feedback.message,
              nextStep: viewModel.feedback.nextStep,
              onDismiss: viewModel.feedback.blocksMutation
                ? undefined
                : controller.dismissFeedback,
              title: viewModel.feedback.title,
              tone: viewModel.feedback.tone,
            },
            {
              impact: t("feedback.unconfirmed_impact"),
              nextStep: t("feedback.unconfirmed_next_step"),
            },
          )
          : null}
      />
    </>
  );
}
