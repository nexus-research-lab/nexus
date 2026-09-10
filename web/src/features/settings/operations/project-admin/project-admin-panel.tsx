// INPUT: 项目与成员资源、权限及创建、刷新、授权命令。
// OUTPUT: 管理员项目列表、成员权限表单和可恢复反馈。
// POS: Operations 项目管理用例；不拥有通用表单或按钮视觉。

"use client";

import {
  Loader2,
  Plus,
  RefreshCw,
  UserPlus,
} from "lucide-react";
import { type FormEvent, useMemo } from "react";

import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_CONTROL_LABEL_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
} from "@/features/settings/shared/settings-panel-ui";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiInput } from "@/shared/ui/form/form-control";
import { completeFeedbackBanner } from "@/shared/ui/feedback/feedback-banner-contract";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { ProjectAccess, SharedProject } from "@/types/settings/project";

import {
  PROJECT_ACCESS_VALUES,
  projectMemberDraftKey,
  projectMemberEntries,
  type ProjectAdminViewModel,
} from "./project-admin-model";
import { useProjectAdmin } from "./use-project-admin";

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
      <div className="grid gap-3 border-b border-(--divider-subtle-color) px-4 py-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <p className={cn(
              "wrap-anywhere",
              SETTINGS_ITEM_TITLE_CLASS_NAME,
            )}>
              {project.project_id}
            </p>
          </div>
          <p className={cn(
            "mt-1 break-all",
            getUiTypographyClassName({ role: "caption", tone: "soft" }),
          )}>
            {t("settings.projects.root")}: {project.root}
          </p>
        </div>
      </div>

      <div className="px-4 py-3">
        <div className="flex items-center gap-2">
          <p className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
            {t("settings.projects.members")}
          </p>
        </div>

        {memberEntries.length === 0 ? (
          <UiResourceState
            size="sm"
            state="empty"
            title={t("settings.projects.members_empty")}
            variant="plain"
          />
        ) : (
          <div className="mt-3 divide-y divide-(--divider-subtle-color)">
            {memberEntries.map(([ownerUserId, access]) => {
              const pending =
                model.pendingKey === `member:${project.project_id}:${ownerUserId}`;
              return (
                <div
                  key={ownerUserId}
                  className="grid gap-2 py-2.5 @min-[480px]/projects:grid-cols-[minmax(0,1fr)_160px] @min-[480px]/projects:items-center"
                >
                  <span className={cn(
                    "min-w-0 wrap-anywhere",
                    getUiTypographyClassName({ role: "supporting", tone: "default", weight: "medium" }),
                  )}>
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
                      size="sm"
                      value={access}
                    />
                  ) : (
                    <span className={cn(
                      "sm:text-right",
                      getUiTypographyClassName({ role: "caption", tone: "soft", weight: "semibold" }),
                    )}>
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
            className="mt-3 grid gap-2 border-t border-(--divider-subtle-color) pt-3 @min-[480px]/projects:grid-cols-[minmax(0,1fr)_auto] @min-[480px]/projects:items-end"
            onSubmit={handleAddMember}
          >
            <label className="grid min-w-0 gap-1.5">
              <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
                {t("settings.projects.member_owner_id")}
              </span>
              <UiInput
                variant="surface"
                disabled={disabled}
                onChange={(event) =>
                  onChangeMemberDraft(project.project_id, event.target.value)}
                placeholder={t("settings.projects.member_owner_placeholder")}
                value={memberDraft}
              />
            </label>
            <UiButton
              disabled={disabled || memberDraft.trim() === ""}
              size="md"
              tone="primary"
              type="submit"
              variant="solid"
            >
              {model.pendingKey === `member:${project.project_id}:${memberDraft.trim()}` ? (
                <Loader2 className={getUiSpinnerClassName({ size: "sm" })} />
              ) : (
                <UserPlus className="h-3.5 w-3.5" />
              )}
              {t("settings.projects.add_member")}
            </UiButton>
          </form>
        ) : (
          <p className={cn(
            "mt-3 border-t border-(--divider-subtle-color) pt-3",
            getUiTypographyClassName({ role: "caption", tone: "soft" }),
          )}>
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
      <div className="@container/projects grid min-w-0 gap-4">
        <WorkspaceContentHeader
          className="mb-0 max-sm:[&_h1]:hidden"
          title={t("settings.projects.title")}
          description={t("settings.projects.description")}
          actions={<UiButton disabled={refreshDisabled} onClick={() => void controller.refreshProjects()} size="sm" variant="text">
            <RefreshCw className={viewModel.loading ? getUiSpinnerClassName({ size: "sm" }) : "h-3.5 w-3.5"} />
            {t("settings.projects.refresh")}
          </UiButton>}
        />
        <UiDisclosure label={t("settings.projects.create")} variant="panel">
          <form
            className="grid items-end gap-3 @min-[480px]/projects:grid-cols-[minmax(0,1fr)_auto]"
            onSubmit={handleCreateProject}
          >
            <label className="grid min-w-0 gap-1.5">
              <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
                {t("settings.projects.create_label")}
              </span>
              <UiInput
                variant="surface"
                disabled={disabled}
                onChange={(event) => controller.setNewProjectId(event.target.value)}
                placeholder={t("settings.projects.create_placeholder")}
                value={viewModel.newProjectId}
              />
            </label>
            <UiButton
              disabled={disabled || viewModel.newProjectId.trim() === ""}
              size="md"
              tone="primary"
              type="submit"
              variant="solid"
            >
              {viewModel.pendingKey === "create-project" ? (
                <Loader2 className={getUiSpinnerClassName({ size: "sm" })} />
              ) : (
                <Plus className="h-3.5 w-3.5" />
              )}
              {t("settings.projects.create")}
            </UiButton>
          </form>
        </UiDisclosure>

        {viewModel.loading ? (
          <UiResourceState
            size="sm"
            state="loading"
            title={t("settings.projects.loading")}
            variant="plain"
          />
        ) : viewModel.projects.length === 0 ? (
          <UiResourceState
            size="sm"
            state="empty"
            title={t("settings.projects.empty")}
            variant="plain"
          />
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
            viewModel.feedback.tone === "success"
              ? {
                  message: viewModel.feedback.message,
                  onDismiss: controller.dismissFeedback,
                  title: viewModel.feedback.title,
                  tone: "success",
                }
              : {
                  action: viewModel.feedback.recoveryAction === "refresh"
                    ? {
                        label: t("state.retry"),
                        onClick: () => {
                          void controller.refreshProjects();
                        },
                      }
                    : undefined,
                  impact: viewModel.feedback.impact,
                  nextStep: viewModel.feedback.nextStep,
                  onDismiss: viewModel.feedback.blocksMutation
                    ? undefined
                    : controller.dismissFeedback,
                  title: viewModel.feedback.title,
                  tone: viewModel.feedback.tone,
                },
            {
              impact: t("feedback.unconfirmed_impact"),
            },
          )
          : null}
      />
    </>
  );
}
