/**
 * INPUT: Provider 可见范围、目录资源、草稿与写命令。
 * OUTPUT: Provider 目录、当前配置和模型操作，首层不展示预设宣传说明。
 * POS: 设置与运营复用的 Provider 管理工作区。
 */
"use client";

import { useState } from "react";
import { Cable } from "lucide-react";

import { isDesktopRuntime } from "@/config/desktop-runtime";
import { ProviderCCSwitchDialog } from "@/features/provider-imports/cc-switch/provider-ccswitch-dialog";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { ProviderConfigRecord } from "@/types/capability/provider";

import { ProviderSettingsConfigForm } from "./components/provider-settings-config-form";
import { ProviderSettingsDetailHeader } from "./components/provider-settings-detail-header";
import { ProviderSettingsModelList } from "./components/provider-settings-model-list";
import { ProviderSettingsSidebar } from "./components/provider-settings-sidebar";
import { ProviderAddModelDialog } from "./dialogs/provider-settings-add-model-dialog";
import { ProviderDeleteUsageDialog } from "./dialogs/provider-settings-delete-usage-dialog";
import { ProviderModelOptionsDialog } from "./dialogs/provider-settings-model-options-dialog";
import { getProviderTitle } from "./model/provider-config-model";
import { SETTINGS_TABS } from "./model/provider-settings-presentation";
import { useProviderSettingsController } from "./use-provider-settings-controller";

interface ProviderSettingsPanelProps {
  embedded?: boolean;
  layout?: "page" | "section";
  visibilityScope?: ProviderConfigRecord["visibility"];
}

