"use client";

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
import type { SkillInfo } from "@/types/capability/skill";
import type {
  CommandCatalogData,
  CommandDescriptor,
} from "@/types/generated/protocol";

import {
  filterSlashCommands,
  filterSlashSkills,
  findSlashCommandTextMatch,
  formatSlashCommandInsertText,
  insertSlashCommand,
  isSelectableSlashCommand,
  SLASH_COMMAND_NAVIGATION_KEYS,
  type SlashCommandTextMatch,
} from "./slash-command-model";

const SKILLS_COMMAND_NAME = "skills";

type SlashCommandMode = "commands" | "skills";

interface UseComposerSlashCommandOptions {
  catalog: CommandCatalogData;
  input: string;
  isGoalMode: boolean;
  setInput: Dispatch<SetStateAction<string>>;
  textareaRef: RefObject<HTMLTextAreaElement | null>;
}

export function useComposerSlashCommand({
  catalog,
  input,
  isGoalMode,
  setInput,
  textareaRef,
}: UseComposerSlashCommandOptions) {
  const [match, setMatch] = useState<SlashCommandTextMatch | null>(null);
  const [mode, setMode] = useState<SlashCommandMode>("commands");
  const [activeIndex, setActiveIndex] = useState(0);
  const [skillQuery, setSkillQuery] = useState("");
  const [skillItems, setSkillItems] = useState<SkillInfo[]>([]);
  const [skillsLoading, setSkillsLoading] = useState(false);
  const [skillsError, setSkillsError] = useState<string | null>(null);
  const skillSearchRef = useRef<HTMLInputElement>(null);
  const skillsRequestRef = useRef(0);

  const filteredCommands = useMemo(
    () => filterSlashCommands(catalog.commands, match?.query ?? ""),
    [catalog.commands, match?.query],
  );
  const filteredSkills = useMemo(
    () => filterSlashSkills(skillItems, skillQuery),
    [skillItems, skillQuery],
  );
  const visibleItems = mode === "skills" ? filteredSkills : filteredCommands;
  const visibleActiveIndex = Math.min(
    activeIndex,
    Math.max(visibleItems.length - 1, 0),
  );
  const activeCommand = filteredCommands[visibleActiveIndex] ?? null;
  const activeSkill = filteredSkills[visibleActiveIndex] ?? null;
  const isOpen = Boolean(match) || mode === "skills";
  const skillCount = filteredSkills.length;

  useEffect(() => {
    setActiveIndex(0);
  }, [catalog.revision, match?.query, skillQuery, mode]);

  useEffect(() => {
    if (isGoalMode) {
      setMatch(null);
      setMode("commands");
      setSkillQuery("");
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
    if (mode !== "skills") {
      return;
    }
    requestAnimationFrame(() => {
      skillSearchRef.current?.focus();
    });
  }, [mode]);

  useEffect(() => {
    if (mode !== "skills" || skillItems.length > 0 || skillsLoading) {
      return;
    }
    const requestID = ++skillsRequestRef.current;
    setSkillsLoading(true);
    setSkillsError(null);
    void (async () => {
      try {
        const nextSkills = await getAvailableSkillsApi();
        if (requestID === skillsRequestRef.current) {
          setSkillItems(nextSkills);
        }
      } catch (error) {
        if (requestID === skillsRequestRef.current) {
          setSkillsError(error instanceof Error ? error.message : "技能列表加载失败");
        }
      } finally {
        if (requestID === skillsRequestRef.current) {
          setSkillsLoading(false);
        }
      }
    })();
  }, [mode, skillItems.length, skillsLoading]);

  const close = useCallback(() => {
    setMatch(null);
    setMode("commands");
    setSkillQuery("");
    setActiveIndex(0);
  }, []);

  const openSkillsPicker = useCallback(() => {
    if (!match) {
      return;
    }
    const nextValue = input.slice(0, match.start) + input.slice(match.end);
    setInput(nextValue);
    setMatch(null);
    setMode("skills");
    setSkillQuery("");
    setActiveIndex(0);
  }, [input, match, setInput]);

  const updateForInput = useCallback((value: string) => {
    if (mode === "skills") {
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
      openSkillsPicker();
      return;
    }
    const insertion = insertSlashCommand(input, match, command);
    setInput(insertion.value);
    setMatch(null);
    setMode("commands");
    setSkillQuery("");
    requestAnimationFrame(() => {
      textareaRef.current?.setSelectionRange(
        insertion.cursorPosition,
        insertion.cursorPosition,
      );
      textareaRef.current?.focus();
    });
  }, [input, match, openSkillsPicker, setInput, textareaRef]);

  const selectSkill = useCallback((skill: SkillInfo) => {
    const commandText = formatSlashCommandInsertText(skill.name);
    const cursorPosition = Math.min(
      textareaRef.current?.selectionStart ?? input.length,
      input.length,
    );
    const nextValue = [
      input.slice(0, cursorPosition),
      commandText,
      input.slice(cursorPosition),
    ].join("");
    setInput(nextValue);
    setMode("commands");
    setSkillQuery("");
    setMatch(null);
    requestAnimationFrame(() => {
      const textarea = textareaRef.current;
      if (!textarea) {
        return;
      }
      const nextCursor = cursorPosition + commandText.length;
      textarea.setSelectionRange(nextCursor, nextCursor);
      textarea.focus();
    });
  }, [input, setInput, textareaRef]);

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

  return {
    activeIndex: visibleActiveIndex,
    close,
    commands: filteredCommands,
    handleCommandKeyDown,
    handleSkillSearchKeyDown,
    isOpen,
    mode,
    query: match?.query ?? "",
    selectCommand,
    selectSkill,
    skillCount,
    skillError: skillsError,
    skillItems: filteredSkills,
    skillLoading: skillsLoading,
    skillQuery,
    skillSearchRef,
    setSkillQuery,
    status: catalog.status,
    updateForInput,
  };
}
