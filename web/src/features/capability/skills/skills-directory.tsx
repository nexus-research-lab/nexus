"use client";

import { useCallback } from "react";
import { useNavigate, useParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { WORKSPACE_DETAIL_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

import type { SkillsRouteParams } from "@/types/app/route";

import { SkillsCatalogGrid } from "./catalog/skills-catalog-grid";
import { SkillsUpdateHighlight } from "./catalog/skills-update-highlight";
import {
  type SkillMarketplaceFeedback,
} from "./controller/skill-marketplace-controller";
import { useSkillMarketplace } from "./controller/use-skill-marketplace";
import { SkillDetailRoute } from "./detail/skill-detail-route";
import { ExternalSkillPreviewDialog } from "./external/external-skill-preview-dialog";
import { buildExternalSkillPreviewModel } from "./external/external-skill-model";
import { SkillSourceManagerDialog } from "./external/skill-source-manager-dialog";
import { SkillsExternalResults } from "./external/skills-external-results";
import { SkillImportDialog } from "./import/skill-import-dialog";
import { SkillsHeader } from "./skills-header";
import { SkillsSearchBar } from "./skills-search-bar";
import { SKILLS_TOUR_ANCHORS } from "@/features/onboarding/tours/skills-tour";

/* ── Skills 页面主编排组件 ────────────────────── */

interface SkillsDirectoryProps {
  onReplayTour?: () => void;
}

export function SkillsDirectory({ onReplayTour }: SkillsDirectoryProps) {
  const { t } = useI18n();
  const {
    catalog,
    discoveryMode,
    external,
    feedback,
    operations,
    setDiscoveryMode,
    sources,
  } = useSkillMarketplace();
  const navigate = useNavigate();
  const { skillName } = useParams<SkillsRouteParams>();
  const openSkillPage = useCallback(
    (name: string) => {
      navigate(AppRouteBuilders.skillDetail(name));
    },
    [navigate],
  );
  const backToSkills = useCallback(() => {
    navigate(AppRouteBuilders.skills());
  }, [navigate]);
  const previewModel = buildExternalSkillPreviewModel(
    external.previewItem,
    catalog.importedExternalSources,
    operations.busyExternalKeys,
    external.previewLoading,
  );
  const feedbackItem = buildFeedbackItem(feedback);

  return (
    <>
      {/* 隐藏的文件选择器 */}
      <input
        accept=".zip,application/zip"
        className="hidden"
        disabled={operations.importing}
        onChange={(e) => {
          const file = e.target.files?.[0];
          if (file) void operations.importLocal(file);
          e.currentTarget.value = "";
        }}
        ref={operations.fileInputRef}
        type="file"
      />

      <WorkspaceSurfaceScaffold
        bodyScrollable
        header={(
          <div data-tour-anchor={SKILLS_TOUR_ANCHORS.header}>
            <SkillsHeader
              catalogCount={catalog.catalogCount}
              checkingUpdates={operations.checkingUpdates}
              discoveryMode={discoveryMode}
              importing={operations.importing}
              onChangeDiscoveryMode={setDiscoveryMode}
              onCheckUpdates={() => void operations.checkUpdates()}
              onOpenImport={operations.setImportDialogMode}
              onOpenSources={sources.openManager}
              onReplayTour={onReplayTour}
            />
          </div>
        )}
        stableGutter
      >
        {skillName ? (
          <SkillDetailRoute
            deleteSkill={operations.deleteSkill}
            key={skillName}
            skillName={skillName}
            onBack={backToSkills}
            onDeleted={backToSkills}
            updateSkill={operations.updateSkill}
          />
        ) : (
          <div className={WORKSPACE_DETAIL_PAGE_CLASS_NAME}>
            <div className="mb-5">
              <h1 className="text-[24px] font-semibold tracking-[-0.03em] text-(--text-strong)">
                {t("capability.skills_intro_title")}
              </h1>
              <p className="mt-1 max-w-[680px] text-[13px] leading-6 text-(--text-muted)">
                {t("capability.skills_intro_description")}
              </p>
            </div>

            <div data-tour-anchor={SKILLS_TOUR_ANCHORS.search}>
              <SkillsSearchBar
                activeCategory={catalog.activeCategory}
                catalogQuery={catalog.query}
                categories={catalog.categories}
                discoveryMode={discoveryMode}
                externalLoading={external.loading}
                externalQuery={external.query}
                onChangeCategory={catalog.setActiveCategory}
                onChangeCatalogQuery={catalog.setQuery}
                onChangeExternalQuery={external.setQuery}
                onSubmitExternalSearch={external.submit}
              />
            </div>

            <div data-tour-anchor={SKILLS_TOUR_ANCHORS.catalog}>
              {discoveryMode === "external" && (
                <SkillsExternalResults
                  busyExternalKeys={operations.busyExternalKeys}
                  importedExternalSources={catalog.importedExternalSources}
                  loading={external.loading}
                  onImport={(item) => void operations.importExternal(item)}
                  onPreview={(item) => void external.preview(item)}
                  results={external.results}
                  sourceStatuses={external.sourceStatuses}
                  sources={sources.items}
                  submittedQuery={external.submittedQuery}
                />
              )}
              {discoveryMode === "catalog" && (
                <>
                  <SkillsUpdateHighlight
                    busySkillNames={operations.busySkillNames}
                    checkUpdateMessage={operations.checkUpdateMessage}
                    checkingUpdates={operations.checkingUpdates}
                    lastUpdateCheckedAt={operations.lastUpdateCheckedAt}
                    onCheckUpdates={() => void operations.checkUpdates()}
                    onOpenSkill={openSkillPage}
                    onUpdateSkill={(name) => void operations.updateSkill(name)}
                    updates={catalog.updateAvailableSkills}
                  />
                  <SkillsCatalogGrid
                    busySkillNames={operations.busySkillNames}
                    groupedSkills={catalog.groupedSkills}
                    loading={catalog.loading}
                    onDeleteSkill={(skill) => void operations.deleteSkill(skill)}
                    onOpenSkill={openSkillPage}
                  />
                </>
              )}
            </div>
          </div>
        )}
      </WorkspaceSurfaceScaffold>

      <FeedbackBannerViewport item={feedbackItem} />

      <SkillImportDialog
        fileInputRef={operations.fileInputRef}
        importing={operations.importing}
        mode={operations.importDialogMode}
        onClose={() => operations.setImportDialogMode(null)}
        onImportGit={(url, branch, path) => void operations.importGit(url, branch, path)}
        onSelectMode={operations.setImportDialogMode}
      />

      <ExternalSkillPreviewDialog
        model={previewModel}
        onClose={external.closePreview}
        onImport={(item) => void operations.importExternal(item)}
      />

      <SkillSourceManagerDialog
        isOpen={sources.managerOpen}
        loading={sources.loading}
        onClose={sources.closeManager}
        onToggle={(source, enabled) => void sources.toggle(source, enabled)}
        sources={sources.items}
      />
    </>
  );
}

function buildFeedbackItem(
  feedback: SkillMarketplaceFeedback | null,
): FeedbackBannerProps | null {
  if (!feedback) return null;
  const titles = {
    error: "操作失败",
    success: "已完成",
    warning: feedback.pending ? "处理中" : "部分完成",
  } as const;
  return {
    message: feedback.message,
    onDismiss: feedback.pending ? undefined : feedback.dismiss,
    title: titles[feedback.tone],
    tone: feedback.tone,
  };
}
