/**
 * INPUT: CC Switch 本地配置预览、选择状态与同步命令。
 * OUTPUT: plain 服务选择器、路径切换和同步反馈。
 * POS: Provider 导入边界；内部路径按需展示，不制造向导式说明。
 */
"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Database,
  Loader2,
  RotateCcw,
} from "lucide-react";

import {
  previewCCSwitchApi,
  syncCCSwitchApi,
} from "@/lib/api/settings/provider-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import { UiInput } from "@/shared/ui/form/form-control";
import type {
  CCSwitchPreview,
  CCSwitchProviderPreview,
  CCSwitchSyncResult,
} from "@/types/capability/provider";

interface ProviderCCSwitchDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onSynced: (result: CCSwitchSyncResult) => Promise<void> | void;
  requireDefault?: boolean;
}

export function ProviderCCSwitchDialog({
  isOpen,
  onClose,
  onSynced,
  requireDefault = false,
}: ProviderCCSwitchDialogProps) {
  const { t } = useI18n();
  const [preview, setPreview] = useState<CCSwitchPreview | null>(null);
  const [configDir, setConfigDir] = useState("");
  const [selectedSources, setSelectedSources] = useState<Set<string>>(new Set());
  const [setDefault, setSetDefault] = useState(false);
  const [editingPath, setEditingPath] = useState(false);
  const [loading, setLoading] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [error, setError] = useState("");
  const requestIdRef = useRef(0);

  const loadPreview = useCallback(async (requestedPath?: string) => {
    const requestId = ++requestIdRef.current;
    setLoading(true);
    setError("");
    try {
      const result = await previewCCSwitchApi(requestedPath);
      if (requestId !== requestIdRef.current) {
        return;
      }
      setPreview(result);
      setConfigDir(result.config_dir || requestedPath || "");
      setSelectedSources(initialCCSwitchSelection(result));
      setSetDefault(requireDefault || result.needs_default);
      setEditingPath(!result.detected);
    } catch (reason) {
      if (requestId !== requestIdRef.current) {
        return;
      }
      setPreview(null);
      setError(reason instanceof Error
        ? reason.message
        : t("settings.providers.ccswitch_detect_failed"));
      setEditingPath(true);
    } finally {
      if (requestId === requestIdRef.current) {
        setLoading(false);
      }
    }
  }, [requireDefault, t]);

  useEffect(() => {
    if (!isOpen) {
      requestIdRef.current += 1;
      return;
    }
    setPreview(null);
    setConfigDir("");
    setSelectedSources(new Set());
    setSetDefault(false);
    setEditingPath(false);
    setSyncing(false);
    setError("");
    void loadPreview();
  }, [isOpen, loadPreview]);

  const selectedProviders = useMemo(
    () => preview?.providers.filter((item) => selectedSources.has(item.source_key)) ?? [],
    [preview, selectedSources],
  );
  const selectedModelCount = useMemo(
    () => selectedProviders.reduce((total, item) => total + item.models.length, 0),
    [selectedProviders],
  );
  const canSetDefault = selectedProviders.some((item) => item.current_runtime_supported);
  const canSync = selectedSources.size > 0 && (!requireDefault || canSetDefault);

  if (!isOpen) {
    return null;
  }

  const toggleSource = (sourceKey: string, checked: boolean) => {
    setSelectedSources((current) => {
      const next = new Set(current);
      if (checked) {
        next.add(sourceKey);
      } else {
        next.delete(sourceKey);
      }
      return next;
    });
  };

  const handleSync = async () => {
    if (!canSync || syncing) {
      return;
    }
    setSyncing(true);
    setError("");
    try {
      const result = await syncCCSwitchApi({
        config_dir: configDir.trim() || undefined,
        source_keys: [...selectedSources],
        set_default: requireDefault || (setDefault && canSetDefault),
      });
      await onSynced(result);
      onClose();
    } catch (reason) {
      setError(reason instanceof Error
        ? reason.message
        : t("settings.providers.ccswitch_sync_failed"));
    } finally {
      setSyncing(false);
    }
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[12000]"
        closeOnBackdrop={!syncing}
        labelledBy="provider-ccswitch-title"
        onClose={syncing ? undefined : onClose}
      >
        <UiDialogFormShell
          className="max-h-[min(82dvh,680px)] !max-w-[620px]"
          onSubmit={(event) => {
            event.preventDefault();
            void handleSync();
          }}
          size="lg"
        >
          <UiDialogHeader
            appearance="plain"
            onClose={syncing ? undefined : onClose}
            title={requireDefault
              ? t("settings.providers.ccswitch_import_title")
              : t("settings.providers.ccswitch_title")}
            titleId="provider-ccswitch-title"
          />

          <CCSwitchSourceBar
            configDir={configDir}
            databasePath={preview?.database_path}
            editingPath={editingPath}
            loading={loading}
            onConfigDirChange={setConfigDir}
            onDetect={() => void loadPreview(configDir)}
            onEditPath={() => setEditingPath(true)}
          />

          <UiDialogBody className="!min-h-0 !flex-1 p-0" scrollable>
            {loading ? (
              <div className="flex h-full min-h-[180px] items-center justify-center gap-2 text-sm text-(--text-muted)">
                <Loader2 className="h-4 w-4 animate-spin" />
                {t("settings.providers.ccswitch_detecting")}
              </div>
            ) : null}

            {!loading && preview && !preview.detected ? (
              <CCSwitchEmptyState
                description={t("settings.providers.ccswitch_not_found_hint")}
                title={t("settings.providers.ccswitch_not_found")}
              />
            ) : null}

            {!loading && preview?.detected && preview.providers.length === 0 ? (
              <CCSwitchEmptyState
                description={t("settings.providers.ccswitch_empty_hint")}
                title={t("settings.providers.ccswitch_empty")}
              />
            ) : null}

            {!loading && preview?.detected && preview.providers.length > 0 ? (
              <div className="divide-y divide-(--divider-subtle-color)">
                {preview.providers.map((item) => (
                  <CCSwitchProviderRow
                    key={item.source_key}
                    checked={selectedSources.has(item.source_key)}
                    item={item}
                    onChange={(checked) => toggleSource(item.source_key, checked)}
                  />
                ))}
              </div>
            ) : null}
          </UiDialogBody>

          {error ? (
            <div
              className="border-t border-[color:color-mix(in_srgb,var(--destructive)_20%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--destructive)_6%,transparent)] px-5 py-2.5 text-xs text-(--destructive)"
              role="alert"
            >
              {error}
            </div>
          ) : null}

          <UiDialogFooter appearance="plain" className="justify-between gap-4">
            <div className="min-w-0">
              {selectedSources.size > 0 && requireDefault ? (
                <div className={cn(
                  "flex items-center gap-2 text-xs text-(--text-muted)",
                  !canSetDefault && "opacity-(--disabled-opacity)",
                )}>
                  {t("settings.providers.ccswitch_set_default")}
                </div>
              ) : null}
              {selectedSources.size > 0 && !requireDefault ? (
                <label className={cn(
                  "flex cursor-pointer items-center gap-2 text-xs text-(--text-muted)",
                  !canSetDefault && "cursor-not-allowed opacity-(--disabled-opacity)",
                )}>
                  <input
                    checked={setDefault && canSetDefault}
                    className="h-3.5 w-3.5 accent-(--primary)"
                    disabled={!canSetDefault}
                    onChange={(event) => setSetDefault(event.target.checked)}
                    type="checkbox"
                  />
                  {t("settings.providers.ccswitch_set_default")}
                </label>
              ) : selectedSources.size === 0 ? (
                <span className="text-xs text-(--text-muted)">
                  {t("settings.providers.ccswitch_select_hint")}
                </span>
              ) : null}
              {selectedSources.size > 1 || selectedModelCount > 1 ? (
                <div className="mt-1 text-[11px] text-(--text-soft)">
                  {t("settings.providers.ccswitch_selected_summary", {
                    models: selectedModelCount,
                    providers: selectedSources.size,
                  })}
                </div>
              ) : null}
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <UiButton disabled={syncing} onClick={onClose} variant="surface">
                {t("common.cancel")}
              </UiButton>
              <UiButton
                disabled={loading || syncing || !canSync}
                tone="primary"
                type="submit"
                variant="solid"
              >
                {syncing ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
                {syncing
                  ? t(requireDefault
                    ? "settings.providers.ccswitch_importing"
                    : "settings.providers.ccswitch_syncing")
                  : t(requireDefault
                    ? "settings.providers.ccswitch_import_action"
                    : "settings.providers.ccswitch_sync_action")}
              </UiButton>
            </div>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

function CCSwitchSourceBar({
  configDir,
  databasePath,
  editingPath,
  loading,
  onConfigDirChange,
  onDetect,
  onEditPath,
}: {
  configDir: string;
  databasePath?: string;
  editingPath: boolean;
  loading: boolean;
  onConfigDirChange: (value: string) => void;
  onDetect: () => void;
  onEditPath: () => void;
}) {
  const { t } = useI18n();
  if (editingPath) {
    return (
      <div className="flex items-center gap-2 border-b border-(--divider-subtle-color) px-5 py-3">
        <UiInput
          aria-label={t("settings.providers.ccswitch_path")}
          className="min-w-0 flex-1 font-mono text-xs"
          controlSize="sm"
          onChange={(event) => onConfigDirChange(event.target.value)}
          placeholder="~/.cc-switch"
          value={configDir}
        />
        <UiButton disabled={loading || !configDir.trim()} onClick={onDetect} size="sm" variant="surface">
          {loading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RotateCcw className="h-3.5 w-3.5" />}
          {t("settings.providers.ccswitch_detect")}
        </UiButton>
      </div>
    );
  }
  return (
    <div className="flex items-center gap-2 border-b border-(--divider-subtle-color) px-5 py-3">
      <Database className="h-4 w-4 shrink-0 text-(--icon-muted)" />
      <span className="min-w-0 flex-1 truncate font-mono text-xs text-(--text-muted)" title={databasePath}>
        {databasePath}
      </span>
      <UiButton onClick={onEditPath} size="xs" variant="ghost">
        {t("settings.providers.ccswitch_change_path")}
      </UiButton>
      <UiButton disabled={loading} onClick={onDetect} size="xs" variant="ghost">
        <RotateCcw className={cn("h-3.5 w-3.5", loading && "animate-spin")} />
        {t("settings.providers.ccswitch_refresh")}
      </UiButton>
    </div>
  );
}

function CCSwitchProviderRow({
  checked,
  item,
  onChange,
}: {
  checked: boolean;
  item: CCSwitchProviderPreview;
  onChange: (checked: boolean) => void;
}) {
  const { t } = useI18n();
  const modelSummary = item.models.length > 0
    ? t("settings.providers.ccswitch_models_count", { count: item.models.length })
    : t("settings.providers.ccswitch_no_models");
  return (
    <label className={cn(
      "flex min-h-[72px] items-center gap-3 px-5 py-3 transition-colors",
      item.can_sync
        ? "cursor-pointer hover:bg-(--surface-interactive-hover-background)"
        : "cursor-not-allowed bg-(--surface-muted-background) opacity-70",
      checked && "bg-[color:color-mix(in_srgb,var(--brand)_5%,transparent)]",
    )}>
      <input
        checked={checked}
        className="h-4 w-4 shrink-0 accent-(--primary)"
        disabled={!item.can_sync}
        onChange={(event) => onChange(event.target.checked)}
        type="checkbox"
      />
      <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[10px] border border-(--divider-subtle-color) bg-(--background) text-[11px] font-semibold text-(--text-default)">
        {item.app_type === "claude" ? "CC" : "CX"}
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex min-w-0 items-center gap-2">
          <span className="truncate text-sm font-semibold text-(--text-strong)">{item.name}</span>
          {item.current ? (
            <span className="shrink-0 rounded-full bg-[color:color-mix(in_srgb,var(--brand)_10%,transparent)] px-2 py-0.5 text-[10px] font-semibold text-(--brand-action)">
              {t("settings.providers.ccswitch_current")}
            </span>
          ) : null}
          {item.existing ? (
            <span className="shrink-0 text-[10px] text-(--text-soft)">
              {t("settings.providers.ccswitch_will_update")}
            </span>
          ) : null}
        </span>
        <span className="mt-1 block truncate text-xs text-(--text-muted)">
          {modelSummary}
        </span>
      </span>
      {!item.can_sync ? (
        <span className="max-w-[190px] shrink-0 text-right text-xs text-(--text-muted)">
          {item.reason || t("settings.providers.ccswitch_unavailable")}
        </span>
      ) : null}
    </label>
  );
}

function CCSwitchEmptyState({ title, description }: { title: string; description: string }) {
  return (
    <div className="flex h-full min-h-[180px] flex-col items-center justify-center px-8 text-center">
      <div className="text-sm font-semibold text-(--text-strong)">{title}</div>
      <div className="mt-1 max-w-[360px] text-xs leading-5 text-(--text-muted)">{description}</div>
    </div>
  );
}

function initialCCSwitchSelection(preview: CCSwitchPreview): Set<string> {
  const current = preview.providers
    .filter((item) => item.can_sync && item.current)
    .map((item) => item.source_key);
  if (current.length > 0) {
    return new Set(current);
  }
  if (preview.recommended_source) {
    return new Set([preview.recommended_source]);
  }
  const firstReady = preview.providers.find((item) => item.can_sync);
  return new Set(firstReady ? [firstReady.source_key] : []);
}
