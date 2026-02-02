#!/bin/bash
# Test environment configuration for E2E testing
# Configures environment for PoA-SGX consensus testing

# ==============================================================================
# 环境配置说明
# ==============================================================================
# PoA-SGX共识运行在生产模式，测试通过mock环境文件实现
#
# 1. Manifest文件 - 提供安全配置合约地址
#    - GRAMINE_MANIFEST_PATH: 指向包含合约地址的manifest文件
#
# 2. Mock SGX设备 - 模拟/dev/attestation
#    - 创建mock MRENCLAVE文件
#    - 创建可写的user_report_data
#    - 创建quote输出文件
#
# 3. 白名单配置 - 代表genesis alloc storage
#    - XCHAIN_CONTRACT_MRENCLAVES: 预设的MRENCLAVE白名单（逗号分隔）
#    - XCHAIN_CONTRACT_MRSIGNERS: 预设的MRSIGNER白名单（逗号分隔）
#
# 4. Gramine环境
#    - GRAMINE_VERSION: 版本标识
#    - SGX_TEST_MODE: true（跳过某些硬件检查）
# ==============================================================================

# Contract addresses from genesis.json
export XCHAIN_GOVERNANCE_CONTRACT="0xd9145CCE52D386f254917e481eB44e9943F39138"
export XCHAIN_SECURITY_CONFIG_CONTRACT="0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"

# Gramine environment (required)
export GRAMINE_VERSION="1.0-test"
export SGX_TEST_MODE="true"

# Mock whitelist from contract storage (represents genesis alloc storage)
# These values should match the mock MRENCLAVE/MRSIGNER in attestation device
export XCHAIN_CONTRACT_MRENCLAVES="40807cade135f3346f59c3b40a45b8cf0ecc262e1b172afc62b82232e662c78a"
export XCHAIN_CONTRACT_MRSIGNERS="68192bc24bc4c220898e2f96d1ebeebd4d8ec778db7891231c55b17d0d0f8983"

# Intel SGX API key for PCCS
export INTEL_SGX_API_KEY="${INTEL_SGX_API_KEY:-a8ece8747e7b4d8d98d23faec065b0b8}"

# Print environment for debugging
print_test_env() {
    echo "=== Test Environment Configuration ==="
    echo "XCHAIN_GOVERNANCE_CONTRACT=$XCHAIN_GOVERNANCE_CONTRACT"
    echo "XCHAIN_SECURITY_CONFIG_CONTRACT=$XCHAIN_SECURITY_CONFIG_CONTRACT"
    echo "XCHAIN_SGX_MODE=${XCHAIN_SGX_MODE:-not set}"
    echo "INTEL_SGX_API_KEY=${INTEL_SGX_API_KEY:0:8}... (first 8 chars)"
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
    
    # 2. 创建/dev/attestation设备（Gramine标准路径）
    # 需要sudo权限创建/dev下的目录
    setup_dev_attestation
    
    # 3. 设置mock manifest文件（用于manifest签名验证）
    setup_mock_manifest_files "$test_dir/manifest"
    
    echo "✓ Test filesystem setup complete"
    echo "  - Root: $test_dir"
    echo "  - Attestation device: /dev/attestation"
    echo "  - Manifest files: $test_dir/manifest"
    echo ""
    echo "注意: 加密和密钥存储路径从安全配置合约读取"
}

