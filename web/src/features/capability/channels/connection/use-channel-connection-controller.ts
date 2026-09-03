// INPUT: Exact Channel snapshot, editable draft, account/config mutations, and login lifecycle.
// OUTPUT: Transaction-stage feedback, read-only reconciliation, and current-page replay locks.
// POS: Channel connection controller; unresolved writes are never repeated by refresh or retry.
import { useCallback, useMemo, useState } from "react";

import {
  deleteChannelAccountApi,
  deleteChannelConfigApi,
  listChannelsApi,
  upsertChannelConfigApi,
  type ChannelAccountView,
  type ChannelConfigView,
  type ChannelCredentialField,
} from "@/lib/api/capability/channel-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner-contract";
import type { Agent } from "@/types/agent/agent";

import { notifyCapabilitySummaryMutated } from "../../capability-summary-events";
import { isChannelPlanned } from "../channel-model";
import {
  buildChannelOperationIssue,
  channelOperationNeedsReconciliation,
  type ChannelOperationIssue,
} from "../channel-operation-recovery";
import {
  buildDiscordOauthUrl,
  createChannelDraft,
  hasCompleteManualChannelCredentials,
  isPersonalWeixinChannel,
  shouldStartChannelQRCodeLogin,
  type PendingChannelDelete,
} from "./channel-connection-model";
import {
  reconcileChannelConnectionIntent,
  type ChannelConnectionIntent,
} from "./channel-connection-recovery";
import { useChannelLoginController } from "./login/use-channel-login-controller";
import { useChannelCommand } from "./use-channel-command";

interface UseChannelConnectionOptions {
  agents: Agent[];
  item: ChannelConfigView;
  onClose: () => void;
  onDeleted: (item: ChannelConfigView) => Promise<void> | void;
  onSaved: (item: ChannelConfigView, announce?: boolean) => void;
}

interface ChannelConnectionRecovery {
  check: "failed" | "not_checked" | "unproven";
  intent: ChannelConnectionIntent;
  issue: ChannelOperationIssue;
}

