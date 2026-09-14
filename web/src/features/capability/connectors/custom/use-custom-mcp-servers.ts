"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { notifyCapabilitySummaryMutated } from "@/features/capability/capability-summary-events";
import {
  createCustomMCPServerApi,
  deleteCustomMCPServerApi,
  getCustomMCPServersApi,
  setCustomMCPServerEnabledApi,
  updateCustomMCPServerApi,
} from "@/lib/api/capability/connector-api";
import { getResourceFailure, projectMutationFailure, type ResourceFailure } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  CustomMCPServer,
  CustomMCPServerInput,
} from "@/types/capability/connector";

import type { ReportConnectorFeedback } from "../controller/connector-controller-types";
import { getCustomMCPDisplayName } from "./custom-mcp-model";

type CustomMCPDialogState =
  | { mode: "create" }
  | { mode: "edit"; server: CustomMCPServer }
  | null;

interface UseCustomMCPServersOptions {
  enabled: boolean;
  onCatalogChanged: () => Promise<void>;
  reportFeedback: ReportConnectorFeedback;
}

export function useCustomMCPServers({
  enabled,
  onCatalogChanged,
  reportFeedback,
}: UseCustomMCPServersOptions) {
  const { t } = useI18n();
  const requestIdRef = useRef(0);
  const commandRef = useRef(false);
  const recoveryRef = useRef(false);
  const accessDeniedRef = useRef(false);
  const [blocked, setBlocked] = useState(false);
  const [servers, setServers] = useState<CustomMCPServer[]>([]);
  const [loading, setLoading] = useState(false);
  const [failure, setFailure] = useState<ResourceFailure | null>(null);
  const [busy, setBusy] = useState(false);
  const [dialogState, setDialogState] = useState<CustomMCPDialogState>(null);
  const [deleteTarget, setDeleteTarget] = useState<CustomMCPServer | null>(null);

  const refresh = useCallback(async (
    onFailure?: () => void,
  ): Promise<boolean> => {
    const requestId = ++requestIdRef.current;
    setLoading(true);
    try {
      const items = await getCustomMCPServersApi();
      if (requestId === requestIdRef.current) {
        setServers(items);
        setFailure(null);
        accessDeniedRef.current = false;
      }
      return requestId === requestIdRef.current;
    } catch (error) {
      if (requestId === requestIdRef.current) {
        const nextFailure = getResourceFailure(error, t("capability.custom_mcp_operation_failed"));
        setFailure(nextFailure);
        accessDeniedRef.current = Boolean(nextFailure.access);
        if (nextFailure.access) setServers([]);
        if (onFailure) {
          onFailure();
        } else {
          reportFeedback({
            action: {
              label: t("state.retry"),
              onClick: () => { void refresh(); },
            },
            impact: t("state.read_failure_impact"),
            nextStep: t("state.retry_next_step"),
            title: t("capability.custom_mcp_operation_failed"),
            tone: "error",
          });
        }
      }
      return false;
    } finally {
      if (requestId === requestIdRef.current) {
        setLoading(false);
      }
    }
  }, [reportFeedback, t]);

  useEffect(() => {
    if (!enabled) {
      requestIdRef.current += 1;
      return;
    }
    void refresh();
    return () => { requestIdRef.current += 1; };
  }, [enabled, refresh]);

  const runCommand = useCallback(async (
    command: () => Promise<void>,
    fallbackMessage: string,
  ): Promise<boolean> => {
    if (commandRef.current || recoveryRef.current || accessDeniedRef.current) return false;
    commandRef.current = true;
    setBusy(true);
    try {
      await command();
      return true;
    } catch (error) {
      const failure = projectMutationFailure(error, fallbackMessage);
      const notApplied = failure.effect === "not_applied";
      if (!notApplied) {
        recoveryRef.current = true;
        setBlocked(true);
      }
      async function reconcile() {
        const refreshed = await refresh(() => reportFailure(true));
        if (!refreshed) return;
        if (failure.effect === "committed") {
          recoveryRef.current = false;
          setBlocked(false);
          reportFeedback({ tone: "success", title: t("capability.connector_reconcile_success_title") });
        } else if (failure.effect === "accepted") {
          reportFailure(false);
        } else {
          reportFeedback({
            action: {
              label: t("capability.connector_new_intent_action"),
              onClick: () => {
                recoveryRef.current = false;
                setBlocked(false);
                reportFeedback({ tone: "warning", title: t("capability.connector_unknown_title"), message: t("capability.connector_checked_unknown_next_step") });
              },
            },
            title: t("capability.custom_mcp_unknown_title"),
            impact: t("capability.connector_checked_unknown_impact"),
            nextStep: t("capability.connector_checked_unknown_next_step"),
            persistent: true,
            tone: "warning",
          });
        }
      }
      function reportFailure(refreshFailed: boolean) {
        reportFeedback({
          action: notApplied
            ? undefined
            : {
                label: t("state.reload_check"),
                onClick: reconcile,
              },
          impact: notApplied
            ? t("capability.custom_mcp_not_applied_impact")
            : failure.effect === "committed"
              ? t("state.committed_refresh_impact")
              : t("feedback.unconfirmed_impact"),
          message: refreshFailed
            ? `${failure.message} ${t("capability.custom_mcp_reconcile_failed_message")}`
            : failure.message,
          nextStep: notApplied
            ? t("capability.custom_mcp_not_applied_next_step")
            : failure.effect === "committed"
              ? t("state.committed_refresh_next_step")
              : t("feedback.unconfirmed_next_step"),
          persistent: !notApplied,
          title: notApplied
            ? t("capability.custom_mcp_operation_failed")
            : failure.effect === "committed"
              ? t("capability.custom_mcp_committed_title")
              : t("capability.custom_mcp_unknown_title"),
          tone: notApplied ? "error" : "warning",
        });
      }
      reportFailure(false);
      return false;
    } finally {
      commandRef.current = false;
      setBusy(false);
    }
  }, [refresh, reportFeedback, t]);

  const save = useCallback(async (
    input: CustomMCPServerInput,
  ): Promise<boolean> => {
    const currentDialog = dialogState;
    if (!currentDialog) return false;
    return runCommand(async () => {
      const item = currentDialog.mode === "edit"
        ? await updateCustomMCPServerApi(
            currentDialog.server.connector_id,
            input,
          )
        : await createCustomMCPServerApi(input);
      requestIdRef.current += 1;
      setLoading(false);
      setServers((current) => sortServers(
        currentDialog.mode === "edit"
          ? current.map((server) => (
              server.connector_id === item.connector_id ? item : server
            ))
          : [...current, item],
      ));
      setDialogState(null);
      notifyCapabilitySummaryMutated({
        action: currentDialog.mode,
        connector_id: item.connector_id,
        source: "custom-mcp",
      });
      reportFeedback({
        message: t(
          currentDialog.mode === "edit"
            ? "capability.custom_mcp_updated_message"
            : "capability.custom_mcp_created_message",
          { name: item.name },
        ),
        title: t(
          currentDialog.mode === "edit"
            ? "capability.custom_mcp_updated_title"
            : "capability.custom_mcp_created_title",
        ),
        tone: "success",
      });
      void onCatalogChanged();
    }, t("capability.custom_mcp_save_failed"));
  }, [dialogState, onCatalogChanged, reportFeedback, runCommand, t]);

  const confirmDelete = useCallback(async (): Promise<boolean> => {
    const target = deleteTarget;
    setDeleteTarget(null);
    if (!target) return false;
    return runCommand(async () => {
      await deleteCustomMCPServerApi(target.connector_id);
      requestIdRef.current += 1;
      setLoading(false);
      setServers((current) => current.filter(
        (server) => server.connector_id !== target.connector_id,
      ));
      notifyCapabilitySummaryMutated({
        action: "delete",
        connector_id: target.connector_id,
        source: "custom-mcp",
      });
      reportFeedback({
        message: t("capability.custom_mcp_deleted_message", {
          name: getCustomMCPDisplayName(
            target,
            t("capability.custom_mcp_recovery_name"),
          ),
        }),
        title: t("capability.custom_mcp_deleted_title"),
        tone: "success",
      });
      void onCatalogChanged();
    }, t("capability.custom_mcp_delete_failed"));
  }, [deleteTarget, onCatalogChanged, reportFeedback, runCommand, t]);

  const setEnabled = useCallback(async (
    server: CustomMCPServer,
    enabledValue: boolean,
  ): Promise<boolean> => runCommand(async () => {
    const item = await setCustomMCPServerEnabledApi(
      server.connector_id,
      enabledValue,
    );
    requestIdRef.current += 1;
    setLoading(false);
    setServers((current) => sortServers(current.map((candidate) => (
      candidate.connector_id === item.connector_id ? item : candidate
    ))));
    notifyCapabilitySummaryMutated({
      action: enabledValue ? "enable" : "disable",
      connector_id: item.connector_id,
      source: "custom-mcp",
    });
    reportFeedback({
      message: t(
        enabledValue
          ? "capability.custom_mcp_enabled_message"
          : "capability.custom_mcp_disabled_message",
        { name: item.name },
      ),
      title: t(
        enabledValue
          ? "capability.custom_mcp_enabled_title"
          : "capability.custom_mcp_disabled_title",
      ),
      tone: "success",
    });
    void onCatalogChanged();
  }, t("capability.custom_mcp_toggle_failed")), [
    onCatalogChanged,
    reportFeedback,
    runCommand,
    t,
  ]);

  return {
    busy,
    blocked: blocked || Boolean(failure?.access),
    closeDialog: () => setDialogState(null),
    confirmDelete,
    deleteTarget,
    dialogState,
    loading,
    failure,
    refresh,
    openCreate: () => setDialogState({ mode: "create" }),
    openEdit: (server: CustomMCPServer) => setDialogState({
      mode: "edit",
      server,
    }),
    requestDelete: setDeleteTarget,
    save,
    setEnabled,
    servers,
  };
}

function sortServers(servers: CustomMCPServer[]): CustomMCPServer[] {
  return [...servers].sort((left, right) => left.name.localeCompare(right.name));
}
