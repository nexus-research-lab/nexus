import { useCallback, useEffect, useState } from "react";

import {
  deleteChannelAccountApi,
  deleteChannelConfigApi,
  getChannelLoginApi,
  listChannelsApi,
  startChannelLoginApi,
  submitChannelLoginVerifyCodeApi,
  upsertChannelConfigApi,
  type ChannelAccountView,
  type ChannelConfigView,
  type ChannelCredentialField,
  type ChannelLoginView,
} from "@/lib/api/capability/channel-api";
import type { Agent } from "@/types/agent/agent";

import { notifyCapabilitySummaryMutated } from "../../capability-summary-events";
import { isChannelPlanned } from "../channel-model";
import {
  buildDiscordOauthUrl,
  createChannelDraft,
  isChannelLoginRunning,
  isPersonalWeixinChannel,
  type PendingChannelDelete,
} from "./channel-connection-model";
import { useChannelCommand } from "./use-channel-command";

interface UseChannelConnectionOptions {
  agents: Agent[];
  item: ChannelConfigView;
  onClose: () => void;
  onDeleted: (item: ChannelConfigView) => Promise<void> | void;
  onError: (message: string) => void;
  onSaved: (item: ChannelConfigView, announce?: boolean) => void;
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

export function useChannelConnectionController({
  agents,
  item,
  onClose,
  onDeleted,
  onError,
  onSaved,
}: UseChannelConnectionOptions) {
  const [currentItem, setCurrentItem] = useState(item);
  const [draft, setDraft] = useState(() => createChannelDraft(
    item,
    agents[0]?.agent_id || "",
  ));
  const [loginView, setLoginView] = useState<ChannelLoginView | null>(null);
  const [pendingDelete, setPendingDelete] =
    useState<PendingChannelDelete | null>(null);
  const { pendingAction, runCommand } = useChannelCommand();

  const supportsPersonalWeixinLogin = isPersonalWeixinChannel(
    currentItem.channel_type,
  );
  const planned = isChannelPlanned(currentItem);
  const loginRunning = isChannelLoginRunning(loginView);

  const updateField = useCallback((
    field: ChannelCredentialField,
    value: string,
  ) => {
    setDraft((current) => field.secret
      ? {
          ...current,
          credentials: { ...current.credentials, [field.key]: value },
        }
      : {
          ...current,
          config: { ...current.config, [field.key]: value },
        });
  }, []);

  const refreshCurrentChannel = useCallback(async () => {
    const items = await listChannelsApi();
    const updated = items.find(
      (value) => value.channel_type === currentItem.channel_type,
    );
    if (updated) {
      setCurrentItem(updated);
      onSaved(updated, false);
    }
  }, [currentItem.channel_type, onSaved]);

  useEffect(() => {
    if (!supportsPersonalWeixinLogin || !loginView?.login_id || !loginRunning) {
      return;
    }
    let disposed = false;
    let timer = 0;
    const poll = async () => {
      try {
        const nextLogin = await getChannelLoginApi(
          currentItem.channel_type,
          loginView.login_id,
        );
        if (disposed) {
          return;
        }
        setLoginView(nextLogin);
        if (nextLogin.status === "succeeded") {
          await refreshCurrentChannel();
          return;
        }
        if (nextLogin.status === "running") {
          timer = window.setTimeout(poll, 1500);
        }
      } catch (error) {
        if (!disposed) {
          onError(errorMessage(error, "扫码登录状态刷新失败"));
        }
      }
    };
    timer = window.setTimeout(poll, 1500);
    return () => {
      disposed = true;
      window.clearTimeout(timer);
    };
  }, [
    currentItem.channel_type,
    loginRunning,
    loginView?.login_id,
    onError,
    refreshCurrentChannel,
    supportsPersonalWeixinLogin,
  ]);

  const saveChannel = useCallback(async () => {
    if (!draft.agentId || planned) {
      return false;
    }
    const result = await runCommand({ kind: "save" }, async () => {
      try {
        const saved = await upsertChannelConfigApi(currentItem.channel_type, {
          agent_id: draft.agentId,
          config: draft.config,
          credentials: draft.credentials,
        });
        setCurrentItem(saved);
        const shouldStartLogin = isPersonalWeixinChannel(saved.channel_type);
        onSaved(saved, !shouldStartLogin);
        if (shouldStartLogin) {
          setLoginView(await startChannelLoginApi(saved.channel_type));
        } else {
          onClose();
        }
        return true;
      } catch (error) {
        onError(errorMessage(error, "连接失败"));
        return false;
      }
    });
    return result ?? false;
  }, [currentItem.channel_type, draft, onClose, onError, onSaved, planned, runCommand]);

  const submitVerifyCode = useCallback(async (value: string) => {
    if (!supportsPersonalWeixinLogin || !loginView?.login_id) {
      return false;
    }
    const result = await runCommand({ kind: "verify-code" }, async () => {
      try {
        const nextLogin = await submitChannelLoginVerifyCodeApi(
          currentItem.channel_type,
          loginView.login_id,
          value,
        );
        setLoginView(nextLogin);
        return true;
      } catch (error) {
        onError(errorMessage(error, "验证码提交失败"));
        return false;
      }
    });
    return result ?? false;
  }, [
    currentItem.channel_type,
    loginView?.login_id,
    onError,
    runCommand,
    supportsPersonalWeixinLogin,
  ]);

  const deleteChannel = useCallback(async () => {
    if (!currentItem.configured || planned) {
      return;
    }
    await runCommand({ kind: "delete-channel" }, async () => {
      try {
        await deleteChannelConfigApi(currentItem.channel_type);
        notifyCapabilitySummaryMutated({
          source: "channels",
          action: "delete",
          channel_type: currentItem.channel_type,
        });
        await onDeleted(currentItem);
        onClose();
      } catch (error) {
        onError(errorMessage(error, "断开频道失败"));
      }
    });
  }, [currentItem, onClose, onDeleted, onError, planned, runCommand]);

  const deleteAccount = useCallback(async (account: ChannelAccountView) => {
    if (!account.account_id) {
      return;
    }
    await runCommand({
      kind: "delete-account",
      accountId: account.account_id,
    }, async () => {
      try {
        const updated = await deleteChannelAccountApi(
          currentItem.channel_type,
          account.account_id,
        );
        setCurrentItem(updated);
        notifyCapabilitySummaryMutated({
          source: "channels",
          action: "delete_account",
          channel_type: currentItem.channel_type,
        });
        onSaved(updated, false);
      } catch (error) {
        onError(errorMessage(error, "删除账号失败"));
      }
    });
  }, [currentItem.channel_type, onError, onSaved, runCommand]);

  const confirmDelete = useCallback(() => {
    const target = pendingDelete;
    setPendingDelete(null);
    if (target?.kind === "channel") {
      void deleteChannel();
    }
    if (target?.kind === "account") {
      void deleteAccount(target.account);
    }
  }, [deleteAccount, deleteChannel, pendingDelete]);

  const deletingAccountId = pendingAction?.kind === "delete-account"
    ? pendingAction.accountId
    : "";
  const busy = pendingAction !== null;

  return {
    busy,
    confirmDelete,
    currentItem,
    deleting: pendingAction?.kind === "delete-channel",
    deletingAccountId,
    discordOauthUrl: currentItem.channel_type === "discord"
      ? buildDiscordOauthUrl(draft.config)
      : "",
    draft,
    loginLoading: pendingAction?.kind === "save"
      || pendingAction?.kind === "verify-code",
    loginRunning,
    loginView,
    pendingDelete,
    planned,
    requestDeleteAccount: (account: ChannelAccountView) => {
      if (account.account_id && !busy) {
        setPendingDelete({ kind: "account", account });
      }
    },
    requestDeleteChannel: () => {
      if (currentItem.configured && !planned && !busy) {
        setPendingDelete({ kind: "channel" });
      }
    },
    saveChannel,
    saving: pendingAction?.kind === "save",
    setAgentId: (agentId: string) => setDraft((current) => ({
      ...current,
      agentId,
    })),
    setPendingDelete,
    submitVerifyCode,
    supportsPersonalWeixinLogin,
    updateField,
  };
}
