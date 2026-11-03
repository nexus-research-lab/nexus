/**
 * INPUT: Saved template identity and typed parameter definitions.
 * OUTPUT: Editable invocation example with defaults and required inputs.
 * POS: Shared copy example for capability/template consumers; no execution or shell interpolation.
 */
import type { WorkGraphWorkflow } from "@/types/conversation/workgraph-workflow";

export function workgraphInvocationExample(workflow: WorkGraphWorkflow): string {
  if (!workflow.parameters?.length) return `/${workflow.slash_name} `;
  const parameters: Record<string, string | number | boolean | string[]> = {};
  for (const parameter of workflow.parameters) {
    if (parameter.default !== undefined) {
      parameters[parameter.name] = parameter.default;
    } else if (parameter.required) {
      switch (parameter.type) {
        case "string": parameters[parameter.name] = parameter.enum?.[0] ?? "…"; break;
        case "boolean": parameters[parameter.name] = false; break;
        case "string_list": parameters[parameter.name] = []; break;
        case "integer": parameters[parameter.name] = Math.ceil(parameter.minimum ?? Math.min(parameter.maximum ?? 0, 0)); break;
        case "number": parameters[parameter.name] = parameter.minimum ?? Math.min(parameter.maximum ?? 0, 0); break;
      }
    }
  }
  return `/${workflow.slash_name} ${JSON.stringify({ parameters, request: "" })}`;
}
