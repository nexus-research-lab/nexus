// INPUT: Owner Pairing/Agent snapshots and exact create/update/delete intents.
// OUTPUT: Stale-safe reads, conservative mutation reconciliation, and replay locks.
// POS: Pairing directory controller; unknown writes are reconciled by reads, never auto-replayed.
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { getAgents } from "@/lib/api/agent/agent-api";
import {
  createPairingApi,
  deletePairingApi,
  listPairingsApi,
  updatePairingApi,
  type CreatePairingPayload,
  type PairingView,
  type UpdatePairingPayload,
} from "@/lib/api/capability/channel-api";
import { useCopyToClipboard } from "@/hooks/ui/use-copy-to-clipboard";
import type { Agent } from "@/types/agent/agent";
import { useI18n } from "@/shared/i18n/i18n-context";

import { notifyCapabilitySummaryMutated } from "../../capability-summary-events";
import type { ChannelFeedback } from "../channel-model";
import {
  buildChannelOperationIssue,
  channelOperationNeedsReconciliation,
  type ChannelOperationIssue,
} from "../channel-operation-recovery";
import {
  countPairingStatuses,
  filterPairings,
  groupPairings,
  pairingDisplayName,
  pairingSessionKey,
  type PairingFilters,
} from "./pairing-model";
import {
  reconcilePairingIntent,
  type PairingMutationIntent,
} from "./pairing-recovery";
import { usePairingCommand } from "./use-pairing-command";

const INITIAL_FILTERS: PairingFilters = {
  agentId: "",
  channel: "",
  query: "",
  status: "",
};

interface PairingMutationRecovery {
  check: "failed" | "not_checked" | "unproven";
  intent: PairingMutationIntent;
  issue: ChannelOperationIssue;
}

