// INPUT: 最近 UiField 的精确控件 ID、可见说明/错误与调用方的 ARIA 属性。
// OUTPUT: 公共字段控件的声明式描述、错误关联和原生校验身份。
// POS: Field 与 Input/Select trigger 共用的内部关联合同；不猜测复合字段的主控件或校验业务值。

import { createContext, useContext, useId, type AriaAttributes } from "react";

interface FieldAccessibility {
  controlId?: string;
  descriptionId?: string;
  errorId: string;
  hasError: boolean;
  nativeInvalidControlId?: string;
}

type FieldControlAttributes = Pick<AriaAttributes,
  "aria-describedby" | "aria-errormessage" | "aria-invalid"
> & { id?: string };

export const FIELD_ACCESSIBILITY_CONTEXT = createContext<FieldAccessibility | null>(null);

/** 只有显式 htmlFor/id 配对接收字段说明和业务错误；原生 invalid 精确指向实际触发控件。 */
export function useFieldControlAttributes(props: FieldControlAttributes): FieldControlAttributes {
  const field = useContext(FIELD_ACCESSIBILITY_CONTEXT);
  const generatedId = useId();
  const id = props.id ?? (field ? generatedId : undefined);
  const isBound = Boolean(field?.controlId && field.controlId === id);
  const isInvalid = Boolean(field && (
    (isBound && field.hasError) || field.nativeInvalidControlId === id
  ));
  const descriptionId = isBound ? field?.descriptionId : undefined;
  const descriptions = [props["aria-describedby"], descriptionId].filter(Boolean).join(" ").split(/\s+/).filter(Boolean);

  return {
    id,
    "aria-describedby": descriptions.length ? [...new Set(descriptions)].join(" ") : undefined,
    "aria-errormessage": isInvalid ? field?.errorId : props["aria-errormessage"],
    "aria-invalid": isInvalid ? true : props["aria-invalid"],
  };
}
