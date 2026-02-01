# Manifest 参数处理机制澄清

## 问题

模块启动时读 manifest 并以 manifest 为准，是否应直接忽略命令行传入的同名参数，无需比对？

## 结论

是。Manifest 参数直接作为最终值，CLI 同名参数被忽略。

## 方案对比

**方案 A（文档原描述）**：加载 Manifest → 加载 CLI → 比对不一致则退出

**方案 B（修正后采用）**：加载 Manifest → 加载 CLI → Manifest 覆盖，忽略 CLI 同名参数

理由：Manifest 参数嵌入 enclave 镜像，影响 MRENCLAVE，外部无法篡改。无需比对用户输入。

## 主要修改

### MergeAndValidate 函数

修正前：
```go
if ok && cliValue != manifestValue {
    return fmt.Errorf("SECURITY VIOLATION: ...")
}
```

修正后：
```go
if _, exists := pv.manifestParams[param.Name]; exists {
    goto nextParam  // 直接忽略 CLI 参数
}
```

### 术语调整

- "参数校验" → "参数处理"
- "不一致则退出进程" → "忽略 CLI 同名参数"

## 影响范围

**文档修正**：`/docs/modules/06-data-storage-sync.md`

**代码**：无。本次仅修正文档设计描述。

## 实现要求

待实现代码需遵循：Manifest 参数直接覆盖 CLI 同名参数，无比对，无退出。

---

**修正日期**：2026-02-01 | **PR**：copilot/clarify-manifest-parameter-handling
