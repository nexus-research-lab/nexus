import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent,
  type TransitionEvent,
} from "react";

interface GlassSwitchInteractionOptions {
  checked: boolean;
  disabled: boolean;
  onChange: (checked: boolean) => void;
}

interface GlassSwitchInteraction {
  buttonHandlers: {
    onBlur: () => void;
    onClick: () => void;
    onKeyDown: (event: KeyboardEvent<HTMLButtonElement>) => void;
    onKeyUp: (event: KeyboardEvent<HTMLButtonElement>) => void;
    onPointerCancel: () => void;
    onPointerDown: (event: PointerEvent<HTMLButtonElement>) => void;
    onPointerUp: (event: PointerEvent<HTMLButtonElement>) => void;
  };
  isPressed: boolean;
  isTransitioning: boolean;
  onThumbTransitionEnd: (event: TransitionEvent<HTMLDivElement>) => void;
}

interface GlassSwitchInteractionState {
  isPressed: boolean;
  isTransitioning: boolean;
}

const IDLE_INTERACTION: GlassSwitchInteractionState = {
  isPressed: false,
  isTransitioning: false,
};

const SWITCH_ACTIVATION_KEYS = new Set([" ", "Enter"]);

/**
 * 交互状态只响应已提交的属性变化，避免组件在 render 阶段修正自身状态。
 */
export function useGlassSwitchInteraction({
  checked,
  disabled,
  onChange,
}: GlassSwitchInteractionOptions): GlassSwitchInteraction {
  const previousCheckedRef = useRef(checked);
  const [interaction, setInteraction] = useState<GlassSwitchInteractionState>(IDLE_INTERACTION);

  useEffect(() => {
    const checkedChanged = previousCheckedRef.current !== checked;
    previousCheckedRef.current = checked;

    setInteraction((current) => {
      if (disabled) {
        return current.isPressed || current.isTransitioning
          ? IDLE_INTERACTION
          : current;
      }
      if (!checkedChanged || current.isTransitioning) {
        return current;
      }
      return { ...current, isTransitioning: true };
    });
  }, [checked, disabled]);

  const press = useCallback(() => {
    if (disabled) {
      return;
    }
    setInteraction((current) => (
      current.isPressed ? current : { ...current, isPressed: true }
    ));
  }, [disabled]);

  const release = useCallback(() => {
    setInteraction((current) => (
      current.isPressed ? { ...current, isPressed: false } : current
    ));
  }, []);

  const finishTransition = useCallback(() => {
    setInteraction((current) => (
      current.isTransitioning ? { ...current, isTransitioning: false } : current
    ));
  }, []);

  const handleClick = useCallback(() => {
    if (!disabled) {
      onChange(!checked);
    }
  }, [checked, disabled, onChange]);

  const handleKeyDown = useCallback((event: KeyboardEvent<HTMLButtonElement>) => {
    if (!disabled && SWITCH_ACTIVATION_KEYS.has(event.key)) {
      press();
    }
  }, [disabled, press]);

  const handleKeyUp = useCallback((event: KeyboardEvent<HTMLButtonElement>) => {
    if (SWITCH_ACTIVATION_KEYS.has(event.key)) {
      release();
    }
  }, [release]);

  const handlePointerDown = useCallback((event: PointerEvent<HTMLButtonElement>) => {
    if (disabled) {
      return;
    }
    event.currentTarget.setPointerCapture(event.pointerId);
    press();
  }, [disabled, press]);

  const handlePointerUp = useCallback((event: PointerEvent<HTMLButtonElement>) => {
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    release();
  }, [release]);

  const handleThumbTransitionEnd = useCallback((event: TransitionEvent<HTMLDivElement>) => {
    if (event.propertyName === "transform") {
      finishTransition();
    }
  }, [finishTransition]);

  return {
    buttonHandlers: {
      onBlur: release,
      onClick: handleClick,
      onKeyDown: handleKeyDown,
      onKeyUp: handleKeyUp,
      onPointerCancel: release,
      onPointerDown: handlePointerDown,
      onPointerUp: handlePointerUp,
    },
    isPressed: interaction.isPressed,
    isTransitioning: interaction.isTransitioning,
    onThumbTransitionEnd: handleThumbTransitionEnd,
  };
}
