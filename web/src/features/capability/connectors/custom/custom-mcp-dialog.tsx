/**
 * INPUT: 自定义 MCP 服务、脱敏秘密草稿与保存命令。
 * OUTPUT: 随内容增长的 plain MCP 表单及可增删参数/秘密行。
 * POS: 自定义 Connector 的创建编辑边界；标题只命名动作。
 */
"use client";

import { Plus, Trash2 } from "lucide-react";
import { useRef, useState, type FormEvent } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { UiSegmentedControl } from "@/shared/ui/form/segmented-control";
import type {
  CustomMCPAuthType,
  CustomMCPServer,
  CustomMCPServerInput,
  CustomMCPServerType,
} from "@/types/capability/connector";

import {
  buildCustomMCPServerInput,
  createCustomMCPDraft,
  type CustomMCPDraft,
  type CustomMCPDraftError,
  type CustomMCPSecretDraft,
  validateCustomMCPDraft,
} from "./custom-mcp-model";

interface CustomMCPDialogProps {
  busy: boolean;
  onClose: () => void;
  onSave: (input: CustomMCPServerInput) => Promise<boolean>;
  server?: CustomMCPServer;
}

const TRANSPORTS: CustomMCPServerType[] = ["stdio", "http", "sse"];
const AUTH_TYPES: CustomMCPAuthType[] = ["none", "bearer", "headers"];

