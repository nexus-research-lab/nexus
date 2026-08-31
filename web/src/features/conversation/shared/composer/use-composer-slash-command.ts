"use client";

// INPUT: Composer Slash 命令输入与 Skill/Model 只读目录请求。
// OUTPUT: 保留草稿的 picker 状态、三问读取失败和显式重载动作。
// POS: Composer Slash picker 编排边界；目录失败从不解释为草稿或设置修改。

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import type {
  Dispatch,
  KeyboardEvent,
  RefObject,
  SetStateAction,
} from "react";

import { getAvailableSkillsApi } from "@/lib/api/capability/skill-api";
import { listProviderOptionsApi } from "@/lib/api/settings/provider-api";
import { getSkillDisplayDescription } from "@/lib/skill-description";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { SkillInfo } from "@/types/capability/skill";
import type {
  CommandCatalogData,
  CommandDescriptor,
} from "@/types/generated/protocol";
import type { AgentRuntimeKind } from "@/types/settings/preferences";

import {
  buildComposerReadFailure,
  type ComposerReadFailure,
} from "./controller/composer-settings-reliability";
import {
  buildSlashModelOptions,
  filterSlashCommands,
  filterSlashModels,
  filterSlashSkills,
  findSlashCommandTextMatch,
  formatSlashCommandInsertText,
  formatSlashModelInsertText,
  insertSlashCommand,
  insertSlashTextAtCursor,
  isSelectableSlashCommand,
  SLASH_COMMAND_NAVIGATION_KEYS,
  type SlashCommandTextMatch,
  type SlashModelOption,
} from "./slash-command-model";

const SKILLS_COMMAND_NAME = "skills";
const MODEL_COMMAND_NAME = "model";

type SlashCommandMode = "commands" | "models" | "skills";
type SlashCommandPickerMode = Exclude<SlashCommandMode, "commands">;

interface UseComposerSlashCommandOptions {
  catalog: CommandCatalogData;
  input: string;
  isGoalMode: boolean;
  runtimeKind: AgentRuntimeKind;
  setInput: Dispatch<SetStateAction<string>>;
  textareaRef: RefObject<HTMLTextAreaElement | null>;
}