export function useChannelConnectionController({
  agents,
  item,
  onClose,
  onDeleted,
  onSaved,
}: UseChannelConnectionOptions) {
  const { t } = useI18n();
  const [currentItem, setCurrentItem] = useState(item);
  const [draft, setDraft] = useState(() => createChannelDraft(
    item,
    agents[0]?.agent_id || "",
  ));
  const [pendingDelete, setPendingDelete] =
    useState<PendingChannelDelete | null>(null);
  const [recovery, setRecovery] =
    useState<ChannelConnectionRecovery | null>(null);
  const { pendingAction, runCommand } = useChannelCommand();

  const personalWeixin = isPersonalWeixinChannel(
    currentItem.channel_type,
  );
  const supportsQRCode = currentItem.supports_qr_code;
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
      return;
    }
    throw new Error("channel snapshot is unavailable");
  }, [currentItem.channel_type, onSaved]);

  const {
    loading: loginLoading,
    running: loginRunning,
    startLogin,
    submitVerifyCode,
    view: loginView,
    mutationBlocked: loginMutationBlocked,
    recoveryNotice: loginRecoveryNotice,
  } = useChannelLoginController({
    channelType: currentItem.channel_type,
    enabled: supportsQRCode,
    onCompleted: refreshCurrentChannel,
    pendingAction,
    runCommand,
  });
  const hasManualCredentials = hasCompleteManualChannelCredentials(
    currentItem.channel_type,
    draft,
  );
  const offersQRCode = shouldStartChannelQRCodeLogin(
    currentItem,
    hasManualCredentials,
  );
  const showsQRCode = offersQRCode || loginView !== null;

  const saveChannel = useCallback(async () => {
    if (!draft.agentId || planned || recovery || loginMutationBlocked) {
      return false;
    }
    const intent: ChannelConnectionIntent = {
      agentId: draft.agentId,
      baseHadCredentials: currentItem.has_credentials,
      channelType: currentItem.channel_type,
      kind: "save",
      publicConfig: { ...draft.config },
      wroteSecrets: Object.values(draft.credentials).some(
        (value) => value.trim() !== "",
      ),
    };
    const result = await runCommand({ kind: "save" }, async () => {
      try {
        const saved = await upsertChannelConfigApi(currentItem.channel_type, {
          agent_id: draft.agentId,
          config: draft.config,
          credentials: draft.credentials,
        });
        setCurrentItem(saved);
        const shouldStartLogin = shouldStartChannelQRCodeLogin(
          saved,
          hasManualCredentials,
        );
        onSaved(saved, !shouldStartLogin);
        if (shouldStartLogin) {
          await startLogin();
        } else {
          onClose();
        }
        return true;
      } catch (error) {
        const issue = buildChannelOperationIssue(error, "channel_save", t);
        setRecovery({ check: "not_checked", intent, issue });
        return false;
      }
    });
    return result ?? false;
  }, [
    currentItem.has_credentials,
    currentItem.channel_type,
    draft,
    hasManualCredentials,
    onClose,
    onSaved,
    planned,
    recovery,
    loginMutationBlocked,
    runCommand,
    startLogin,
    t,
  ]);

  const deleteChannel = useCallback(async () => {
    if (!currentItem.configured || planned || recovery || loginMutationBlocked) {
      return;
    }
    const intent: ChannelConnectionIntent = {
      channelType: currentItem.channel_type,
      kind: "delete-channel",
    };
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
        const issue = buildChannelOperationIssue(error, "channel_delete", t);
        setRecovery({ check: "not_checked", intent, issue });
      }
    });
  }, [
    currentItem,
    loginMutationBlocked,
    onClose,
    onDeleted,
    planned,
    recovery,
    runCommand,
    t,
  ]);

  const deleteAccount = useCallback(async (account: ChannelAccountView) => {
    if (!account.account_id || recovery || loginMutationBlocked) {
      return;
    }
    const intent: ChannelConnectionIntent = {
      accountId: account.account_id,
      channelType: currentItem.channel_type,
      kind: "delete-account",
    };
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
        const issue = buildChannelOperationIssue(error, "account_delete", t);
        setRecovery({ check: "not_checked", intent, issue });
      }
    });
  }, [
    currentItem.channel_type,
    loginMutationBlocked,
    onSaved,
    recovery,
    runCommand,
    t,
  ]);

  const reconcileConnection = useCallback(async () => {
    if (!recovery) {
      return;
    }
    try {
      const items = await listChannelsApi();
      const updated = items.find(
        (value) => value.channel_type === recovery.intent.channelType,
      );
      if (updated) {
        setCurrentItem(updated);
        onSaved(updated, false);
      }
      const observed = reconcileChannelConnectionIntent(recovery.intent, items);
      if (observed !== "applied" && recovery.issue.effect !== "committed") {
        setRecovery((current) => current
          ? { ...current, check: "unproven" }
          : null);
        return;
      }
      setRecovery(null);
      if (recovery.intent.kind === "delete-channel" && !updated?.configured) {
        await onDeleted(currentItem);
        onClose();
      }
    } catch {
      setRecovery((current) => current
        ? { ...current, check: "failed" }
        : null);
    }
  }, [currentItem, onClose, onDeleted, onSaved, recovery]);

  const connectionRecoveryNotice = useMemo<FeedbackBannerProps | null>(() => {
    if (!recovery) {
      return null;
    }
    const needsReconciliation = channelOperationNeedsReconciliation(
      recovery.issue,
    );
    const unproven = recovery.check === "unproven";
    const base = {
      impact: unproven
        ? t("capability.channel_reconcile_unproven_impact")
        : recovery.issue.impact,
      title: recovery.issue.title,
      tone: recovery.issue.tone,
    } as const;
    if (!needsReconciliation) {
      return {
        ...base,
        action: {
          label: t("capability.channel_continue_action"),
          onClick: () => setRecovery(null),
        },
      };
    }
    return {
      ...base,
      action: {
        label: unproven
          ? t("capability.channel_start_new_intent_action")
          : t("capability.channel_reconcile_action"),
        onClick: () => {
          if (unproven) {
            setRecovery(null);
          } else {
            void reconcileConnection();
          }
        },
      },
    };
  }, [reconcileConnection, recovery, t]);

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
  const closeBlocked = pendingAction !== null
    || recovery !== null
    || loginMutationBlocked;
  const busy = closeBlocked || loginRecoveryNotice !== null;
  const close = useCallback(() => {
    if (!closeBlocked) {
      onClose();
    }
  }, [closeBlocked, onClose]);

  return {
    busy,
    close,
    closeBlocked,
    connectionRecoveryNotice,
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
    loginMutationBlocked,
    loginRecoveryNotice,
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
    showsQRCode,
    submitVerifyCode,
    personalWeixin,
    offersQRCode,
    supportsQRCode,
    updateField,
  };
}
