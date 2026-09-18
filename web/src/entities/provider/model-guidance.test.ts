import { expect, it } from "vitest";
import { modelGuidanceHint, modelGuidanceLabel, recommendedModelsFirst } from "./model-guidance";
import type { ModelGuidance } from "@/types/capability/provider";
const guidance: ModelGuidance = {
 catalog_version: "test", capabilities: { vision: true, image_output: true }, sources: {},
 eligibility: {chat:{available:true}, vision:{available:true}, image_generation:{available:true}, image_editing:{available:false}},
 recommendations: { image_generation: "image_generation" },
};
it("uses purpose-specific advice independently of capability badges", () => {
 const t = (key: string) => key;
 expect(modelGuidanceLabel(guidance,"chat",t)).not.toContain("model_recommended");
 expect(modelGuidanceLabel(guidance,"image_generation",t)).toContain("model_recommended");
 expect(modelGuidanceLabel(guidance,"chat",t)).toContain("model_multimodal");
 expect(modelGuidanceLabel(undefined,"chat",t)).toBe("");
});
it("sorts recommendations without mutating the original or saved default", () => {
 const models=[{id:"custom",is_default:true,guidance:undefined},{id:"image",is_default:false,guidance}];
 expect(recommendedModelsFirst(models,"image_generation").map(m=>m.id)).toEqual(["image","custom"]);
 expect(models[0].id).toBe("custom");
 expect(models[0].is_default).toBe(true);
 expect(recommendedModelsFirst(models,"chat")).toEqual(models);
});

it("does not equate missing vision with text-only", () => {
 const t = (key: string) => key;
 expect(modelGuidanceLabel({...guidance,capabilities:{},recommendations:{}},"chat",t)).not.toContain("model_text_only");
 expect(modelGuidanceLabel({...guidance,capabilities:{vision:false},text_only:true},"chat",t)).toContain("model_text_only");
});

it("explains recommendations and plan restrictions without claiming access", () => {
 const t = (key: string) => key;
 const value = {...guidance,recommendations:{chat:"flagship"},evidence:{reviewed_at:"2026-09-17",urls:[],notice:"kimi_k3"}};
 expect(modelGuidanceHint(value,"chat",t)).toContain("advice_flagship");
 expect(modelGuidanceHint(value,"chat",t)).toContain("advice_kimi_k3");
 expect(modelGuidanceHint(value,"chat",t)).toContain("2026-09-17");
 expect(modelGuidanceHint(value,"image_generation",t)).not.toContain("advice_flagship");
});
