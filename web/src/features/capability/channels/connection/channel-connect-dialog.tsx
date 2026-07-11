"use client";

import type { FormEvent } from "react";
import {
  ExternalLink,
  Loader2,
  Power,
  QrCode,
  Trash2,
} from "lucide-react";

import type { ChannelConfigView } from "@/lib/api/channel-api";
import { UiButton } from "@/shared/ui/button";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import { UiField, UiInput } from "@/shared/ui/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { UiStateBlock } from "@/shared/ui/state-block";
import type { Agent } from "@/types/agent/agent";

import { ChannelIcon } from "../channel-icon";
import { ChannelAccountsPanel } from "./channel-accounts-panel";
import {
  channelFieldAutocomplete,
  channelFieldInputName,
  type PendingChannelDelete,
} from "./channel-connection-model";
import { ChannelGuide } from "./channel-guide";
import { ChannelLoginPanel } from "./channel-login-panel";
import { useChannelConnectionController } from "./use-channel-connection-controller";

interface ChannelConnectDialogProps {
  agents: Agent[];
  item: ChannelConfigView;
  onClose: () => void;
  onDeleted: (item: ChannelConfigView) => Promise<void> | void;
  onError: (message: string) => void;
  onSaved: (item: ChannelConfigView, announce?: boolean) => void;
}

function submitLabel(options: {
  loginLoading: boolean;
  loginRunning: boolean;
  planned: boolean;
  saving: boolean;
  supportsPersonalWeixinLogin: boolean;
}): string {
  const rules = [
    { matches: options.planned, label: "未上线" },
    { matches: options.saving, label: "保存中..." },
    { matches: options.loginLoading, label: "拉起二维码..." },
    { matches: options.loginRunning, label: "等待扫码..." },
    {
      matches: options.supportsPersonalWeixinLogin,
      label: "拉起二维码",
    },
    { matches: true, label: "连接" },
  ];
  return rules.find(({ matches }) => matches)?.label ?? "连接";
}

function deleteDialogCopy(
  target: PendingChannelDelete | null,
  item: ChannelConfigView,
): { confirmText: string; message: string; title: string } {
  if (target?.kind === "channel") {
    return {
      confirmText: "断开频道",
      message: `确认断开 ${item.title} 吗？这会停止该频道的机器人连接，但不会删除已有配对。`,
      title: "断开频道",
    };
  }
  const account = target?.kind === "account" ? target.account : null;
  return {
    confirmText: "删除账号",
    message: account
      ? `确认删除微信账号 ${account.user_id || account.account_id} 吗？已有配对不会删除，但该账号会停止接收和回投消息。`
      : "",
    title: "删除微信账号",
  };
}

export function ChannelConnectDialog({
  agents,
  item,
  onClose,
  onDeleted,
  onError,
  onSaved,
}: ChannelConnectDialogProps) {
  const controller = useChannelConnectionController({
    agents,
    item,
    onClose,
    onDeleted,
    onError,
    onSaved,
  });
  const deleteCopy = deleteDialogCopy(
    controller.pendingDelete,
    controller.currentItem,
  );

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault();
    void controller.saveChannel();
  };

  return (
    <>
      <UiDialogPortal>
        <UiDialogBackdrop
          className="z-[9999]"
          labelledBy="channel-connect-dialog-title"
          onClose={onClose}
        >
          <UiDialogFormShell
            autoComplete="off"
            className="max-h-[86vh]"
            onSubmit={handleSubmit}
            size="lg"
          >
            <UiDialogHeader
              icon={<ChannelIcon type={controller.currentItem.channel_type} size="dialog" />}
              iconClassName="h-[52px] w-[52px] overflow-visible border-0 bg-transparent p-0 shadow-none"
              onClose={onClose}
              title={`连接 ${controller.currentItem.title}`}
              titleId="channel-connect-dialog-title"
            />

            <UiDialogBody className="space-y-5" scrollable>
              {controller.planned ? (
                <UiStateBlock
                  description="频道接入将在后续版本补充，当前版本暂不支持配置机器人或配对。"
                  size="sm"
                  title="该频道未上线"
                  variant="inset"
                />
              ) : (
                <ChannelConnectionFields
                  agents={agents}
                  controller={controller}
                />
              )}
            </UiDialogBody>

            <UiDialogFooter>
              <div className="flex w-full flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-h-10">
                  {controller.currentItem.configured && !controller.planned ? (
                    <UiButton
                      className="min-w-[118px]"
                      disabled={controller.busy}
                      onClick={controller.requestDeleteChannel}
                      size="lg"
                      tone="danger"
                      type="button"
                    >
                      {controller.deleting
                        ? <Loader2 className="h-5 w-5 animate-spin" />
                        : <Trash2 className="h-5 w-5" />}
                      {controller.deleting ? "断开中..." : "断开频道"}
                    </UiButton>
                  ) : null}
                </div>
                <div className="flex justify-end gap-3">
                  <UiButton
                    className="min-w-[104px]"
                    disabled={controller.deleting}
                    onClick={onClose}
                    size="lg"
                    type="button"
                  >
                    取消
                  </UiButton>
                  <UiButton
                    className="min-w-[124px]"
                    disabled={controller.busy
                      || controller.loginRunning
                      || !controller.draft.agentId
                      || controller.planned}
                    size="lg"
                    tone="primary"
                    type="submit"
                    variant="solid"
                  >
                    {controller.supportsPersonalWeixinLogin
                      ? <QrCode className="h-5 w-5" />
                      : <Power className="h-5 w-5" />}
                    {submitLabel(controller)}
                  </UiButton>
                </div>
              </div>
            </UiDialogFooter>
          </UiDialogFormShell>
        </UiDialogBackdrop>
      </UiDialogPortal>
      <ConfirmDialog
        confirmText={deleteCopy.confirmText}
        isOpen={controller.pendingDelete !== null}
        message={deleteCopy.message}
        onCancel={() => controller.setPendingDelete(null)}
        onConfirm={controller.confirmDelete}
        title={deleteCopy.title}
        variant="danger"
      />
    </>
  );
}

