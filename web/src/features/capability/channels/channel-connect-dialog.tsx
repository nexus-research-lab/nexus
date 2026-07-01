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
  delete_channel_account_api,
  delete_channel_config_api,
  get_channel_login_api,
  list_channels_api,
  start_channel_login_api,
  submit_channel_login_verify_code_api,
  upsert_channel_config_api,
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
import { notify_capability_summary_mutated } from "../capability-summary-events";
import { ChannelAccountsPanel } from "./channel-accounts-panel";
import { ChannelGuide } from "./channel-guide";
import { ChannelLoginPanel } from "./channel-login-panel";
import {
  is_channel_login_running,
  is_channel_planned,
  is_personal_weixin_channel,
} from "./channel-model";
import { ChannelIcon } from "./channel-ui-model";

type PendingChannelDelete =
  | { kind: "channel" }
  | { kind: "account"; account: ChannelAccountView };

interface ChannelConnectDialogProps {
  item: ChannelConfigView;
  agents: Agent[];
  on_close: () => void;
  on_deleted: (item: ChannelConfigView) => Promise<void> | void;
  on_saved: (item: ChannelConfigView, announce?: boolean) => void;
  on_error: (message: string) => void;
}

function build_discord_oauth_url(config: Record<string, string>) {
  const app_id = config.application_id?.trim();
  if (!app_id) return "";
  const params = new URLSearchParams({
    client_id: app_id,
    permissions: "274877975552",
    scope: "bot applications.commands",
  });
  return `https://discord.com/oauth2/authorize?${params.toString()}`;
}

function channel_field_autocomplete(field: ChannelCredentialField) {
  return field.secret ? "new-password" : "off";
}

function channel_field_input_name(channel_type: ChannelConfigView["channel_type"], index: number) {
  return `nexus-im-channel-${channel_type}-field-${index}`;
}

