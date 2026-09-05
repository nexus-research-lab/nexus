// INPUT: 飞书连接入口状态、扫码/手工分支与提交动作。
// OUTPUT: 两种连接方式或手工凭据字段组成的 plain 弹窗。
// POS: 飞书 Connector 连接方式选择边界，扫码为主、手工配置仅作明确兜底。
"use client";

import {
  ArrowLeft,
  ChevronRight,
} from "lucide-react";
import { type FormEvent } from "react";

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
        <UiDialogBackdrop layer="dialog" onClose={onClose}>
          <UiDialogFormShell onSubmit={handleSubmit} size="sm">
            <UiDialogHeader
              appearance="plain"
              onClose={onClose}
              title="手动连接飞书"
            />
            <UiDialogBody className="space-y-4 px-5">
              <UiButton
                className="w-fit"
                disabled={busy}
                onClick={() => setView("choice")}
                size="sm"
                type="button"
                variant="text"
              >
                <ArrowLeft className="h-3.5 w-3.5" />
                返回
              </UiButton>
              <p className={getUiTypographyClassName({ role: "supporting", tone: "muted" })}>
                仅在扫码不可用时填写应用凭据。
              </p>
              <UiField htmlFor="feishu-existing-app-id" label="App ID" required>
                <UiInput
                  autoCapitalize="off"
                  autoCorrect="off"
                  disabled={busy}
                  id="feishu-existing-app-id"
                  name="feishu-existing-app-id"
                  onChange={(event) => setClientId(event.target.value)}
                  pattern=".*\S.*"
                  placeholder="飞书开放平台中的 App ID"
                  required
                  spellCheck={false}
                  value={clientId}
                  variant="dialog"
                />
              </UiField>
              <UiField
                htmlFor="feishu-existing-app-secret"
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
                  id="feishu-existing-app-secret"
                  name="feishu-existing-app-secret"
                  onChange={(event) => setClientSecret(event.target.value)}
                  pattern=".*\S.*"
                  placeholder="飞书开放平台中的 App Secret"
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
                取消
              </UiButton>
              <UiButton
                disabled={busy}
                tone="primary"
                type="submit"
                variant="solid"
              >
                继续
              </UiButton>
            </UiDialogFooter>
          </UiDialogFormShell>
        </UiDialogBackdrop>
      </UiDialogPortal>
    );
  }

  return (
    <UiDialogPortal>
      <UiDialogBackdrop layer="dialog" onClose={onClose}>
        <UiDialogShell size="sm">
          <UiDialogHeader
            appearance="plain"
            onClose={onClose}
            title="连接飞书云文档"
          />
          <UiDialogBody className="px-5">
            <div className="radius-control-lg divide-y divide-(--divider-subtle-color) overflow-hidden border border-(--divider-subtle-color)">
              <UiListRow
                aria-disabled={busy}
                className={busy ? "opacity-(--disabled-opacity)" : ""}
                description="在飞书页面选择或创建应用。"
                onClick={busy ? undefined : onScan}
                right={<ChevronRight className="h-4 w-4 text-(--icon-muted)" />}
                title="扫码连接"
              />
              <UiListRow
                aria-disabled={busy}
                className={busy ? "opacity-(--disabled-opacity)" : ""}
                description="填写 App ID 和 App Secret。"
                onClick={busy ? undefined : () => setView("manual")}
                right={<ChevronRight className="h-4 w-4 text-(--icon-muted)" />}
                title="手动配置"
              />
            </div>
          </UiDialogBody>
          <UiDialogFooter appearance="plain">
            <UiButton disabled={busy} onClick={onClose} type="button">
              取消
            </UiButton>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
