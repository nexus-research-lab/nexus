"use client";

import {
  ArrowLeft,
  ChevronRight,
  KeyRound,
  ScanLine,
} from "lucide-react";
import { type FormEvent } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
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
import { UiPanel } from "@/shared/ui/panel";

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
        <UiDialogBackdrop className="z-[9999]" onClose={onClose}>
          <UiDialogFormShell onSubmit={handleSubmit} size="sm">
            <UiDialogHeader
              icon={<KeyRound className="h-4 w-4" />}
              onClose={onClose}
              subtitle="仅在官方扫码流程不可用时手工提供应用凭据。"
              title="手动配置飞书应用"
            />
            <UiDialogBody className="space-y-4">
              <UiButton
                className="w-fit"
                disabled={busy}
                onClick={() => setView("choice")}
                size="sm"
                type="button"
                variant="text"
              >
                <ArrowLeft className="h-3.5 w-3.5" />
                返回选择连接方式
              </UiButton>
              <UiPanel
                className="text-compact leading-6 text-(--text-muted)"
                padding="sm"
                variant="inset"
              >
                这是兜底方式。提交 App ID 和 App Secret 后，Nexus 会直接显示当前用户的飞书授权链接；该阶段不再把链接生成为二维码。
              </UiPanel>
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
            <UiDialogFooter>
              <UiButton disabled={busy} onClick={onClose} type="button">
                取消
              </UiButton>
              <UiButton
                disabled={busy}
                tone="primary"
                type="submit"
                variant="solid"
              >
                保存并继续授权
              </UiButton>
            </UiDialogFooter>
          </UiDialogFormShell>
        </UiDialogBackdrop>
      </UiDialogPortal>
    );
  }

  return (
    <UiDialogPortal>
      <UiDialogBackdrop className="z-[9999]" onClose={onClose}>
        <UiDialogShell size="sm">
          <UiDialogHeader
            icon={<ScanLine className="h-4 w-4" />}
            onClose={onClose}
            subtitle="优先使用飞书官方扫码；历史 App ID 不会被自动复用。"
            title="连接飞书云文档"
          />
          <UiDialogBody>
            <UiPanel className="divide-y divide-(--divider-subtle-color)" padding="none" variant="inset">
              <UiListRow
                aria-disabled={busy}
                className={busy ? "opacity-(--disabled-opacity)" : ""}
                description="在飞书官方页面选择已有应用或创建新应用，并自动补齐云文档权限。"
                leading={(
                  <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-(--divider-subtle-color)">
                    <ScanLine className="h-4 w-4 text-(--icon-default)" />
                  </span>
                )}
                onClick={busy ? undefined : onScan}
                right={<ChevronRight className="h-4 w-4 text-(--icon-muted)" />}
                title="扫码连接"
              />
              <UiListRow
                aria-disabled={busy}
                className={busy ? "opacity-(--disabled-opacity)" : ""}
                description="扫码不可用时，手动填写 App ID 和 App Secret 继续连接。"
                leading={(
                  <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-(--divider-subtle-color)">
                    <KeyRound className="h-4 w-4 text-(--icon-default)" />
                  </span>
                )}
                onClick={busy ? undefined : () => setView("manual")}
                right={<ChevronRight className="h-4 w-4 text-(--icon-muted)" />}
                title="手动配置（兜底）"
              />
            </UiPanel>
          </UiDialogBody>
          <UiDialogFooter>
            <UiButton disabled={busy} onClick={onClose} type="button">
              取消
            </UiButton>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
