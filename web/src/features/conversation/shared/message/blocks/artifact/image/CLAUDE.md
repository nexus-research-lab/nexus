# image/

L7 | 父级: web/src/features/conversation/shared/message/blocks/artifact

## 职责

- `image-artifact-model.ts`: 按优先级解析内联、远程、工作区和原始图片来源
- `image-block.tsx`: 展示图片、缺失态、标题及工作区动作

图片来源解析使用有序解析器，新增来源不得扩展条件链。工作区动作复用 Artifact 根域能力。
