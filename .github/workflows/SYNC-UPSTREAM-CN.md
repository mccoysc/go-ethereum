# 上游同步工作流说明

## 概述

本项目已创建GitHub Actions工作流，用于自动检测上游仓库（ethereum/go-ethereum）的新版本发布，并将最新代码同步到本仓库。

## 工作流位置

- 工作流文件：`.github/workflows/sync-upstream.yml`
- 文档文件：`.github/workflows/README.md`

## 功能特性

### 1. 自动检测
- 每天UTC时间00:00自动运行
- 检测上游仓库的最新release标签

### 2. 自动合并
- 当检测到新的release时，自动尝试合并到本仓库
- 合并成功后自动推送到GitHub

### 3. 冲突处理
- 如果合并遇到冲突，会自动创建一个issue
- issue包含详细的手动解决冲突的步骤说明
- 使用`upstream-sync`和`merge-conflict`标签标记

### 4. 手动触发
- 可以在GitHub Actions页面手动触发工作流
- 路径：Actions → Sync with Upstream Releases → Run workflow

## 工作流程

1. **检出代码**：获取完整的仓库历史
2. **添加上游远程**：添加ethereum/go-ethereum作为upstream
3. **检测新版本**：获取上游最新的release标签
4. **尝试合并**：
   - 如果发现新版本，尝试自动合并
   - 合并成功：推送更改到仓库
   - 遇到冲突：创建issue提供手动解决指南
5. **生成摘要**：在工作流运行页面显示同步结果摘要

## 手动解决冲突

如果自动合并失败，请按照以下步骤手动解决：

```bash
# 添加上游远程仓库
git remote add upstream https://github.com/ethereum/go-ethereum.git

# 获取上游标签
git fetch upstream --tags

# 合并指定版本（将<tag-name>替换为实际的版本标签）
git merge <tag-name>

# 解决冲突后提交
git add .
git commit
git push
```

## 安全性

- 工作流使用最小权限原则
- 仅需要`contents: write`和`issues: write`权限
- 使用GitHub官方的actions（v4/v5/v7版本）
- 已通过CodeQL安全检查

## 注意事项

- 工作流仅同步release标签，不同步所有提交
- 合并冲突必须手动解决
- 工作流需要`GITHUB_TOKEN`具有写入权限
- 建议定期检查issues中是否有合并冲突通知
