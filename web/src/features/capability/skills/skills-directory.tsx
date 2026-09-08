/**
 * INPUT: Skill 目录/社区资源、筛选、导入更新动作与可选详情路由。
 * OUTPUT: 一行摘要的 Skill 管理目录或当前 Skill 详情。
 * POS: “能力 > 技能”的唯一页面装配入口。
 */
"use client";

import { useCallback } from "react";
import { useNavigate, useParams } from "react-router-dom";

import { AppRouteBuilders } from "@/shared/navigation/route-paths";
import { CapabilityPageLayout } from "@/features/capability/shared/capability-page-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  completeFeedbackBanner,
  type FeedbackBannerProps,
} from "@/shared/ui/feedback/feedback-banner-contract";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
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
import { SkillsHeaderActions } from "./skills-header-actions";
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
    { t },
  );
  const feedbackItem = buildFeedbackItem(feedback, t);

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
        stableGutter
      >
        {skillName ? (
          <SkillDetailRoute
            deleteSkill={operations.deleteSkill}
            key={skillName}
            onAgentBindingChanged={async () => {
              await catalog.refresh();
            }}
            skillName={skillName}
            onBack={backToSkills}
            onDeleted={backToSkills}
            updateSkill={operations.updateSkill}
          />
        ) : (
          <CapabilityPageLayout
            actions={(
              <SkillsHeaderActions
                checkingUpdates={operations.checkingUpdates}
                importing={operations.importing}
                onCheckUpdates={() => void operations.checkUpdates()}
                onOpenImport={operations.setImportDialogMode}
                onOpenSources={sources.openManager}
                onReplayTour={onReplayTour}
              />
            )}
            description={t("capability.skills_intro_description")}
            headerAnchor={SKILLS_TOUR_ANCHORS.header}
            title={t("capability.skills_intro_title")}
          >
            <div data-tour-anchor={SKILLS_TOUR_ANCHORS.search}>
              <SkillsSearchBar
                activeCategory={catalog.activeCategory}
                catalogQuery={catalog.query}
                categories={catalog.categories}
                discoveryMode={discoveryMode}
                externalLoading={external.loading}
                externalQuery={external.query}
                externalSourceId={external.sourceId}
                externalSources={[
                  {
                    label: t("capability.skills_external_all_sources"),
                    value: "",
                  },
                  ...sources.items.map((source) => ({
                    disabled: !source.enabled,
                    label: source.name,
                    value: source.source_id,
                  })),
                ]}
                onChangeCategory={catalog.setActiveCategory}
                onChangeCatalogQuery={catalog.setQuery}
                onChangeDiscoveryMode={setDiscoveryMode}
                onChangeExternalQuery={external.setQuery}
                onChangeExternalSource={external.setSourceId}
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
                  onSelectSource={(sourceId) => external.setSourceId(sourceId || "")}
                  results={external.results}
                  selectedSourceKey={external.sourceId || null}
                  sourceStatuses={external.sourceStatuses}
                  sources={sources.items}
                  submittedQuery={external.submittedQuery}
                />
              )}
              {discoveryMode === "catalog" && (
                <>
                  <SkillsUpdateHighlight
                    busySkillNames={operations.busySkillNames}
                    checkUpdateNotice={operations.checkUpdateNotice}
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
          </CapabilityPageLayout>
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
        onDelete={(source) => void sources.remove(source)}
        onSave={sources.save}
        onToggle={(source, enabled) => void sources.toggle(source, enabled)}
        sources={sources.items}
      />
    </>
  );
}

function buildFeedbackItem(
  feedback: SkillMarketplaceFeedback | null,
  t: ReturnType<typeof useI18n>["t"],
): FeedbackBannerProps | null {
  if (!feedback) return null;
  const titles = {
    error: t("capability.skills_feedback_error"),
    success: t("capability.skills_feedback_success"),
    warning: feedback.pending
      ? t("capability.skills_feedback_processing")
      : t("capability.skills_feedback_partial"),
  } as const;
  const recovery = feedback.pending
    ? {
        impact: t("feedback.processing_impact"),
        nextStep: t("feedback.processing_next_step"),
      }
    : feedback.tone === "warning"
      ? {
          impact: t("feedback.partial_impact"),
          nextStep: t("feedback.partial_next_step"),
        }
      : {
          impact: t("feedback.unconfirmed_impact"),
          nextStep: t("feedback.unconfirmed_next_step"),
        };
  return completeFeedbackBanner(
    feedback.tone === "success"
      ? {
          message: feedback.message ?? feedback.title ?? titles.success,
          onDismiss: feedback.dismiss,
          title: feedback.title ?? titles.success,
          tone: "success",
        }
      : {
          action: feedback.action,
          impact: feedback.impact,
          nextStep: feedback.nextStep,
          onDismiss: feedback.pending || feedback.persistent
            ? undefined
            : feedback.dismiss,
          title: feedback.title ?? titles[feedback.tone],
          tone: feedback.tone,
        },
    { impact: recovery.impact },
  );
}
