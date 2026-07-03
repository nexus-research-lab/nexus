"use client";

import { FormEvent, useCallback, useEffect } from "react";
import {
  ExternalLink,
  Loader2,
  Power,
  QrCode,
  Trash2,
} from "lucide-react";

import {
  ChannelAccountView,
  ChannelConfigView,
  ChannelCredentialField,
  ChannelLoginView,
  deleteChannelAccountApi,
  deleteChannelConfigApi,
  getChannelLoginApi,
  listChannelsApi,
  startChannelLoginApi,
  submitChannelLoginVerifyCodeApi,
  upsertChannelConfigApi,
} from "@/lib/api/channel-api";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
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
import { UiSelectMenu } from "@/shared/ui/select-menu";
import { UiStateBlock } from "@/shared/ui/state-block";
import type { Agent } from "@/types/agent/agent";
import { notifyCapabilitySummaryMutated } from "../capability-summary-events";
import { ChannelAccountsPanel } from "./channel-accounts-panel";
import { ChannelGuide } from "./channel-guide";
import { ChannelLoginPanel } from "./channel-login-panel";
import {
  isChannelLoginRunning,
  isChannelPlanned,
  isPersonalWeixinChannel,
} from "./channel-model";
import { ChannelIcon } from "./channel-ui-model";

type PendingChannelDelete =
  | { kind: "channel" }
  | { kind: "account"; account: ChannelAccountView };

interface ChannelConnectDialogProps {
  item: ChannelConfigView;
  agents: Agent[];
  onClose: () => void;
  onDeleted: (item: ChannelConfigView) => Promise<void> | void;
  onSaved: (item: ChannelConfigView, announce?: boolean) => void;
  onError: (message: string) => void;
}

function buildDiscordOauthUrl(config: Record<string, string>) {
  const appId = config.application_id?.trim();
  if (!appId) return "";
  const params = new URLSearchParams({
    client_id: appId,
    permissions: "274877975552",
    scope: "bot applications.commands",
  });
  return `https://discord.com/oauth2/authorize?${params.toString()}`;
}

function channelFieldAutocomplete(field: ChannelCredentialField) {
  return field.secret ? "new-password" : "off";
}

function channelFieldInputName(channelType: ChannelConfigView["channel_type"], index: number) {
  return `nexus-im-channel-${channelType}-field-${index}`;
}

