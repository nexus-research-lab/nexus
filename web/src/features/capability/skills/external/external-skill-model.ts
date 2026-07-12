import type { ExternalSkillSearchItem } from "@/types/capability/skill";

type ExternalSkillImportKind = "available" | "conflict" | "imported";

export interface ExternalSkillImportModel {
  busy: boolean;
  canImport: boolean;
  kind: ExternalSkillImportKind;
  label: string;
  tone: "default" | "success" | "warning";
}

export interface ExternalSkillListItemModel {
  description: string;
  importState: ExternalSkillImportModel;
  installLabel: string;
  sourceLabel: string;
  sourceReference: string;
  title: string;
}

export interface ExternalSkillPreviewModel {
  detailUrl: string;
  importState: ExternalSkillImportModel;
  item: ExternalSkillSearchItem;
  markdown: string;
  sourceLabel: string;
  subtitle: string;
  title: string;
}

const IMPORT_PRESENTATIONS: Record<
  ExternalSkillImportKind,
  Omit<ExternalSkillImportModel, "busy">
> = {
  available: {
    canImport: true,
    kind: "available",
    label: "可导入",
    tone: "default",
  },
  conflict: {
    canImport: false,
    kind: "conflict",
    label: "同名冲突",
    tone: "warning",
  },
  imported: {
    canImport: false,
    kind: "imported",
    label: "已导入",
    tone: "success",
  },
};

export function externalSkillKey(item: ExternalSkillSearchItem): string {
  return `${item.source_key || item.package_spec}@@${item.skill_slug}`;
}

export function isExternalSkillPreviewUnavailable(
  item: ExternalSkillSearchItem,
): boolean {
  return item.source_kind === "skills_sh" || item.import_mode === "skills_sh";
}

export function buildExternalSkillListItemModel(
  item: ExternalSkillSearchItem,
  importedSources: Map<string, Set<string>>,
  busyKeys: ReadonlySet<string>,
): ExternalSkillListItemModel {
  return {
    description: item.description || item.readme_markdown || "暂无描述",
    importState: buildExternalSkillImportModel(item, importedSources, busyKeys),
    installLabel: `${formatInstallCount(item.installs)} 次安装`,
    sourceLabel: externalSkillSourceLabel(item),
    sourceReference: externalSkillSourceReference(item),
    title: item.title || item.skill_slug,
  };
}

export function buildExternalSkillPreviewModel(
  item: ExternalSkillSearchItem | null,
  importedSources: Map<string, Set<string>>,
  busyKeys: ReadonlySet<string>,
  loading: boolean,
): ExternalSkillPreviewModel | null {
  if (!item) return null;
  const listItem = buildExternalSkillListItemModel(item, importedSources, busyKeys);
  return {
    detailUrl: item.detail_url,
    importState: listItem.importState,
    item,
    markdown: buildPreviewMarkdown(item, loading),
    sourceLabel: listItem.sourceLabel,
    subtitle: `${listItem.sourceReference} · ${listItem.installLabel}`,
    title: listItem.title,
  };
}

function buildExternalSkillImportModel(
  item: ExternalSkillSearchItem,
  importedSources: Map<string, Set<string>>,
  busyKeys: ReadonlySet<string>,
): ExternalSkillImportModel {
  const kind = resolveExternalSkillImportKind(item, importedSources);
  return {
    ...IMPORT_PRESENTATIONS[kind],
    busy: busyKeys.has(externalSkillKey(item)),
  };
}

function resolveExternalSkillImportKind(
  item: ExternalSkillSearchItem,
  importedSources: Map<string, Set<string>>,
): ExternalSkillImportKind {
  const sources = importedSources.get(item.skill_slug);
  if (!sources) return "available";
  return sources.has(item.package_spec) ? "imported" : "conflict";
}

function buildPreviewMarkdown(
  item: ExternalSkillSearchItem,
  loading: boolean,
): string {
  if (loading && !item.readme_markdown) return "正在加载预览内容...";
  if (isExternalSkillPreviewUnavailable(item)) {
    return "skills.sh 暂不提供内置预览，请打开原始页面查看。";
  }
  return item.readme_markdown || item.description || "暂无预览内容";
}

function externalSkillSourceLabel(item: ExternalSkillSearchItem): string {
  return item.source_name || item.source_kind || "社区";
}

function externalSkillSourceReference(item: ExternalSkillSearchItem): string {
  return item.package_spec || item.git_url || item.raw_url || item.source;
}

function formatInstallCount(count: number): string {
  if (count < 1000) return `${count}`;
  return `${(count / 1000).toFixed(count >= 100000 ? 0 : 1)}K`;
}
