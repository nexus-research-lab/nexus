"use client";

import { Hash } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiDialogBackdrop,
  UiDialogCloseButton,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import {
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_HEADER_LEADING_CLASS_NAME,
  getDialogActionClassName,
} from "@/shared/ui/dialog/dialog-styles";

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
  initialPrivateMessagesEnabled,
  initialRoomSkillNames,
  initialSelectedAgentIds,
  isCreating,
  mode,
  onCancel,
  onConfirm,
}: RoomDialogContentProps) {
  const { t } = useI18n();
  const form = useCreateRoomForm({
    agents,
    initialAvatar,
    initialHostAgentId,
    initialHostAutoReplyEnabled,
    initialName,
    initialPrivateMessagesEnabled,
    initialRoomSkillNames,
    initialSelectedAgentIds,
  });
  const skills = useRoomSkillOptions(form.state.skillQuery);
  const labels = resolveRoomDialogLabels(mode, t);
  const canSubmit = form.canSubmit && !isCreating;
  const handleSubmit = () => {
    if (canSubmit) {
      onConfirm(form.submission);
    }
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[9998]"
        labelledBy="create-room-dialog-title"
        onClose={onCancel}
      >
        <UiDialogShell
          className="max-h-[min(80vh,720px)] pointer-events-auto"
          size="lg"
        >
          <UiDialogHeader>
            <div
              className={cn(
                DIALOG_HEADER_LEADING_CLASS_NAME,
                "min-w-0 flex-1 items-center",
              )}
            >
              <div
                className={cn(
                  DIALOG_HEADER_ICON_CLASS_NAME,
                  "h-11 w-11 rounded-[16px] text-primary",
                )}
              >
                <Hash className="h-5 w-5" />
              </div>
              <div className="min-w-0">
                <h2
                  className="dialog-title truncate"
                  id="create-room-dialog-title"
                >
                  {labels.title}
                </h2>
                <p className="dialog-subtitle truncate">{labels.subtitle}</p>
              </div>
            </div>
            <UiDialogCloseButton onClose={onCancel} />
          </UiDialogHeader>

          <div className="dialog-body flex min-h-0 flex-col gap-4 overflow-y-auto">
            <div className="flex min-h-0 gap-5">
              <RoomSettingsForm
                avatarFallbackTitle={labels.title}
                canSubmit={canSubmit}
                isCreating={isCreating}
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
                onQueryChange={form.setMemberQuery}
                onToggleAgent={form.toggleAgent}
                query={form.state.memberQuery}
                selectedAgentIds={form.selectedAgentIdSet}
              />
            </div>
            <RoomSkillsSelector
              disabled={isCreating}
              error={skills.error}
              isLoading={skills.loading}
              onChange={form.setSelectedSkillNames}
              onQueryChange={form.setSkillQuery}
              options={skills.options}
              query={form.state.skillQuery}
              value={form.state.selectedSkillNames}
            />
          </div>

          <div className="dialog-footer justify-end gap-3">
            <button
              className={getDialogActionClassName("default")}
              onClick={onCancel}
              type="button"
            >
              {t("common.cancel")}
            </button>
            <button
              className={getDialogActionClassName(
                canSubmit ? "primary" : "default",
              )}
              disabled={!canSubmit}
              onClick={handleSubmit}
              type="button"
            >
              {isCreating ? t("room.creating_action") : labels.confirm}
            </button>
          </div>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
