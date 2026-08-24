/**
 * INPUT: Skill 来源目录、来源写命令与私有来源草稿。
 * OUTPUT: 扁平来源管理列表、私有来源表单与受控删除确认。
 * POS: 技能市场的来源管理边界；不展示来源教程或回显私密 Token。
 */
"use client";

import { Loader2, Pencil, Plus, Trash2 } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";

import type { PrivateSkillSourceDraft } from "@/features/capability/skills/controller/skill-marketplace-controller";
import {
  useI18n,
  type I18nContextValue,
} from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import type { ExternalSkillSourceInfo } from "@/types/capability/skill";

interface SkillSourceManagerDialogProps {
  isOpen: boolean;
  loading: boolean;
  onClose: () => void;
  onDelete: (source: ExternalSkillSourceInfo) => void;
  onSave: (
    source: ExternalSkillSourceInfo | null,
    draft: PrivateSkillSourceDraft,
  ) => Promise<boolean>;
  onToggle: (source: ExternalSkillSourceInfo, enabled: boolean) => void;
  sources: ExternalSkillSourceInfo[];
}

const SOURCE_KIND_LABELS: Record<string, string> = {
  browse_sh: "browse.sh",
  claude_plugins: "claude-plugins.dev",
  clawhub: "clawhub.ai",
  git: "Git",
  hermes_index: "Hermes Index",
  private_registry: "Private Registry",
  skills_sh: "skills.sh",
  url: "URL",
  well_known: "Well-known",
};

const EMPTY_PRIVATE_SOURCE_DRAFT: PrivateSkillSourceDraft = {
  authType: "none",
  name: "",
  token: "",
  url: "",
};

function sourceKindLabel(kind: string, t: I18nContextValue["t"]): string {
  if (kind === "private_registry") {
    return t("capability.skill_source_private");
  }
  return SOURCE_KIND_LABELS[kind] || kind;
}