export function useComposerSlashCommand({
  catalog,
  input,
  isGoalMode,
  runtimeKind,
  setInput,
  textareaRef,
}: UseComposerSlashCommandOptions) {
  const { t } = useI18n();
  const [match, setMatch] = useState<SlashCommandTextMatch | null>(null);
  const [mode, setMode] = useState<SlashCommandMode>("commands");
  const [activeIndex, setActiveIndex] = useState(0);
  const [skillQuery, setSkillQuery] = useState("");
  const [skillItems, setSkillItems] = useState<SkillInfo[]>([]);
  const [skillsLoading, setSkillsLoading] = useState(false);
  const [skillsError, setSkillsError] =
    useState<ComposerReadFailure | null>(null);
  const [loadedSkillsAgentID, setLoadedSkillsAgentID] = useState<string | null>(
    null,
  );
  const [modelItems, setModelItems] = useState<SlashModelOption[]>([]);
  const [modelLoading, setModelLoading] = useState(false);
  const [modelError, setModelError] =
    useState<ComposerReadFailure | null>(null);
  const [modelQuery, setModelQuery] = useState("");
  const [loadedModelRuntimeKind, setLoadedModelRuntimeKind] = useState("");
  const skillSearchRef = useRef<HTMLInputElement>(null);
  const modelSearchRef = useRef<HTMLInputElement>(null);
  const skillsRequestRef = useRef(0);
  const modelsRequestRef = useRef(0);
  const skillsAgentID = catalog.agent_id?.trim() || "";
  const modelRuntimeKind = catalog.runtime_kind?.trim()
    || runtimeKind.trim()
    || "nxs";

  const filteredCommands = useMemo(
    () => filterSlashCommands(catalog.commands, match?.query ?? ""),
    [catalog.commands, match?.query],
  );
  const filteredSkills = useMemo(
    () => filterSlashSkills(
      skillItems,
      skillQuery,
      (skill) => getSkillDisplayDescription(skill, t),
    ),
    [skillItems, skillQuery, t],
  );
  const filteredModels = useMemo(
    () => filterSlashModels(modelItems, modelQuery),
    [modelItems, modelQuery],
  );
  const visibleItems = mode === "skills"
    ? filteredSkills
    : mode === "models"
      ? filteredModels
      : filteredCommands;
  const visibleActiveIndex = Math.min(
    activeIndex,
    Math.max(visibleItems.length - 1, 0),
  );
  const activeCommand = filteredCommands[visibleActiveIndex] ?? null;
  const activeSkill = filteredSkills[visibleActiveIndex] ?? null;
  const activeModel = filteredModels[visibleActiveIndex] ?? null;
  const isOpen = Boolean(match) || mode !== "commands";
  const skillCount = filteredSkills.length;
  const modelCount = filteredModels.length;

  useEffect(() => {
    setActiveIndex(0);
  }, [catalog.revision, match?.query, modelQuery, skillQuery, mode]);

  useEffect(() => {
    if (isGoalMode) {
      setMatch(null);
      setMode("commands");
      setSkillQuery("");
      setModelQuery("");
    }
  }, [isGoalMode]);

  useEffect(() => {
    if (!match) {
      return;
    }
    const cursorPosition = Math.min(
      textareaRef.current?.selectionStart ?? input.length,
      input.length,
    );
    if (!findSlashCommandTextMatch(input, cursorPosition, !isGoalMode)) {
      setMatch(null);
      setMode("commands");
    }
  }, [input, isGoalMode, match, textareaRef]);

  useEffect(() => {
    if (mode !== "skills" && mode !== "models") {
      return;
    }
    requestAnimationFrame(() => {
      (mode === "skills" ? skillSearchRef : modelSearchRef).current?.focus();
    });
  }, [mode]);

  useEffect(() => {
    if (
      mode !== "skills"
      || loadedSkillsAgentID === skillsAgentID
      || skillsLoading
    ) {
      return;
    }
    const requestID = ++skillsRequestRef.current;
    setSkillsLoading(true);
    if (loadedSkillsAgentID !== null && loadedSkillsAgentID !== skillsAgentID) {
      setSkillItems([]);
      setSkillsError(null);
    }
    void (async () => {
      try {
        const nextSkills = await getAvailableSkillsApi(
          skillsAgentID ? { agent_id: skillsAgentID } : undefined,
        );
        if (requestID === skillsRequestRef.current) {
          setSkillItems(nextSkills);
          setSkillsError(null);
        }
      } catch (error) {
        if (requestID === skillsRequestRef.current) {
          setSkillsError(buildComposerReadFailure(
            error,
            "skills",
            t("composer.skills_load_failed"),
            t,
          ));
        }
      } finally {
        if (requestID === skillsRequestRef.current) {
          setSkillsLoading(false);
          setLoadedSkillsAgentID(skillsAgentID);
        }
      }
    })();
  }, [
    loadedSkillsAgentID,
    mode,
    skillsAgentID,
    skillsLoading,
    t,
  ]);

  useEffect(() => {
    if (
      mode !== "models"
      || loadedModelRuntimeKind === modelRuntimeKind
      || modelLoading
    ) {
      return;
    }
    const requestID = ++modelsRequestRef.current;
    setModelLoading(true);
    if (
      loadedModelRuntimeKind
      && loadedModelRuntimeKind !== modelRuntimeKind
    ) {
      setModelItems([]);
      setModelError(null);
    }
    void (async () => {
      try {
        const response = await listProviderOptionsApi(modelRuntimeKind);
        if (requestID === modelsRequestRef.current) {
          setModelItems(buildSlashModelOptions(response));
          setModelError(null);
        }
      } catch (error) {
        if (requestID === modelsRequestRef.current) {
          setModelError(buildComposerReadFailure(
            error,
            "models",
            t("composer.models_load_failed"),
            t,
          ));
        }
      } finally {
        if (requestID === modelsRequestRef.current) {
          setModelLoading(false);
          setLoadedModelRuntimeKind(modelRuntimeKind);
        }
      }
    })();
  }, [
    loadedModelRuntimeKind,
    mode,
    modelLoading,
    modelRuntimeKind,
    t,
  ]);

  const retrySkills = useCallback(() => {
    setLoadedSkillsAgentID(null);
  }, []);
  const retryModels = useCallback(() => {
    setLoadedModelRuntimeKind("");
  }, []);

  const close = useCallback(() => {
    setMatch(null);
    setMode("commands");
    setSkillQuery("");
    setModelQuery("");
    setActiveIndex(0);
  }, []);

  const openPicker = useCallback((pickerMode: SlashCommandPickerMode) => {
    if (!match) {
      return;
    }
    const nextValue = input.slice(0, match.start) + input.slice(match.end);
    setInput(nextValue);
    setMatch(null);
    setMode(pickerMode);
    setSkillQuery("");
    setModelQuery("");
    setActiveIndex(0);
  }, [input, match, setInput]);

  const updateForInput = useCallback((value: string) => {
    if (mode !== "commands") {
      return;
    }
    const cursorPosition = textareaRef.current?.selectionStart ?? value.length;
    setMatch(findSlashCommandTextMatch(
      value,
      cursorPosition,
      !isGoalMode,
    ));
  }, [isGoalMode, mode, textareaRef]);

  const selectCommand = useCallback((command: CommandDescriptor) => {
    if (!match || !isSelectableSlashCommand(command)) {
      return;
    }
    if (command.name === SKILLS_COMMAND_NAME) {
      openPicker("skills");
      return;
    }
    if (command.name === MODEL_COMMAND_NAME) {
      openPicker("models");
      return;
    }
    const insertion = insertSlashCommand(input, match, command);
    setInput(insertion.value);
    setMatch(null);
    setMode("commands");
    setSkillQuery("");
    setModelQuery("");
    requestAnimationFrame(() => {
      textareaRef.current?.setSelectionRange(
        insertion.cursorPosition,
        insertion.cursorPosition,
      );
      textareaRef.current?.focus();
    });
  }, [
    input,
    match,
    openPicker,
    setInput,
    textareaRef,
  ]);

  const insertPickerSelection = useCallback((commandText: string) => {
    const insertion = insertSlashTextAtCursor(
      input,
      textareaRef.current?.selectionStart ?? input.length,
      commandText,
    );
    setInput(insertion.value);
    close();
    requestAnimationFrame(() => {
      const textarea = textareaRef.current;
      if (!textarea) {
        return;
      }
      textarea.setSelectionRange(
        insertion.cursorPosition,
        insertion.cursorPosition,
      );
      textarea.focus();
    });
  }, [close, input, setInput, textareaRef]);

  const selectSkill = useCallback((skill: SkillInfo) => {
    insertPickerSelection(formatSlashCommandInsertText(skill.name));
  }, [insertPickerSelection]);

  const selectModel = useCallback((model: SlashModelOption) => {
    insertPickerSelection(formatSlashModelInsertText(model));
  }, [insertPickerSelection]);

  const handleCommandKeyDown = useCallback((
    event: KeyboardEvent<HTMLTextAreaElement>,
  ): boolean => {
    if (mode !== "commands" || !match || !SLASH_COMMAND_NAVIGATION_KEYS.has(event.key)) {
      return false;
    }
    if (event.key === "Escape") {
      event.preventDefault();
      close();
      return true;
    }
    if (event.key === "ArrowDown" && filteredCommands.length > 0) {
      event.preventDefault();
      setActiveIndex((current) => (
        current + 1
      ) % filteredCommands.length);
      return true;
    }
    if (event.key === "ArrowUp" && filteredCommands.length > 0) {
      event.preventDefault();
      setActiveIndex((current) => (
        current - 1 + filteredCommands.length
      ) % filteredCommands.length);
      return true;
    }
    if (
      (event.key === "Enter" || event.key === "Tab")
      && activeCommand
    ) {
      event.preventDefault();
      selectCommand(activeCommand);
      return true;
    }
    return false;
  }, [activeCommand, close, filteredCommands.length, match, mode, selectCommand]);

  const handleSkillSearchKeyDown = useCallback((
    event: KeyboardEvent<HTMLInputElement>,
  ): boolean => {
    if (mode !== "skills" || !SLASH_COMMAND_NAVIGATION_KEYS.has(event.key)) {
      return false;
    }
    if (event.key === "Escape") {
      event.preventDefault();
      close();
      requestAnimationFrame(() => {
        textareaRef.current?.focus();
      });
      return true;
    }
    if (event.key === "ArrowDown" && skillCount > 0) {
      event.preventDefault();
      setActiveIndex((current) => (
        current + 1
      ) % skillCount);
      return true;
    }
    if (event.key === "ArrowUp" && skillCount > 0) {
      event.preventDefault();
      setActiveIndex((current) => (
        current - 1 + skillCount
      ) % skillCount);
      return true;
    }
    if ((event.key === "Enter" || event.key === "Tab") && activeSkill) {
      event.preventDefault();
      selectSkill(activeSkill);
      return true;
    }
    return false;
  }, [activeSkill, close, mode, selectSkill, skillCount, textareaRef]);

  const handleModelSearchKeyDown = useCallback((
    event: KeyboardEvent<HTMLInputElement>,
  ): boolean => {
    if (mode !== "models" || !SLASH_COMMAND_NAVIGATION_KEYS.has(event.key)) {
      return false;
    }
    if (event.key === "Escape") {
      event.preventDefault();
      close();
      requestAnimationFrame(() => {
        textareaRef.current?.focus();
      });
      return true;
    }
    if (event.key === "ArrowDown" && modelCount > 0) {
      event.preventDefault();
      setActiveIndex((current) => (current + 1) % modelCount);
      return true;
    }
    if (event.key === "ArrowUp" && modelCount > 0) {
      event.preventDefault();
      setActiveIndex((current) => (
        (current - 1 + modelCount) % modelCount
      ));
      return true;
    }
    if ((event.key === "Enter" || event.key === "Tab") && activeModel) {
      event.preventDefault();
      selectModel(activeModel);
      return true;
    }
    return false;
  }, [activeModel, close, mode, modelCount, selectModel, textareaRef]);

  return {
    activeIndex: visibleActiveIndex,
    close,
    commands: filteredCommands,
    handleCommandKeyDown,
    handleModelSearchKeyDown,
    handleSkillSearchKeyDown,
    isOpen,
    mode,
    modelError,
    modelItems: filteredModels,
    modelLoading,
    modelQuery,
    retryModels,
    modelSearchRef,
    query: match?.query ?? "",
    selectCommand,
    selectModel,
    selectSkill,
    skillCount,
    skillError: skillsError,
    skillItems: filteredSkills,
    skillLoading: skillsLoading,
    skillQuery,
    retrySkills,
    skillSearchRef,
    setSkillQuery,
    setModelQuery,
    status: catalog.status,
    updateForInput,
  };
}
