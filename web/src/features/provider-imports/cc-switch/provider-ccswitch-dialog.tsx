/**
 * INPUT: CC Switch 本地配置预览、选择状态与同步命令。
 * OUTPUT: plain 服务选择器、路径切换，以及区分只读、同步结果未知和已提交后刷新失败的完整反馈。
 * POS: Provider 导入边界；相同来源同步保持幂等，已提交后的页面刷新失败不得重放同步写入。
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
import {
  projectMutationFailure,
  type MutationFailureEffect,
} from "@/lib/error-message";
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
import { UiCheckbox } from "@/shared/ui/form/checkbox";
import { UiResourceState } from "@/shared/ui/display/resource-state";
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

type CCSwitchFailure =
  | {
      kind: "read";
    }
  | {
      effect: MutationFailureEffect;
      kind: "sync";
    }
  | {
      kind: "committed_refresh";
      result: CCSwitchSyncResult;
    };

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
  const [failure, setFailure] = useState<CCSwitchFailure | null>(null);
  const requestIdRef = useRef(0);

  const loadPreview = useCallback(async (requestedPath?: string) => {
    const requestId = ++requestIdRef.current;
    setLoading(true);
    setFailure(null);
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
    } catch {
      if (requestId !== requestIdRef.current) {
        return;
      }
      setPreview(null);
      setFailure({
        kind: "read",
      });
      setEditingPath(true);
    } finally {
      if (requestId === requestIdRef.current) {
        setLoading(false);
      }
    }
  }, [requireDefault]);

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
    setFailure(null);
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
    setFailure(null);
    try {
      const result = await syncCCSwitchApi({
        config_dir: configDir.trim() || undefined,
        source_keys: [...selectedSources],
        set_default: requireDefault || (setDefault && canSetDefault),
      });
      try {
        await onSynced(result);
        onClose();
      } catch {
        setFailure({
          kind: "committed_refresh",
          result,
        });
      }
    } catch (reason) {
      const mutation = projectMutationFailure(
        reason,
        t("settings.providers.ccswitch_sync_failed"),
      );
      setFailure({
        effect: mutation.effect,
        kind: "sync",
      });
    } finally {
      setSyncing(false);
    }
  };

  const handleRefreshAfterSync = async () => {
    if (!failure || failure.kind !== "committed_refresh" || syncing) return;
    setSyncing(true);
    try {
      await onSynced(failure.result);
      onClose();
    } catch {
      setFailure(failure);
    } finally {
      setSyncing(false);
    }
  };

  const syncResultPending = failure?.kind === "sync"
    && (failure.effect === "accepted" || failure.effect === "committed");
  const controlsLocked = failure?.kind === "committed_refresh" || (
    failure?.kind === "sync" && failure.effect !== "not_applied"
  );

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        layer="systemDialog"
        closeOnBackdrop={!syncing}
        labelledBy="provider-ccswitch-title"
        onClose={syncing ? undefined : onClose}
      >
        <UiDialogFormShell
          onSubmit={(event) => {
            event.preventDefault();
            if (failure?.kind === "committed_refresh") {
              void handleRefreshAfterSync();
            } else {
              void handleSync();
            }
          }}
          size="lg"
          viewport="adaptiveMax"
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
            locked={controlsLocked || syncing}
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

            {!loading && failure ? (
              <CCSwitchFailureState failure={failure} />
            ) : null}

            {!loading && failure?.kind !== "read" && preview && !preview.detected ? (
              <CCSwitchEmptyState
                description={t("settings.providers.ccswitch_not_found_hint")}
                title={t("settings.providers.ccswitch_not_found")}
              />
            ) : null}

            {!loading && failure?.kind !== "read" && preview?.detected && preview.providers.length === 0 ? (
              <CCSwitchEmptyState
                description={t("settings.providers.ccswitch_empty_hint")}
                title={t("settings.providers.ccswitch_empty")}
              />
            ) : null}

            {!loading && failure?.kind !== "read" && preview?.detected && preview.providers.length > 0 ? (
              <div className="divide-y divide-(--divider-subtle-color)">
                {preview.providers.map((item) => (
                  <CCSwitchProviderRow
                    key={item.source_key}
                    checked={selectedSources.has(item.source_key)}
                    disabled={controlsLocked || syncing}
                    item={item}
                    onChange={(checked) => toggleSource(item.source_key, checked)}
                  />
                ))}
              </div>
            ) : null}
          </UiDialogBody>

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
                  <UiCheckbox
                    checked={setDefault && canSetDefault}
                    checkboxSize="small"
                    disabled={!canSetDefault || controlsLocked || syncing}
                    onChange={(event) => setSetDefault(event.target.checked)}
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
                disabled={loading || syncing || (!canSync && failure?.kind !== "committed_refresh") || syncResultPending}
                tone="primary"
                type="submit"
                variant="solid"
              >
                {syncing ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
                {syncing
                  ? t(requireDefault
                    ? "settings.providers.ccswitch_importing"
                    : "settings.providers.ccswitch_syncing")
                  : failure?.kind === "committed_refresh"
                    ? t("settings.providers.ccswitch_refresh_settings")
                    : failure?.kind === "sync"
                      ? t("settings.providers.ccswitch_sync_again")
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

function CCSwitchFailureState({ failure }: { failure: CCSwitchFailure }) {
  const { t } = useI18n();
  if (failure.kind === "read") {
    return (
      <UiResourceState
        className="m-5 min-h-0 py-5"
        impact={t("settings.providers.ccswitch_detect_failed_impact")}
        nextStep={t("settings.providers.ccswitch_detect_failed_next_step")}
        size="sm"
        state="error"
        title={t("settings.providers.ccswitch_detect_failed")}
        urgency="polite"
        variant="card"
      />
    );
  }
  if (failure.kind === "committed_refresh") {
    return (
      <UiResourceState
        className="m-5 min-h-0 py-5"
        impact={t("settings.providers.ccswitch_refresh_after_sync_failed_impact")}
        nextStep={t("settings.providers.ccswitch_refresh_after_sync_failed_next_step")}
        size="sm"
        state="error"
        title={t("settings.providers.ccswitch_refresh_after_sync_failed_title")}
        urgency="polite"
        variant="card"
      />
    );
  }

  const notApplied = failure.effect === "not_applied";
  const pending = failure.effect === "accepted" || failure.effect === "committed";
  return (
    <UiResourceState
      className="m-5 min-h-0 py-5"
      impact={t(notApplied
        ? "settings.providers.ccswitch_sync_not_applied_impact"
        : pending
          ? "settings.providers.ccswitch_sync_pending_impact"
          : "settings.providers.ccswitch_sync_unknown_impact")}
      nextStep={t(notApplied
        ? "settings.providers.ccswitch_sync_not_applied_next_step"
        : pending
          ? "settings.providers.ccswitch_sync_pending_next_step"
          : "settings.providers.ccswitch_sync_unknown_next_step")}
      size="sm"
      state="error"
      title={t(notApplied
        ? "settings.providers.ccswitch_sync_not_applied_title"
        : pending
          ? "settings.providers.ccswitch_sync_pending_title"
          : "settings.providers.ccswitch_sync_unknown_title")}
      urgency="polite"
      variant="card"
    />
  );
}

function CCSwitchSourceBar({
  configDir,
  databasePath,
  editingPath,
  loading,
  locked,
  onConfigDirChange,
  onDetect,
  onEditPath,
}: {
  configDir: string;
  databasePath?: string;
  editingPath: boolean;
  loading: boolean;
  locked: boolean;
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
          disabled={locked}
          onChange={(event) => onConfigDirChange(event.target.value)}
          placeholder="~/.cc-switch"
          value={configDir}
        />
        <UiButton disabled={loading || locked || !configDir.trim()} onClick={onDetect} size="sm" variant="surface">
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
      <UiButton disabled={locked} onClick={onEditPath} size="xs" variant="ghost">
        {t("settings.providers.ccswitch_change_path")}
      </UiButton>
      <UiButton disabled={loading || locked} onClick={onDetect} size="xs" variant="ghost">
        <RotateCcw className={cn("h-3.5 w-3.5", loading && "animate-spin")} />
        {t("settings.providers.ccswitch_refresh")}
      </UiButton>
    </div>
  );
}

function CCSwitchProviderRow({
  checked,
  disabled,
  item,
  onChange,
}: {
  checked: boolean;
  disabled: boolean;
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
      item.can_sync && !disabled
        ? "cursor-pointer hover:bg-(--surface-interactive-hover-background)"
        : "cursor-not-allowed bg-(--surface-muted-background) opacity-70",
      checked && "bg-[color:color-mix(in_srgb,var(--brand)_5%,transparent)]",
    )}>
      <UiCheckbox
        checked={checked}
        disabled={!item.can_sync || disabled}
        onChange={(event) => onChange(event.target.checked)}
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
