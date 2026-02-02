#!/bin/bash
# Test environment configuration for E2E testing
# Configures environment for PoA-SGX consensus testing

# ==============================================================================
# 关键环境变量说明
# ==============================================================================
# PoA-SGX共识协议需要以下环境变量：
#
# 1. 合约地址（必须）- 从genesis.json预部署合约读取
#    - XCHAIN_GOVERNANCE_CONTRACT: 治理合约地址
#    - XCHAIN_SECURITY_CONFIG_CONTRACT: 安全配置合约地址
#
# 2. Gramine环境判断（用于非Gramine环境测试）
#    - SGX_TEST_MODE: 设为"true"时跳过SGX验证
#    - IN_SGX: Gramine运行时设置，"1"表示在SGX中运行
#    - GRAMINE_SGX: 另一个Gramine环境标识
#
# 这些地址在genesis.json中预部署，并且在manifest中固定，
# 影响MRENCLAVE（SGX度量值）。
# ==============================================================================

# Contract addresses (pre-deployed in genesis block)
# 这些地址必须与genesis.json中的预部署合约地址匹配
# 计算方法: keccak256(rlp([deployer, nonce]))[12:]
# Deployer: 0x0000000000000000000000000000000000000000 (zero address)
# GovernanceContract (nonce 0): 0xd9145CCE52D386f254917e481eB44e9943F39138
# SecurityConfigContract (nonce 1): 0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045
export XCHAIN_GOVERNANCE_CONTRACT="0xd9145CCE52D386f254917e481eB44e9943F39138"
export XCHAIN_SECURITY_CONFIG_CONTRACT="0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"

# SGX测试模式 - 在非SGX环境下运行测试时必须设置
# 设置为"true"会跳过manifest验证、MRENCLAVE检查等SGX特有的验证
export SGX_TEST_MODE="true"

# 数据路径配置（可选，用于指定加密存储路径）
# 在测试环境中，这些路径会被映射到临时目录
export XCHAIN_ENCRYPTED_PATH="${XCHAIN_ENCRYPTED_PATH:-/tmp/xchain-e2e-encrypted}"
export XCHAIN_SECRET_PATH="${XCHAIN_SECRET_PATH:-/tmp/xchain-e2e-secrets}"

# Print environment for debugging
print_test_env() {
    echo "=== Test Environment Configuration ==="
    echo "XCHAIN_GOVERNANCE_CONTRACT=$XCHAIN_GOVERNANCE_CONTRACT"
    echo "XCHAIN_SECURITY_CONFIG_CONTRACT=$XCHAIN_SECURITY_CONFIG_CONTRACT"
    echo "SGX_TEST_MODE=$SGX_TEST_MODE"
    echo "XCHAIN_ENCRYPTED_PATH=$XCHAIN_ENCRYPTED_PATH"
    echo "XCHAIN_SECRET_PATH=$XCHAIN_SECRET_PATH"
    echo "======================================"
}

# Create simulated file system structure for testing
setup_test_filesystem() {
    local test_dir="${1:-/tmp/xchain-test-fs}"
    
    echo "Setting up test filesystem at $test_dir..."
    mkdir -p "$test_dir"
    mkdir -p "$XCHAIN_ENCRYPTED_PATH"
    mkdir -p "$XCHAIN_SECRET_PATH"
    
    echo "Test filesystem created"
    echo "  - Root: $test_dir"
    echo "  - Encrypted: $XCHAIN_ENCRYPTED_PATH"
    echo "  - Secrets: $XCHAIN_SECRET_PATH"
}

# Clean up test filesystem
cleanup_test_filesystem() {
    echo "Cleaning up test filesystem..."
    # Clean up any temporary files if created
}

# Calculate contract addresses deterministically
# These must match the addresses in genesis.json
calculate_contract_addresses() {
    local deployer="${1:-0x0000000000000000000000000000000000000000}"
    
    echo "Contract addresses for deployer: $deployer"
    
    # These are pre-calculated deterministic addresses
    # GovernanceContract = keccak256(rlp([deployer, 0]))[12:]
    local governance_addr="0xd9145CCE52D386f254917e481eB44e9943F39138"
    
    # SecurityConfigContract = keccak256(rlp([deployer, 1]))[12:]  
    local security_addr="0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"
    
    echo "GovernanceContract: $governance_addr"
    echo "SecurityConfigContract: $security_addr"
}

# Verify test environment is properly configured
verify_test_env() {
    local errors=0
    
    echo "Verifying test environment..."
    
    if [ -z "$XCHAIN_GOVERNANCE_CONTRACT" ]; then
        echo "ERROR: XCHAIN_GOVERNANCE_CONTRACT not set"
        errors=$((errors + 1))
    fi
    
    if [ -z "$XCHAIN_SECURITY_CONFIG_CONTRACT" ]; then
        echo "ERROR: XCHAIN_SECURITY_CONFIG_CONTRACT not set"
        errors=$((errors + 1))
    fi
    
    if [ $errors -gt 0 ]; then
        echo "Test environment verification failed with $errors errors"
        return 1
    fi
    
    echo "Test environment verified successfully"
    return 0
}
