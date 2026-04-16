/**
 * =====================================================
 * @File   : skills-view-model.ts
 * @Date   : 2026-04-16 13:35
 * @Author : leemysw
 * 2026-04-16 13:35   Create
 * =====================================================
 */

import type { ExternalSkillSearchItem, SkillInfo } from "@/types/capability/skill";
import type { RefObject } from "react";

export type DiscoveryMode = "catalog" | "external";

export interface SkillMarketplaceController {
  skills: SkillInfo[];
  search_query: string;
  discovery_mode: DiscoveryMode;
  active_category: string;
  selected_skill: string | null;
  external_query: string;
  external_results: ExternalSkillSearchItem[];
  preview_external_item: ExternalSkillSearchItem | null;
  external_loading: boolean;
  external_preview_loading: boolean;
  git_prompt_open: boolean;
  loading: boolean;
  busy_skill_name: string | null;
  busy_external_key: string | null;
  status_message: string | null;
  error_message: string | null;
  file_input_ref: RefObject<HTMLInputElement | null>;
  categories: Array<{ key: string; label: string }>;
  visible_skills: SkillInfo[];
  grouped_skills: Array<[string, SkillInfo[]]>;
  catalog_count: number;
  imported_external_sources: Map<string, Set<string>>;
  set_search_query: (value: string) => void;
  set_discovery_mode: (value: DiscoveryMode) => void;
  set_active_category: (value: string) => void;
  set_selected_skill: (value: string | null) => void;
  set_external_query: (value: string) => void;
  set_preview_external_item: (value: ExternalSkillSearchItem | null) => void;
  set_git_prompt_open: (value: boolean) => void;
  set_status_message: (value: string | null) => void;
  set_error_message: (value: string | null) => void;
  refresh_marketplace: () => Promise<void>;
  handle_update_single: (skill_name: string) => Promise<void>;
  handle_delete_skill: (skill: SkillInfo) => Promise<void>;
  handle_update_installed: () => Promise<void>;
  handle_local_import: (file: File) => Promise<void>;
  handle_git_import: (url: string) => Promise<void>;
  handle_preview_external: (item: ExternalSkillSearchItem) => Promise<void>;
  handle_import_external: (item: ExternalSkillSearchItem) => Promise<void>;
}
