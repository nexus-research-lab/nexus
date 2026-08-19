/**
 * [INPUT]: Nexus DM 会话、模型配置、角色选择及 Agent/Room 创建命令。
 * [OUTPUT]: 可恢复的新手引导状态、固定引导消息与真实资源创建动作。
 * [POS]: Nexus DM 新手引导的主控制器。
 */

/**
 * INPUT: DM 会话、引导持久状态、模型配置与 Agent/Room 写命令。
 * OUTPUT: Nexus 主页引导的消息投影、交互动作和真实模型交接控制器。
 * POS: DM 面板的新手引导编排边界；模型配置前走固定流程，角色选择后交给 Skill。
 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  HOME_ONBOARDING_CONNECTING_MESSAGE,
  HOME_ONBOARDING_INVALID_TOKEN_MESSAGE,
  HOME_ONBOARDING_ROUND_ID,
  HOME_ONBOARDING_TOKEN_STORAGE_KEY,
  HOME_ONBOARDING_VALID_TOKEN_MESSAGE,
  HOME_ONBOARDING_WELCOME_MESSAGE,
  buildHomeOnboardingTokenPrompt,
  clearHomeOnboardingDefaultModelConfirmed,
  clearHomeOnboardingProviderPresetKey,
  completeHomeOnboarding,
  getHomeOnboardingProviderPresetKey,
  getHomeOnboardingRole,
  getHomeOnboardingStage,
  isHomeOnboardingDefaultModelConfirmed,
  maskHomeOnboardingToken,
  rememberHomeOnboardingReturnPath,
  setHomeOnboardingProviderPresetKey,
  setHomeOnboardingDefaultModelConfirmed,
  setHomeOnboardingRole,
  setHomeOnboardingStage,
  type HomeOnboardingStage,
} from "@/features/onboarding/home-agent-onboarding";
import {
  buildHomeOnboardingAgentCreateParams,
  createHomeOnboardingAgentTaskDraft,
  getHomeOnboardingAgentTaskDraft,
  getHomeOnboardingAgentTaskProfile,
  setHomeOnboardingAgentTaskDraft,
  type HomeOnboardingAgentStyleChoice,
  type HomeOnboardingAgentTaskDraft,
} from "@/features/onboarding/home-onboarding-agent-task";
import {
  PRODUCT_MANAGER_ROOM_AVATAR,
  PRODUCT_MANAGER_ROOM_SUPPORT_AGENTS,
  buildHomeOnboardingSupportAgentCreateParams,
  createHomeOnboardingRoomTaskDraft,
  getHomeOnboardingRoomTaskDraft,
  setHomeOnboardingRoomTaskDraft,
  type HomeOnboardingRoomSupportAgentDefinition,
  type HomeOnboardingRoomTaskDraft,
} from "@/features/onboarding/home-onboarding-room-task";
import {
  configureHomeOnboardingProvider,
  listHomeOnboardingProviderChoices,
  resolveHomeOnboardingModelOptions,
  setHomeOnboardingDefaultModel,
  type HomeOnboardingProviderChoice,
} from "@/features/onboarding/home-onboarding-provider";
import { validateAgentNameApi } from "@/lib/api/agent/agent-api";
import { createRoom } from "@/lib/api/conversation/room-command-api";
import { getErrorMessage } from "@/lib/error-message";
import { useAgentStore } from "@/store/agent";
import type { ProviderModelSelection } from "@/types/capability/provider";
import type {
  AgentConversationSendOptions,
} from "@/types/agent/agent-conversation";
import type {
  AssistantMessage,
  Message,
  UserMessage,
} from "@/types/conversation/message/entity";

interface UseDmOnboardingControllerOptions {
  agentId: string | null;
  agentName: string | null;
  conversationId: string | null;
  enabled: boolean;
  roomId: string | null;
  sendMessage: (
    content: string,
    options?: AgentConversationSendOptions,
  ) => Promise<void>;
  sessionKey: string | null;
}

export interface DmOnboardingControllerModel {
  agentStyleChoices: HomeOnboardingAgentStyleChoice[];
  agentTaskDraft: HomeOnboardingAgentTaskDraft;
  isActive: boolean;
  isCreatingAgent: boolean;
  isCreatingRoom: boolean;
  messages: Message[];
  defaultModelChoices: ProviderModelSelection[];
  providerChoices: HomeOnboardingProviderChoice[];
  providerChoicesError: string | null;
  providerChoicesLoading: boolean;
  roomTaskDraft: HomeOnboardingRoomTaskDraft;
  showDefaultModelCard: boolean;
  showAgentConfirmationCard: boolean;
  showAgentIdentityCard: boolean;
  showAgentStyleCard: boolean;
  showProviderConfigCard: boolean;
  showProviderSelectionCard: boolean;
  showRoleCard: boolean;
  showRoomCompletionCard: boolean;
  showRoomLaunchCard: boolean;
  showRoomPlanCard: boolean;
  showRoomStartCard: boolean;
  defaultModelSelection: ProviderModelSelection | null;
  retryProviderChoices: () => void;
  selectDefaultModel: (selection: ProviderModelSelection) => void;
  selectProvider: (choice: HomeOnboardingProviderChoice) => void;
  confirmDefaultModel: () => Promise<void>;
  confirmAgentCreation: () => Promise<void>;
  confirmRoomPlan: () => Promise<void>;
  finishOnboarding: () => void;
  launchRoomCollaboration: () => void;
  restartAgentDraft: () => void;
  restartRoomIdea: () => void;
  startRoomTask: () => void;
  selectAgentStyle: (choice: HomeOnboardingAgentStyleChoice) => void;
  selectRole: (role: string) => void;
  sendMessage: (
    content: string,
    options?: AgentConversationSendOptions,
  ) => Promise<void>;
}

function createMessageBase(
  agentId: string | null,
  conversationId: string | null,
  roomId: string | null,
  sessionKey: string | null,
  messageId: string,
) {
  return {
    agent_id: agentId ?? "nexus",
    conversation_id: conversationId,
    delivery_mode: "ephemeral" as const,
    message_id: messageId,
    room_id: roomId,
    round_id: HOME_ONBOARDING_ROUND_ID,
    session_key: sessionKey ?? "nexus-home-onboarding",
    timestamp: Date.now(),
  };
}

function createAssistantMessage(
  text: string,
  options: UseDmOnboardingControllerOptions,
): AssistantMessage {
  return {
    ...createMessageBase(
      options.agentId,
      options.conversationId,
      options.roomId,
      options.sessionKey,
      `nexus-onboarding-assistant-${Date.now()}-${Math.random()}`,
    ),
    content: [{ type: "text", text }],
    is_complete: true,
    role: "assistant",
    stream_status: "done",
  };
}

function createUserMessage(
  content: string,
  options: UseDmOnboardingControllerOptions,
): UserMessage {
  return {
    ...createMessageBase(
      options.agentId,
      options.conversationId,
      options.roomId,
      options.sessionKey,
      `nexus-onboarding-user-${Date.now()}-${Math.random()}`,
    ),
    content,
    role: "user",
  };
}

function buildInitialMessages(
  options: UseDmOnboardingControllerOptions,
  stage: HomeOnboardingStage,
): Message[] {
  if (!options.enabled) {
    return [];
  }
  if (stage === "skill_guided") {
    return [];
  }
  if (stage === "provider_configuration") {
    return [
      createAssistantMessage(
        "欢迎回来。我正在检查你刚刚完成的模型配置，请稍等一下。",
        options,
      ),
    ];
  }
  if (stage === "default_model") {
    return [
      createAssistantMessage(
        "我正在确认当前的模型配置，请稍等一下。",
        options,
      ),
    ];
  }
  if (stage === "role") {
    return [
      createAssistantMessage(
        "模型已设为默认对话模型。下一步我们选择一下你的角色。",
        options,
      ),
    ];
  }
  const role = getHomeOnboardingRole() ?? "";
  const agentDraft = getHomeOnboardingAgentTaskDraft(role);
  if (stage === "agent_name") {
    return [createAssistantMessage(buildAgentNamePrompt(role), options)];
  }
  if (stage === "agent_description") {
    return [
      createAssistantMessage(
        buildAgentDescriptionPrompt(agentDraft.name, role),
        options,
      ),
    ];
  }
  if (stage === "agent_style") {
    return [
      createAssistantMessage(
        `很好，我已经记下 ${agentDraft.name || "这个智能体"} 的核心职责。现在选择一种工作风格，我会把它转换成 Agent 的身份标签。`,
        options,
      ),
    ];
  }
  if (stage === "agent_confirm") {
    return [
      createAssistantMessage(
        "Agent 草案已经准备好了。请检查下面的配置，确认后我会调用 Nexus 的真实创建接口。",
        options,
      ),
    ];
  }
  if (stage === "agent_created") {
    return [
      createAssistantMessage(
        `${agentDraft.name || "你的专属 Agent"} 已创建成功。点击身份卡片可以进入它在 Nexus 中的真实 Agent 页面。`,
        options,
      ),
    ];
  }
  const roomDraft = getHomeOnboardingRoomTaskDraft(
    agentDraft.createdAgentId,
    agentDraft.name,
    options.agentId ?? "",
    options.agentName ?? "Nexus",
  );
  if (stage === "room_idea") {
    return [
      createAssistantMessage(
        "现在开始任务 2：创建一个产品评审 Room。请用 1～3 句话描述一个你真实想评审的产品想法或需求，我会据此组织协作任务。",
        options,
      ),
    ];
  }
  if (stage === "room_plan") {
    return [
      createAssistantMessage(
        `我已把「${roomDraft.productIdea}」整理成跨角色产品评审方案。请检查下面的 Room 成员和执行步骤。`,
        options,
      ),
    ];
  }
  if (stage === "room_ready") {
    return [
      createAssistantMessage(
        `${roomDraft.roomName}已经创建成功。进入 Room 后，我会自动发起真实的多 Agent 评审。`,
        options,
      ),
    ];
  }
  if (stage === "room_running") {
    return [
      createAssistantMessage(
        "产品评审仍在进行中。请返回协作 Room，等待用户研究、技术评审和产品经理完成两阶段协作。",
        options,
      ),
    ];
  }
  if (stage === "room_completed") {
    return [
      createAssistantMessage(
        "跨角色产品评审已经完成。你创建了自己的 Agent，也让它在真实 Room 中完成了一次协作。",
        options,
      ),
    ];
  }
  if (stage === "token") {
    return [
      createAssistantMessage(
        "已记住你选择的模型厂商。请提供该厂商的模型 Token，它会保存到你的 Nexus 模型配置中。",
        options,
      ),
    ];
  }
  return [createAssistantMessage(HOME_ONBOARDING_WELCOME_MESSAGE, options)];
}

function buildAgentNamePrompt(role: string): string {
  const profile = getHomeOnboardingAgentTaskProfile(role);
  return `已记录你的角色：${role || "当前角色"}。接下来我们会完成两个真实任务：先创建一个自己的智能体，再创建一个 Room 完成协作。\n\n先给你的${profile.agentType}起个名字。推荐使用「${profile.suggestedName}」，也可以直接输入你喜欢的名称。`;
}

function buildAgentDescriptionPrompt(name: string, role: string): string {
  const profile = getHomeOnboardingAgentTaskProfile(role);
  return `很好，「${name}」这个名字可以使用。接下来请用一句话告诉我：你希望它重点帮你完成哪些工作？\n\n${profile.descriptionExample}`;
}

function selectionFromProviderResult(
  provider: string,
  model: string,
): ProviderModelSelection {
  return {
    model,
    model_display_name: model,
    provider,
    provider_display_name: provider,
  };
}

function isLegacyModelDrivenStage(stage: HomeOnboardingStage): boolean {
  return stage === "agent_name"
    || stage === "agent_description"
    || stage === "agent_style"
    || stage === "agent_confirm"
    || stage === "agent_created"
    || stage === "room_idea"
    || stage === "room_plan"
    || stage === "room_ready"
    || stage === "room_running";
}

function buildRoleOnboardingKickoff(role: string): string {
  return `我是${role}。这是一次全新的 Nexus 新手旅程。请先加载对应角色的新手引导 Skill 和 nexus-onboarding-card-kit，严格使用预置卡片收集引导信息，带我完成两个任务：创建一个属于我的 Agent，再创建一个 Room 并完成一次真实协作。只把当前会话中确认或创建的资源视为本次进度；历史会话和既有 Agent、Room 不得自动标记为本次已完成。请从第一个尚未在当前会话确认的卡片步骤开始。`;
}

export function useDmOnboardingController({
  agentId,
  agentName,
  conversationId,
  enabled,
  roomId,
  sendMessage,
  sessionKey,
}: UseDmOnboardingControllerOptions): DmOnboardingControllerModel {
  const createAgent = useAgentStore((state) => state.create_agent);
  const agents = useAgentStore((state) => state.agents);
  const options = useMemo(
    () => ({
      agentId,
      agentName,
      conversationId,
      enabled,
      roomId,
      sendMessage,
      sessionKey,
    }),
    [agentId, agentName, conversationId, enabled, roomId, sendMessage, sessionKey],
  );
  const initialStage = enabled ? getHomeOnboardingStage() : "provider";
  const [stage, setStage] = useState<HomeOnboardingStage>(initialStage);
  const [messages, setMessages] = useState<Message[]>(() =>
    buildInitialMessages(options, initialStage),
  );
  const [isConfiguring, setIsConfiguring] = useState(false);
  const [isSettingDefault, setIsSettingDefault] = useState(false);
  const [isCreatingAgent, setIsCreatingAgent] = useState(false);
  const [isCreatingRoom, setIsCreatingRoom] = useState(false);
  const [isValidatingAgentName, setIsValidatingAgentName] = useState(false);
  const [isValidated, setIsValidated] = useState(false);
  const [validationFailureCount, setValidationFailureCount] = useState(0);
  const [defaultModelSelection, setDefaultModelSelection] =
    useState<ProviderModelSelection | null>(null);
  const [defaultModelChoices, setDefaultModelChoices] = useState<
    ProviderModelSelection[]
  >([]);
  const [selectedProviderPresetKey, setSelectedProviderPresetKey] = useState<
    string | null
  >(() => enabled ? getHomeOnboardingProviderPresetKey() : null);
  const [providerChoices, setProviderChoices] = useState<
    HomeOnboardingProviderChoice[]
  >([]);
  const [providerChoicesError, setProviderChoicesError] = useState<
    string | null
  >(null);
  const [providerChoicesLoading, setProviderChoicesLoading] = useState(false);
  const [agentTaskDraft, setAgentTaskDraft] =
    useState<HomeOnboardingAgentTaskDraft>(() => (
      getHomeOnboardingAgentTaskDraft(getHomeOnboardingRole() ?? "")
    ));
  const [roomTaskDraft, setRoomTaskDraft] =
    useState<HomeOnboardingRoomTaskDraft>(() => (
      getHomeOnboardingRoomTaskDraft(
        getHomeOnboardingAgentTaskDraft(
          getHomeOnboardingRole() ?? "",
        ).createdAgentId,
        getHomeOnboardingAgentTaskDraft(
          getHomeOnboardingRole() ?? "",
        ).name,
        agentId ?? "",
        agentName ?? "Nexus",
      )
    ));
  const modelResolutionAttempted = useRef(false);

  const append = useCallback((nextMessages: Message[]) => {
    setMessages((current) => [...current, ...nextMessages]);
  }, []);
  const recordValidationFailure = useCallback(() => {
    setValidationFailureCount((current) => current + 1);
  }, []);
  const persistAgentTaskDraft = useCallback(
    (nextDraft: HomeOnboardingAgentTaskDraft) => {
      setAgentTaskDraft(nextDraft);
      setHomeOnboardingAgentTaskDraft(nextDraft);
    },
    [],
  );
  const persistRoomTaskDraft = useCallback(
    (nextDraft: HomeOnboardingRoomTaskDraft) => {
      setRoomTaskDraft(nextDraft);
      setHomeOnboardingRoomTaskDraft(nextDraft);
    },
    [],
  );
  const loadProviderChoices = useCallback(() => {
    setProviderChoicesLoading(true);
    setProviderChoicesError(null);
    void listHomeOnboardingProviderChoices()
      .then((choices) => {
        setProviderChoices(choices);
        if (choices.length === 0) {
          setProviderChoicesError(
            "暂时没有读取到可用于引导的模型厂商，请前往模型配置页手动接入。",
          );
        }
      })
      .catch((error) => {
        console.error("Failed to load Nexus onboarding providers:", error);
        setProviderChoicesError(
          "模型厂商列表加载失败，请重新加载或前往模型配置页。",
        );
      })
      .finally(() => {
        setProviderChoicesLoading(false);
      });
  }, []);

  useEffect(() => {
    if (!enabled) {
      return;
    }
    rememberHomeOnboardingReturnPath();
  }, [enabled]);

  useEffect(() => {
    if (!enabled || stage !== "provider" || providerChoicesLoading) {
      return;
    }
    if (providerChoices.length === 0 && !providerChoicesError) {
      loadProviderChoices();
    }
  }, [
    enabled,
    loadProviderChoices,
    providerChoices.length,
    providerChoicesError,
    providerChoicesLoading,
    stage,
  ]);

  useEffect(() => {
    if (
      !enabled
      || stage !== "role"
      || getHomeOnboardingRole()
      || isHomeOnboardingDefaultModelConfirmed()
    ) {
      return;
    }
    modelResolutionAttempted.current = false;
    setStage("default_model");
    setHomeOnboardingStage("default_model");
    setIsValidated(false);
  }, [enabled, stage]);

  useEffect(() => {
    if (
      !enabled
      || (stage !== "provider_configuration" && stage !== "default_model")
      || modelResolutionAttempted.current
    ) {
      return;
    }
    modelResolutionAttempted.current = true;
    void resolveHomeOnboardingModelOptions(selectedProviderPresetKey)
      .then((resolution) => {
        setDefaultModelChoices(resolution.choices);
        const resolvedDefaultSelection = resolution.defaultSelection;
        const defaultSelectionIsAvailable = resolvedDefaultSelection
          ? resolution.choices.some((choice) => (
              choice.provider === resolvedDefaultSelection.provider
              && choice.model === resolvedDefaultSelection.model
            ))
          : false;
        if (stage === "default_model") {
          const pendingSelection = defaultSelectionIsAvailable
            ? resolvedDefaultSelection
            : resolution.suggestedSelection ?? resolution.choices[0] ?? null;
          if (pendingSelection) {
            setDefaultModelSelection(pendingSelection);
            setIsValidated(false);
            return;
          }
        }
        if (stage === "provider_configuration" && resolvedDefaultSelection) {
          setDefaultModelSelection(resolvedDefaultSelection);
          setIsValidated(false);
          setHomeOnboardingDefaultModelConfirmed();
          setStage("role");
          setHomeOnboardingStage("role");
          append([
            createAssistantMessage(
              "系统中已经有默认对话模型。下一步我们选择一下你的角色。",
              options,
            ),
          ]);
          return;
        }
        const selection = resolution.suggestedSelection;
        if (!selection) {
          if (stage === "provider_configuration") {
            append([
              createAssistantMessage(
                "我还没有检测到可用模型。请完成模型配置后，点击左上角 NX 回到这里。",
                options,
              ),
            ]);
          } else {
            clearHomeOnboardingProviderPresetKey();
            setSelectedProviderPresetKey(null);
            setDefaultModelSelection(null);
            setDefaultModelChoices([]);
            setStage("provider");
            setHomeOnboardingStage("provider");
            setIsValidated(false);
            modelResolutionAttempted.current = false;
            append([
              createAssistantMessage(
                "之前的模型配置已经失效。请重新选择模型厂商并提供 Token，我会重新完成校验和模型同步。",
                options,
              ),
            ]);
          }
          return;
        }
        setDefaultModelSelection(selection);
        setIsValidated(false);
        if (stage === "provider_configuration") {
          setStage("default_model");
          setHomeOnboardingStage("default_model");
          append([
            createAssistantMessage(
              `我已检测到可用模型：${selection.provider_display_name || selection.provider} / ${selection.model_display_name || selection.model}。要将它设为默认对话模型吗？`,
              options,
            ),
          ]);
        }
      })
      .catch((error) => {
        console.error("Failed to resolve Nexus onboarding model:", error);
        if (stage === "provider_configuration") {
          append([
            createAssistantMessage(
              "我暂时还无法读取模型配置。请确认模型已启用，然后点击左上角 NX 再试一次。",
              options,
            ),
          ]);
        }
      });
  }, [append, enabled, options, selectedProviderPresetKey, stage]);

  const handleSendMessage = useCallback(
    async (content: string, sendOptions?: AgentConversationSendOptions) => {
      if (!enabled || isValidated) {
        await sendMessage(content, sendOptions);
        return;
      }
      if (
        isConfiguring
        || isCreatingAgent
        || isCreatingRoom
        || isValidatingAgentName
      ) {
        return;
      }

      const normalized = content.trim();
      if (!normalized) {
        return;
      }

      if (stage === "skill_guided" || isLegacyModelDrivenStage(stage)) {
        if (stage !== "skill_guided") {
          setStage("skill_guided");
          setHomeOnboardingStage("skill_guided");
        }
        await sendMessage(normalized, sendOptions);
        return;
      }

      if (stage === "room_idea") {
        const userMessage = createUserMessage(normalized, options);
        if (normalized.length < 10) {
          append([
            userMessage,
            createAssistantMessage(
              "可以再具体一点吗？请至少说明目标用户、要解决的问题或你设想的产品能力。",
              options,
            ),
          ]);
          return;
        }
        const nextDraft = {
          ...roomTaskDraft,
          phase: "plan" as const,
          productIdea: normalized,
        };
        persistRoomTaskDraft(nextDraft);
        setStage("room_plan");
        setHomeOnboardingStage("room_plan");
        append([
          userMessage,
          createAssistantMessage(
            "我已经把这个想法整理成一次跨角色产品评审。用户研究和技术评审会先并行分析，再由你的产品经理 Agent 收敛最终方案。",
            options,
          ),
        ]);
        return;
      }

      if (stage === "agent_name") {
        const userMessage = createUserMessage(normalized, options);
        append([userMessage]);
        setIsValidatingAgentName(true);
        try {
          const validation = await validateAgentNameApi(normalized);
          if (!validation.is_valid || !validation.is_available) {
            append([
              createAssistantMessage(
                validation.reason?.trim()
                  ? `这个名称暂时不能使用：${validation.reason.trim()}。请换一个名称再试。`
                  : "这个名称暂时不能使用，请换一个名称再试。",
                options,
              ),
            ]);
            return;
          }
          const nextDraft = {
            ...agentTaskDraft,
            name: validation.normalized_name.trim() || normalized,
          };
          persistAgentTaskDraft(nextDraft);
          setStage("agent_description");
          setHomeOnboardingStage("agent_description");
          append([
            createAssistantMessage(
              buildAgentDescriptionPrompt(nextDraft.name, nextDraft.role),
              options,
            ),
          ]);
        } catch (error) {
          console.error("Failed to validate onboarding Agent name:", error);
          append([
            createAssistantMessage(
              "我暂时无法校验这个名称，请稍后再试。",
              options,
            ),
          ]);
        } finally {
          setIsValidatingAgentName(false);
        }
        return;
      }

      if (stage === "agent_description") {
        const userMessage = createUserMessage(normalized, options);
        if (normalized.length < 8) {
          append([
            userMessage,
            createAssistantMessage(
              "可以再具体一点吗？请至少说明它要处理的工作和你期望的产出。",
              options,
            ),
          ]);
          return;
        }
        const nextDraft = {
          ...agentTaskDraft,
          description: normalized,
        };
        persistAgentTaskDraft(nextDraft);
        setStage("agent_style");
        setHomeOnboardingStage("agent_style");
        append([
          userMessage,
          createAssistantMessage(
            `很好，我已经记下「${nextDraft.name}」的核心职责。最后选择一种工作风格，我会把它转换成 Agent 的身份标签。`,
            options,
          ),
        ]);
        return;
      }

      if (stage !== "token") {
        return;
      }

      if (!selectedProviderPresetKey) {
        setStage("provider");
        setHomeOnboardingStage("provider");
        append([
          createAssistantMessage(
            "请先选择模型厂商，我才能使用正确的接口校验 Token。",
            options,
          ),
        ]);
        return;
      }

      const userMessage = createUserMessage(
        maskHomeOnboardingToken(normalized),
        options,
      );
      if (normalized.length < 12) {
        append([
          userMessage,
          createAssistantMessage(HOME_ONBOARDING_INVALID_TOKEN_MESSAGE, options),
        ]);
        recordValidationFailure();
        setStage("token");
        setHomeOnboardingStage("token");
        return;
      }

      append([
        userMessage,
        createAssistantMessage(HOME_ONBOARDING_CONNECTING_MESSAGE, options),
      ]);
      setIsConfiguring(true);

      try {
        const result = await configureHomeOnboardingProvider(
          normalized,
          selectedProviderPresetKey,
        );
        if (!result.success) {
          append([
            createAssistantMessage(
              HOME_ONBOARDING_INVALID_TOKEN_MESSAGE,
              options,
            ),
          ]);
          recordValidationFailure();
          setStage("token");
          setHomeOnboardingStage("token");
          return;
        }

        if (typeof window !== "undefined") {
          window.localStorage.removeItem(HOME_ONBOARDING_TOKEN_STORAGE_KEY);
        }
        setValidationFailureCount(0);
        clearHomeOnboardingDefaultModelConfirmed();
        setIsValidated(false);
        const suggestedSelection = result.models.find(
          (selection) => selection.model === result.model,
        ) ?? selectionFromProviderResult(result.provider, result.model);
        setDefaultModelSelection(suggestedSelection);
        setDefaultModelChoices(result.models);
        setStage("default_model");
        setHomeOnboardingStage("default_model");
        append([
          createAssistantMessage(
            `${HOME_ONBOARDING_VALID_TOKEN_MESSAGE} 已找到 ${result.models.length} 个模型，请选择一个作为默认对话模型。`,
            options,
          ),
        ]);
      } catch (error) {
        console.error("Failed to configure Nexus onboarding provider:", error);
        append([
          createAssistantMessage(
            HOME_ONBOARDING_INVALID_TOKEN_MESSAGE,
            options,
          ),
        ]);
        recordValidationFailure();
        setStage("token");
        setHomeOnboardingStage("token");
      } finally {
        setIsConfiguring(false);
      }
    },
    [
      append,
      agentTaskDraft,
      enabled,
      isConfiguring,
      isCreatingAgent,
      isCreatingRoom,
      isValidated,
      isValidatingAgentName,
      options,
      persistRoomTaskDraft,
      persistAgentTaskDraft,
      recordValidationFailure,
      selectedProviderPresetKey,
      sendMessage,
      stage,
      roomTaskDraft,
    ],
  );

  const selectProvider = useCallback(
    (choice: HomeOnboardingProviderChoice) => {
      if (stage !== "provider") {
        return;
      }
      setHomeOnboardingProviderPresetKey(choice.presetKey);
      clearHomeOnboardingDefaultModelConfirmed();
      setSelectedProviderPresetKey(choice.presetKey);
      modelResolutionAttempted.current = false;
      setValidationFailureCount(0);
      setStage("token");
      setHomeOnboardingStage("token");
      append([
        createAssistantMessage(
          buildHomeOnboardingTokenPrompt(choice.displayName, choice.keyUrl),
          options,
        ),
      ]);
    },
    [append, options, stage],
  );

  const confirmDefaultModel = useCallback(async () => {
    if (!defaultModelSelection || isSettingDefault) {
      return;
    }
    setIsSettingDefault(true);
    try {
      await setHomeOnboardingDefaultModel(defaultModelSelection);
      setHomeOnboardingDefaultModelConfirmed();
      setStage("role");
      setHomeOnboardingStage("role");
      setIsValidated(false);
      append([
        createAssistantMessage(
          "模型已设为默认对话模型。下一步我们选择一下你的角色。",
          options,
        ),
      ]);
    } catch (error) {
      console.error("Failed to set Nexus onboarding default model:", error);
      append([
        createAssistantMessage(
          "设置默认对话模型时遇到问题，请稍后再试。",
          options,
        ),
      ]);
    } finally {
      setIsSettingDefault(false);
    }
  }, [append, defaultModelSelection, isSettingDefault, options]);

  const selectDefaultModel = useCallback(
    (selection: ProviderModelSelection) => {
      if (stage !== "default_model") {
        return;
      }
      setDefaultModelSelection(selection);
    },
    [stage],
  );

  const selectAgentStyle = useCallback(
    (choice: HomeOnboardingAgentStyleChoice) => {
      if (stage !== "agent_style") {
        return;
      }
      const nextDraft = {
        ...agentTaskDraft,
        vibeTags: [...choice.tags],
      };
      persistAgentTaskDraft(nextDraft);
      setStage("agent_confirm");
      setHomeOnboardingStage("agent_confirm");
      append([
        createAssistantMessage(
          `已选择「${choice.label}」。我已经整理好完整的 Agent 草案，请确认后创建。`,
          options,
        ),
      ]);
    },
    [agentTaskDraft, append, options, persistAgentTaskDraft, stage],
  );

  const restartAgentDraft = useCallback(() => {
    const role = agentTaskDraft.role || getHomeOnboardingRole() || "";
    const nextDraft = createHomeOnboardingAgentTaskDraft(role);
    persistAgentTaskDraft(nextDraft);
    setStage("agent_name");
    setHomeOnboardingStage("agent_name");
    setIsValidated(false);
    append([
      createAssistantMessage(
        `没问题，我们重新整理。${buildAgentNamePrompt(role)}`,
        options,
      ),
    ]);
  }, [agentTaskDraft.role, append, options, persistAgentTaskDraft]);

  const confirmAgentCreation = useCallback(async () => {
    if (
      stage !== "agent_confirm"
      || isCreatingAgent
      || !agentTaskDraft.name
      || !agentTaskDraft.description
      || agentTaskDraft.vibeTags.length === 0
    ) {
      return;
    }
    setIsCreatingAgent(true);
    try {
      const agentId = await createAgent(
        buildHomeOnboardingAgentCreateParams(agentTaskDraft),
      );
      const nextDraft = {
        ...agentTaskDraft,
        createdAgentId: agentId,
      };
      persistAgentTaskDraft(nextDraft);
      setStage("agent_created");
      setHomeOnboardingStage("agent_created");
      setIsValidated(false);
      append([
        createAssistantMessage(
          `创建成功！「${nextDraft.name}」已经成为你的 Nexus Agent。点击下面的身份卡片，可以进入它的真实 Agent 页面继续查看或调整配置。`,
          options,
        ),
      ]);
    } catch (error) {
      console.error("Failed to create onboarding Agent:", error);
      append([
        createAssistantMessage(
          `Agent 还没有创建成功：${getErrorMessage(error, "服务暂时不可用")}。草案已经保留，你可以直接重试。`,
          options,
        ),
      ]);
    } finally {
      setIsCreatingAgent(false);
    }
  }, [
    agentTaskDraft,
    append,
    createAgent,
    isCreatingAgent,
    options,
    persistAgentTaskDraft,
    stage,
  ]);

  const selectRole = useCallback(
    (role: string) => {
      const normalizedRole = role.trim();
      if (!normalizedRole || stage !== "role") {
        return;
      }
      const nextDraft = createHomeOnboardingAgentTaskDraft(normalizedRole);
      setHomeOnboardingRole(normalizedRole);
      persistAgentTaskDraft(nextDraft);
      setHomeOnboardingStage("skill_guided");
      setStage("skill_guided");
      setIsValidated(false);
      void sendMessage(buildRoleOnboardingKickoff(normalizedRole));
    },
    [persistAgentTaskDraft, sendMessage, stage],
  );

  const startRoomTask = useCallback(() => {
    if (
      stage !== "agent_created"
      || !agentTaskDraft.createdAgentId
      || !agentTaskDraft.name
    ) {
      return;
    }
    const nextDraft = createHomeOnboardingRoomTaskDraft(
      agentTaskDraft.createdAgentId,
      agentTaskDraft.name,
      agentId ?? "",
      agentName ?? "Nexus",
    );
    persistRoomTaskDraft(nextDraft);
    setStage("room_idea");
    setHomeOnboardingStage("room_idea");
    setIsValidated(false);
    append([
      createAssistantMessage(
        "现在开始任务 2：创建一个产品评审 Room。请用 1～3 句话描述一个你真实想评审的产品想法或需求，我会据此组织协作任务。",
        options,
      ),
    ]);
  }, [
    agentTaskDraft.createdAgentId,
    agentTaskDraft.name,
    agentId,
    agentName,
    append,
    options,
    persistRoomTaskDraft,
    stage,
  ]);

  const restartRoomIdea = useCallback(() => {
    if (stage !== "room_plan" || isCreatingRoom) {
      return;
    }
    const nextDraft = {
      ...roomTaskDraft,
      phase: "idle" as const,
      productIdea: "",
    };
    persistRoomTaskDraft(nextDraft);
    setStage("room_idea");
    setHomeOnboardingStage("room_idea");
    append([
      createAssistantMessage(
        "没问题，我们换一个主题。请重新描述你希望评审的产品想法或需求。",
        options,
      ),
    ]);
  }, [
    append,
    isCreatingRoom,
    options,
    persistRoomTaskDraft,
    roomTaskDraft,
    stage,
  ]);

  const confirmRoomPlan = useCallback(async () => {
    if (
      stage !== "room_plan"
      || isCreatingRoom
      || !roomTaskDraft.productIdea
      || !roomTaskDraft.productManagerAgentId
    ) {
      return;
    }
    setIsCreatingRoom(true);
    let nextDraft = roomTaskDraft;
    const ensureSupportAgent = async (
      definition: HomeOnboardingRoomSupportAgentDefinition,
      currentId: string,
    ): Promise<string> => {
      if (currentId) {
        return currentId;
      }
      const existing = agents.find(
        (agent) => agent.name.trim() === definition.name,
      );
      return existing?.agent_id
        ?? createAgent(buildHomeOnboardingSupportAgentCreateParams(definition));
    };

    try {
      const researcherAgentId = await ensureSupportAgent(
        PRODUCT_MANAGER_ROOM_SUPPORT_AGENTS.researcher,
        nextDraft.researcherAgentId,
      );
      nextDraft = { ...nextDraft, researcherAgentId };
      persistRoomTaskDraft(nextDraft);

      const technicalReviewerAgentId = await ensureSupportAgent(
        PRODUCT_MANAGER_ROOM_SUPPORT_AGENTS.technicalReviewer,
        nextDraft.technicalReviewerAgentId,
      );
      nextDraft = { ...nextDraft, technicalReviewerAgentId };
      persistRoomTaskDraft(nextDraft);

      if (!nextDraft.roomId || !nextDraft.conversationId) {
        const hostAgentId = (agentId ?? nextDraft.hostAgentId).trim();
        if (!hostAgentId) {
          throw new Error("Nexus 主持人尚未准备好");
        }
        nextDraft = {
          ...nextDraft,
          hostAgentId,
          hostAgentName: agentName?.trim() || nextDraft.hostAgentName || "Nexus",
        };
        persistRoomTaskDraft(nextDraft);
        const context = await createRoom({
          allow_main_agent_host: true,
          agent_ids: Array.from(new Set([
            hostAgentId,
            nextDraft.productManagerAgentId,
            researcherAgentId,
            technicalReviewerAgentId,
          ])),
          description:
            `围绕「${nextDraft.productIdea}」完成用户价值、技术可行性与产品范围的跨角色评审。`,
          avatar: PRODUCT_MANAGER_ROOM_AVATAR,
          host_agent_id: hostAgentId,
          host_auto_reply_enabled: true,
          name: nextDraft.roomName,
          private_messages_enabled: false,
        });
        nextDraft = {
          ...nextDraft,
          conversationId: context.conversation.id,
          roomId: context.room.id,
        };
        persistRoomTaskDraft(nextDraft);
      }

      nextDraft = { ...nextDraft, phase: "ready" };
      persistRoomTaskDraft(nextDraft);
      setStage("room_ready");
      setHomeOnboardingStage("room_ready");
      append([
        createAssistantMessage(
          `${nextDraft.roomName}已经创建成功，Nexus 主持人和三位协作 Agent 已就位。进入 Room 后，我会先发布固定评审任务，后续接力与输出都由 Room 的真实模型服务完成。`,
          options,
        ),
      ]);
    } catch (error) {
      console.error("Failed to create onboarding Room:", error);
      append([
        createAssistantMessage(
          `Room 还没有创建成功：${getErrorMessage(error, "服务暂时不可用")}。已完成的成员配置会保留，你可以直接重试。`,
          options,
        ),
      ]);
    } finally {
      setIsCreatingRoom(false);
    }
  }, [
    agents,
    agentId,
    agentName,
    append,
    createAgent,
    isCreatingRoom,
    options,
    persistRoomTaskDraft,
    roomTaskDraft,
    stage,
  ]);

  const launchRoomCollaboration = useCallback(() => {
    if (stage !== "room_ready" && stage !== "room_running") {
      return;
    }
    if (stage === "room_ready") {
      const nextDraft = { ...roomTaskDraft, phase: "review" as const };
      persistRoomTaskDraft(nextDraft);
    }
    completeHomeOnboarding();
    setIsValidated(true);
  }, [persistRoomTaskDraft, roomTaskDraft, stage]);

  const finishOnboarding = useCallback(() => {
    if (stage !== "room_completed") {
      return;
    }
    completeHomeOnboarding();
    setIsValidated(true);
  }, [stage]);

  return useMemo(
    () => ({
      agentStyleChoices:
        getHomeOnboardingAgentTaskProfile(agentTaskDraft.role).styleChoices,
      agentTaskDraft,
      confirmAgentCreation,
      confirmDefaultModel,
      confirmRoomPlan,
      defaultModelChoices,
      defaultModelSelection,
      finishOnboarding,
      isActive: enabled && !isValidated,
      isCreatingAgent,
      isCreatingRoom,
      launchRoomCollaboration,
      messages,
      providerChoices,
      providerChoicesError,
      providerChoicesLoading,
      restartAgentDraft,
      restartRoomIdea,
      retryProviderChoices: loadProviderChoices,
      roomTaskDraft,
      selectAgentStyle,
      selectRole,
      selectDefaultModel,
      selectProvider,
      showDefaultModelCard:
        enabled
        && stage === "default_model"
        && defaultModelSelection !== null
        && defaultModelChoices.length > 0,
      showAgentConfirmationCard:
        enabled
        && stage === "agent_confirm",
      showAgentIdentityCard:
        enabled
        && stage === "agent_created"
        && Boolean(agentTaskDraft.createdAgentId),
      showAgentStyleCard:
        enabled
        && stage === "agent_style",
      showProviderConfigCard:
        enabled
        && (
          (stage === "token" && validationFailureCount >= 2)
          || (stage === "provider_configuration"
            && defaultModelSelection === null)
        ),
      showProviderSelectionCard: enabled && stage === "provider",
      showRoleCard: enabled && stage === "role",
      showRoomCompletionCard: false,
      showRoomLaunchCard:
        enabled
        && (stage === "room_ready" || stage === "room_running")
        && Boolean(roomTaskDraft.roomId)
        && Boolean(roomTaskDraft.conversationId),
      showRoomPlanCard: enabled && stage === "room_plan",
      showRoomStartCard:
        enabled
        && stage === "agent_created"
        && agentTaskDraft.role === "产品经理",
      startRoomTask,
      sendMessage: handleSendMessage,
    }),
    [
      agentTaskDraft,
      confirmAgentCreation,
      confirmDefaultModel,
      confirmRoomPlan,
      defaultModelChoices,
      defaultModelSelection,
      enabled,
      finishOnboarding,
      handleSendMessage,
      isCreatingAgent,
      isCreatingRoom,
      isValidated,
      launchRoomCollaboration,
      loadProviderChoices,
      messages,
      providerChoices,
      providerChoicesError,
      providerChoicesLoading,
      restartAgentDraft,
      restartRoomIdea,
      roomTaskDraft,
      selectAgentStyle,
      selectRole,
      selectDefaultModel,
      selectProvider,
      stage,
      startRoomTask,
      validationFailureCount,
    ],
  );
}