export function usePairingsController() {
  const { t } = useI18n();
  const requestIdRef = useRef(0);
  const { copy } = useCopyToClipboard();
  const [items, setItems] = useState<PairingView[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [filters, setFilters] = useState<PairingFilters>(INITIAL_FILTERS);
  const [loading, setLoading] = useState(true);
  const [readFailed, setReadFailed] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [createFailure, setCreateFailure] = useState<ChannelFeedback | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<PairingView | null>(null);
  const [feedback, setFeedback] = useState<ChannelFeedback | null>(null);
  const [recovery, setRecovery] = useState<PairingMutationRecovery | null>(null);
  const { pendingAction, runCommand } = usePairingCommand();

  const refresh = useCallback(async (reportError = true): Promise<boolean> => {
    const requestId = ++requestIdRef.current;
    setLoading(true);
    try {
      const [pairingResult, agentResult] = await Promise.allSettled([
        listPairingsApi(),
        getAgents(),
      ]);
      if (requestId !== requestIdRef.current) {
        return false;
      }
      if (pairingResult.status === "fulfilled") {
        setItems(pairingResult.value);
        setReadFailed(false);
      } else {
        setReadFailed(true);
      }
      if (agentResult.status === "fulfilled") {
        setAgents(agentResult.value);
      }
      const complete = pairingResult.status === "fulfilled"
        && agentResult.status === "fulfilled";
      if (!complete && reportError) {
        setFeedback({
          impact: t("capability.channel_pairing_catalog_load_failed_impact"),
          nextStep: t("capability.channel_pairing_catalog_load_failed_next_step"),
          tone: "error",
          title: t("capability.channel_pairing_catalog_load_failed_title"),
          message: t("capability.channel_pairing_catalog_load_failed_message"),
        });
      }
      return complete;
    } catch {
      if (requestId === requestIdRef.current) {
        setReadFailed(true);
        if (reportError) {
          setFeedback({
            impact: t("capability.channel_pairing_catalog_load_failed_impact"),
            nextStep: t("capability.channel_pairing_catalog_load_failed_next_step"),
            tone: "error",
            title: t("capability.channel_pairing_catalog_load_failed_title"),
            message: t("capability.channel_pairing_catalog_load_failed_message"),
          });
        }
      }
      return false;
    } finally {
      if (requestId === requestIdRef.current) {
        setLoading(false);
      }
    }
  }, [t]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const visibleItems = useMemo(
    () => filterPairings(items, filters),
    [filters, items],
  );
  const statusScopeItems = useMemo(
    () => filterPairings(items, { ...filters, status: "" }),
    [filters, items],
  );
  const statusCounts = useMemo(
    () => countPairingStatuses(statusScopeItems),
    [statusScopeItems],
  );
  const pendingItems = useMemo(
    () => visibleItems.filter((item) => item.status === "pending"),
    [visibleItems],
  );
  const groups = useMemo(
    () => groupPairings(
      visibleItems.filter((item) => item.status !== "pending"),
      agents,
    ),
    [agents, visibleItems],
  );

  const updatePairing = useCallback(async (
    item: PairingView,
    next: UpdatePairingPayload,
  ) => {
    if (recovery) {
      return;
    }
    const intent: PairingMutationIntent = {
      kind: "update",
      pairingId: item.pairing_id,
      patch: next,
    };
    await runCommand({ kind: "update", pairingId: item.pairing_id }, async () => {
      try {
        const updated = await updatePairingApi(item.pairing_id, next);
        notifyCapabilitySummaryMutated({
          source: "pairings",
          action: "update",
          pairing_id: updated.pairing_id,
        });
        const refreshed = await refresh(false);
        setFeedback(refreshed
          ? {
              tone: "success",
              title: t("capability.channel_pairing_updated_title"),
              message: t("capability.channel_pairing_updated_message", {
                name: pairingDisplayName(updated),
              }),
            }
          : {
              impact: t("state.committed_refresh_impact"),
              nextStep: t("state.committed_refresh_next_step"),
              tone: "error",
              title: t("capability.channel_pairing_updated_refresh_failed_title"),
              message: t("capability.channel_pairing_refresh_failed_message"),
            });
      } catch (error) {
        const issue = buildChannelOperationIssue(error, "pairing_update", t);
        if (channelOperationNeedsReconciliation(issue)) {
          setRecovery({ check: "not_checked", intent, issue });
        } else {
          setFeedback({ ...issue, dismissible: true });
        }
      }
    });
  }, [recovery, refresh, runCommand, t]);

  const deletePairing = useCallback(async (item: PairingView) => {
    if (recovery) {
      return;
    }
    const intent: PairingMutationIntent = {
      kind: "delete",
      pairingId: item.pairing_id,
    };
    await runCommand({ kind: "delete", pairingId: item.pairing_id }, async () => {
      try {
        await deletePairingApi(item.pairing_id);
        notifyCapabilitySummaryMutated({
          source: "pairings",
          action: "delete",
          pairing_id: item.pairing_id,
        });
        const refreshed = await refresh(false);
        setFeedback(refreshed
          ? {
              tone: "success",
              title: t("capability.channel_pairing_deleted_title"),
              message: t("capability.channel_pairing_deleted_message", {
                name: pairingDisplayName(item),
              }),
            }
          : {
              impact: t("state.committed_refresh_impact"),
              nextStep: t("state.committed_refresh_next_step"),
              tone: "error",
              title: t("capability.channel_pairing_deleted_refresh_failed_title"),
              message: t("capability.channel_pairing_refresh_failed_message"),
            });
      } catch (error) {
        const issue = buildChannelOperationIssue(error, "pairing_delete", t);
        if (channelOperationNeedsReconciliation(issue)) {
          setRecovery({ check: "not_checked", intent, issue });
        } else {
          setFeedback({ ...issue, dismissible: true });
        }
      }
    });
  }, [recovery, refresh, runCommand, t]);

  const confirmDelete = useCallback(() => {
    const target = deleteTarget;
    setDeleteTarget(null);
    if (target) {
      void deletePairing(target);
    }
  }, [deletePairing, deleteTarget]);

  const createPairing = useCallback(async (
    payload: CreatePairingPayload,
  ): Promise<boolean> => {
    if (recovery) {
      return false;
    }
    setCreateFailure(null);
    const intent: PairingMutationIntent = { kind: "create", payload };
    try {
      const created = await createPairingApi(payload);
      notifyCapabilitySummaryMutated({
        source: "pairings",
        action: "create",
        pairing_id: created.pairing_id,
      });
      const refreshed = await refresh(false);
      setFeedback(refreshed
        ? {
            tone: "success",
            title: t("capability.channel_pairing_created_title"),
            message: t("capability.channel_pairing_created_message"),
          }
        : {
            impact: t("state.committed_refresh_impact"),
            nextStep: t("state.committed_refresh_next_step"),
            tone: "error",
            title: t("capability.channel_pairing_created_refresh_failed_title"),
            message: t("capability.channel_pairing_created_refresh_failed_message"),
          });
      return true;
    } catch (error) {
      const issue = buildChannelOperationIssue(error, "pairing_create", t);
      if (channelOperationNeedsReconciliation(issue)) {
        setRecovery({ check: "not_checked", intent, issue });
      } else {
        setCreateFailure({ ...issue, dismissible: true });
      }
      return false;
    }
  }, [recovery, refresh, t]);

  const copySessionKey = useCallback(async (item: PairingView) => {
    const key = pairingSessionKey(item);
    const copied = await copy(key);
    setFeedback(copied
      ? {
          tone: "success",
          title: t("capability.channel_pairing_session_copied_title"),
          message: t("capability.channel_pairing_session_copied_message"),
        }
      : {
          impact: t("state.local_failure_impact"),
          message: t("capability.channel_pairing_session_copy_failed_message"),
          nextStep: t("state.local_failure_next_step"),
          tone: "error",
          title: t("capability.channel_pairing_session_copy_failed_title"),
        });
  }, [copy, t]);

  const reconcileMutation = useCallback(async () => {
    if (!recovery) {
      return;
    }
    try {
      const latest = await listPairingsApi();
      setItems(latest);
      setReadFailed(false);
      const outcome = reconcilePairingIntent(recovery.intent, latest);
      if (outcome !== "applied" && recovery.issue.effect !== "committed") {
        setRecovery((current) => current
          ? { ...current, check: "unproven" }
          : null);
        return;
      }
      const created = recovery.intent.kind === "create";
      setRecovery(null);
      if (created) {
        setCreateOpen(false);
      }
      setFeedback({
        message: t("capability.channel_reconcile_success_message"),
        title: t("capability.channel_reconcile_success_title"),
        tone: "success",
      });
    } catch {
      setRecovery((current) => current
        ? { ...current, check: "failed" }
        : null);
    }
  }, [recovery, t]);

  const recoveryFeedback = useMemo<ChannelFeedback | null>(() => {
    if (!recovery) {
      return null;
    }
    const failed = recovery.check === "failed";
    const unproven = recovery.check === "unproven";
    return {
      action: {
        label: unproven
          ? t("capability.channel_start_new_intent_action")
          : t("capability.channel_reconcile_action"),
        onClick: () => {
          if (unproven) {
            setRecovery(null);
          } else {
            void reconcileMutation();
          }
        },
      },
      dismissible: false,
      impact: unproven
        ? t("capability.channel_reconcile_unproven_impact")
        : recovery.issue.impact,
      message: failed
        ? t("capability.channel_reconcile_failed_message")
        : unproven
          ? t("capability.channel_reconcile_unproven_message")
          : recovery.issue.message,
      nextStep: failed
        ? t("capability.channel_reconcile_failed_next_step")
        : unproven
          ? t("capability.channel_reconcile_unproven_next_step")
          : recovery.issue.nextStep,
      title: recovery.issue.title,
      tone: recovery.issue.tone,
    };
  }, [reconcileMutation, recovery, t]);

  const setFilter = useCallback(<Key extends keyof PairingFilters>(
    key: Key,
    value: PairingFilters[Key],
  ) => {
    setFilters((current) => ({ ...current, [key]: value }));
  }, []);
  const clearFilters = useCallback(() => setFilters(INITIAL_FILTERS), []);
  const clearFeedback = useCallback(() => setFeedback(null), []);
  const closeCreate = useCallback(() => {
    if (recovery?.intent.kind !== "create") {
      setCreateOpen(false);
      setCreateFailure(null);
    }
  }, [recovery]);
  const openCreate = useCallback(() => {
    if (!recovery) {
      setCreateFailure(null);
      setCreateOpen(true);
    }
  }, [recovery]);

  return {
    agents,
    busy: pendingAction !== null || recovery !== null,
    clearFeedback,
    clearFilters,
    closeCreate,
    confirmDelete,
    copySessionKey,
    createPairing,
    createRecoveryFeedback: recovery?.intent.kind === "create"
      ? recoveryFeedback
      : createFailure,
    createOpen,
    deleteTarget,
    feedback: recoveryFeedback ?? feedback,
    filters,
    groups,
    items,
    loading,
    openCreate,
    pendingItems,
    readFailed,
    refresh,
    requestDelete: setDeleteTarget,
    setFilter,
    statusCounts,
    updatePairing,
    visibleItems,
  };
}
