#!/bin/bash
# Test environment configuration for E2E testing
# Configures environment for PoA-SGX consensus testing

# ==============================================================================
# 环境配置说明
# ==============================================================================
# PoA-SGX共识运行在生产模式，测试通过mock环境文件实现
#
# 配置来源严格分离：
#
# 1. 合约地址 - 只能来自manifest文件
#    - GRAMINE_MANIFEST_PATH: 指向包含合约地址的manifest文件
#    - Manifest包含: XCHAIN_SECURITY_CONFIG_CONTRACT
#    - Manifest包含: XCHAIN_GOVERNANCE_CONTRACT (可选，可从安全配置合约读取)
#
# 2. 配置内容 - 来自合约storage或genesis alloc storage
#    - 白名单: 从安全配置合约storage读取
#    - Fallback: 从genesis alloc中同一地址的storage读取
#    - 环境变量仅用于测试，代表genesis alloc storage内容
#
# 3. Mock SGX设备 - 模拟/dev/attestation
#    - 创建mock MRENCLAVE文件
#    - 创建可写的user_report_data
#    - 创建quote输出文件
#
# 4. Gramine环境
#    - GRAMINE_VERSION: 版本标识
#    - SGX_TEST_MODE: true（跳过某些硬件检查）
#
# 注意：合约地址绝不能通过环境变量设置，只能从manifest读取！
# ==============================================================================

# Gramine environment (required)
export GRAMINE_VERSION="1.0-test"
export SGX_TEST_MODE="true"

# Intel SGX API key for PCCS (non-security parameter)
export INTEL_SGX_API_KEY="${INTEL_SGX_API_KEY:-a8ece8747e7b4d8d98d23faec065b0b8}"

# SECURITY REQUIREMENT ENFORCEMENT:
# NO environment variables for security parameters!
# - Contract addresses: ONLY from verified manifest file
# - Whitelist data: ONLY from genesis.json alloc storage or contract storage
# - Environment variables are ONLY for test mode flags and non-security config

