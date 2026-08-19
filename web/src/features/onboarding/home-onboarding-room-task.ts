/**
 * [INPUT]: 产品经理引导中收集的产品想法、已创建 Agent 和真实 Room 上下文。
 * [OUTPUT]: Room 任务草稿、协作成员定义、主持人开场脚本与跨页面恢复信息。
 * [POS]: 首页引导下半程“创建 Room 并完成协作”的状态真相源。
 */

import { AppRouteBuilders } from "@/app/router/route-paths";
import { getInitialAgentOptions } from "@/config/runtime-options";
import type { CreateAgentParams } from "@/types/agent/agent";

export const HOME_ONBOARDING_ROOM_TASK_STORAGE_KEY =
  "nexus.home_onboarding_room_task";

export type HomeOnboardingRoomTaskPhase =
  | "idle"
  | "plan"
  | "ready"
  | "review";

export interface HomeOnboardingRoomTaskDraft {
  conversationId: string;
  hostAgentId: string;
  hostAgentName: string;
  phase: HomeOnboardingRoomTaskPhase;
  productIdea: string;
  productManagerAgentId: string;
  productManagerAgentName: string;
  researcherAgentId: string;
  researcherAgentName: string;
  roomId: string;
  roomName: string;
  technicalReviewerAgentId: string;
  technicalReviewerAgentName: string;
}

export interface HomeOnboardingRoomSupportAgentDefinition {
  description: string;
  name: string;
  roleLabel: string;
  vibeTags: string[];
}

export const PRODUCT_MANAGER_ROOM_NAME = "产品评审室";
export const PRODUCT_MANAGER_ROOM_AVATAR = "24";

export const PRODUCT_MANAGER_ROOM_SUPPORT_AGENTS = {
  researcher: {
    description:
      "从目标用户、使用场景、痛点证据和行为动机出发，评审产品想法并指出需要继续验证的假设。",
    name: "用户研究顾问",
    roleLabel: "用户研究",
    vibeTags: ["用户洞察", "证据意识", "善于追问"],
  },
  technicalReviewer: {
    description:
      "从技术可行性、系统依赖、交付成本和实现风险出发，评审产品方案并给出可落地建议。",
    name: "技术可行性顾问",
    roleLabel: "技术评审",
    vibeTags: ["技术可行性", "风险意识", "工程实践"],
  },
} satisfies Record<string, HomeOnboardingRoomSupportAgentDefinition>;

export function createHomeOnboardingRoomTaskDraft(
  productManagerAgentId: string,
  productManagerAgentName: string,
  hostAgentId = "",
  hostAgentName = "Nexus",
): HomeOnboardingRoomTaskDraft {
  return {
    conversationId: "",
    hostAgentId: hostAgentId.trim(),
    hostAgentName: hostAgentName.trim() || "Nexus",
    phase: "idle",
    productIdea: "",
    productManagerAgentId: productManagerAgentId.trim(),
    productManagerAgentName: productManagerAgentName.trim(),
    researcherAgentId: "",
    researcherAgentName: PRODUCT_MANAGER_ROOM_SUPPORT_AGENTS.researcher.name,
    roomId: "",
    roomName: PRODUCT_MANAGER_ROOM_NAME,
    technicalReviewerAgentId: "",
    technicalReviewerAgentName:
      PRODUCT_MANAGER_ROOM_SUPPORT_AGENTS.technicalReviewer.name,
  };
}

export function getHomeOnboardingRoomTaskDraft(
  productManagerAgentId = "",
  productManagerAgentName = "",
  hostAgentId = "",
  hostAgentName = "Nexus",
): HomeOnboardingRoomTaskDraft {
  const fallback = createHomeOnboardingRoomTaskDraft(
    productManagerAgentId,
    productManagerAgentName,
    hostAgentId,
    hostAgentName,
  );
  if (typeof window === "undefined") {
    return fallback;
  }
  const stored = window.localStorage.getItem(
    HOME_ONBOARDING_ROOM_TASK_STORAGE_KEY,
  );
  if (!stored) {
    return fallback;
  }
  try {
    const parsed = JSON.parse(stored) as Partial<HomeOnboardingRoomTaskDraft>;
    return {
      conversationId: normalizedText(parsed.conversationId),
      hostAgentId:
        normalizedText(parsed.hostAgentId) || fallback.hostAgentId,
      hostAgentName:
        normalizedText(parsed.hostAgentName) || fallback.hostAgentName,
      phase: normalizeRoomTaskPhase(parsed.phase),
      productIdea: normalizedText(parsed.productIdea),
      productManagerAgentId:
        normalizedText(parsed.productManagerAgentId)
        || fallback.productManagerAgentId,
      productManagerAgentName:
        normalizedText(parsed.productManagerAgentName)
        || fallback.productManagerAgentName,
      researcherAgentId: normalizedText(parsed.researcherAgentId),
      researcherAgentName:
        normalizedText(parsed.researcherAgentName)
        || fallback.researcherAgentName,
      roomId: normalizedText(parsed.roomId),
      roomName: normalizedText(parsed.roomName) || fallback.roomName,
      technicalReviewerAgentId:
        normalizedText(parsed.technicalReviewerAgentId),
      technicalReviewerAgentName:
        normalizedText(parsed.technicalReviewerAgentName)
        || fallback.technicalReviewerAgentName,
    };
  } catch {
    return fallback;
  }
}

