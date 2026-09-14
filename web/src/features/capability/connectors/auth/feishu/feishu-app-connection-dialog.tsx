// INPUT: 飞书连接入口状态、扫码/手工分支与提交动作。
// OUTPUT: 两种连接方式或手工凭据字段组成的 plain 弹窗。
// POS: 飞书 Connector 连接方式选择边界，扫码为主、手工配置仅作明确兜底。
"use client";

import {
  ArrowLeft,
  ChevronRight,
} from "lucide-react";
import { type FormEvent, useId } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { UiButton } from "@/shared/ui/button/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { UiListRow } from "@/shared/ui/list/list-row";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { feishuManualCredentialsComplete } from "./feishu-app-connection-model";

interface FeishuAppConnectionDialogProps {
  busy: boolean;
  isOpen: boolean;
  onClose: () => void;
  onConnectManually: (clientId: string, clientSecret: string) => void;
  onScan: () => void;
}

type FeishuAppConnectionView = "choice" | "manual";

export function FeishuAppConnectionDialog({
  busy,
  isOpen,
  onClose,
  onConnectManually,
  onScan,
}: FeishuAppConnectionDialogProps) {
  const { t } = useI18n();
  const fieldId = useId();
  const resetKey = isOpen ? "open" : "closed";
  const [view, setView] = useResettableState<FeishuAppConnectionView>(
    "choice",
    resetKey,
  );
  const [clientId, setClientId] = useResettableState("", resetKey);
  const [clientSecret, setClientSecret] = useResettableState("", resetKey);

  if (!isOpen) {
    return null;
  }
  if (view === "manual") {
    const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (!busy && feishuManualCredentialsComplete(clientId, clientSecret)) {
        onConnectManually(clientId.trim(), clientSecret.trim());
      }
    };
    return (
      <UiDialogPortal>
        <UiDialogBackdrop labelledBy={`${fieldId}-title`} layer="dialog" onClose={onClose}>
          <UiDialogFormShell onSubmit={handleSubmit} size="sm" viewport="compactMax">
            <UiDialogHeader
              appearance="plain"
            titleId={`${fieldId}-title`}
              onClose={onClose}
              title={t("capability.feishu_manual_title")}
            />
            <UiDialogBody className="space-y-4 px-5" scrollable>
              <UiButton
                className="w-fit"
                disabled={busy}
                onClick={() => setView("choice")}
                size="sm"
                type="button"
                variant="text"
              >
                <ArrowLeft className="h-3.5 w-3.5" />
                {t("common.back")}
              </UiButton>
              <p className={getUiTypographyClassName({ role: "supporting", tone: "muted" })}>
                {t("capability.feishu_manual_hint")}
              </p>
              <UiField htmlFor={`${fieldId}-app-id`} label="App ID" required>
                <UiInput
                  autoCapitalize="off"
                  autoCorrect="off"
                  disabled={busy}
                  id={`${fieldId}-app-id`}
                  name="feishu-existing-app-id"
                  onChange={(event) => setClientId(event.target.value)}
                  pattern=".*\S.*"
                  placeholder={t("capability.feishu_app_id_placeholder")}
                  required
                  spellCheck={false}
                  value={clientId}
                  variant="dialog"
                />
              </UiField>
              <UiField
                htmlFor={`${fieldId}-app-secret`}
                label="App Secret"
                required
              >
                <UiInput
                  autoCapitalize="off"
                  autoComplete="off"
                  autoCorrect="off"
                  data-1p-ignore="true"
                  data-form-type="other"
                  data-lpignore="true"
                  disabled={busy}
                  id={`${fieldId}-app-secret`}
                  name="feishu-existing-app-secret"
                  onChange={(event) => setClientSecret(event.target.value)}
                  pattern=".*\S.*"
                  placeholder={t("capability.feishu_app_secret_placeholder")}
                  required
                  spellCheck={false}
                  type="password"
                  value={clientSecret}
                  variant="dialog"
                />
              </UiField>
            </UiDialogBody>
            <UiDialogFooter appearance="plain">
              <UiButton disabled={busy} onClick={onClose} type="button">
                {t("common.cancel")}
              </UiButton>
              <UiButton
                disabled={busy}
                tone="primary"
                type="submit"
                variant="solid"
              >
                {t("capability.feishu_continue")}
              </UiButton>
            </UiDialogFooter>
          </UiDialogFormShell>
        </UiDialogBackdrop>
      </UiDialogPortal>
    );
  }

  return (
    <UiDialogPortal>
      <UiDialogBackdrop labelledBy={`${fieldId}-title`} layer="dialog" onClose={onClose}>
        <UiDialogShell size="sm" viewport="compactMax">
          <UiDialogHeader
            appearance="plain"
            titleId={`${fieldId}-title`}
            onClose={onClose}
            title={t("capability.feishu_connect_title")}
          />
          <UiDialogBody className="px-5" scrollable>
            <div className="radius-control-lg divide-y divide-(--divider-subtle-color) overflow-hidden border border-(--divider-subtle-color)">
              <UiListRow
                disabled={busy}
                description={t("capability.feishu_scan_description")}
                onClick={onScan}
                right={<ChevronRight className="h-4 w-4 text-(--icon-muted)" />}
                title={t("capability.feishu_scan_title")}
              />
              <UiListRow
                disabled={busy}
                description={t("capability.feishu_manual_description")}
                onClick={() => setView("manual")}
                right={<ChevronRight className="h-4 w-4 text-(--icon-muted)" />}
                title={t("capability.feishu_manual_choice")}
              />
            </div>
          </UiDialogBody>
          <UiDialogFooter appearance="plain">
            <UiButton disabled={busy} onClick={onClose} type="button">
              {t("common.cancel")}
            </UiButton>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