export function ProviderSettingsPanel({
  embedded = false,
  layout = "page",
  visibilityScope = "private",
}: ProviderSettingsPanelProps) {
  const { t } = useI18n();
  const [ccSwitchOpen, setCCSwitchOpen] = useState(false);
  const canImportFromCCSwitch = visibilityScope === "private" && isDesktopRuntime();
  const { state, actions, modelActions } =
    useProviderSettingsController(visibilityScope);

  const panelContent = (
    <div className={cn(
      layout === "page" ? WORKSPACE_CONTENT_PAGE_CLASS_NAME : undefined,
      "flex min-h-0 flex-col sm:h-full",
    )}>
      {layout === "page" ? (
        <WorkspaceContentHeader
          className="max-sm:hidden"
          description={t(visibilityScope === "public"
            ? "settings.providers.public_section_description"
            : "settings.providers.section_description")}
          title={t(visibilityScope === "public"
            ? "operations.tabs.subscription_providers"
            : "settings.providers.section_title")}
        />
      ) : null}
      <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-visible sm:flex-row sm:items-stretch sm:gap-5 sm:overflow-hidden">
        <ProviderSettingsSidebar
          configuredByPreset={state.configuredByPreset}
          customProviders={state.customProviders}
          draftPresetKey={state.draft.preset_key}
          isCreating={state.isCreating}
          isEditing={state.isEditing}
          loading={state.loading}
          onCreateFromPreset={actions.handleCreateFromPreset}
          onOpenCCSwitchImport={() => setCCSwitchOpen(true)}
          onRequestDeleteProvider={actions.handleRequestDeleteProvider}
          onSelectProvider={actions.handleSelectProvider}
          pendingAction={state.pendingAction}
          presetSidebarItems={state.presetSidebarItems}
          selectedProvider={state.selectedProvider}
          showCCSwitchImport={canImportFromCCSwitch}
        />

        <section className="flex min-h-0 min-w-0 flex-1 flex-col overflow-visible sm:overflow-hidden">
          {state.isEmptyMode ? null : (
            <div className="flex min-h-0 flex-1 flex-col bg-transparent py-2 sm:px-5">
              <ProviderSettingsDetailHeader
                detailTitle={state.detailTitle}
                enabled={state.draft.enabled}
                hasSelectedRecord={!!state.selectedRecord}
                isApiFormatConfigurable={state.isApiFormatConfigurable}
                isEditing={state.isEditing}
                onEnabledChange={actions.handleEnabledChange}
                onTestSelection={modelActions.handleTestSelection}
                pendingAction={state.pendingAction}
                selectedCanManage={state.selectedCanManage}
                testModelOptions={modelActions.testModelOptions}
              />

              <div className="flex min-h-0 flex-1 flex-col gap-4">
                <ProviderSettingsConfigForm
                  builtinEndpointFormats={state.builtinEndpointFormats}
                  currentFormat={state.currentFormat}
                  currentPreset={state.currentPreset}
                  detailTitle={state.detailTitle}
                  draft={state.draft}
                  formatOptions={state.formatOptions}
                  isEditing={state.isEditing}
                  onApiFormatChange={actions.handleApiFormatChange}
                  onAuthTokenChange={(value) => actions.updateDraft({
                    auth_token: value,
                  })}
                  onBaseUrlChange={(value) => actions.updateDraft({
                    base_url: value,
                  })}
                  onFieldBlur={actions.handleProviderFieldBlur}
                  onProviderDisplayNameChange={
                    actions.handleProviderDisplayNameChange
                  }
                  onProviderKindChange={actions.handleProviderKindChange}
                  providerKindOptions={state.providerKindOptions}
                  selectedCanManage={state.selectedCanManage}
                  selectedRecord={state.selectedRecord}
                  showProviderShapeControls={state.showProviderShapeControls}
                  showRuntimeFormatBadge={state.showRuntimeFormatBadge}
                  usesBuiltinEndpoint={state.usesBuiltinEndpoint}
                />

                <ProviderSettingsModelList
                  displayedModels={modelActions.displayedModels}
                  hasModelsEndpoint={state.hasModelsEndpoint}
                  isApiFormatConfigurable={state.isApiFormatConfigurable}
                  isEditing={state.isEditing}
                  modelQuery={modelActions.modelQuery}
                  onDefaultModelDisableAttempt={
                    modelActions.handleDefaultModelDisableAttempt
                  }
                  onFetchModels={modelActions.handleFetchModels}
                  onModelOptions={modelActions.setModelOptionsFromRecord}
                  onModelQueryChange={modelActions.setModelQuery}
                  onOpenAddModel={modelActions.handleOpenAddModel}
                  onToggleModel={modelActions.handleToggleModel}
                  pendingAction={state.pendingAction}
                  selectedCanManage={state.selectedCanManage}
                  selectedRecord={state.selectedRecord}
                />
              </div>
            </div>
          )}
        </section>
      </div>
    </div>
  );

  return (
    <>
      {embedded ? panelContent : (
        <WorkspaceSurfaceScaffold
          bodyScrollable
          stableGutter
          header={(
            <WorkspaceSurfaceHeader
              activeTab="providers"
              leading={<Cable className="h-4 w-4" />}
              tabs={SETTINGS_TABS.map((item) => ({
                key: item.key,
                label: t(item.labelKey),
              }))}
              title={t("settings.title")}
            />
          )}
        >
          {panelContent}
        </WorkspaceSurfaceScaffold>
      )}

      <FeedbackBannerViewport
        item={state.feedback ? {
          message: state.feedback.message,
          onDismiss: actions.dismissFeedback,
          title: state.feedback.title,
          tone: state.feedback.tone,
        } : null}
      />

      <ConfirmDialog
        confirmText={t("common.delete")}
        isOpen={state.deleteConfirmOpen}
        message={t("settings.providers.delete_confirm_runtime_message", {
          name: state.deleteTargetRecord
            ? getProviderTitle(state.deleteTargetRecord)
            : "",
        })}
        onCancel={actions.closeDeleteDialog}
        onConfirm={() => actions.handleDelete()}
        title={t("settings.providers.delete_provider")}
        variant="danger"
      />

      <ProviderDeleteUsageDialog
        deleteTargetRecord={state.deleteTargetRecord}
        isOpen={state.deleteUsageOpen}
        onCancel={actions.closeDeleteDialog}
        onForceDelete={() => actions.handleDelete(true)}
        pendingAction={state.pendingAction}
      />

      <ProviderAddModelDialog
        isOpen={modelActions.addModelOpen}
        manualModelEnabled={modelActions.manualModelEnabled}
        manualModelId={modelActions.manualModelId}
        manualModelPlaceholder={modelActions.manualModelPlaceholder}
        onAdd={modelActions.handleAddModel}
        onClose={() => modelActions.setAddModelOpen(false)}
        pendingAction={state.pendingAction}
        selectedCanManage={state.selectedCanManage}
        setManualModelEnabled={modelActions.setManualModelEnabled}
        setManualModelId={modelActions.setManualModelId}
      />

      <ProviderModelOptionsDialog
        modelOptions={modelActions.modelOptions}
        onClose={() => modelActions.setModelOptions(null)}
        onSave={modelActions.handleSaveModelOptions}
        pendingAction={state.pendingAction}
        selectedCanManage={state.selectedCanManage}
        setModelOptions={modelActions.setModelOptions}
      />

      {canImportFromCCSwitch ? (
        <ProviderCCSwitchDialog
          isOpen={ccSwitchOpen}
          onClose={() => setCCSwitchOpen(false)}
          onSynced={actions.handleCCSwitchSynced}
        />
      ) : null}
    </>
  );
}