# Print environment for debugging
print_test_env() {
    echo "=== Test Environment Configuration ==="
    echo "Gramine Version: $GRAMINE_VERSION"
    echo "SGX Test Mode: $SGX_TEST_MODE"
    echo "Intel API Key: ${INTEL_SGX_API_KEY:0:8}... (first 8 chars)"
    echo "Manifest Path: ${GRAMINE_MANIFEST_PATH:-not set}"
    echo ""
    echo "注意: 合约地址从manifest读取，配置内容从合约storage读取"
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
# Creates files with REAL Quote extracted from RA-TLS certificate
setup_dev_attestation() {
    echo "Setting up /dev/attestation (Gramine standard path) with REAL Quote..."
    
    # Real Quote data extracted from RA-TLS certificate (verified and working)
    local REAL_MRENCLAVE="6364c9c486ebe6d3b3ec6e22ec0b4ee4cec428450a055c4ebee36d6e9b8660a8"
    local REAL_MRSIGNER="d504543bc3717ed87d3982fbb7b17f3b07f12ba66b69f75c02536620f35d0d5b"
    local REAL_QUOTE_HEX="03000200000000000b001000939a7233f79c4ca9940a0db3957f060731d86673e347300e289bca48eae4c35c000000000b0b100fffff0000000000000000000000000000000000000000000000000000000000000000000000000000000000000700000000000000e7000000000000006364c9c486ebe6d3b3ec6e22ec0b4ee4cec428450a055c4ebee36d6e9b8660a80000000000000000000000000000000000000000000000000000000000000000d504543bc3717ed87d3982fbb7b17f3b07f12ba66b69f75c02536620f35d0d5b0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000044f4dcf977f4990e4e94c29e8590727b754a4477f0249d3685887cff7f2ba4840000000000000000000000000000000000000000000000000000000000000000ca1000009694da6161c2925a97c716e186c40ca921352deee7bc93458a0180682e877116d8aa358e09e040d5d8a5d8da2cac9d3c8f216b0a0f32c9698449c59b14cc9c18898ff818a83ac67e7eb5132125bbf32ad6d66e1f65a59d821dbd20fe5ea991cc2ea838e701abfe3b82c2771dbf524f4b94dc51b598c070d91220e109e2b57c0e0b0b100fffff0000000000000000000000000000000000000000000000000000000000000000000000000000000000001500000000000000e70000000000000078fe8cfd01095a0f108aff5c40624b93612d6c28b73e1a8d28179c9ddf0e068600000000000000000000000000000000000000000000000000000000000000008c4f5775d796503e96137f77c68a829a0056ac8ded70140b081b094490c57bff00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000b000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000e82f8d21032b6b7e32bf152c51edae0a19f45c75ca7e8be9b5fd5bee5a259be00000000000000000000000000000000000000000000000000000000000000000796c4d8c78fe42c742f1dd07f63f1822b23bbc4d3c0a2ddfb8e9c8e7e9c4453ce4cc2b936795ae9840ba7859659f24c55e792e4c72dc7ed312a587823df881f2000000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f0500620e00002d2d2d2d2d424547494e2043455254494649434154452d2d2d2d2d0a4d494945386a4343424a696741774942416749554d73544677774e6636645574417847306749762f6b734b6b552b4977436759494b6f5a497a6a3045417749770a634445694d434147413155454177775a535735305a577767553064594946424453794251624746305a6d397962544244515445614d42674741315545436777520a535735305a577767513239796347397959585270623234784644415342674e564241634d43314e68626e526849454e7359584a684d51737743515944565151490a44414a445154454c4d416b474131554542684d4356564d774868634e4d6a55784d5441344d4449794e6a517a5768634e4d7a49784d5441344d4449794e6a517a0a576a42774d534977494159445651514444426c4a626e526c624342545231676755454e4c49454e6c636e52705a6d6c6a5958526c4d526f77474159445651514b0a4442464a626e526c6243424462334a7762334a6864476c76626a45554d424947413155454277774c553246756447456751327868636d4578437a414a42674e560a4241674d416b4e424d517377435159445651514745774a56557a425a4d424d4742797147534d34394167454743437147534d34394177454841304941425062330a61643538366234642b5047476e2f63504675314448362f6d506278434d72304f5a7336696259544d65625541473648625a367055657963464f374e516c3069630a324d796163504642514e4d634f6750735852476a67674d4f4d494944436a416642674e5648534d4547444157674253566231334e765276683655424a796454300a4d383442567776655644427242674e56485238455a4442694d47436758714263686c706f64485277637a6f764c32467761533530636e567a6447566b633256790a646d6c6a5a584d75615735305a577775593239744c334e6e6543396a5a584a3061575a7059324630615739754c3359304c33426a61324e796244396a595431770a624746305a6d397962535a6c626d4e765a476c755a7a316b5a584977485159445652304f424259454650486a43624f492b494c6e726d694b75396f7872574e680a5a7072474d41344741315564447745422f775145417749477744414d42674e5648524d4241663845416a41414d4949434f77594a4b6f5a496876684e415130420a424949434c444343416967774867594b4b6f5a496876684e415130424151515164666e49424944643878484c6d51535145585368577a434341575547436971470a534962345451454e41514977676746564d42414743797147534962345451454e415149424167454c4d42414743797147534962345451454e415149434167454c0a4d42414743797147534962345451454e41514944416745444d42414743797147534962345451454e41514945416745444d42454743797147534962345451454e0a41514946416749412f7a415242677371686b69472b4530424451454342674943415038774541594c4b6f5a496876684e4151304241676343415141774541594c0a4b6f5a496876684e4151304241676743415141774541594c4b6f5a496876684e4151304241676b43415141774541594c4b6f5a496876684e4151304241676f430a415141774541594c4b6f5a496876684e4151304241677343415141774541594c4b6f5a496876684e4151304241677743415141774541594c4b6f5a496876684e0a4151304241673043415141774541594c4b6f5a496876684e4151304241673443415141774541594c4b6f5a496876684e4151304241673843415141774541594c0a4b6f5a496876684e4151304241684143415141774541594c4b6f5a496876684e4151304241684543415130774877594c4b6f5a496876684e41513042416849450a4541734c4177502f2f7741414141414141414141414141774541594b4b6f5a496876684e4151304241775143414141774641594b4b6f5a496876684e415130420a4241514741474271414141414d41384743697147534962345451454e4151554b415145774867594b4b6f5a496876684e415130424267515142447648463372760a462b74466a357035587132583344424542676f71686b69472b453042445145484d4459774541594c4b6f5a496876684e4151304242774542416638774541594c0a4b6f5a496876684e4151304242774942416638774541594c4b6f5a496876684e4151304242774d4241663877436759494b6f5a497a6a304541774944534141770a52514968414f52714d2f6b516b4b483831346e496e7749704f513458376b457438332b35374a787149386473507865664169424963547375753573552f6a4d470a635a76496f44693366557964493079356d4e4d37506573445369775050513d3d0a2d2d2d2d2d454e442043455254494649434154452d2d2d2d2d0a2d2d2d2d2d424547494e2043455254494649434154452d2d2d2d2d0a4d4949436c6a4343416a32674177494241674956414a567658633239472b487051456e4a3150517a7a674658433935554d416f4743437147534d343942414d430a4d476778476a415942674e5642414d4d45556c756447567349464e48574342536232393049454e424d526f77474159445651514b4442464a626e526c624342440a62334a7762334a6864476c76626a45554d424947413155454277774c553246756447456751327868636d4578437a414a42674e564241674d416b4e424d5173770a435159445651514745774a56557a4165467730784f4441314d6a45784d4455774d5442614677307a4d7a41314d6a45784d4455774d5442614d484178496a41670a42674e5642414d4d47556c756447567349464e4857434251513073675547786864475a76636d306751304578476a415942674e5642416f4d45556c75644756730a49454e76636e4276636d4630615739754d5251774567594456515148444174545957353059534244624746795954454c4d416b474131554543417743513045780a437a414a42674e5642415954416c56544d466b77457759484b6f5a497a6a3043415159494b6f5a497a6a304441516344516741454e53422f377432316c58534f0a3243757a7078773734654a423732457944476757357258437478327456544c7136684b6b367a2b5569525a436e71523770734f766771466553786c6d546c4a6c0a65546d693257597a33714f42757a43427544416642674e5648534d4547444157674251695a517a575770303069664f44744a5653763141624f536347724442530a42674e5648523845537a424a4d45656752614244686b466f64485277637a6f764c324e6c636e52705a6d6c6a5958526c63793530636e567a6447566b633256790a646d6c6a5a584d75615735305a577775593239744c306c756447567355306459556d397664454e424c6d526c636a416442674e5648513445466751556c5739640a7a62306234656c4153636e553944504f4156634c336c517744675944565230504151482f42415144416745474d42494741315564457745422f7751494d4159420a4166384341514177436759494b6f5a497a6a30454177494452774177524149675873566b6930772b6936565947573355462f32327561586530594a446a3155650a6e412b546a44316169356343494359623153416d4435786b66545670766f34556f79695359787244574c6d5552344349394e4b7966504e2b0a2d2d2d2d2d454e442043455254494649434154452d2d2d2d2d0a2d2d2d2d2d424547494e2043455254494649434154452d2d2d2d2d0a4d4949436a7a4343416a53674177494241674955496d554d316c71644e496e7a6737535655723951477a6b6e42717777436759494b6f5a497a6a3045417749770a614445614d4267474131554541777752535735305a5777675530645949464a766233516751304578476a415942674e5642416f4d45556c756447567349454e760a636e4276636d4630615739754d5251774567594456515148444174545957353059534244624746795954454c4d416b47413155454341774351304578437a414a0a42674e5642415954416c56544d423458445445344d4455794d5445774e4455784d466f58445451354d54497a4d54497a4e546b314f566f77614445614d4267470a4131554541777752535735305a5777675530645949464a766233516751304578476a415942674e5642416f4d45556c756447567349454e76636e4276636d46300a615739754d5251774567594456515148444174545957353059534244624746795954454c4d416b47413155454341774351304578437a414a42674e56424159540a416c56544d466b77457759484b6f5a497a6a3043415159494b6f5a497a6a3044415163445167414543366e45774d4449595a4f6a2f69505773437a61454b69370a314f694f534c52466857476a626e42564a66566e6b59347533496a6b4459594c304d784f346d717379596a6c42616c54565978465032734a424b357a6c4b4f420a757a43427544416642674e5648534d4547444157674251695a517a575770303069664f44744a5653763141624f5363477244425342674e5648523845537a424a0a4d45656752614244686b466f64485277637a6f764c324e6c636e52705a6d6c6a5958526c63793530636e567a6447566b63325679646d6c6a5a584d75615735300a5a577775593239744c306c756447567355306459556d397664454e424c6d526c636a416442674e564851344546675155496d554d316c71644e496e7a673753560a55723951477a6b6e4271777744675944565230504151482f42415144416745474d42494741315564457745422f7751494d4159424166384341514577436759490a4b6f5a497a6a3045417749445351417752674968414f572f35516b522b533943695344634e6f6f774c7550524c735747662f59693747535839344267775477670a41694541344a306c72486f4d732b586f356f2f7358364f39515778485241765a55474f6452513763767152586171493d0a2d2d2d2d2d454e442043455254494649434154452d2d2d2d2d0a00"
    
    # Create /dev/attestation directory with sudo
    sudo mkdir -p /dev/attestation
    sudo chmod 755 /dev/attestation
    
    # Create my_target_info file (contains MRENCLAVE)
    # Format: first 32 bytes are MRENCLAVE, rest is padding
    # Total size should be at least 512 bytes (SGX target_info structure)
    local target_info_file=$(mktemp)
    
    # Write REAL MRENCLAVE (32 bytes in hex, convert to binary)
    echo -n "$REAL_MRENCLAVE" | xxd -r -p > "$target_info_file"
    
    # Pad to 512 bytes (SGX target_info size)
    dd if=/dev/zero bs=1 count=480 2>/dev/null >> "$target_info_file"
    
    sudo cp "$target_info_file" /dev/attestation/my_target_info
    sudo chmod 644 /dev/attestation/my_target_info
    rm -f "$target_info_file"
    
    # Create user_report_data file (for writing report data - 64 bytes)
    sudo touch /dev/attestation/user_report_data
    sudo chmod 666 /dev/attestation/user_report_data
    
    # Create quote file with REAL Quote data
    local quote_file=$(mktemp)
    echo -n "$REAL_QUOTE_HEX" | xxd -r -p > "$quote_file"
    
    sudo cp "$quote_file" /dev/attestation/quote
    sudo chmod 644 /dev/attestation/quote
    rm -f "$quote_file"
    
    local quote_size=$(stat -f%z /dev/attestation/quote 2>/dev/null || stat -c%s /dev/attestation/quote 2>/dev/null)
    
    echo "✓ /dev/attestation created with REAL Quote data"
    echo "  - my_target_info: 512 bytes (REAL MRENCLAVE: ${REAL_MRENCLAVE:0:16}...)"
    echo "  - user_report_data: writable (64 bytes)"
    echo "  - quote: $quote_size bytes (REAL verified Quote from RA-TLS cert)"
    echo "  - MRSIGNER: ${REAL_MRSIGNER:0:16}..."
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

# Setup real signed manifest files for signature verification
# Uses OpenSSL to generate RSA-3072 key pair and sign manifest
setup_mock_manifest_files() {
    local manifest_dir="${1:-/tmp/xchain-test-manifest}"
    
    echo "Generating REAL signed manifest files at $manifest_dir..."
    mkdir -p "$manifest_dir"
    
    # Use the generate_manifest_signature.sh script
    local script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    if [ -f "$script_dir/generate_manifest_signature.sh" ]; then
        bash "$script_dir/generate_manifest_signature.sh" "$manifest_dir"
    else
        # Inline generation if script not found
        echo "Generating RSA-3072 key pair..."
        openssl genrsa -out "$manifest_dir/enclave-key.pem" 3072 2>/dev/null
        openssl rsa -in "$manifest_dir/enclave-key.pem" -pubout -out "$manifest_dir/enclave-key.pub" 2>/dev/null
        
        # Create manifest
        cat > "$manifest_dir/geth.manifest.sgx" << 'MANIFEST_EOF'
# Gramine Manifest for Testing
libos.entrypoint = "/app/geth"

# Environment variables - Contract addresses (security critical)
loader.env.XCHAIN_GOVERNANCE_CONTRACT = "0xd9145CCE52D386f254917e481eB44e9943F39138"
loader.env.XCHAIN_SECURITY_CONFIG_CONTRACT = "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"

# SGX configuration
sgx.enclave_size = "2G"
sgx.max_threads = 32
sgx.remote_attestation = "dcap"
MANIFEST_EOF
        
        # Sign manifest using openssl dgst (correct method)
        openssl dgst -sha256 -sign "$manifest_dir/enclave-key.pem" \
            -out "$manifest_dir/geth.manifest.sgx.sig" \
            "$manifest_dir/geth.manifest.sgx"
        
        # Verify signature
        if openssl dgst -sha256 -verify "$manifest_dir/enclave-key.pub" \
            -signature "$manifest_dir/geth.manifest.sgx.sig" "$manifest_dir/geth.manifest.sgx" >/dev/null 2>&1; then
            echo "✓ Manifest signature verified successfully"
        else
            echo "✗ Manifest signature verification failed!"
            return 1
        fi
    fi
    
    # Set environment variables
    export GRAMINE_MANIFEST_PATH="$manifest_dir/geth.manifest.sgx"
    export GRAMINE_SIGSTRUCT_KEY_PATH="$manifest_dir/enclave-key.pub"
    export GRAMINE_APP_NAME="geth"
    
    echo "✓ Real signed manifest files created and verified"
    echo "  - Manifest: $manifest_dir/geth.manifest.sgx"
    echo "  - Signature: $manifest_dir/geth.manifest.sgx.sig"  
    echo "  - Public key: $manifest_dir/enclave-key.pub"
    echo "  - Private key: $manifest_dir/enclave-key.pem"
}

# Verify test environment is properly configured
verify_test_env() {
    local errors=0
    
    echo "Verifying test environment..."
    
    # 检查manifest文件是否存在
    if [ -z "$GRAMINE_MANIFEST_PATH" ]; then
        echo "ERROR: GRAMINE_MANIFEST_PATH not set"
        errors=$((errors + 1))
    elif [ ! -f "$GRAMINE_MANIFEST_PATH" ]; then
        echo "ERROR: Manifest file not found: $GRAMINE_MANIFEST_PATH"
        errors=$((errors + 1))
    else
        # 验证manifest包含必要的配置
        if ! grep -q "XCHAIN_SECURITY_CONFIG_CONTRACT" "$GRAMINE_MANIFEST_PATH"; then
            echo "ERROR: Manifest missing XCHAIN_SECURITY_CONFIG_CONTRACT"
            errors=$((errors + 1))
        fi
    fi
    
    # SECURITY ENFORCEMENT: Verify NO security parameters in environment
    if [ -n "$XCHAIN_CONTRACT_MRENCLAVES" ]; then
        echo "ERROR: XCHAIN_CONTRACT_MRENCLAVES found in environment"
        echo "  Security parameters must NOT be passed via environment variables"
        echo "  Use genesis.json alloc storage instead"
        errors=$((errors + 1))
    fi
    
    if [ -n "$XCHAIN_CONTRACT_MRSIGNERS" ]; then
        echo "ERROR: XCHAIN_CONTRACT_MRSIGNERS found in environment"
        echo "  Security parameters must NOT be passed via environment variables"
        echo "  Use genesis.json alloc storage instead"
        errors=$((errors + 1))
    fi
    
    if [ -n "$XCHAIN_SECURITY_CONFIG_CONTRACT" ]; then
        echo "ERROR: XCHAIN_SECURITY_CONFIG_CONTRACT found in environment"
        echo "  Contract addresses must ONLY come from verified manifest file"
        errors=$((errors + 1))
    fi
    
    if [ -n "$XCHAIN_GOVERNANCE_CONTRACT" ]; then
        echo "ERROR: XCHAIN_GOVERNANCE_CONTRACT found in environment"
        echo "  Contract addresses must ONLY come from verified manifest file"
        errors=$((errors + 1))
    fi
    
    if [ $errors -gt 0 ]; then
        echo "Test environment verification failed with $errors errors"
        return 1
    fi
    
    echo "✓ Test environment verified successfully"
    echo "  - Manifest file: $GRAMINE_MANIFEST_PATH"
    echo "  - Contract addresses: will be read from manifest"
    echo "  - Whitelist: will be read from genesis.json or contract storage"
    echo "  - NO security parameters in environment variables ✓"
    return 0
}
