# Manifest 参数处理机制

## 设计规范

Manifest 参数为权威来源，启动时直接使用，忽略 CLI 同名参数。

### 处理流程

1. 加载 Manifest 环境变量参数
2. 加载 CLI 命令行参数  
3. 合并：Manifest 参数直接作为最终值，CLI 同名参数被忽略

### 实现要求

```go
// MergeAndValidate 实现
func (pv *ParamValidator) MergeAndValidate() error {
    // Manifest 参数优先
    for name, value := range pv.manifestParams {
        pv.mergedParams[name] = value
    }
    
    // 处理 CLI 参数
    for cliFlag, cliValue := range pv.cliParams {
        for _, param := range SecurityParams {
            if param.CliFlag == cliFlag {
                if _, exists := pv.manifestParams[param.Name]; exists {
                    // Manifest 已定义，忽略 CLI
                    goto nextParam
                }
                pv.mergedParams[param.Name] = cliValue
                goto nextParam
            }
        }
        pv.mergedParams[cliFlag] = cliValue
    nextParam:
    }
    return nil
}
```

### 技术依据

Manifest 参数嵌入 enclave 镜像，影响 MRENCLAVE 度量值，外部无法篡改。

## 文档修正说明

模块 06 文档 `/docs/modules/06-data-storage-sync.md` 原描述为参数比对机制（比对 Manifest 与 CLI，不一致则退出），现已修正为参数覆盖机制（Manifest 直接覆盖 CLI 同名参数）。

术语统一："参数校验" 修正为 "参数处理"。

---

**修正日期**：2026-02-01 | **PR**：copilot/clarify-manifest-parameter-handling
