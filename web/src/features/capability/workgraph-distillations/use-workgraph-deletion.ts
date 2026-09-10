// INPUT: Explicit owner workflow deletion and fresh directory snapshots.
// OUTPUT: Independent pending/unconfirmed locks and read-only reconciliation.
// POS: Named WorkGraph deletion recovery; GET never replays deletion.
import { useCallback, useRef, useState } from "react";
import { deleteWorkGraphWorkflowApi } from "@/lib/api/conversation/execution-api";
import { projectMutationFailure, type MutationFailureEffect } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner-contract";
import type { WorkGraphWorkflow } from "@/types/conversation/workgraph-workflow";

export function useWorkGraphDeletion({ onDeleted, onRefresh, reportFeedback }: {
  onDeleted: (id: string) => void;
  onRefresh: () => void;
  reportFeedback: (feedback: FeedbackBannerProps | null) => void;
}) {
  const { t } = useI18n();
  const pendingRef = useRef(false);
  const recoveryRef = useRef<{ id: string; effect: MutationFailureEffect } | null>(null);
  const [busy, setBusy] = useState(false);
  const [blocked, setBlocked] = useState(false);
  const [feedback, setFeedback] = useState<FeedbackBannerProps | null>(null);
  const reconcile = useCallback((items: readonly WorkGraphWorkflow[]) => {
    const recovery = recoveryRef.current;
    if (!recovery) return;
    if (!items.some((item) => item.id === recovery.id)) {
      recoveryRef.current = null;
      setBlocked(false);
      setFeedback(null);
      reportFeedback(null);
      return;
    }
    const unknown = recovery.effect !== "accepted" && recovery.effect !== "committed";
    setFeedback({
      title: t("capability.workgraph_delete_unconfirmed"),
      impact: t("capability.workgraph_delete_unconfirmed_impact"),
      action: {
        label: t(unknown ? "capability.workgraph_delete_new_action" : "state.reload_check"),
        onClick: unknown ? () => {
          if (recoveryRef.current !== recovery) return;
          recoveryRef.current = null;
          setBlocked(false);
          setFeedback(null);
          reportFeedback(null);
        } : onRefresh,
      },
      tone: "warning",
    });
  }, [onRefresh, reportFeedback, t]);
  const remove = async (item: WorkGraphWorkflow) => {
    if (item.built_in || pendingRef.current || recoveryRef.current) return;
    pendingRef.current = true;
    setBusy(true);
    try {
      await deleteWorkGraphWorkflowApi(item.id);
    } catch (reason) {
      const failure = projectMutationFailure(reason, t("capability.workgraph_delete_failed"));
      const notApplied = failure.effect === "not_applied";
      if (!notApplied) {
        recoveryRef.current = { id: item.id, effect: failure.effect };
        setBlocked(true);
      }
      const nextFeedback: FeedbackBannerProps = {
        title: t(notApplied ? "capability.workgraph_delete_failed" : "capability.workgraph_delete_unconfirmed"),
        impact: t(notApplied ? "capability.workgraph_delete_failure_impact" : "capability.workgraph_delete_unconfirmed_impact"),
        action: notApplied ? undefined : { label: t("state.reload_check"), onClick: onRefresh },
        onDismiss: notApplied ? () => reportFeedback(null) : undefined,
        tone: notApplied ? "error" : "warning",
      };
      if (!notApplied) setFeedback(nextFeedback);
      reportFeedback(nextFeedback);
      return;
    } finally {
      pendingRef.current = false;
      setBusy(false);
    }
    onDeleted(item.id);
    reportFeedback(null);
  };
  return { busy, blocked, feedback, reconcile, remove };
}
