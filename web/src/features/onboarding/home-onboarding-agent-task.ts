/**
 * [INPUT]: 用户选择的角色、对话中收集到的 Agent 草案与浏览器恢复状态。
 * [OUTPUT]: 角色化 Agent 创建建议、草案读写与创建完成恢复信息。
 * [POS]: 首页引导下半程“创建专属 Agent”任务的状态真相源。
 */

import type { CreateAgentParams } from "@/types/agent/agent";

import { getInitialAgentOptions } from "@/config/runtime-options";

export const HOME_ONBOARDING_AGENT_TASK_STORAGE_KEY =
  "nexus.home_onboarding_agent_task";

export interface HomeOnboardingAgentStyleChoice {
  description: string;
  label: string;
  tags: string[];
}

export interface HomeOnboardingAgentTaskProfile {
  agentType: string;
  descriptionExample: string;
  roomName: string;
  styleChoices: HomeOnboardingAgentStyleChoice[];
  suggestedName: string;
}

export interface HomeOnboardingAgentTaskDraft {
  createdAgentId: string;
  description: string;
  name: string;
  role: string;
  vibeTags: string[];
}

const ROLE_PROFILES: Record<string, HomeOnboardingAgentTaskProfile> = {
  学生: {
    agentType: "课程学习助教",
    descriptionExample: "例如：帮我拆解课程资料，整理知识框架、复习计划和练习题。",
    roomName: "小组作业协作室",
    styleChoices: [
      {
        description: "把复杂知识拆成容易掌握的小步骤",
        label: "循序渐进",
        tags: ["循序渐进", "善于讲解", "耐心"],
      },
      {
        description: "围绕考试和作业产出清晰的学习计划",
        label: "目标导向",
        tags: ["目标导向", "结构化", "注重反馈"],
      },
    ],
    suggestedName: "课程学习助教",
  },
  产品经理: {
    agentType: "产品经理智能体",
    descriptionExample:
      "例如：帮我澄清用户问题，把产品想法整理成需求范围、优先级和验收标准。",
    roomName: "产品评审室",
    styleChoices: [
      {
        description: "先澄清用户问题，再拆解目标、范围和验收标准",
        label: "用户与需求导向",
        tags: ["用户视角", "结构化分析", "善于追问"],
      },
      {
        description: "快速收敛优先级、风险和可执行的下一步",
        label: "结果与推进导向",
        tags: ["结果导向", "优先级意识", "跨团队协作"],
      },
      {
        description: "从用户、业务与研发多个视角审视产品方案",
        label: "评审与决策导向",
        tags: ["多视角评审", "逻辑严谨", "风险意识"],
      },
    ],
    suggestedName: "需求分析助手",
  },
  研发: {
    agentType: "代码分析助手",
    descriptionExample:
      "例如：帮我理解代码、拆解技术方案，并输出问题排查与验证路径。",
    roomName: "Bug 排查战情室",
    styleChoices: [
      {
        description: "先建立证据链，再给出可以逐项验证的排查路径",
        label: "严谨排查",
        tags: ["证据优先", "逻辑严谨", "善于验证"],
      },
      {
        description: "兼顾实现效率、代码质量和长期维护成本",
        label: "工程实践",
        tags: ["工程化", "务实", "风险意识"],
      },
    ],
    suggestedName: "代码分析助手",
  },
  运营: {
    agentType: "活动运营助手",
    descriptionExample:
      "例如：根据目标和人群策划活动主题、渠道节奏、素材清单与复盘指标。",
    roomName: "活动作战室",
    styleChoices: [
      {
        description: "围绕目标人群设计内容、渠道和触达节奏",
        label: "用户增长导向",
        tags: ["用户洞察", "增长意识", "节奏感"],
      },
      {
        description: "快速形成可以直接推进的执行清单和复盘指标",
        label: "执行复盘导向",
        tags: ["执行力", "数据意识", "善于复盘"],
      },
    ],
    suggestedName: "活动运营助手",
  },
  解决方案及售前: {
    agentType: "解决方案助手",
    descriptionExample:
      "例如：根据客户背景梳理需求澄清问题、方案框架和交付风险。",
    roomName: "客户方案评审室",
    styleChoices: [
      {
        description: "先理解客户场景，再匹配价值主张与解决方案",
        label: "客户价值导向",
        tags: ["客户视角", "价值导向", "善于澄清"],
      },
      {
        description: "兼顾方案竞争力、可交付性与项目风险",
        label: "方案落地导向",
        tags: ["方案思维", "交付意识", "风险意识"],
      },
    ],
    suggestedName: "解决方案助手",
  },
};

const FALLBACK_PROFILE: HomeOnboardingAgentTaskProfile = {
  agentType: "专属智能体",
  descriptionExample: "例如：说明你希望它重点帮你处理的工作和最终产出。",
  roomName: "协作 Room",
  styleChoices: [
    {
      description: "把复杂任务拆解成清晰、可执行的步骤",
      label: "结构化推进",
      tags: ["结构化", "结果导向", "善于协作"],
    },
  ],
  suggestedName: "我的专属助手",
};

export function getHomeOnboardingAgentTaskProfile(
  role: string,
): HomeOnboardingAgentTaskProfile {
  return ROLE_PROFILES[role.trim()] ?? FALLBACK_PROFILE;
}

export function createHomeOnboardingAgentTaskDraft(
  role: string,
): HomeOnboardingAgentTaskDraft {
  return {
    createdAgentId: "",
    description: "",
    name: "",
    role: role.trim(),
    vibeTags: [],
  };
}

export function getHomeOnboardingAgentTaskDraft(
  role = "",
): HomeOnboardingAgentTaskDraft {
  const fallback = createHomeOnboardingAgentTaskDraft(role);
  if (typeof window === "undefined") {
    return fallback;
  }
  const stored = window.localStorage.getItem(
    HOME_ONBOARDING_AGENT_TASK_STORAGE_KEY,
  );
  if (!stored) {
    return fallback;
  }
  try {
    const parsed = JSON.parse(stored) as Partial<HomeOnboardingAgentTaskDraft>;
    return {
      createdAgentId: normalizedText(parsed.createdAgentId),
      description: normalizedText(parsed.description),
      name: normalizedText(parsed.name),
      role: normalizedText(parsed.role) || fallback.role,
      vibeTags: Array.isArray(parsed.vibeTags)
        ? parsed.vibeTags.map(normalizedText).filter(Boolean)
        : [],
    };
  } catch {
    return fallback;
  }
}

export function setHomeOnboardingAgentTaskDraft(
  draft: HomeOnboardingAgentTaskDraft,
): void {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(
    HOME_ONBOARDING_AGENT_TASK_STORAGE_KEY,
    JSON.stringify(draft),
  );
}

export function clearHomeOnboardingAgentTaskDraft(): void {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.removeItem(HOME_ONBOARDING_AGENT_TASK_STORAGE_KEY);
}

export function buildHomeOnboardingAgentCreateParams(
  draft: HomeOnboardingAgentTaskDraft,
): CreateAgentParams {
  return {
    name: draft.name.trim(),
    options: getInitialAgentOptions(),
    description: draft.description.trim(),
    vibe_tags: draft.vibeTags,
  };
}

function normalizedText(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}
