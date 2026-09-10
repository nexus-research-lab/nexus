/**
 * INPUT: 自定义 MCP 服务、脱敏秘密草稿与保存命令。
 * OUTPUT: 具有实例级字段关联、稳定动态行身份与明确行操作名称的脱敏配置表单。
 * POS: 自定义 Connector 的创建编辑边界；标题只命名动作。
 */
"use client";

import { Plus, Trash2 } from "lucide-react";
import { useId, useRef, useState, type FormEvent } from "react";

import { generateUuid } from "@/lib/uuid";
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
  createCustomMCPArgumentDraft,
  createCustomMCPDraft,
  createCustomMCPSecretDraft,
  type CustomMCPArgumentDraft,
  type CustomMCPDraft,
  type CustomMCPDraftError,
  type CustomMCPSecretDraft,
  validateCustomMCPDraft,
} from "./custom-mcp-model";

interface CustomMCPDialogProps {
  busy: boolean;
  blocked?: boolean;
  onClose: () => void;
  onSave: (input: CustomMCPServerInput) => Promise<boolean>;
  server?: CustomMCPServer;
}

const TRANSPORTS: CustomMCPServerType[] = ["stdio", "http", "sse"];
const AUTH_TYPES: CustomMCPAuthType[] = ["none", "bearer", "headers"];

