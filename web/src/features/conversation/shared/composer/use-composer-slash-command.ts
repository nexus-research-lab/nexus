"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import type {
  Dispatch,
  KeyboardEvent,
  RefObject,
  SetStateAction,
} from "react";

import type {
  CommandCatalogData,
  CommandDescriptor,
} from "@/types/generated/protocol";

import {
  filterSlashCommands,
  findSlashCommandTextMatch,
  insertSlashCommand,
  isSelectableSlashCommand,
  SLASH_COMMAND_NAVIGATION_KEYS,
  type SlashCommandTextMatch,
} from "./slash-command-model";

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
  const [activeIndex, setActiveIndex] = useState(0);
  const filteredCommands = useMemo(
    () => filterSlashCommands(catalog.commands, match?.query ?? ""),
    [catalog.commands, match?.query],
  );
  const visibleActiveIndex = Math.min(
    activeIndex,
    Math.max(filteredCommands.length - 1, 0),
  );
  const activeCommand = filteredCommands[visibleActiveIndex] ?? null;
  const isOpen = Boolean(match);

  useEffect(() => {
    setActiveIndex(0);
  }, [catalog.revision, match?.query]);

  useEffect(() => {
    if (isGoalMode) {
      setMatch(null);
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
    }
  }, [input, isGoalMode, match, textareaRef]);

  const close = useCallback(() => {
    setMatch(null);
  }, []);

  const updateForInput = useCallback((value: string) => {
    const cursorPosition = textareaRef.current?.selectionStart ?? value.length;
    setMatch(findSlashCommandTextMatch(
      value,
      cursorPosition,
      !isGoalMode,
    ));
  }, [isGoalMode, textareaRef]);

  const select = useCallback((command: CommandDescriptor) => {
    if (!match || !isSelectableSlashCommand(command)) {
      return;
    }
    const insertion = insertSlashCommand(input, match, command);
    setInput(insertion.value);
    setMatch(null);
    requestAnimationFrame(() => {
      textareaRef.current?.setSelectionRange(
        insertion.cursorPosition,
        insertion.cursorPosition,
      );
      textareaRef.current?.focus();
    });
  }, [input, match, setInput, textareaRef]);

  const handleKeyDown = useCallback((
    event: KeyboardEvent<HTMLTextAreaElement>,
  ): boolean => {
    if (!match || !SLASH_COMMAND_NAVIGATION_KEYS.has(event.key)) {
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
      select(activeCommand);
      return true;
    }
    // 未匹配或目录尚未返回时保留普通发送/焦点行为，用户仍可手动发送 slash 文本。
    return false;
  }, [
    activeCommand,
    close,
    filteredCommands.length,
    match,
    select,
  ]);

  return {
    activeIndex: visibleActiveIndex,
    close,
    commands: filteredCommands,
    handleKeyDown,
    isOpen,
    query: match?.query ?? "",
    select,
    status: catalog.status,
    updateForInput,
  };
}
