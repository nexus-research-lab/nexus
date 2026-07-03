"use client";

import { useCallback } from "react";
import { useNavigate, useParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { useI18n } from "@/shared/i18n/i18n-context";
import { FeedbackBannerStack, type FeedbackBannerItem } from "@/shared/ui/feedback/feedback-banner-stack";
import { WORKSPACE_DETAIL_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

import { useSkillMarketplace } from "@/hooks/capability/use-skill-marketplace";

import { ExternalSkillPreviewDialog } from "./external-skill-preview-dialog";
import { SkillDetailView } from "./skill-detail-view";
import { SkillImportDialog } from "./skill-import-dialog";
import { SkillSourceManagerDialog } from "./skill-source-manager-dialog";
import { SkillsCatalogGrid } from "./skills-catalog-grid";
import { SkillsExternalResults } from "./skills-external-results";
import { SkillsHeader } from "./skills-header";
import { SkillsSearchBar } from "./skills-search-bar";
import { SKILLS_TOUR_ANCHORS } from "./skills-tour";

/* ── Skills 页面主编排组件 ────────────────────── */

interface SkillsDirectoryProps {
  onReplayTour?: () => void;
}

export function SkillsDirectory({ onReplayTour: onReplayTour }: SkillsDirectoryProps) {
  const { t } = useI18n();
  const ctrl = useSkillMarketplace();
  const navigate = useNavigate();
  const { skill_name: skillName } = useParams<{ skill_name?: string }>();
  const openSkillPage = useCallback(
    (name: string) => {
      navigate(AppRouteBuilders.skillDetail(name));
    },
    [navigate],
  );
  const backToSkills = useCallback(() => {
    navigate(AppRouteBuilders.skills());
  }, [navigate]);
  const handleSkillDeleted = useCallback(async () => {
    await ctrl.refreshMarketplace();
    navigate(AppRouteBuilders.skills());
  }, [ctrl, navigate]);

  const feedbackItems: FeedbackBannerItem[] = [];
  if (ctrl.statusMessage) {
    feedbackItems.push({
      key: "status",
      message: ctrl.statusMessage,
      onDismiss: () => ctrl.setStatusMessage(null),
      title: "操作完成",
      tone: "success",
    });
  }
  if (ctrl.errorMessage) {
    feedbackItems.push({
      key: "error",
      message: ctrl.errorMessage,
      onDismiss: () => ctrl.setErrorMessage(null),
      title: "操作失败",
      tone: "error",
    });
  }

  return (
    <>
      {/* 隐藏的文件选择器 */}
      <input
        accept=".zip,application/zip"
        aria-label="导入 Skill 压缩包"
        className="hidden"
        onChange={(e) => {
          const file = e.target.files?.[0];
          if (file) void ctrl.handleLocalImport(file);
          e.currentTarget.value = "";
        }}
        ref={ctrl.fileInputRef}
        type="file"
      />

      <WorkspaceSurfaceScaffold
        bodyScrollable
        header={(
          <div data-tour-anchor={SKILLS_TOUR_ANCHORS.header}>
            <SkillsHeader ctrl={ctrl} onReplayTour={onReplayTour} />
          </div>
        )}
        stableGutter
      >
        {skillName ? (
          <SkillDetailView
            skillName={skillName}
            onBack={backToSkills}
            onDeleted={handleSkillDeleted}
            onRefreshed={ctrl.refreshMarketplace}
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
              <SkillsSearchBar ctrl={ctrl} />
            </div>

            <div data-tour-anchor={SKILLS_TOUR_ANCHORS.catalog}>
              {ctrl.discoveryMode === "external" && <SkillsExternalResults ctrl={ctrl} />}
              {ctrl.discoveryMode === "catalog" && (
                <SkillsCatalogGrid ctrl={ctrl} onOpenSkill={openSkillPage} />
              )}
            </div>
          </div>
        )}
      </WorkspaceSurfaceScaffold>

      <FeedbackBannerStack items={feedbackItems} />

      <SkillImportDialog ctrl={ctrl} />

      <ExternalSkillPreviewDialog
        alreadyImported={
          !!ctrl.previewExternalItem &&
          !!ctrl.importedExternalSources
            .get(ctrl.previewExternalItem.skill_slug)
            ?.has(ctrl.previewExternalItem.package_spec)
        }
        nameConflict={
          !!ctrl.previewExternalItem &&
          !!ctrl.importedExternalSources.get(ctrl.previewExternalItem.skill_slug) &&
          !ctrl.importedExternalSources
            .get(ctrl.previewExternalItem.skill_slug)
            ?.has(ctrl.previewExternalItem.package_spec)
        }
        busy={
          !!ctrl.previewExternalItem &&
          ctrl.busyExternalKey === `${ctrl.previewExternalItem.source_key || ctrl.previewExternalItem.package_spec}@@${ctrl.previewExternalItem.skill_slug}`
        }
        isOpen={!!ctrl.previewExternalItem}
        item={ctrl.previewExternalItem}
        previewLoading={ctrl.externalPreviewLoading}
        onClose={() => ctrl.setPreviewExternalItem(null)}
        onImportOnly={() => {
          if (ctrl.previewExternalItem) void ctrl.handleImportExternal(ctrl.previewExternalItem);
        }}
      />

      <SkillSourceManagerDialog ctrl={ctrl} />
    </>
  );
}
