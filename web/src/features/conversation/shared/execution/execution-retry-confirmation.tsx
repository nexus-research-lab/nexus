/**
 * INPUT: Revision-bound retry impact and shared conversation action owner.
 * OUTPUT: Explicit confirmation of submitted work and downstream invalidation.
 * POS: WorkGraph retry confirmation uses the shared decision dialog.
 */
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { ExecutionRecoveryAction } from "./use-execution-recovery";

export function ExecutionRetryConfirmation({ action }: { action: ExecutionRecoveryAction }) {
  const { t } = useI18n();
  const preview = action.confirmation;
  if (!preview) return null;
  const names = preview.affected.map((item) => `${item.subject || item.work_item_id} (${t(item.running ? "execution.retry_impact_running" : item.submitted ? "execution.retry_impact_submitted" : "execution.retry_impact_waiting")})`).join(" · ");
  return <ConfirmDialog
    isOpen
    title={t("execution.retry_confirm_title")}
    message={`${t("execution.retry_confirm_hint")}\n${names}`}
    confirmText={t("execution.retry_confirm_action")}
    onConfirm={() => action.confirmRetry?.()}
    onCancel={() => action.cancelRetry?.()}
  />;
}
