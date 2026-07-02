import { useState, type Dispatch, type SetStateAction } from "react";

export function useResettableState<T>(
  initialValue: T,
  resetKey: unknown,
): [T, Dispatch<SetStateAction<T>>] {
  const [state, setState] = useState(initialValue);
  const [stateResetKey, setStateResetKey] = useState(resetKey);

  if (!Object.is(stateResetKey, resetKey)) {
    setStateResetKey(resetKey);
    setState(initialValue);
    return [initialValue, setState];
  }

  return [state, setState];
}
