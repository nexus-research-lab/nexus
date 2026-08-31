"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { notifyCapabilitySummaryMutated } from "@/features/capability/capability-summary-events";
import {
  createCustomMCPServerApi,
  deleteCustomMCPServerApi,
  getCustomMCPServersApi,
  updateCustomMCPServerApi,
} from "@/lib/api/capability/connector-api";
import { getErrorMessage, projectMutationFailure } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  CustomMCPServer,
  CustomMCPServerInput,
} from "@/types/capability/connector";

import type { ReportConnectorFeedback } from "../controller/connector-controller-types";

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
  const [servers, setServers] = useState<CustomMCPServer[]>([]);
  const [loading, setLoading] = useState(false);
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
      }
      return requestId === requestIdRef.current;
    } catch (error) {
      if (requestId === requestIdRef.current) {
        if (onFailure) {
          onFailure();
        } else {
          reportFeedback({
            action: {
              label: t("state.retry"),
              onClick: () => window.location.reload(),
            },
            impact: t("state.read_failure_impact"),
            message: getErrorMessage(error, t("capability.custom_mcp_load_failed")),
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
  }, [enabled, refresh]);

  const runCommand = useCallback(async (
    command: () => Promise<void>,
    fallbackMessage: string,
  ): Promise<boolean> => {
    if (commandRef.current) return false;
    commandRef.current = true;
    setBusy(true);
    try {
      await command();
      return true;
    } catch (error) {
      const failure = projectMutationFailure(error, fallbackMessage);
      const notApplied = failure.effect === "not_applied";
      function reconcile() {
        void refresh(() => reportFailure(true));
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

  const confirmDelete = useCallback(async (): Promise<void> => {
    const target = deleteTarget;
    setDeleteTarget(null);
    if (!target) return;
    await runCommand(async () => {
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
          name: target.name,
        }),
        title: t("capability.custom_mcp_deleted_title"),
        tone: "success",
      });
      void onCatalogChanged();
    }, t("capability.custom_mcp_delete_failed"));
  }, [deleteTarget, onCatalogChanged, reportFeedback, runCommand, t]);

  return {
    busy,
    closeDialog: () => setDialogState(null),
    confirmDelete,
    deleteTarget,
    dialogState,
    loading,
    openCreate: () => setDialogState({ mode: "create" }),
    openEdit: (server: CustomMCPServer) => setDialogState({
      mode: "edit",
      server,
    }),
    requestDelete: setDeleteTarget,
    save,
    servers,
  };
}

function sortServers(servers: CustomMCPServer[]): CustomMCPServer[] {
  return [...servers].sort((left, right) => left.name.localeCompare(right.name));
}