export function CustomMCPDialog({
  busy,
  onClose,
  onSave,
  server,
}: CustomMCPDialogProps) {
  const { t } = useI18n();
  const nameInputRef = useRef<HTMLInputElement | null>(null);
  const [draft, setDraft] = useState(() => createCustomMCPDraft(server));
  const [validationError, setValidationError] =
    useState<CustomMCPDraftError | null>(null);
  const recoveryRequired = server?.configuration_state === "recovery_required";

  const updateDraft = <Key extends keyof CustomMCPDraft>(
    key: Key,
    value: CustomMCPDraft[Key],
  ) => {
    setValidationError(null);
    setDraft((current) => ({ ...current, [key]: value }));
  };
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const error = validateCustomMCPDraft(draft);
    setValidationError(error);
    if (error) return;
    await onSave(buildCustomMCPServerInput(draft));
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        closeOnBackdrop={!busy}
        initialFocusRef={nameInputRef}
        inset="compact"
        labelledBy="custom-mcp-dialog-title"
        onClose={busy ? undefined : onClose}
      >
        <UiDialogFormShell
          onSubmit={(event) => void submit(event)}
          size="lg"
          viewport="adaptiveMax"
        >
          <UiDialogHeader
            appearance="plain"
            onClose={busy ? undefined : onClose}
            title={server
              ? t(recoveryRequired
                ? "capability.custom_mcp_recovery_title"
                : "capability.custom_mcp_edit_title")
              : t("capability.custom_mcp_add_title")}
            titleId="custom-mcp-dialog-title"
          />
          <UiDialogBody className="space-y-5" scrollable>
            {recoveryRequired ? (
              <UiInlineNotice
                message={t("capability.custom_mcp_recovery_form_description")}
                tone="warning"
              />
            ) : null}
            {validationError ? (
              <UiInlineNotice
                message={t(`capability.custom_mcp_error_${validationError}`)}
                role="alert"
                tone="danger"
              />
            ) : null}

            <UiField
              htmlFor="custom-mcp-name"
              label={t("capability.custom_mcp_name")}
              required
            >
              <UiInput
                ref={nameInputRef}
                autoComplete="off"
                id="custom-mcp-name"
                onChange={(event) => updateDraft("name", event.target.value)}
                placeholder="my_mcp_server"
                required
                value={draft.name}
              />
            </UiField>

            <UiField label={t("capability.custom_mcp_transport")}>
              <UiSegmentedControl
                density="compact"
                onChange={(value) => updateDraft("type", value)}
                options={TRANSPORTS.map((value) => ({
                  label: value.toUpperCase(),
                  value,
                }))}
                title={t("capability.custom_mcp_transport")}
                value={draft.type}
              />
            </UiField>

            {draft.type === "stdio" ? (
              <StdioFields draft={draft} updateDraft={updateDraft} />
            ) : (
              <RemoteFields draft={draft} updateDraft={updateDraft} />
            )}
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
              {busy ? t("common.saving") : t("common.save")}
            </UiButton>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

function StdioFields({
  draft,
  updateDraft,
}: {
  draft: CustomMCPDraft;
  updateDraft: <Key extends keyof CustomMCPDraft>(
    key: Key,
    value: CustomMCPDraft[Key],
  ) => void;
}) {
  const { t } = useI18n();
  return (
    <>
      <UiField
        htmlFor="custom-mcp-command"
        label={t("capability.custom_mcp_command")}
        required
      >
        <UiInput
          autoComplete="off"
          id="custom-mcp-command"
          onChange={(event) => updateDraft("command", event.target.value)}
          placeholder="npx"
          required
          value={draft.command}
        />
      </UiField>
      <StringListEditor
        addLabel={t("capability.custom_mcp_add_argument")}
        label={t("capability.custom_mcp_arguments")}
        onChange={(value) => updateDraft("args", value)}
        placeholder="-y"
        values={draft.args}
      />
      <SecretListEditor
        addLabel={t("capability.custom_mcp_add_environment")}
        label={t("capability.custom_mcp_environment")}
        onChange={(value) => updateDraft("env", value)}
        rows={draft.env}
      />
    </>
  );
}

function RemoteFields({
  draft,
  updateDraft,
}: {
  draft: CustomMCPDraft;
  updateDraft: <Key extends keyof CustomMCPDraft>(
    key: Key,
    value: CustomMCPDraft[Key],
  ) => void;
}) {
  const { t } = useI18n();
  return (
    <>
      <UiField
        htmlFor="custom-mcp-url"
        label={t("capability.custom_mcp_url")}
        required
      >
        <UiInput
          autoComplete="off"
          id="custom-mcp-url"
          onChange={(event) => updateDraft("url", event.target.value)}
          placeholder="https://example.com/mcp"
          required
          type="url"
          value={draft.url}
        />
      </UiField>
      <UiField label={t("capability.custom_mcp_auth")}>
        <UiSegmentedControl
          density="compact"
          onChange={(value) => updateDraft("authType", value)}
          options={AUTH_TYPES.map((value) => ({
            label: t(`capability.custom_mcp_auth_${value}`),
            value,
          }))}
          title={t("capability.custom_mcp_auth")}
          value={draft.authType}
        />
      </UiField>
      {draft.authType === "bearer" ? (
        <UiField
          description={t("capability.custom_mcp_bearer_hint")}
          htmlFor="custom-mcp-bearer-token"
          label={t("capability.custom_mcp_bearer_token")}
          required={!draft.bearerTokenConfigured}
        >
          <UiInput
            autoComplete="off"
            id="custom-mcp-bearer-token"
            onChange={(event) => {
              updateDraft("bearerToken", event.target.value);
              updateDraft("bearerTokenConfigured", false);
            }}
            placeholder={draft.bearerTokenConfigured
              ? t("capability.custom_mcp_secret_saved")
              : t("capability.custom_mcp_bearer_placeholder")}
            required={!draft.bearerTokenConfigured}
            type="password"
            value={draft.bearerToken}
          />
        </UiField>
      ) : null}
      {draft.authType === "headers" ? (
        <SecretListEditor
          addLabel={t("capability.custom_mcp_add_header")}
          label={t("capability.custom_mcp_headers")}
          onChange={(value) => updateDraft("headers", value)}
          rows={draft.headers}
        />
      ) : null}
    </>
  );
}

function StringListEditor({
  addLabel,
  label,
  onChange,
  placeholder,
  values,
}: {
  addLabel: string;
  label: string;
  onChange: (values: string[]) => void;
  placeholder: string;
  values: string[];
}) {
  return (
    <UiField label={label}>
      <div className="space-y-2">
        {values.map((value, index) => (
          <div className="flex items-center gap-2" key={index}>
            <UiInput
              aria-label={`${label} ${index + 1}`}
              onChange={(event) => onChange(values.map((item, itemIndex) => (
                itemIndex === index ? event.target.value : item
              )))}
              placeholder={placeholder}
              required
              value={value}
            />
            <RemoveRowButton onClick={() => onChange(
              values.filter((_, itemIndex) => itemIndex !== index)
            )} />
          </div>
        ))}
        <AddRowButton label={addLabel} onClick={() => onChange([...values, ""])} />
      </div>
    </UiField>
  );
}

function SecretListEditor({
  addLabel,
  label,
  onChange,
  rows,
}: {
  addLabel: string;
  label: string;
  onChange: (rows: CustomMCPSecretDraft[]) => void;
  rows: CustomMCPSecretDraft[];
}) {
  const { t } = useI18n();
  const updateRow = (
    index: number,
    patch: Partial<CustomMCPSecretDraft>,
  ) => onChange(rows.map((row, rowIndex) => (
    rowIndex === index ? { ...row, ...patch } : row
  )));

  return (
    <UiField label={label}>
      <div className="space-y-2">
        {rows.map((row, index) => (
          <div
            className="grid grid-cols-1 items-center gap-2 sm:grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)_auto]"
            key={index}
          >
            <UiInput
              aria-label={`${label} ${t("capability.custom_mcp_key")}`}
              onChange={(event) => updateRow(index, { key: event.target.value })}
              placeholder={t("capability.custom_mcp_key")}
              required
              value={row.key}
            />
            <UiInput
              aria-label={`${label} ${t("capability.custom_mcp_value")}`}
              onChange={(event) => updateRow(index, {
                configured: false,
                value: event.target.value,
              })}
              placeholder={row.configured
                ? t("capability.custom_mcp_secret_saved")
                : t("capability.custom_mcp_value")}
              required={!row.configured}
              type="password"
              value={row.value}
            />
            <RemoveRowButton onClick={() => onChange(
              rows.filter((_, rowIndex) => rowIndex !== index)
            )} />
          </div>
        ))}
        <AddRowButton
          label={addLabel}
          onClick={() => onChange([
            ...rows,
            { configured: false, key: "", value: "" },
          ])}
        />
      </div>
    </UiField>
  );
}

function AddRowButton({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <UiButton className="w-fit" onClick={onClick} size="sm" type="button" variant="text">
      <Plus className="h-3.5 w-3.5" />
      {label}
    </UiButton>
  );
}

function RemoveRowButton({ onClick }: { onClick: () => void }) {
  const { t } = useI18n();
  return (
    <UiIconButton
      aria-label={t("common.delete")}
      className="justify-self-end"
      onClick={onClick}
      size="md"
      title={t("common.delete")}
      tone="danger"
      type="button"
      variant="ghost"
    >
      <Trash2 className="h-4 w-4" />
    </UiIconButton>
  );
}
