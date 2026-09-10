// INPUT: 本地初始值与由调用者定义身份的 reset key。
// OUTPUT: key 改变时在同一渲染中重置的本地状态及标准 setter。
// POS: 中立 React 状态适配；不解释业务身份或持久化草稿。
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
