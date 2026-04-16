"use client";

import { PromptDialog } from "@/shared/ui/dialog/confirm-dialog";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

import { useSkillMarketplace } from "@/hooks/capability/use-skill-marketplace";

import { ExternalSkillPreviewDialog } from "./external-skill-preview-dialog";
import { FeedbackBanner } from "./feedback-banner";
import { SkillDetailDialog } from "./skill-detail-dialog";
import { SkillsCatalogGrid } from "./skills-catalog-grid";
import { SkillsExternalResults } from "./skills-external-results";
import { SkillsHeader } from "./skills-header";
import { SkillsSearchBar } from "./skills-search-bar";

/* ── Skills 页面主编排组件 ────────────────────── */

export function SkillsDirectory() {
  const ctrl = useSkillMarketplace();

  return (
    <>
      {/* 隐藏的文件选择器 */}
      <input
        accept=".zip,application/zip"
        className="hidden"
        onChange={(e) => {
          const file = e.target.files?.[0];
          if (file) void ctrl.handle_local_import(file);
          e.currentTarget.value = "";
        }}
        ref={ctrl.file_input_ref}
        type="file"
      />

      <WorkspaceSurfaceScaffold
        body_class_name="px-5 py-5 xl:px-6"
        body_scrollable
        header={<SkillsHeader ctrl={ctrl} />}
        stable_gutter
      >
        <SkillsSearchBar ctrl={ctrl} />

        {ctrl.discovery_mode === "external" && <SkillsExternalResults ctrl={ctrl} />}
        {ctrl.discovery_mode === "catalog" && <SkillsCatalogGrid ctrl={ctrl} />}
      </WorkspaceSurfaceScaffold>

      {(ctrl.status_message || ctrl.error_message) && (
        <div className="pointer-events-none fixed right-6 top-24 z-40 flex flex-col gap-2">
          {ctrl.status_message && (
            <FeedbackBanner
              message={ctrl.status_message}
              on_dismiss={() => ctrl.set_status_message(null)}
              title="操作完成"
              tone="success"
            />
          )}
          {ctrl.error_message && (
            <FeedbackBanner
              message={ctrl.error_message}
              on_dismiss={() => ctrl.set_error_message(null)}
              title="操作失败"
              tone="error"
            />
          )}
        </div>
      )}

      {/* 弹窗 */}
      {ctrl.selected_skill && (
        <SkillDetailDialog
          is_open={!!ctrl.selected_skill}
          on_close={() => ctrl.set_selected_skill(null)}
          on_refresh={ctrl.refresh_marketplace}
          skill_name={ctrl.selected_skill}
        />
      )}

      <PromptDialog
        default_value=""
        is_open={ctrl.git_prompt_open}
        message="输入包含 SKILL.md 的 Git 仓库地址"
        on_cancel={() => ctrl.set_git_prompt_open(false)}
        on_confirm={(value) => void ctrl.handle_git_import(value)}
        placeholder="https://github.com/owner/repo.git"
        title="通过 Git 导入"
      />

      <ExternalSkillPreviewDialog
        already_imported={
          !!ctrl.preview_external_item &&
          !!ctrl.imported_external_sources
            .get(ctrl.preview_external_item.skill_slug)
            ?.has(ctrl.preview_external_item.package_spec)
        }
        name_conflict={
          !!ctrl.preview_external_item &&
          !!ctrl.imported_external_sources.get(ctrl.preview_external_item.skill_slug) &&
          !ctrl.imported_external_sources
            .get(ctrl.preview_external_item.skill_slug)
            ?.has(ctrl.preview_external_item.package_spec)
        }
        busy={
          !!ctrl.preview_external_item &&
          ctrl.busy_external_key === `${ctrl.preview_external_item.package_spec}@@${ctrl.preview_external_item.skill_slug}`
        }
        is_open={!!ctrl.preview_external_item}
        item={ctrl.preview_external_item}
        preview_loading={ctrl.external_preview_loading}
        on_close={() => ctrl.set_preview_external_item(null)}
        on_import_only={() => {
          if (ctrl.preview_external_item) void ctrl.handle_import_external(ctrl.preview_external_item);
        }}
      />
    </>
  );
}