export function SkillSourceManagerDialog({
  isOpen,
  loading,
  onClose,
  onDelete,
  onSave,
  onToggle,
  sources,
}: SkillSourceManagerDialogProps) {
  const { t } = useI18n();
  const [editingSource, setEditingSource] = useState<ExternalSkillSourceInfo | null>(null);
  const [draft, setDraft] = useState<PrivateSkillSourceDraft>(EMPTY_PRIVATE_SOURCE_DRAFT);
  const [deleteTarget, setDeleteTarget] = useState<ExternalSkillSourceInfo | null>(null);
  const [editorOpen, setEditorOpen] = useState(false);

  useEffect(() => {
    if (isOpen) return;
    setEditingSource(null);
    setDraft(EMPTY_PRIVATE_SOURCE_DRAFT);
    setDeleteTarget(null);
    setEditorOpen(false);
  }, [isOpen]);

  if (!isOpen) return null;

  const sortedSources = [...sources].sort(
    (a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name),
  );
  const closeEditor = () => {
    setEditingSource(null);
    setDraft(EMPTY_PRIVATE_SOURCE_DRAFT);
    setEditorOpen(false);
  };
  const openCreateEditor = () => {
    setEditingSource(null);
    setDraft(EMPTY_PRIVATE_SOURCE_DRAFT);
    setEditorOpen(true);
  };
  const openEditEditor = (source: ExternalSkillSourceInfo) => {
    setEditingSource(source);
    setDraft({
      authType: source.auth_type === "bearer" ? "bearer" : "none",
      name: source.name,
      token: "",
      url: source.url,
    });
    setEditorOpen(true);
  };
  const submitForm = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!draft.name.trim() || !draft.url.trim()) return;
    if (
      draft.authType === "bearer"
      && !draft.token.trim()
      && !editingSource?.credential_configured
    ) return;
    if (await onSave(editingSource, draft)) closeEditor();
  };

  if (editorOpen) {
    return (
      <PrivateSourceEditorDialog
        draft={draft}
        editingSource={editingSource}
        loading={loading}
        onCancel={closeEditor}
        onChange={setDraft}
        onSubmit={submitForm}
      />
    );
  }

  return (
    <>
      <UiDialogPortal>
        <UiDialogBackdrop className="z-[9999]" onClose={onClose}>
          <UiDialogShell className="max-h-[min(68dvh,560px)]" size="lg">
            <UiDialogHeader
              appearance="plain"
              onClose={onClose}
              title={t("capability.skill_sources_title")}
            />
            <UiDialogBody scrollable>
            {loading && !sortedSources.length ? (
              <div className="flex items-center justify-center gap-2 py-12 text-sm text-(--text-soft)">
                <Loader2 className="h-4 w-4 animate-spin" />
                {t("capability.skill_sources_loading")}
              </div>
            ) : sortedSources.length ? (
              <div className="divide-y divide-(--divider-subtle-color) overflow-hidden rounded-[10px] border border-(--divider-subtle-color)">
                {sortedSources.map((source) => (
                  <SourceRow
                    key={source.source_id}
                    disabled={loading}
                    onDelete={() => setDeleteTarget(source)}
                    onEdit={() => openEditEditor(source)}
                    onToggle={(enabled) => onToggle(source, enabled)}
                    source={source}
                  />
                ))}
              </div>
            ) : (
              <div className="rounded-[8px] border border-dashed border-(--divider-subtle-color) px-4 py-6 text-center text-compact text-(--text-soft)">
                {t("capability.skill_sources_empty")}
              </div>
            )}
            </UiDialogBody>

            <UiDialogFooter appearance="plain" className="gap-2">
              <UiButton
                className="mr-auto"
                disabled={loading}
                onClick={openCreateEditor}
                size="sm"
                tone="primary"
                variant="solid"
              >
                <Plus className="h-3.5 w-3.5" />
                {t("capability.skill_source_add")}
              </UiButton>
              <UiButton
                disabled={loading}
                onClick={onClose}
                size="sm"
                variant="surface"
              >
                {t("common.close")}
              </UiButton>
            </UiDialogFooter>
          </UiDialogShell>
        </UiDialogBackdrop>
      </UiDialogPortal>
      <ConfirmDialog
        confirmText={t("common.delete")}
        isOpen={deleteTarget !== null}
        message={deleteTarget
          ? t("capability.skill_source_delete_confirm", { name: deleteTarget.name })
          : ""}
        onCancel={() => setDeleteTarget(null)}
        onConfirm={() => {
          if (deleteTarget) onDelete(deleteTarget);
          setDeleteTarget(null);
        }}
        title={t("capability.skill_source_delete")}
        variant="danger"
      />
    </>
  );
}

interface SourceRowProps {
  disabled: boolean;
  onDelete: () => void;
  onEdit: () => void;
  onToggle: (enabled: boolean) => void;
  source: ExternalSkillSourceInfo;
}

function SourceRow({
  disabled,
  onDelete,
  onEdit,
  onToggle,
  source,
}: SourceRowProps) {
  const { t } = useI18n();
  return (
    <div className="flex min-w-0 items-center gap-3 bg-(--surface-raised-background) px-3.5 py-3">
      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-medium text-(--text-strong)">
          {source.name}
        </div>
        <div className="mt-0.5 truncate text-xs text-(--text-muted)">
          {sourceKindLabel(source.kind, t)} · {source.url}
        </div>
        {source.deletable ? (
          <div className="mt-1 text-xs text-(--text-soft)">
            {source.credential_configured
              ? t("capability.skill_source_credential_configured")
              : t("capability.skill_source_auth_none")}
          </div>
        ) : null}
        {source.last_error ? (
          <div className="mt-1 truncate text-xs text-(--destructive)">
            {source.last_error}
          </div>
        ) : null}
      </div>
      <div className="flex shrink-0 items-center gap-1">
        {source.deletable ? (
          <>
            <UiIconButton
              disabled={disabled}
              onClick={onEdit}
              size="sm"
              title={t("capability.skill_source_edit")}
              variant="ghost"
            >
              <Pencil className="h-3.5 w-3.5" />
            </UiIconButton>
            <UiIconButton
              disabled={disabled}
              onClick={onDelete}
              size="sm"
              title={t("capability.skill_source_delete")}
              tone="danger"
              variant="ghost"
            >
              <Trash2 className="h-3.5 w-3.5" />
            </UiIconButton>
          </>
        ) : null}
        <GlassSwitch
          aria-label={t("capability.skill_source_toggle", {
            name: source.name,
          })}
          checked={source.enabled}
          disabled={disabled}
          onChange={onToggle}
          size="sm"
        />
      </div>
    </div>
  );
}

