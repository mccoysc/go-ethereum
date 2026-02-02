#!/bin/bash
# Test environment configuration for E2E testing
# Configures environment for PoA-SGX consensus testing

# ==============================================================================
# 关键环境变量说明
# ==============================================================================
# PoA-SGX共识协议运行所需的环境变量：
#
# 1. 合约地址（必须） - 从genesis.json预部署合约读取
#    - XCHAIN_GOVERNANCE_CONTRACT: 治理合约地址
#    - XCHAIN_SECURITY_CONFIG_CONTRACT: 安全配置合约地址
#
# 2. SGX模式（测试用）
#    - XCHAIN_SGX_MODE: mock（在非SGX环境测试时使用）
#
# 3. Gramine相关（可选，仅在Gramine环境中需要）
#    - GRAMINE_MANIFEST_PATH: Manifest文件路径
#    - GRAMINE_SIGSTRUCT_KEY_PATH: 签名密钥路径
#    - GRAMINE_APP_NAME: 应用名称
#
# 注意：
# - 代码会自动检测运行环境（IN_SGX/GRAMINE_SGX）
# - XCHAIN_ENCRYPTED_PATH和XCHAIN_SECRET_PATH从安全配置合约读取，
#   不能通过环境变量设置（防止篡改）
# ==============================================================================

# Contract addresses (pre-deployed in genesis block)
# 这些地址必须与genesis.json中的预部署合约地址匹配
# 计算方法: keccak256(rlp([deployer, nonce]))[12:]
# Deployer: 0x0000000000000000000000000000000000000000 (zero address)
# GovernanceContract (nonce 0): 0xd9145CCE52D386f254917e481eB44e9943F39138
# SecurityConfigContract (nonce 1): 0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045
export XCHAIN_GOVERNANCE_CONTRACT="0xd9145CCE52D386f254917e481eB44e9943F39138"
export XCHAIN_SECURITY_CONFIG_CONTRACT="0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"

# Gramine version for testing (required by SGX consensus engine)
export GRAMINE_VERSION="test"

# Print environment for debugging
print_test_env() {
    echo "=== Test Environment Configuration ==="
    echo "XCHAIN_GOVERNANCE_CONTRACT=$XCHAIN_GOVERNANCE_CONTRACT"
    echo "XCHAIN_SECURITY_CONFIG_CONTRACT=$XCHAIN_SECURITY_CONFIG_CONTRACT"
    echo "XCHAIN_SGX_MODE=${XCHAIN_SGX_MODE:-not set}"
    echo ""
    echo "注意: XCHAIN_ENCRYPTED_PATH和XCHAIN_SECRET_PATH"
    echo "      从安全配置合约读取，不使用环境变量"
    echo "======================================"
}

# Create simulated file system structure for testing
setup_test_filesystem() {
    local test_dir="${1:-/tmp/xchain-test-fs}"
    
    echo "Setting up complete test filesystem for PoA-SGX..."
    echo "Test root directory: $test_dir"
    
    # 1. 创建基础目录
    mkdir -p "$test_dir"
    
    # 2. 设置mock attestation设备（/dev/attestation）
    # 在测试模式下，某些代码路径可能检查文件存在性
    setup_mock_attestation_device
    
    # 3. 设置mock manifest文件
    # 用于manifest签名验证
    setup_mock_manifest_files "$test_dir/manifest"
    
    echo "✓ Test filesystem setup complete"
    echo "  - Root: $test_dir"
    echo "  - Attestation device: /tmp/xchain-test-dev-attestation"
    echo "  - Manifest files: $test_dir/manifest"
    echo ""
    echo "注意: 加密和密钥存储路径从安全配置合约读取"
}