export function CustomMCPDialog({
  busy,
  blocked = false,
  onClose,
  onSave,
  server,
}: CustomMCPDialogProps) {
  const { t } = useI18n();
  const dialogId = useId();
  const nameInputRef = useRef<HTMLInputElement | null>(null);
  const [draft, setDraft] = useState(() => createCustomMCPDraft(server, dialogId));
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
    if (busy || blocked) return;
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
        labelledBy={`${dialogId}-title`}
        onClose={busy ? undefined : onClose}
      >
        <UiDialogFormShell
          aria-busy={busy}
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
            titleId={`${dialogId}-title`}
          />
          <UiDialogBody scrollable>
            <fieldset className="space-y-5" disabled={busy || blocked}>
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
              htmlFor={`${dialogId}-name`}
              label={t("capability.custom_mcp_name")}
              required
            >
              <UiInput
                ref={nameInputRef}
                autoComplete="off"
                id={`${dialogId}-name`}
                onChange={(event) => updateDraft("name", event.target.value)}
                placeholder="my_mcp_server"
                required
                value={draft.name}
              />
            </UiField>

            <UiSegmentedControl
              density="compact"
              onChange={(value) => updateDraft("type", value)}
              options={TRANSPORTS.map((value) => ({
                label: value.toUpperCase(),
                value,
              }))}
              title={t("capability.custom_mcp_transport")}
              value={draft.type}
              showLabel
            />

            {draft.type === "stdio" ? (
              <StdioFields draft={draft} updateDraft={updateDraft} />
            ) : (
              <RemoteFields draft={draft} updateDraft={updateDraft} />
            )}
            </fieldset>
          </UiDialogBody>
          <UiDialogFooter appearance="plain">
            <UiButton disabled={busy} onClick={onClose} type="button">
              {t("common.cancel")}
            </UiButton>
            <UiButton
              disabled={busy || blocked}
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
  const fieldId = useId();
  return (
    <>
      <UiField
        htmlFor={`${fieldId}-command`}
        label={t("capability.custom_mcp_command")}
        required
      >
        <UiInput
          autoComplete="off"
          id={`${fieldId}-command`}
          onChange={(event) => updateDraft("command", event.target.value)}
          placeholder="npx"
          required
          textRole="code"
          value={draft.command}
        />
      </UiField>
      <ArgumentListEditor
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
  const fieldId = useId();
  return (
    <>
      <UiField
        htmlFor={`${fieldId}-url`}
        label={t("capability.custom_mcp_url")}
        required
      >
        <UiInput
          autoComplete="off"
          id={`${fieldId}-url`}
          onChange={(event) => updateDraft("url", event.target.value)}
          placeholder="https://example.com/mcp"
          required
          type="url"
          value={draft.url}
        />
      </UiField>
      <UiSegmentedControl
        density="compact"
        onChange={(value) => updateDraft("authType", value)}
        options={AUTH_TYPES.map((value) => ({
          label: t(`capability.custom_mcp_auth_${value}`),
          value,
        }))}
        title={t("capability.custom_mcp_auth")}
        value={draft.authType}
        showLabel
      />
      {draft.authType === "bearer" ? (
        <UiField
          description={t("capability.custom_mcp_bearer_hint")}
          htmlFor={`${fieldId}-bearer-token`}
          label={t("capability.custom_mcp_bearer_token")}
          required={!draft.bearerTokenConfigured}
        >
          <UiInput
            autoComplete="off"
            id={`${fieldId}-bearer-token`}
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

function ArgumentListEditor({
  addLabel,
  label,
  onChange,
  placeholder,
  values,
}: {
  addLabel: string;
  label: string;
  onChange: (values: CustomMCPArgumentDraft[]) => void;
  placeholder: string;
  values: CustomMCPArgumentDraft[];
}) {
  const { t } = useI18n();
  return (
    <UiField label={label}>
      <div className="space-y-2">
        {values.map((row, index) => {
          const rowLabel = t("capability.custom_mcp_row_label", {
            group: label, index: index + 1,
          });
          return (
            <div className="flex items-center gap-2" key={row.id}>
              <UiInput
                aria-label={rowLabel}
                onChange={(event) => onChange(values.map((item) => (
                  item.id === row.id ? { ...item, value: event.target.value } : item
                )))}
                placeholder={placeholder}
                required
                textRole="code"
                value={row.value}
              />
              <RemoveRowButton
                label={t("capability.custom_mcp_remove_row", { row: rowLabel })}
                onClick={() => onChange(values.filter((item) => item.id !== row.id))}
              />
            </div>
          );
        })}
        <AddRowButton label={addLabel} onClick={() => onChange([...values, createCustomMCPArgumentDraft(generateUuid())])} />
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
    id: string,
    patch: Partial<Omit<CustomMCPSecretDraft, "id">>,
  ) => onChange(rows.map((row) => row.id === id ? { ...row, ...patch } : row));
  return (
    <UiField label={label}>
      <div className="space-y-2">
        {rows.map((row, index) => {
          const rowLabel = t("capability.custom_mcp_row_label", {
            group: label, index: index + 1,
          });
          return (
            <div className="grid grid-cols-1 items-center gap-2 sm:grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)_auto]" key={row.id}>
              <UiInput
                aria-label={`${rowLabel} ${t("capability.custom_mcp_key")}`}
                onChange={(event) => updateRow(row.id, { key: event.target.value })}
                placeholder={t("capability.custom_mcp_key")}
                required
                textRole="code"
                value={row.key}
              />
              <UiInput
                aria-label={`${rowLabel} ${t("capability.custom_mcp_value")}`}
                onChange={(event) => updateRow(row.id, { configured: false, value: event.target.value })}
                placeholder={row.configured ? t("capability.custom_mcp_secret_saved") : t("capability.custom_mcp_value")}
                required={!row.configured}
                type="password"
                value={row.value}
              />
              <RemoveRowButton
                label={t("capability.custom_mcp_remove_row", { row: rowLabel })}
                onClick={() => onChange(rows.filter((item) => item.id !== row.id))}
              />
            </div>
          );
        })}
        <AddRowButton label={addLabel} onClick={() => onChange([...rows, createCustomMCPSecretDraft(generateUuid())])} />
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

function RemoveRowButton({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <UiIconButton
      aria-label={label}
      className="justify-self-end"
      onClick={onClick}
      size="md"
      title={label}
      tone="danger"
      type="button"
      variant="ghost"
    >
      <Trash2 className="h-4 w-4" />
    </UiIconButton>
  );
}