# Setup /dev/attestation device (Gramine standard path)
# Creates mock files that satisfy Gramine's interface requirements
setup_dev_attestation() {
    echo "Setting up /dev/attestation (Gramine standard path)..."
    
    # Create /dev/attestation directory with sudo
    sudo mkdir -p /dev/attestation
    sudo chmod 755 /dev/attestation
    
    # Create my_target_info file (contains MRENCLAVE)
    # Format: first 32 bytes are MRENCLAVE, rest is padding
    # Total size should be at least 512 bytes (SGX target_info structure)
    local target_info_file=$(mktemp)
    
    # Write mock MRENCLAVE (32 bytes) - deterministic test value
    printf '\x00\x11\x22\x33\x44\x55\x66\x77\x88\x99\xaa\xbb\xcc\xdd\xee\xff' > "$target_info_file"
    printf '\x00\x11\x22\x33\x44\x55\x66\x77\x88\x99\xaa\xbb\xcc\xdd\xee\xff' >> "$target_info_file"
    
    # Pad to 512 bytes (SGX target_info size)
    dd if=/dev/zero bs=1 count=480 2>/dev/null >> "$target_info_file"
    
    sudo cp "$target_info_file" /dev/attestation/my_target_info
    sudo chmod 644 /dev/attestation/my_target_info
    rm -f "$target_info_file"
    
    # Create user_report_data file (for writing report data - 64 bytes)
    sudo touch /dev/attestation/user_report_data
    sudo chmod 666 /dev/attestation/user_report_data
    
    # Create quote file (will be read after writing user_report_data)
    # We'll use a script to generate quote when user_report_data is written
    # For now, create a mock quote
    local quote_file=$(mktemp)
    
    # Generate a minimal valid DCAP Quote v3 structure
    # Quote format: Header (48) + Report (384) + Signature Data (variable)
    # Total minimum: 432 bytes
    
    # Header (48 bytes)
    printf '\x03\x00' > "$quote_file"  # Version 3
    printf '\x02\x00' >> "$quote_file" # Attestation key type: ECDSA P-256
    printf '\x00\x00\x00\x00' >> "$quote_file" # Reserved
    printf '\x01\x00' >> "$quote_file" # QE SVN
    printf '\x01\x00' >> "$quote_file" # PCE SVN
    # QE Vendor ID (16 bytes) - Intel
    printf '\x93\x9a\x72\x33\xf7\x9c\x4c\xa9\x94\x0a\x0d\xb3\x95\x7f\x06\x07' >> "$quote_file"
    # User data (20 bytes) - zeros
    dd if=/dev/zero bs=1 count=20 2>/dev/null >> "$quote_file"
    
    # Report body (384 bytes)
    # CPUSVN (16 bytes)
    dd if=/dev/zero bs=1 count=16 2>/dev/null >> "$quote_file"
    # MISCSELECT (4 bytes)
    printf '\x00\x00\x00\x00' >> "$quote_file"
    # Reserved (28 bytes)
    dd if=/dev/zero bs=1 count=28 2>/dev/null >> "$quote_file"
    # ATTRIBUTES (16 bytes)
    dd if=/dev/zero bs=1 count=16 2>/dev/null >> "$quote_file"
    # MRENCLAVE (32 bytes) - same as in my_target_info
    printf '\x00\x11\x22\x33\x44\x55\x66\x77\x88\x99\xaa\xbb\xcc\xdd\xee\xff' >> "$quote_file"
    printf '\x00\x11\x22\x33\x44\x55\x66\x77\x88\x99\xaa\xbb\xcc\xdd\xee\xff' >> "$quote_file"
    # MRSIGNER (32 bytes)
    dd if=/dev/zero bs=1 count=32 2>/dev/null >> "$quote_file"
    # Reserved (96 bytes)
    dd if=/dev/zero bs=1 count=96 2>/dev/null >> "$quote_file"
    # ISVPRODID (2 bytes)
    printf '\x00\x00' >> "$quote_file"
    # ISVSVN (2 bytes)
    printf '\x01\x00' >> "$quote_file"
    # Reserved (60 bytes)
    dd if=/dev/zero bs=1 count=60 2>/dev/null >> "$quote_file"
    # REPORTDATA (64 bytes) - will contain block hash
    dd if=/dev/zero bs=1 count=64 2>/dev/null >> "$quote_file"
    
    sudo cp "$quote_file" /dev/attestation/quote
    sudo chmod 644 /dev/attestation/quote
    rm -f "$quote_file"
    
    echo "✓ /dev/attestation created"
    echo "  - my_target_info: 512 bytes (MRENCLAVE + padding)"
    echo "  - user_report_data: writable (64 bytes)"
    echo "  - quote: 432+ bytes (mock DCAP Quote v3)"
}

# Clean up test filesystem
cleanup_test_filesystem() {
    echo "Cleaning up test filesystem..."
    # Remove /dev/attestation
    sudo rm -rf /dev/attestation 2>/dev/null || true
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
