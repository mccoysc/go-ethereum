// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.

package sgx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// VerifyManifestFileHash 验证整个manifest.sgx文件的哈希
// 这确保SIGSTRUCT和TOML内容都未被篡改
func VerifyManifestFileHash(manifestPath string, expectedHash string) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}
	
	hash := sha256.Sum256(data)
	actualHash := hex.EncodeToString(hash[:])
	
	if actualHash != expectedHash {
		return fmt.Errorf("manifest file hash mismatch: expected %s, got %s",
			expectedHash, actualHash)
	}
	
	return nil
}

// ConfigFromMREnclave 从MRENCLAVE获取配置
// 配置在构建时嵌入MRENCLAVE（通过环境变量），因此MRENCLAVE唯一确定配置
type MREnclaveConfig struct {
	MRENCLAVE              string
	GovernanceContract     string
	SecurityConfigContract string
}

// GetConfigByMREnclave 通过MRENCLAVE查找配置
// 这避免从可能被篡改的manifest TOML读取配置
func GetConfigByMREnclave(mrenclave []byte) (*MREnclaveConfig, error) {
	mrenclaveHex := hex.EncodeToString(mrenclave)
	
	// 从已知配置表查找
	config, ok := knownMREnclaveConfigs[mrenclaveHex]
	if !ok {
		return nil, fmt.Errorf("unknown MRENCLAVE: %s (not in trusted config table)", mrenclaveHex)
	}
	
	return &config, nil
}

// 已知的MRENCLAVE到配置的映射
// 这些值在构建时生成并嵌入代码
var knownMREnclaveConfigs = map[string]MREnclaveConfig{
	// Test manifest MRENCLAVE
	"faa284c4d200890541c4515810ef8ad2065c18a4c979cfb1e16ee5576fe014ee": {
		MRENCLAVE:              "faa284c4d200890541c4515810ef8ad2065c18a4c979cfb1e16ee5576fe014ee",
		GovernanceContract:     "0xd9145CCE52D386f254917e481eB44e9943F39138",
		SecurityConfigContract: "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
	},
	// 可以添加更多生产环境的MRENCLAVE配置
}