export function ChannelConnectDialog({ item, agents, onClose: onClose, onDeleted: onDeleted, onSaved: onSaved, onError: onError }: ChannelConnectDialogProps) {
  const initialAgentId = item.agent_id || agents[0]?.agent_id || "";
  const itemResetKey = [
    item.channel_type,
    item.agent_id || "",
    initialAgentId,
    JSON.stringify(item.public_config || {}),
  ].join("\x1f");
  const [currentItem, setCurrentItem] = useResettableState(item, itemResetKey);
  const [agentId, setAgentId] = useResettableState(initialAgentId, itemResetKey);
  const [config, setConfig] = useResettableState<Record<string, string>>(item.public_config || {}, itemResetKey);
  const [credentials, setCredentials] = useResettableState<Record<string, string>>({}, itemResetKey);
  const [saving, setSaving] = useResettableState(false, itemResetKey);
  const [deleting, setDeleting] = useResettableState(false, itemResetKey);
  const [deletingAccountId, setDeletingAccountId] = useResettableState("", itemResetKey);
  const [pendingDelete, setPendingDelete] = useResettableState<PendingChannelDelete | null>(null, itemResetKey);
  const [loginLoading, setLoginLoading] = useResettableState(false, itemResetKey);
  const [loginView, setLoginView] = useResettableState<ChannelLoginView | null>(null, itemResetKey);
  const isPlanned = isChannelPlanned(currentItem);
  const discordOauthUrl = currentItem.channel_type === "discord" ? buildDiscordOauthUrl(config) : "";
  const supportsPersonalWeixinLogin = isPersonalWeixinChannel(currentItem.channel_type);
  const loginRunning = isChannelLoginRunning(loginView);
  const loginId = loginView?.login_id || "";
  const loginStatus = loginView?.status || "";

  const handleFieldChange = (field: ChannelCredentialField, value: string) => {
    if (field.secret) {
      setCredentials((current) => ({ ...current, [field.key]: value }));
      return;
    }
    setConfig((current) => ({ ...current, [field.key]: value }));
  };

  const submitVerifyCode = useCallback(async (value: string) => {
    if (!supportsPersonalWeixinLogin || !loginId) return;
    setLoginLoading(true);
    try {
      const nextLogin = await submitChannelLoginVerifyCodeApi(currentItem.channel_type, loginId, value);
      setLoginView(nextLogin);
    } catch (error) {
      onError(error instanceof Error ? error.message : "验证码提交失败");
    } finally {
      setLoginLoading(false);
    }
  }, [currentItem.channel_type, loginId, onError, supportsPersonalWeixinLogin]);

  const refreshCurrentChannel = useCallback(async () => {
    const items = await listChannelsApi();
    const updated = items.find((value) => value.channel_type === currentItem.channel_type);
    if (!updated) return;
    setCurrentItem(updated);
    onSaved(updated, false);
  }, [currentItem.channel_type, onSaved]);

  useEffect(() => {
    if (!supportsPersonalWeixinLogin || !loginId || loginStatus !== "running") {
      return;
    }
    const timer = window.setInterval(async () => {
      try {
        const nextLogin = await getChannelLoginApi(currentItem.channel_type, loginId);
        setLoginView(nextLogin);
        if (nextLogin.status === "succeeded") {
          void refreshCurrentChannel();
        }
      } catch (error) {
        window.clearInterval(timer);
        onError(error instanceof Error ? error.message : "扫码登录状态刷新失败");
      }
    }, 1500);
    return () => window.clearInterval(timer);
  }, [currentItem.channel_type, loginId, loginStatus, onError, refreshCurrentChannel, supportsPersonalWeixinLogin]);

  const saveChannel = useCallback(async (closeOnSuccess: boolean) => {
    if (!agentId) return;
    if (isPlanned) return;
    setSaving(true);
    try {
      const saved = await upsertChannelConfigApi(currentItem.channel_type, {
        agent_id: agentId,
        config,
        credentials,
      });
      setCurrentItem(saved);
      const shouldStartLogin = isPersonalWeixinChannel(saved.channel_type);
      onSaved(saved, !shouldStartLogin);
      if (closeOnSuccess && shouldStartLogin) {
        setLoginLoading(true);
        const nextLogin = await startChannelLoginApi(saved.channel_type);
        setLoginView(nextLogin);
        return;
      }
      if (closeOnSuccess) onClose();
    } catch (error) {
      onError(error instanceof Error ? error.message : "连接失败");
    } finally {
      setSaving(false);
      setLoginLoading(false);
    }
  }, [agentId, config, credentials, currentItem.channel_type, isPlanned, onClose, onError, onSaved]);

  const requestDeleteChannel = useCallback(() => {
    if (!currentItem.configured || isPlanned || deleting) return;
    setPendingDelete({ kind: "channel" });
  }, [currentItem.configured, deleting, isPlanned]);

  const deleteChannel = useCallback(async () => {
    if (!currentItem.configured || isPlanned || deleting) return;
    setDeleting(true);
    try {
      await deleteChannelConfigApi(currentItem.channel_type);
      notifyCapabilitySummaryMutated({ source: "channels", action: "delete", channel_type: currentItem.channel_type });
      await onDeleted(currentItem);
      setDeleting(false);
      onClose();
    } catch (error) {
      onError(error instanceof Error ? error.message : "断开频道失败");
      setDeleting(false);
    }
  }, [currentItem, deleting, isPlanned, onClose, onDeleted, onError]);

  const requestDeleteAccount = useCallback((account: ChannelAccountView) => {
    if (!account.account_id || deletingAccountId) return;
    setPendingDelete({ kind: "account", account });
  }, [deletingAccountId]);

  const deleteAccount = useCallback(async (account: ChannelAccountView) => {
    if (!account.account_id || deletingAccountId) return;
    setDeletingAccountId(account.account_id);
    try {
      const updated = await deleteChannelAccountApi(currentItem.channel_type, account.account_id);
      setCurrentItem(updated);
      notifyCapabilitySummaryMutated({ source: "channels", action: "delete_account", channel_type: currentItem.channel_type });
      onSaved(updated, false);
    } catch (error) {
      onError(error instanceof Error ? error.message : "删除账号失败");
    } finally {
      setDeletingAccountId("");
    }
  }, [currentItem.channel_type, deletingAccountId, onError, onSaved]);

  const confirmDelete = useCallback(() => {
    const target = pendingDelete;
    if (!target) return;
    setPendingDelete(null);
    if (target.kind === "channel") {
      void deleteChannel();
      return;
    }
    void deleteAccount(target.account);
  }, [deleteAccount, deleteChannel, pendingDelete]);

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    await saveChannel(true);
  };

  return (
    <>
      <UiDialogPortal>
        <UiDialogBackdrop className="z-[9999]" labelledBy="channel-connect-dialog-title" onClose={onClose}>
          <UiDialogFormShell
            autoComplete="off"
            className="max-h-[86vh]"
            onSubmit={handleSubmit}
            size="lg"
          >
            <UiDialogHeader
              icon={<ChannelIcon type={currentItem.channel_type} size="dialog" />}
              iconClassName="h-[52px] w-[52px] overflow-visible border-0 bg-transparent p-0 shadow-none"
              onClose={onClose}
              title={`连接 ${currentItem.title}`}
              titleId="channel-connect-dialog-title"
            />

            <UiDialogBody className="space-y-5" scrollable>
              {isPlanned ? (
                <UiStateBlock
                  description="频道接入将在后续版本补充，当前版本暂不支持配置机器人或配对。"
                  size="sm"
                  title="该频道未上线"
                  variant="inset"
                />
              ) : (
                <>
                  <ChannelGuide item={currentItem} />

                  {currentItem.runtime_note ? (
                    <div className="rounded-[14px] border border-(--divider-subtle-color) bg-transparent px-4 py-3 text-[13px] font-medium leading-5 text-(--text-default)">
                      {currentItem.runtime_note}
                    </div>
                  ) : null}

                  {supportsPersonalWeixinLogin ? (
                    <ChannelLoginPanel
                      loading={loginLoading || saving}
                      loginView={loginView}
                      onSubmitVerifyCode={submitVerifyCode}
                    />
                  ) : null}

                  {supportsPersonalWeixinLogin ? (
                    <ChannelAccountsPanel
                      accounts={currentItem.accounts || []}
                      deletingAccountId={deletingAccountId}
                      onDelete={requestDeleteAccount}
                    />
                  ) : null}

                  <UiField label={<>处理智能体 <span className="text-(--destructive)">*</span></>}>
                    <UiSelectMenu
                      ariaLabel="选择频道处理智能体"
                      onChange={setAgentId}
                      options={agents.map((agent) => ({
                        value: agent.agent_id,
                        label: agent.name,
                      }))}
                      size="sm"
                      value={agentId}
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
                          onChange={(event) => handleFieldChange(field, event.target.value)}
                          placeholder={field.placeholder || ""}
                          required={field.required && !(field.secret && currentItem.has_credentials)}
                          type={field.kind === "password" ? "password" : "text"}
                          value={field.secret ? credentials[field.key] || "" : config[field.key] || ""}
                          variant="dialog"
                        />
                      </UiField>
                    ))}
                  </div>

                  {currentItem.channel_type === "discord" ? (
                    <UiField label="授权机器人到服务器">
                      <UiButton
                        className="w-full"
                        disabled={!discordOauthUrl}
                        onClick={() => discordOauthUrl && window.open(discordOauthUrl, "_blank", "noopener,noreferrer")}
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
              )}
            </UiDialogBody>

            <UiDialogFooter>
              <div className="flex w-full flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-h-10">
                  {currentItem.configured && !isPlanned ? (
                    <UiButton
                      className="min-w-[118px]"
                      disabled={saving || deleting || loginLoading}
                      onClick={requestDeleteChannel}
                      size="lg"
                      tone="danger"
                      type="button"
                    >
                      {deleting ? <Loader2 className="h-5 w-5 animate-spin" /> : <Trash2 className="h-5 w-5" />}
                      {deleting ? "断开中..." : "断开频道"}
                    </UiButton>
                  ) : null}
                </div>
                <div className="flex justify-end gap-3">
                  <UiButton
                    className="min-w-[104px]"
                    disabled={deleting}
                    onClick={onClose}
                    size="lg"
                    type="button"
                  >
                    取消
                  </UiButton>
                  <UiButton
                    className="min-w-[124px]"
                    disabled={saving || deleting || loginLoading || loginRunning || !agentId || isPlanned}
                    size="lg"
                    tone="primary"
                    type="submit"
                    variant="solid"
                  >
                    {supportsPersonalWeixinLogin ? <QrCode className="h-5 w-5" /> : <Power className="h-5 w-5" />}
                    {isPlanned
                      ? "未上线"
                      : saving
                        ? "保存中..."
                        : loginLoading
                          ? "拉起二维码..."
                          : loginRunning
                            ? "等待扫码..."
                            : supportsPersonalWeixinLogin
                              ? "拉起二维码"
                              : "连接"}
                  </UiButton>
                </div>
              </div>
            </UiDialogFooter>
          </UiDialogFormShell>
        </UiDialogBackdrop>
      </UiDialogPortal>
      <ConfirmDialog
        confirmText={pendingDelete?.kind === "channel" ? "断开频道" : "删除账号"}
        isOpen={pendingDelete !== null}
        message={pendingDelete?.kind === "channel"
          ? `确认断开 ${currentItem.title} 吗？这会停止该频道的机器人连接，但不会删除已有配对。`
          : pendingDelete
            ? `确认删除微信账号 ${pendingDelete.account.user_id || pendingDelete.account.account_id} 吗？已有配对不会删除，但该账号会停止接收和回投消息。`
            : ""}
        onCancel={() => setPendingDelete(null)}
        onConfirm={confirmDelete}
        title={pendingDelete?.kind === "channel" ? "断开频道" : "删除微信账号"}
        variant="danger"
      />
    </>
  );
}
