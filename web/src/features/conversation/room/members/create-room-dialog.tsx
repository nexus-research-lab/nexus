// INPUT: 创建或管理 Room 的完整初值、Agent 目录和提交/取消动作。
// OUTPUT: 名称/设置、成员和 Skill 表单；提交期间冻结草稿并同步防重，失败保留草稿。
// POS: Room 创建与管理的模态装配层，不用图标或副标题重复表单要求。
"use client";

import { useEffect, useId, useRef, useState } from "react";

import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";

import {
  buildRoomDialogInstanceKey,
  resolveRoomDialogContentProps,
  resolveRoomDialogLabels,
  type RoomDialogContentProps,
} from "./create-room-dialog-model";
import type { CreateRoomDialogProps } from "./create-room-dialog-types";
import { RoomMemberSelector } from "./room-member-selector";
import { RoomSettingsForm } from "./room-settings-form";
import { RoomSkillsSelector } from "./skills/room-skills-selector";
import { useRoomSkillOptions } from "./skills/use-room-skill-options";
import { useCreateRoomForm } from "./use-create-room-form";

export type {
  CreateRoomDialogProps,
  RoomDialogSubmission,
} from "./create-room-dialog-types";

export function CreateRoomDialog(props: CreateRoomDialogProps) {
  if (!props.isOpen) {
    return null;
  }
  const contentProps = resolveRoomDialogContentProps(props);
  return (
    <CreateRoomDialogContent
      key={buildRoomDialogInstanceKey(contentProps)}
      {...contentProps}
    />
  );
}

function CreateRoomDialogContent({
  agents,
  initialAvatar,
  initialHostAgentId,
  initialHostAutoReplyEnabled,
  initialName,
  initialPausedAgentIds,
  initialPrivateMessagesEnabled,
  initialRoomSkillNames,
  initialSelectedAgentIds,
  isCreating,
  mode,
  onCancel,
  onConfirm,
}: RoomDialogContentProps) {
  const { t } = useI18n();
  const titleId = useId();
  const submitting = useRef(false);
  const mounted = useRef(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitFailed, setSubmitFailed] = useState(false);
  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);
  const pending = isCreating || isSubmitting;
  const form = useCreateRoomForm({
    agents,
    initialAvatar,
    initialHostAgentId,
    initialHostAutoReplyEnabled,
    initialName,
    initialPausedAgentIds,
    initialPrivateMessagesEnabled,
    initialRoomSkillNames,
    initialSelectedAgentIds,
  });
  const skills = useRoomSkillOptions(form.state.skillQuery);
  const labels = resolveRoomDialogLabels(mode, t);
  const canSubmit = form.canSubmit && !pending && !submitFailed;
  const handleSubmit = () => {
    if (!canSubmit || submitting.current) return;
    submitting.current = true;
    setIsSubmitting(true);
    setSubmitFailed(false);
    void (async () => {
      try {
        await onConfirm(form.submission);
      } catch {
        if (mounted.current) setSubmitFailed(true);
      } finally {
        submitting.current = false;
        if (mounted.current) setIsSubmitting(false);
      }
    })();
  };

  const handleCancel = () => {
    if (!pending && !submitting.current) onCancel();
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        inset="compact"
        layer="dialogUnderlay"
        labelledBy={titleId}
        onClose={handleCancel}
      >
        <UiDialogShell
          className="pointer-events-auto"
          size="xl"
          viewport="adaptiveMax"
        >
          <UiDialogHeader
            appearance="plain"
            onClose={handleCancel}
            title={labels.title}
            titleId={titleId}
          />

          <UiDialogBody className="flex min-h-0 flex-1 flex-col gap-5 px-5" scrollable>
            <div className="grid min-h-0 grid-cols-[minmax(260px,0.9fr)_minmax(0,1.1fr)] gap-6 max-md:grid-cols-1">
              <RoomSettingsForm
                avatarFallbackTitle={labels.title}
                canSubmit={canSubmit}
                isCreating={pending}
                onSubmit={handleSubmit}
                selectedAgents={form.selectedAgents}
                setters={{
                  setAvatar: form.setAvatar,
                  setHostAgentId: form.setHostAgentId,
                  setHostAutoReplyEnabled: form.setHostAutoReplyEnabled,
                  setName: form.setName,
                  setPrivateMessagesEnabled:
                    form.setPrivateMessagesEnabled,
                }}
                state={form.state}
              />
              <RoomMemberSelector
                agents={form.filteredAgents}
                disabled={pending}
                canManageParticipation={mode === "manage"}
                onQueryChange={form.setMemberQuery}
                onToggleAgent={form.toggleAgent}
                onToggleParticipation={form.toggleParticipation}
                pausedAgentIds={form.pausedAgentIdSet}
                query={form.state.memberQuery}
                selectedAgentIds={form.selectedAgentIdSet}
              />
            </div>
            <RoomSkillsSelector
              disabled={pending}
              error={skills.error}
              isLoading={skills.loading}
              onChange={form.setSelectedSkillNames}
              onQueryChange={form.setSkillQuery}
              options={skills.options}
              query={form.state.skillQuery}
              value={form.state.selectedSkillNames}
            />
          </UiDialogBody>

          {submitFailed ? (
            <UiInlineNotice className="mx-5" tone="warning" message={t("room.save_unconfirmed")} />
          ) : null}
          <UiDialogFooter appearance="plain">
            <UiButton
              disabled={pending}
              onClick={handleCancel}
              size="sm"
              type="button"
            >
              {t("common.cancel")}
            </UiButton>
            <UiButton
              aria-busy={pending}
              disabled={!canSubmit}
              onClick={handleSubmit}
              size="sm"
              tone="primary"
              type="button"
              variant="solid"
            >
              {pending ? t(mode === "manage" ? "common.saving" : "room.creating_action") : labels.confirm}
            </UiButton>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
