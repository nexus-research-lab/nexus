export interface LoopStep {
  name: string;
  name_zh?: string;
  prompt: string;
  prompt_zh?: string;
  shell_check?: string;
}

export interface LoopExitCondition {
  type: string;
  command?: string;
  description: string;
  description_zh?: string;
  max_iterations?: number;
}

export interface LoopExample {
  title: string;
  title_zh?: string;
  summary: string;
  summary_zh?: string;
}

export interface LoopCatalogItem {
  id: string;
  slug: string;
  title: string;
  title_zh?: string;
  description: string;
  description_zh?: string;
  category: string;
  category_zh?: string;
  trigger_type: string;
  trigger_config: Record<string, unknown>;
  steps: LoopStep[];
  exit_condition: LoopExitCondition;
  kickoff_prompt: string;
  install_bundle: Record<string, unknown>;
  compatible_agents: string[];
  best_for_agents: string[];
  author: string;
  author_slug: string;
  author_official: boolean;
  source: string;
  tags: string[];
  guardrails: string[];
  guardrails_zh?: string[];
  examples: LoopExample[];
  copies: number;
  installs: number;
  views: number;
  featured: boolean;
  is_published: boolean;
  created_at: string;
}
