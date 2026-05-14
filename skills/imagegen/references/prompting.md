# Prompting Reference

本文件对齐 Codex imagegen sample 的 prompting 规则，但去掉底层模型/API 分支。Nexus skill 只负责组织图片任务；实际 Provider 调用由 Go 服务处理。

## 结构

- 使用稳定顺序：场景/背景 -> 主体 -> 关键细节 -> 约束 -> 输出用途。
- 复杂请求使用短标签行，不要写成长段自然语言。
- 包含 intended use，例如广告、UI mockup、信息图、产品图，以便生成合适的精度和构图。
- 有输入图时明确标注每张图的角色：编辑目标、风格参考、构图参考、插入对象、mask。

## 具体度策略

- 如果用户 prompt 已经具体，保留具体度，只做结构化、排序和消歧。
- 如果用户 prompt 很泛，可以补充构图、媒介、光线、用途、留白、质量约束和避免项。
- 示例是完整 recipe，不代表每次都要扩写到同样长度。

可以补充：

- 构图和取景。
- 使用场景或打磨程度。
- 版式和可读性约束。
- 支持用户目标的合理场景细节。

不要补充：

- 用户没有暗示的人物、物体、品牌、Logo、标语或剧情。
- 任意配色、品牌调性或左右位置。
- 与仓库现有设计系统冲突的风格。

## 通用 Prompt Schema

```text
Use case: <taxonomy slug>
Asset type: <图片会用于哪里>
Primary request: <用户核心需求>
Input images: <Image 1: role; Image 2: role> (optional)
Scene/backdrop: <环境或背景>
Subject: <主体>
Style/medium: <照片/插画/3D/像素/贴纸/UI 等>
Composition/framing: <构图、视角、留白>
Lighting/mood: <光线与氛围>
Color palette: <必要时说明>
Materials/textures: <必要时说明>
Text (verbatim): "<必须出现的精确文字>"
Constraints: <必须保留或必须避免>
Avoid: no watermark, no random text, no unintended logos
```

说明：

- `Asset type`、`Input images`、`Scene/backdrop` 都只是 prompt 脚手架，不是 CLI 参数。
- 执行参数只通过 CLI flag 表达，例如 `--size`、`--quality`、`--output-format`、`--image-path`、`--mask-path`。
- 有文字要求时，把必须出现的文字放到 `Text (verbatim)`，要求精确渲染且不要出现其他随机文字。
- 对编辑任务，重复 invariants：`change only X; keep Y unchanged`。

## Use-case Taxonomy

生成类：

- `photorealistic-natural`：真实照片、生活方式、自然光、真实材质。
- `product-mockup`：产品图、包装图、目录图、商品概念。
- `ui-mockup`：应用/Web 界面 mockup、产品截图风格、线框图。
- `infographic-diagram`：信息图、结构图、流程图、带标签说明图。
- `scientific-educational`：科学教育图、课堂讲义、带准确标签的解释图。
- `ads-marketing`：广告视觉、活动图、宣传海报、社媒图。
- `productivity-visual`：幻灯片、图表、工作流、数据型商务视觉。
- `logo-brand`：Logo/标志探索，倾向可矢量化。
- `illustration-story`：漫画、儿童书、叙事插画。
- `stylized-concept`：风格概念图、3D/stylized render、场景设定。
- `historical-scene`：历史场景，强调时代准确性。

编辑类：

- `text-localization`：替换/翻译图中文字，保持版式和层级。
- `identity-preserve`：人物/主体身份保持，改变局部元素。
- `precise-object-edit`：精确移除、替换或修改对象。
- `lighting-weather`：只改时间、季节、光照、天气或氛围。
- `background-extraction`：透明背景、干净 cutout。
- `style-transfer`：风格迁移，保留指定构图或主体。
- `compositing`：多图合成，匹配光照、透视和比例。
- `sketch-to-render`：草图/线稿转渲染，保持比例和透视。

## 文本准确性

- 必须出现的文字放在 `Text (verbatim)`，用引号包住。
- 要求不要生成随机文字、额外标语或水印。
- 不常见词、品牌名、产品名要重复强调“exactly as written”。
- 信息图、海报、UI mockup 需要明确层级、留白和可读性。

## 输入图和编辑

- 不要默认每张输入图都是编辑目标。
- 编辑目标要写清楚：`Image 1 is the edit target`。
- 风格参考要写清楚：`Image 2 is style reference only`。
- 合成任务要描述图片之间的关系，例如把 Image 2 的主体放入 Image 1。
- 每次迭代都重复保持项，减少主体漂移和构图漂移。

## 透明图

简单不透明主体优先使用 chroma key 后处理：

1. 生成时要求主体在完全平整的纯色背景上，默认 `#00ff00`；如果主体是绿色，用 `#ff00ff`。
2. 明确要求背景无阴影、无渐变、无纹理、无地面、无反射、无光照变化。
3. 要求主体边缘清晰、周围留白充足，主体中不要出现 key color。
4. 生成后使用本 skill 的后处理脚本移除背景：

```bash
python .agents/skills/imagegen/scripts/remove_chroma_key.py \
  --input <source> \
  --out <final.png> \
  --auto-key border \
  --soft-matte \
  --transparent-threshold 12 \
  --opaque-threshold 220 \
  --despill
```

5. 检查输出是否有 alpha 通道、四角透明、主体覆盖合理、没有明显 key color 边缘。
6. 如果还有细边，最多重试一次 `--edge-contract 1`；边缘阶梯感明显时可谨慎加 `--edge-feather 0.25`。

复杂透明图，例如头发、毛发、烟雾、玻璃、液体、半透明材质、强反光物体或软阴影，先说明 chroma key 可能不稳，再让用户确认是否接受局部瑕疵或改成非透明背景方案。不要在 skill 里根据底层生成能力自动切换路线。

透明图 prompt 片段：

```text
Create the requested subject on a perfectly flat solid #00ff00 chroma-key background for background removal.
The background must be one uniform color with no shadows, gradients, texture, reflections, floor plane, or lighting variation.
Keep the subject fully separated from the background with crisp edges and generous padding.
Do not use #00ff00 anywhere in the subject.
No cast shadow, no contact shadow, no reflection, no watermark, and no text unless explicitly requested.
```

## 输出验收

- 主体是否符合用户核心需求。
- 风格、构图、取景是否可用于目标场景。
- 有文字时检查拼写、额外文字和可读性。
- 编辑任务检查未选区域是否被意外改变。
- 透明图检查 alpha 通道、透明边角、边缘 halo 和 key color 残留。
- 项目引用资产必须在 workspace 中，不要只留临时目录。