interface PrivateSourceEditorDialogProps {
  draft: PrivateSkillSourceDraft;
  editingSource: ExternalSkillSourceInfo | null;
  loading: boolean;
  onCancel: () => void;
  onChange: (draft: PrivateSkillSourceDraft) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
}

function PrivateSourceEditorDialog({
  draft,
  editingSource,
  loading,
  onCancel,
  onChange,
  onSubmit,
}: PrivateSourceEditorDialogProps) {
  const { t } = useI18n();
  const updateDraft = <K extends keyof PrivateSkillSourceDraft>(
    key: K,
    value: PrivateSkillSourceDraft[K],
  ) => onChange({ ...draft, [key]: value });
  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[9999]"
        closeOnBackdrop={!loading}
        onClose={loading ? undefined : onCancel}
      >
        <UiDialogFormShell
          className="max-h-[calc(100dvh-2rem)]"
          onSubmit={onSubmit}
          size="md"
        >
          <UiDialogHeader
            appearance="plain"
            onClose={loading ? undefined : onCancel}
            title={t(editingSource
              ? "capability.skill_source_edit_title"
              : "capability.skill_source_add_title")}
          />
          <UiDialogBody className="space-y-4" scrollable>
            <UiField
              htmlFor="private-skill-source-name"
              label={t("capability.skill_source_name")}
              required
            >
              <UiInput
                data-autofocus="true"
                disabled={loading}
                id="private-skill-source-name"
                onChange={(event) => updateDraft("name", event.target.value)}
                pattern=".*\S.*"
                placeholder={t("capability.skill_source_name_placeholder")}
                required
                value={draft.name}
              />
            </UiField>
            <UiField
              description={editingSource
                ? t("capability.skill_source_url_immutable")
                : t("capability.skill_source_url_description")}
              htmlFor="private-skill-source-url"
              label={t("capability.skill_source_url")}
              required
            >
              <UiInput
                disabled={loading || Boolean(editingSource)}
                id="private-skill-source-url"
                onChange={(event) => updateDraft("url", event.target.value)}
                placeholder="https://skills.example.com/registry"
                required
                type="url"
                value={draft.url}
              />
            </UiField>
            <UiField label={t("capability.skill_source_auth_type")}>
              <div className="flex flex-wrap gap-1.5">
                {(["none", "bearer"] as const).map((authType) => (
                  <UiButton
                    disabled={loading}
                    key={authType}
                    onClick={() => updateDraft("authType", authType)}
                    size="sm"
                    tone={draft.authType === authType ? "primary" : undefined}
                    variant={draft.authType === authType ? "solid" : "surface"}
                  >
                    {t(authType === "none"
                      ? "capability.skill_source_auth_none"
                      : "capability.skill_source_auth_bearer")}
                  </UiButton>
                ))}
              </div>
            </UiField>
            {draft.authType === "bearer" ? (
              <UiField
                description={editingSource?.credential_configured
                  ? t("capability.skill_source_token_keep")
                  : t("capability.skill_source_token_description")}
                htmlFor="private-skill-source-token"
                label={t("capability.skill_source_token")}
                required={!editingSource?.credential_configured}
              >
                <UiInput
                  autoComplete="new-password"
                  disabled={loading}
                  id="private-skill-source-token"
                  onChange={(event) => updateDraft("token", event.target.value)}
                  pattern=".*\S.*"
                  placeholder={editingSource?.credential_configured ? "••••••••" : "token"}
                  required={!editingSource?.credential_configured}
                  type="password"
                  value={draft.token}
                />
              </UiField>
            ) : null}
          </UiDialogBody>
          <UiDialogFooter appearance="plain" className="gap-2">
            <UiButton
              disabled={loading}
              onClick={onCancel}
              size="sm"
              variant="surface"
            >
              {t("common.cancel")}
            </UiButton>
            <UiButton
              disabled={loading}
              size="sm"
              tone="primary"
              type="submit"
              variant="solid"
            >
              {loading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
              {t(editingSource
                ? "capability.skill_source_validate_and_save"
                : "capability.skill_source_validate_and_add")}
            </UiButton>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
