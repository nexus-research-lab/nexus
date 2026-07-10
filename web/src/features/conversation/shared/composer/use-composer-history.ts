import { useCallback, useState } from "react";
import type { Dispatch, SetStateAction } from "react";

interface UseComposerHistoryOptions {
  clearError: () => void;
  input: string;
  setInput: Dispatch<SetStateAction<string>>;
}

const MAX_HISTORY_ITEMS = 50;

export function useComposerHistory({
  clearError,
  input,
  setInput,
}: UseComposerHistoryOptions) {
  const [items, setItems] = useState<string[]>([]);
  const [index, setIndex] = useState(-1);
  const [draft, setDraft] = useState("");

  const record = useCallback((value: string) => {
    if (value) {
      setItems((current) => [
        value,
        ...current.slice(0, MAX_HISTORY_ITEMS - 1),
      ]);
    }
    setIndex(-1);
    setDraft("");
  }, []);

  const recallPrevious = useCallback(() => {
    if (items.length === 0) {
      return;
    }
    if (index < 0) {
      setDraft(input);
    }
    const nextIndex = Math.min(index + 1, items.length - 1);
    setIndex(nextIndex);
    setInput(items[nextIndex] ?? "");
    clearError();
  }, [clearError, index, input, items, setInput]);

  const recallNext = useCallback(() => {
    if (index > 0) {
      const nextIndex = index - 1;
      setIndex(nextIndex);
      setInput(items[nextIndex] ?? "");
      return;
    }
    if (index === 0) {
      setIndex(-1);
      setInput(draft);
      setDraft("");
    }
  }, [draft, index, items, setInput]);

  return {
    index,
    itemCount: items.length,
    recallNext,
    recallPrevious,
    record,
  };
}
