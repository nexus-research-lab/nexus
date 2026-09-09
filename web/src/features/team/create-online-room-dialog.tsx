// INPUT: 在线 Room 创建状态与提交/取消动作。
// OUTPUT: 只收集首阶段必需名称的建群对话框。
// POS: Relay Room 创建入口；成员邀请在建群后独立管理。

import { FormEvent, useState } from "react";

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
import { UiField, UiInput } from "@/shared/ui/form/form-control";

export function CreateOnlineRoomDialog({
  error,
  isCreating,
  isOpen,
  onCancel,
  onConfirm,
}: {
  error: boolean;
  isCreating: boolean;
  isOpen: boolean;
  onCancel: () => void;
  onConfirm: (name: string) => void;
}) {
  if (!isOpen) {
    return null;
  }
  return (
    <CreateOnlineRoomDialogContent
      error={error}
      isCreating={isCreating}
      onCancel={onCancel}
      onConfirm={onConfirm}
    />
  );
}

function CreateOnlineRoomDialogContent({
  error,
  isCreating,
  onCancel,
  onConfirm,
}: Omit<Parameters<typeof CreateOnlineRoomDialog>[0], "isOpen">) {
  const { t } = useI18n();
  const [name, setName] = useState("");
  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (name.trim() && !isCreating) {
      onConfirm(name);
    }
  };
  return (
    <UiDialogPortal>
      <UiDialogBackdrop inset="compact" layer="dialogUnderlay" labelledBy="online-room-title" onClose={onCancel}>
        <UiDialogShell size="xs">
          <form onSubmit={submit}>
            <UiDialogHeader onClose={onCancel} title={t("team.create_room")} titleId="online-room-title" />
            <UiDialogBody className="px-5 py-4">
              <UiField
                error={error ? t("team.error_create") : undefined}
                htmlFor="online-room-name"
                label={t("team.room_name")}
                required
              >
                <UiInput
                  disabled={isCreating}
                  id="online-room-name"
                  maxLength={100}
                  onChange={(event) => setName(event.target.value)}
                  required
                  value={name}
                />
              </UiField>
            </UiDialogBody>
            <UiDialogFooter>
              <UiButton disabled={isCreating} onClick={onCancel} size="sm" type="button">
                {t("common.cancel")}
              </UiButton>
              <UiButton disabled={!name.trim() || isCreating} size="sm" tone="primary" type="submit" variant="solid">
                {isCreating ? t("room.creating_action") : t("team.create_room")}
              </UiButton>
            </UiDialogFooter>
          </form>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
