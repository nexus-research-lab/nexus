import { useState, type Dispatch, type SetStateAction } from "react";

export function useResettableState<T>(
  initial_value: T,
  reset_key: unknown,
): [T, Dispatch<SetStateAction<T>>] {
  const [state, set_state] = useState(initial_value);
  const [state_reset_key, set_state_reset_key] = useState(reset_key);

  if (!Object.is(state_reset_key, reset_key)) {
    set_state_reset_key(reset_key);
    set_state(initial_value);
    return [initial_value, set_state];
  }

  return [state, set_state];
}