export function ChannelConnectDialog({ item, agents, on_close, on_deleted, on_saved, on_error }: ChannelConnectDialogProps) {
  const initial_agent_id = item.agent_id || agents[0]?.agent_id || "";
  const item_reset_key = [
    item.channel_type,
    item.agent_id || "",
    initial_agent_id,
    JSON.stringify(item.public_config || {}),
  ].join("\x1f");
  const [current_item, set_current_item] = useResettableState(item, item_reset_key);
  const [agent_id, set_agent_id] = useResettableState(initial_agent_id, item_reset_key);
  const [config, set_config] = useResettableState<Record<string, string>>(item.public_config || {}, item_reset_key);
  const [credentials, set_credentials] = useResettableState<Record<string, string>>({}, item_reset_key);
  const [saving, set_saving] = useResettableState(false, item_reset_key);
  const [deleting, set_deleting] = useResettableState(false, item_reset_key);
  const [deleting_account_id, set_deleting_account_id] = useResettableState("", item_reset_key);
  const [pending_delete, set_pending_delete] = useResettableState<PendingChannelDelete | null>(null, item_reset_key);
  const [login_loading, set_login_loading] = useResettableState(false, item_reset_key);
  const [login_view, set_login_view] = useResettableState<ChannelLoginView | null>(null, item_reset_key);
  const is_planned = is_channel_planned(current_item);
  const discord_oauth_url = current_item.channel_type === "discord" ? build_discord_oauth_url(config) : "";
  const supports_personal_weixin_login = is_personal_weixin_channel(current_item.channel_type);
  const login_running = is_channel_login_running(login_view);
  const login_id = login_view?.login_id || "";
  const login_status = login_view?.status || "";

  const handle_field_change = (field: ChannelCredentialField, value: string) => {
    if (field.secret) {
      set_credentials((current) => ({ ...current, [field.key]: value }));
      return;
    }
    set_config((current) => ({ ...current, [field.key]: value }));
  };

  const submit_verify_code = useCallback(async (value: string) => {
    if (!supports_personal_weixin_login || !login_id) return;
    set_login_loading(true);
    try {
      const next_login = await submit_channel_login_verify_code_api(current_item.channel_type, login_id, value);
      set_login_view(next_login);
    } catch (error) {
      on_error(error instanceof Error ? error.message : "验证码提交失败");
    } finally {
      set_login_loading(false);
    }
  }, [current_item.channel_type, login_id, on_error, supports_personal_weixin_login]);

  const refresh_current_channel = useCallback(async () => {
    const items = await list_channels_api();
    const updated = items.find((value) => value.channel_type === current_item.channel_type);
    if (!updated) return;
    set_current_item(updated);
    on_saved(updated, false);
  }, [current_item.channel_type, on_saved]);

  useEffect(() => {
    if (!supports_personal_weixin_login || !login_id || login_status !== "running") {
      return;
    }
    const timer = window.setInterval(async () => {
      try {
        const next_login = await get_channel_login_api(current_item.channel_type, login_id);
        set_login_view(next_login);
        if (next_login.status === "succeeded") {
          void refresh_current_channel();
        }
      } catch (error) {
        window.clearInterval(timer);
        on_error(error instanceof Error ? error.message : "扫码登录状态刷新失败");
      }
    }, 1500);
    return () => window.clearInterval(timer);
  }, [current_item.channel_type, login_id, login_status, on_error, refresh_current_channel, supports_personal_weixin_login]);

  const save_channel = useCallback(async (close_on_success: boolean) => {
    if (!agent_id) return;
    if (is_planned) return;
    set_saving(true);
    try {
      const saved = await upsert_channel_config_api(current_item.channel_type, {
        agent_id,
        config,
        credentials,
      });
      set_current_item(saved);
      const should_start_login = is_personal_weixin_channel(saved.channel_type);
      on_saved(saved, !should_start_login);
      if (close_on_success && should_start_login) {
        set_login_loading(true);
        const next_login = await start_channel_login_api(saved.channel_type);
        set_login_view(next_login);
        return;
      }
      if (close_on_success) on_close();
    } catch (error) {
      on_error(error instanceof Error ? error.message : "连接失败");
    } finally {
      set_saving(false);
      set_login_loading(false);
    }
  }, [agent_id, config, credentials, current_item.channel_type, is_planned, on_close, on_error, on_saved]);

  const request_delete_channel = useCallback(() => {
    if (!current_item.configured || is_planned || deleting) return;
    set_pending_delete({ kind: "channel" });
  }, [current_item.configured, deleting, is_planned]);

  const delete_channel = useCallback(async () => {
    if (!current_item.configured || is_planned || deleting) return;
    set_deleting(true);
    try {
      await delete_channel_config_api(current_item.channel_type);
      notify_capability_summary_mutated({ source: "channels", action: "delete", channel_type: current_item.channel_type });
      await on_deleted(current_item);
      set_deleting(false);
      on_close();
    } catch (error) {
      on_error(error instanceof Error ? error.message : "断开频道失败");
      set_deleting(false);
    }
  }, [current_item, deleting, is_planned, on_close, on_deleted, on_error]);

  const request_delete_account = useCallback((account: ChannelAccountView) => {
    if (!account.account_id || deleting_account_id) return;
    set_pending_delete({ kind: "account", account });
  }, [deleting_account_id]);

  const delete_account = useCallback(async (account: ChannelAccountView) => {
    if (!account.account_id || deleting_account_id) return;
    set_deleting_account_id(account.account_id);
    try {
      const updated = await delete_channel_account_api(current_item.channel_type, account.account_id);
      set_current_item(updated);
      notify_capability_summary_mutated({ source: "channels", action: "delete_account", channel_type: current_item.channel_type });
      on_saved(updated, false);
    } catch (error) {
      on_error(error instanceof Error ? error.message : "删除账号失败");
    } finally {
      set_deleting_account_id("");
    }
  }, [current_item.channel_type, deleting_account_id, on_error, on_saved]);

  const confirm_delete = useCallback(() => {
    const target = pending_delete;
    if (!target) return;
    set_pending_delete(null);
    if (target.kind === "channel") {
      void delete_channel();
      return;
    }
    void delete_account(target.account);
  }, [delete_account, delete_channel, pending_delete]);

  const handle_submit = async (event: FormEvent) => {
    event.preventDefault();
    await save_channel(true);
  };

  return (
    <>
      <UiDialogPortal>
        <UiDialogBackdrop class_name="z-[9999]" labelled_by="channel-connect-dialog-title" on_close={on_close}>
          <UiDialogFormShell
            autoComplete="off"
            class_name="max-h-[86vh]"
            onSubmit={handle_submit}
            size="lg"
          >
            <UiDialogHeader
              icon={<ChannelIcon type={current_item.channel_type} size="dialog" />}
              icon_class_name="h-[52px] w-[52px] overflow-visible border-0 bg-transparent p-0 shadow-none"
              on_close={on_close}
              title={`连接 ${current_item.title}`}
              title_id="channel-connect-dialog-title"
            />

            <UiDialogBody class_name="space-y-5" scrollable>
              {is_planned ? (
                <UiStateBlock
                  description="频道接入将在后续版本补充，当前版本暂不支持配置机器人或配对。"
                  size="sm"
                  title="该频道未上线"
                  variant="inset"
                />
              ) : (
                <>
                  <ChannelGuide item={current_item} />

                  {current_item.runtime_note ? (
                    <div className="rounded-[14px] border border-(--divider-subtle-color) bg-transparent px-4 py-3 text-[13px] font-medium leading-5 text-(--text-default)">
                      {current_item.runtime_note}
                    </div>
                  ) : null}

                  {supports_personal_weixin_login ? (
                    <ChannelLoginPanel
                      loading={login_loading || saving}
                      login_view={login_view}
                      on_submit_verify_code={submit_verify_code}
                    />
                  ) : null}

                  {supports_personal_weixin_login ? (
                    <ChannelAccountsPanel
                      accounts={current_item.accounts || []}
                      deleting_account_id={deleting_account_id}
                      on_delete={request_delete_account}
                    />
                  ) : null}

                  <UiField label={<>处理智能体 <span className="text-(--destructive)">*</span></>}>
                    <UiSelectMenu
                      aria_label="选择频道处理智能体"
                      on_change={set_agent_id}
                      options={agents.map((agent) => ({
                        value: agent.agent_id,
                        label: agent.name,
                      }))}
                      size="sm"
                      value={agent_id}
                    />
                  </UiField>

                  <div className="space-y-4">
                    {current_item.credential_fields.map((field, index) => (
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
                          autoComplete={channel_field_autocomplete(field)}
                          autoCorrect="off"
                          data-1p-ignore="true"
                          data-form-type="other"
                          data-lpignore="true"
                          name={channel_field_input_name(current_item.channel_type, index)}
                          onChange={(event) => handle_field_change(field, event.target.value)}
                          placeholder={field.placeholder || ""}
                          required={field.required && !(field.secret && current_item.has_credentials)}
                          type={field.kind === "password" ? "password" : "text"}
                          value={field.secret ? credentials[field.key] || "" : config[field.key] || ""}
                          variant="dialog"
                        />
                      </UiField>
                    ))}
                  </div>

                  {current_item.channel_type === "discord" ? (
                    <UiField label="授权机器人到服务器">
                      <UiButton
                        class_name="w-full"
                        disabled={!discord_oauth_url}
                        onClick={() => discord_oauth_url && window.open(discord_oauth_url, "_blank", "noopener,noreferrer")}
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
                  {current_item.configured && !is_planned ? (
                    <UiButton
                      class_name="min-w-[118px]"
                      disabled={saving || deleting || login_loading}
                      onClick={request_delete_channel}
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
                    class_name="min-w-[104px]"
                    disabled={deleting}
                    onClick={on_close}
                    size="lg"
                    type="button"
                  >
                    取消
                  </UiButton>
                  <UiButton
                    class_name="min-w-[124px]"
                    disabled={saving || deleting || login_loading || login_running || !agent_id || is_planned}
                    size="lg"
                    tone="primary"
                    type="submit"
                    variant="solid"
                  >
                    {supports_personal_weixin_login ? <QrCode className="h-5 w-5" /> : <Power className="h-5 w-5" />}
                    {is_planned
                      ? "未上线"
                      : saving
                        ? "保存中..."
                        : login_loading
                          ? "拉起二维码..."
                          : login_running
                            ? "等待扫码..."
                            : supports_personal_weixin_login
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
        confirm_text={pending_delete?.kind === "channel" ? "断开频道" : "删除账号"}
        is_open={pending_delete !== null}
        message={pending_delete?.kind === "channel"
          ? `确认断开 ${current_item.title} 吗？这会停止该频道的机器人连接，但不会删除已有配对。`
          : pending_delete
            ? `确认删除微信账号 ${pending_delete.account.user_id || pending_delete.account.account_id} 吗？已有配对不会删除，但该账号会停止接收和回投消息。`
            : ""}
        on_cancel={() => set_pending_delete(null)}
        on_confirm={confirm_delete}
        title={pending_delete?.kind === "channel" ? "断开频道" : "删除微信账号"}
        variant="danger"
      />
    </>
  );
}