export function setHomeOnboardingRoomTaskDraft(
  draft: HomeOnboardingRoomTaskDraft,
): void {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(
    HOME_ONBOARDING_ROOM_TASK_STORAGE_KEY,
    JSON.stringify(draft),
  );
  window.dispatchEvent(new Event("nexus:home-onboarding-room-task-change"));
}

export function clearHomeOnboardingRoomTaskDraft(): void {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.removeItem(HOME_ONBOARDING_ROOM_TASK_STORAGE_KEY);
  window.dispatchEvent(new Event("nexus:home-onboarding-room-task-change"));
}

export function buildHomeOnboardingSupportAgentCreateParams(
  definition: HomeOnboardingRoomSupportAgentDefinition,
): CreateAgentParams {
  return {
    description: definition.description,
    name: definition.name,
    options: getInitialAgentOptions(),
    vibe_tags: definition.vibeTags,
  };
}

export function buildProductManagerRoomReviewPrompt(
  draft: HomeOnboardingRoomTaskDraft,
): string {
  const hostAgentName = draft.hostAgentName || "Nexus";
  return [
    `@${draft.researcherAgentName} @${draft.technicalReviewerAgentName}`,
    "我们正在进行一次跨角色产品评审。请围绕下面的产品想法分别完成你负责的评审，不要替另一个角色作答。",
    "",
    `产品想法：${draft.productIdea}`,
    "",
    `- ${draft.researcherAgentName}：从目标用户、核心场景、痛点证据和待验证假设出发，给出用户研究意见。`,
    `- ${draft.technicalReviewerAgentName}：从技术可行性、关键依赖、交付成本和风险出发，给出技术评审意见。`,
    "",
    "请输出明确结论、关键风险和建议的下一步。",
    "",
    `两位顾问完成各自评审后，回复末尾必须写“\`@${hostAgentName}\` 请检查本次 Room 的两份评审；若均已完成，请立即邀请产品经理进行最终收敛。”；实际输出时不要把这个 mention 放进代码格式。`,
    `${hostAgentName}：每次收到顾问交回的评审，都只以本次 Room 的公开消息为准。确认两份评审均已完成后，你的公开回复第一行必须是“\`@${draft.productManagerAgentName}\` 请结合本次 Room 的两份真实评审，完成最终方案收敛。”，以真实 @mention 唤醒产品经理；随后只需补充交接要求，不要替产品经理输出最终方案。若只收到一份评审，请等待另一份，不要提前总结。实际输出时不要使用代码格式。`,
    `${draft.productManagerAgentName}：被主持人邀请后，请结合两位顾问的真实输出，收敛最终方案、关键风险、优先级、下一步行动和验收标准。最终结论不要再 @ 其他成员。`,
  ].join("\n");
}

export function buildHomeOnboardingRoomLaunchRoute(
  draft: HomeOnboardingRoomTaskDraft,
  includeInitialPrompt: boolean,
): string {
  if (!draft.roomId || !draft.conversationId) {
    return AppRouteBuilders.home();
  }
  const route = AppRouteBuilders.roomConversation(
    draft.roomId,
    draft.conversationId,
  );
  if (!includeInitialPrompt) {
    return route;
  }
  const params = new URLSearchParams({
    initial: buildProductManagerRoomReviewPrompt(draft),
    scripted_host_message: "1",
  });
  return `${route}?${params.toString()}`;
}

function normalizeRoomTaskPhase(
  value: unknown,
): HomeOnboardingRoomTaskPhase {
  if (
    value === "plan"
    || value === "ready"
    || value === "review"
  ) {
    return value;
  }
  return "idle";
}

function normalizedText(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}
