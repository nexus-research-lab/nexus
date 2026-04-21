# Skill 规范

## 1. 文档目标

本文档定义 skill 的当前有效结构：

- skill 从哪里来
- runtime 实际消费哪一层
- 哪些属于公开能力，哪些属于系统内部能力

## 2. skill 的几层形态

### 2.1 源目录

- 仓库内置 skill
- 外部导入到本地注册表的 skill

### 2.2 workspace 部署副本

- `<workspace>/.agents/skills/<skill_name>/`

这是运行时真正消费的 skill 副本。

### 2.3 runtime 发现入口

- `<workspace>/.claude/skills/<skill_name>`

它是指向 `.agents/skills/<skill_name>` 的入口，不应承载真实源文件。

## 3. 当前真相源

### 3.1 执行真相源

skill 执行真相源是文件系统，不是数据库记录。

### 3.2 控制面来源

控制面元数据来自：

- 内置 catalog
- 外部 registry manifest
- workspace 实际部署状态

## 4. skill 分类

### 4.1 public skill

- 对外可见
- 可在能力页 / marketplace 中展示

### 4.2 internal skill

- 系统内部使用
- 不进入普通公开 catalog

### 4.3 system managed skill

- 平台托管
- 可自动补齐到指定 workspace

## 5. 生命周期

### 5.1 导入

外部 skill 导入后，先进入本地 registry，再决定是否部署到 workspace。

### 5.2 部署

部署时：

- 复制到 `.agents/skills`
- 建立 `.claude/skills` 入口

### 5.3 卸载

卸载只清理 workspace 部署副本，不直接破坏源目录或 registry。

### 5.4 更新

更新先刷新源，再同步到已部署 workspace。

## 6. 当前约束

- skill 运行时只读 workspace 部署副本
- `.claude/skills` 只是发现入口
- internal skill 不应以公开 marketplace 形式暴露

## 7. 禁止项

- 直接把真实 skill 文件写进 `.claude/skills`
- 让数据库记录脱离文件系统单独宣称“已安装”
- 把 internal skill 混进 public catalog

## 8. 一句话总结

skill 是文件系统实体，catalog 和 UI 只是它的投影层。