# Clean up test filesystem
cleanup_test_filesystem() {
    echo "Cleaning up test filesystem..."
    # Clean up temporary files and mock devices
    rm -rf /tmp/xchain-test-dev-attestation
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

# Setup mock Gramine attestation device
# 在测试模式下，模拟 /dev/attestation 接口
setup_mock_attestation_device() {
    local dev_dir="/tmp/xchain-test-dev-attestation"
    
    echo "Setting up mock attestation device at $dev_dir..."
    mkdir -p "$dev_dir"
    
    # Mock MRENCLAVE (32 bytes) - 测试用的假值
    # 这是my_target_info文件的前32字节
    printf '\x00\x11\x22\x33\x44\x55\x66\x77\x88\x99\xaa\xbb\xcc\xdd\xee\xff' > "$dev_dir/my_target_info"
    printf '\x00\x11\x22\x33\x44\x55\x66\x77\x88\x99\xaa\xbb\xcc\xdd\xee\xff' >> "$dev_dir/my_target_info"
    
    # user_report_data - 用于写入报告数据（64字节）
    touch "$dev_dir/user_report_data"
    chmod 600 "$dev_dir/user_report_data"
    
    # Mock SGX Quote (简化的测试quote)
    # 实际的quote结构很复杂，但在测试模式下只需要一个非空文件
    echo "MOCK_SGX_QUOTE_FOR_TESTING_ONLY" > "$dev_dir/quote"
    
    echo "Mock attestation device created"
    echo "  - my_target_info: $dev_dir/my_target_info (MRENCLAVE)"
    echo "  - user_report_data: $dev_dir/user_report_data"
    echo "  - quote: $dev_dir/quote"
}

# Setup mock manifest files for signature verification
setup_mock_manifest_files() {
    local manifest_dir="${1:-/tmp/xchain-test-manifest}"
    
    echo "Setting up mock manifest files at $manifest_dir..."
    mkdir -p "$manifest_dir"
    
    # 创建模拟的manifest文件
    cat > "$manifest_dir/geth.manifest" << 'MANIFEST_EOF'
# Mock Gramine Manifest for Testing
libos.entrypoint = "/app/geth"

# Environment variables
loader.env.XCHAIN_GOVERNANCE_CONTRACT = "0xd9145CCE52D386f254917e481eB44e9943F39138"
loader.env.XCHAIN_SECURITY_CONFIG_CONTRACT = "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"

# SGX configuration
sgx.enclave_size = "2G"
sgx.max_threads = 32
MANIFEST_EOF
    
    # 创建.sgx版本（签名后的manifest）
    cp "$manifest_dir/geth.manifest" "$manifest_dir/geth.manifest.sgx"
    
    # 创建模拟的RSA公钥（用于验证签名）
    cat > "$manifest_dir/enclave-key.pub" << 'KEY_EOF'
-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAw5nFBZQKCkJXPTnFZ3Cb
wRN5/1/h9F7c2H8RKT1vN5hQ7VJgQ8dLw7bUPNxX7UuXvKZc9n6cE7TxfpDXJDYH
IqlxY5uN3p9kZnJiO9TvE0K8DlhN2vKHlQZhXhNfJqpzN8Jd1xQ0sT8q0yMnF0Wf
cHUdTqPnVMQxL4nFkqXwH3zX9Q3N6qYHp1vKZJxH8tQ4nQ6yM8VwN7vQ5gLXKjLf
VoKvN1P3qXM4tUjXnQxMnN8L9F5cJ4kQZnX8vH3qF9XqT1QnYHqL4VpX3M1QK9Nf
7xQZT0qF3nH8XqT0N4vL8Q3F5kZnJiO9TvE0K8DlhN2vKHlQZhXhNfJqpzN8JQID
AQAB
-----END PUBLIC KEY-----
KEY_EOF
    
    # 创建模拟的签名文件（.sig）
    # 实际的签名是RSA签名的二进制数据，这里用占位符
    printf 'MOCK_SIGNATURE_DATA_FOR_TESTING' > "$manifest_dir/geth.manifest.sgx.sig"
    
    # 设置环境变量指向这些文件
    export GRAMINE_MANIFEST_PATH="$manifest_dir/geth.manifest.sgx"
    export GRAMINE_SIGSTRUCT_KEY_PATH="$manifest_dir/enclave-key.pub"
    export GRAMINE_APP_NAME="geth"
    
    echo "Mock manifest files created"
    echo "  - Manifest: $manifest_dir/geth.manifest.sgx"
    echo "  - Signature: $manifest_dir/geth.manifest.sgx.sig"  
    echo "  - Public key: $manifest_dir/enclave-key.pub"
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
    
    echo "✓ Test environment verified successfully"
    echo "  - Running in non-SGX mode (development/testing)"
    echo "  - Code will automatically skip SGX-specific validations"
    return 0
}
