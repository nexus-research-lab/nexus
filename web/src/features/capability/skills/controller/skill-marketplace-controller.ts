import type { RefObject } from "react";

import type {
  ExternalSkillSearchItem,
  ExternalSkillSourceInfo,
  ExternalSkillSourceStatus,
  SkillInfo,
} from "@/types/capability/skill";

export type DiscoveryMode = "catalog" | "external";
export type SkillImportDialogMode = "local" | "git";
export type SkillMarketplaceFeedbackTone = "error" | "success" | "warning";

export interface SkillMarketplaceFeedback {
  dismiss: () => void;
  message: string;
  pending: boolean;
  tone: SkillMarketplaceFeedbackTone;
}

export interface SkillMarketplaceFeedbackActions {
  clear: () => void;
  error: (message: string) => void;
  start: (message: string) => void;
  success: (message: string) => void;
  warning: (message: string) => void;
}

export interface SkillCatalogController {
  activeCategory: string;
  catalogCount: number;
  categories: Array<{ key: string; label: string }>;
  groupedSkills: Array<[string, SkillInfo[]]>;
  importedExternalSources: Map<string, Set<string>>;
  loading: boolean;
  query: string;
  refresh: () => Promise<void>;
  setActiveCategory: (category: string) => void;
  setQuery: (query: string) => void;
  skills: SkillInfo[];
  updateAvailableSkills: SkillInfo[];
}

export interface ExternalSkillSearchController {
  closePreview: () => void;
  loading: boolean;
  preview: (item: ExternalSkillSearchItem) => Promise<void>;
  previewItem: ExternalSkillSearchItem | null;
  previewLoading: boolean;
  query: string;
  results: ExternalSkillSearchItem[];
  setQuery: (query: string) => void;
  sourceStatuses: ExternalSkillSourceStatus[];
  submit: () => void;
  submittedQuery: string;
}

export interface ExternalSkillSourcesController {
  closeManager: () => void;
  items: ExternalSkillSourceInfo[];
  loading: boolean;
  managerOpen: boolean;
  openManager: () => void;
  revision: number;
  toggle: (source: ExternalSkillSourceInfo, enabled: boolean) => Promise<void>;
}

export interface SkillOperationsController {
  busyExternalKeys: ReadonlySet<string>;
  busySkillNames: ReadonlySet<string>;
  checkUpdateMessage: string | null;
  checkUpdates: () => Promise<void>;
  checkingUpdates: boolean;
  deleteSkill: (skill: SkillInfo) => Promise<boolean>;
  fileInputRef: RefObject<HTMLInputElement | null>;
  importDialogMode: SkillImportDialogMode | null;
  importExternal: (item: ExternalSkillSearchItem) => Promise<void>;
  importGit: (url: string, branch?: string, path?: string) => Promise<void>;
  importLocal: (file: File) => Promise<void>;
  importing: boolean;
  lastUpdateCheckedAt: number | null;
  setImportDialogMode: (mode: SkillImportDialogMode | null) => void;
  updateSkill: (skillName: string) => Promise<boolean>;
}

export interface SkillMarketplaceController {
  catalog: SkillCatalogController;
  discoveryMode: DiscoveryMode;
  external: ExternalSkillSearchController;
  feedback: SkillMarketplaceFeedback | null;
  operations: SkillOperationsController;
  setDiscoveryMode: (mode: DiscoveryMode) => void;
  sources: ExternalSkillSourcesController;
}