type ChannelConnectionFieldsController = Pick<
  ReturnType<typeof useChannelConnectionController>,
  | "currentItem"
  | "deletingAccountId"
  | "discordOauthUrl"
  | "draft"
  | "loginLoading"
  | "loginView"
  | "requestDeleteAccount"
  | "setAgentId"
  | "submitVerifyCode"
  | "supportsPersonalWeixinLogin"
  | "updateField"
>;

function ChannelConnectionFields({
  agents,
  controller,
}: {
  agents: Agent[];
  controller: ChannelConnectionFieldsController;
}) {
  const { currentItem, draft } = controller;
  return (
    <>
      <ChannelGuide item={currentItem} />

      {currentItem.runtime_note ? (
        <div className="rounded-[14px] border border-(--divider-subtle-color) bg-transparent px-4 py-3 text-[13px] font-medium leading-5 text-(--text-default)">
          {currentItem.runtime_note}
        </div>
      ) : null}

      {controller.supportsPersonalWeixinLogin ? (
        <ChannelLoginPanel
          loading={controller.loginLoading}
          loginView={controller.loginView}
          onSubmitVerifyCode={(value) => {
            void controller.submitVerifyCode(value);
          }}
        />
      ) : null}

      {controller.supportsPersonalWeixinLogin ? (
        <ChannelAccountsPanel
          accounts={currentItem.accounts || []}
          deletingAccountId={controller.deletingAccountId}
          onDelete={controller.requestDeleteAccount}
        />
      ) : null}

      <UiField label={<>处理智能体 <span className="text-(--destructive)">*</span></>}>
        <UiSelectMenu
          ariaLabel="选择频道处理智能体"
          onChange={controller.setAgentId}
          options={agents.map((agent) => ({
            value: agent.agent_id,
            label: agent.name,
          }))}
          size="sm"
          value={draft.agentId}
        />
      </UiField>

      <div className="space-y-4">
        {currentItem.credential_fields.map((field, index) => (
          <UiField
            key={field.key}
            label={(
              <>
                {field.label} {field.required ? <span className="text-(--destructive)">*</span> : null}
              </>
            )}
          >
            <UiInput
              autoCapitalize="none"
              autoComplete={channelFieldAutocomplete(field)}
              autoCorrect="off"
              data-1p-ignore="true"
              data-form-type="other"
              data-lpignore="true"
              name={channelFieldInputName(currentItem.channel_type, index)}
              onChange={(event) => controller.updateField(field, event.target.value)}
              placeholder={field.placeholder || ""}
              required={field.required && !(field.secret && currentItem.has_credentials)}
              type={field.kind === "password" ? "password" : "text"}
              value={field.secret
                ? draft.credentials[field.key] || ""
                : draft.config[field.key] || ""}
              variant="dialog"
            />
          </UiField>
        ))}
      </div>

      {currentItem.channel_type === "discord" ? (
        <UiField label="授权机器人到服务器">
          <UiButton
            className="w-full"
            disabled={!controller.discordOauthUrl}
            onClick={() => {
              if (controller.discordOauthUrl) {
                window.open(
                  controller.discordOauthUrl,
                  "_blank",
                  "noopener,noreferrer",
                );
              }
            }}
            size="lg"
            tone="primary"
            type="button"
            variant="solid"
          >
            <ExternalLink className="h-5 w-5" />
            授权机器人
          </UiButton>
        </UiField>
      ) : null}
    </>
  );
}
