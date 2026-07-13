import { useCallback, useState } from "react";

import {
  deleteChannelAccountApi,
  deleteChannelConfigApi,
  listChannelsApi,
  upsertChannelConfigApi,
  type ChannelAccountView,
  type ChannelConfigView,
  type ChannelCredentialField,
} from "@/lib/api/capability/channel-api";
import { getErrorMessage } from "@/lib/error-message";
import type { Agent } from "@/types/agent/agent";

import { notifyCapabilitySummaryMutated } from "../../capability-summary-events";
import { isChannelPlanned } from "../channel-model";
import {
  buildDiscordOauthUrl,
  createChannelDraft,
  isPersonalWeixinChannel,
  type PendingChannelDelete,
} from "./channel-connection-model";
import { useChannelLoginController } from "./login/use-channel-login-controller";
import { useChannelCommand } from "./use-channel-command";

interface UseChannelConnectionOptions {
  agents: Agent[];
  item: ChannelConfigView;
  onClose: () => void;
  onDeleted: (item: ChannelConfigView) => Promise<void> | void;
  onError: (message: string) => void;
  onSaved: (item: ChannelConfigView, announce?: boolean) => void;
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
  const [pendingDelete, setPendingDelete] =
    useState<PendingChannelDelete | null>(null);
  const { pendingAction, runCommand } = useChannelCommand();

  const supportsPersonalWeixinLogin = isPersonalWeixinChannel(
    currentItem.channel_type,
  );
  const planned = isChannelPlanned(currentItem);
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

  const {
    loading: loginLoading,
    running: loginRunning,
    startLogin,
    submitVerifyCode,
    view: loginView,
  } = useChannelLoginController({
    channelType: currentItem.channel_type,
    enabled: supportsPersonalWeixinLogin,
    onCompleted: refreshCurrentChannel,
    onError,
    pendingAction,
    runCommand,
  });

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
          await startLogin();
        } else {
          onClose();
        }
        return true;
      } catch (error) {
        onError(getErrorMessage(error, "连接失败"));
        return false;
      }
    });
    return result ?? false;
  }, [
    currentItem.channel_type,
    draft,
    onClose,
    onError,
    onSaved,
    planned,
    runCommand,
    startLogin,
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
        onError(getErrorMessage(error, "断开频道失败"));
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
        onError(getErrorMessage(error, "删除账号失败"));
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
    loginLoading,
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
